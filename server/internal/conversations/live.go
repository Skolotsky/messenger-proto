package conversations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"contexia/internal/shared/call"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
)

// The live connection's timings (conversations-to-screen.md, screen-to-conversations.md).
const (
	lastSentEvery = 5 * time.Second
	checkEvery    = 5 * time.Second
	pingEvery     = 20 * time.Second
	pongWithin    = 20 * time.Second // gone after 40 s without an answer
	replayLimit   = 500
	maxReplayScan = 20000 // log entries one replay may walk before "load fresh"
	logPage       = 200
	connQueue     = 1024
	writeWithin   = 5 * time.Second
)

// LiveOptions says where the parts the door reads from are.
type LiveOptions struct {
	Feed    string
	Search  string
	SelfURL string // the organizations' address, for nudges to whichever copy serves them
	Copy    string // this copy's identity; empty picks a fresh one
}

// Live is the live connection, the door's reads and housekeeping of one copy.
type Live struct {
	s      *Store
	feed   *call.Client
	search *call.Client
	nudger *call.Client

	mu       sync.Mutex
	hubs     map[ids.Org]*orgHub
	identity string
	ends     []connectionEnd // ends that could not be saved yet
	pongs    atomic.Int64
}

// PingsAnswered counts the connection checks screens have answered.
func (l *Live) PingsAnswered() int64 { return l.pongs.Load() }

// NewLive builds the live connection and hooks it to the store's saves.
func NewLive(s *Store, opt LiveOptions) *Live {
	l := &Live{s: s, hubs: map[ids.Org]*orgHub{}, identity: opt.Copy}
	if l.identity == "" {
		l.identity = "copy_" + randomID()
	}
	if opt.Feed != "" {
		l.feed = call.New(opt.Feed, 0)
	}
	if opt.Search != "" {
		l.search = call.New(opt.Search, 0)
	}
	if opt.SelfURL != "" {
		l.nudger = call.New(opt.SelfURL, time.Second)
	}
	s.OnSaved = l.Saved
	s.Outside = l.outside
	return l
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Copy is this copy's current identity.
func (l *Live) Copy() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.identity
}

// Following says whether a screen of this person has been placed in the
// conversations stream, so a test can tell "not yet connected" from "lost".
func (l *Live) Following(org ids.Org, person string) bool {
	h := l.hub(org)
	if h == nil {
		return false
	}
	for _, c := range h.connections() {
		c.mu.Lock()
		placed := c.person == person && c.last >= 0 && c.resumeConv == nil
		c.mu.Unlock()
		if placed {
			return true
		}
	}
	return false
}

// Routes adds the live connection, the door's reads and the nudge.
func (l *Live) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /orgs/{org}/live", l.handleLive)
	mux.HandleFunc("POST /orgs/{org}/nudge", l.handleNudge)
	mux.HandleFunc("GET /orgs/{org}/people/{person}/feed", l.s.door(l.handleFeed))
	mux.HandleFunc("GET /orgs/{org}/people/{person}/history", l.s.door(l.handleHistory))
	mux.HandleFunc("GET /orgs/{org}/people/{person}/settings", l.s.door(l.handleSettings))
	mux.HandleFunc("GET /orgs/{org}/people/{person}/preview", l.s.door(l.handlePreview))
	mux.HandleFunc("GET /orgs/{org}/people/{person}/search", l.s.door(l.handleSearch))
}

// Saved is told a save committed. It only wakes the organization's log reader;
// what reaches a screen is read back from the log, in number order, so two saves
// that reach this point swapped cannot swap on a screen.
func (l *Live) Saved(org ids.Org, first, last int64) {
	h := l.hub(org)
	if h == nil {
		return
	}
	if hubSendsWhatItIsHanded && first > 0 {
		h.sendHanded(first, last) // test-only fault
		return
	}
	h.wakeLog()
}

func (l *Live) hub(org ids.Org) *orgHub {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.hubs[org]
}

func (l *Live) allHubs() []*orgHub {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]*orgHub, 0, len(l.hubs))
	for _, h := range l.hubs {
		out = append(out, h)
	}
	return out
}

func (l *Live) handleNudge(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	if h := l.hub(org); h != nil {
		h.wakeLog()
		h.wakeFeed()
	}
	w.WriteHeader(http.StatusNoContent)
}

// outside sends a message that has no number to the screens it concerns.
func (l *Live) outside(org ids.Org, conversation string, message any) {
	h := l.hub(org)
	if h == nil {
		return
	}
	b, err := json.Marshal(message)
	if err != nil {
		slog.Error("message outside the numbering", "err", err)
		return
	}
	for _, c := range h.connections() {
		if conversation == "" || c.isIn(conversation) {
			c.enqueue(outMessage{body: b})
		}
	}
}

// orgState says whether the organization exists and whether it is retired.
func (s *Store) orgState(ctx context.Context, org ids.Org) (exists, retired bool, head int64, err error) {
	err = orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		e := tx.QueryRow(ctx, `SELECT head, retired_at IS NOT NULL FROM org_log WHERE org = $1`, string(org)).Scan(&head, &retired)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		exists = true
		return nil
	})
	return exists, retired, head, err
}

func (s *Store) personKnown(ctx context.Context, org ids.Org, person string) (bool, error) {
	var ok bool
	err := orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM people WHERE org = $1 AND id = $2)`, string(org), person).Scan(&ok)
	})
	return ok, err
}

// handleLive serves one screen's connection. A browser cannot read the status
// of a refused upgrade, so a retired or deleted organization is said over the
// connection itself and then closed.
func (l *Live) handleLive(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	person := r.URL.Query().Get("person")
	exists, retired, _, err := l.s.orgState(r.Context(), org)
	if err != nil {
		fail(w, r, err)
		return
	}
	if exists && !retired {
		known, err := l.s.personKnown(r.Context(), org, person)
		if err != nil {
			fail(w, r, err)
			return
		}
		if !known {
			send(w, http.StatusNotFound, contract.Refusal{Refused: refusedUnknownPerson})
			return
		}
	}
	// Sign-in and the access model are outside this prototype (D-009).
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	ws.SetReadLimit(64 << 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !exists || retired {
		writeCtx, done := context.WithTimeout(ctx, writeWithin)
		defer done()
		_ = ws.Write(writeCtx, websocket.MessageText, goneMessage)
		_ = ws.Close(websocket.StatusNormalClosure, "organization gone")
		return
	}
	c := &conn{l: l, org: org, person: person, id: "conn_" + randomID(), ws: ws, cancel: cancel,
		out: make(chan outMessage, connQueue), last: -1, watching: map[watchKey]struct{}{}, mine: map[string]bool{}}
	h := l.join(c)
	go c.writeLoop(ctx)
	go c.keepAlive(ctx)
	go c.lastSentLoop(ctx)
	c.readLoop(ctx, h)
	cancel()
	l.leave(h, c)
	c.endAllWatching()
	_ = ws.CloseNow()
}

func (l *Live) join(c *conn) *orgHub {
	l.mu.Lock()
	defer l.mu.Unlock()
	h := l.hubs[c.org]
	if h == nil {
		h = newOrgHub(l, c.org)
		l.hubs[c.org] = h
	}
	h.add(c)
	return h
}

func (l *Live) leave(h *orgHub, c *conn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if h.remove(c) == 0 && l.hubs[c.org] == h {
		delete(l.hubs, c.org)
		h.stop()
	}
}

var goneMessage, _ = json.Marshal(contract.OrganizationGone{OrganizationGone: true})

type outMessage struct {
	body []byte
	last bool // close the connection after this one
}

type watchKey struct {
	conversation string
	thread       string
	hasThread    bool
}

func keyOf(conversation string, thread *string) watchKey {
	if thread == nil {
		return watchKey{conversation: conversation}
	}
	return watchKey{conversation: conversation, thread: *thread, hasThread: true}
}

// conn is one screen's connection.
type conn struct {
	l      *Live
	org    ids.Org
	person string
	id     string
	ws     *websocket.Conn
	cancel context.CancelFunc
	out    chan outMessage

	mu          sync.Mutex
	last        int64 // the conversations stream's last number sent; -1 until a resume
	feedAt      *int64
	edition     *int64
	mine        map[string]bool
	resumeConv  *int64
	resumeFeed  *contract.ResumeNumbers
	watching    map[watchKey]struct{}
	goneWritten bool
}

func (c *conn) isIn(conversation string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mine[conversation]
}

// enqueue never blocks the organization's reader: a screen that cannot keep up
// is dropped, reconnects and resumes by number.
func (c *conn) enqueue(m outMessage) {
	select {
	case c.out <- m:
	default:
		c.cancel()
	}
}

func (c *conn) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-c.out:
			writeCtx, done := context.WithTimeout(ctx, writeWithin)
			err := c.ws.Write(writeCtx, websocket.MessageText, m.body)
			done()
			if err != nil {
				c.cancel()
				return
			}
			if m.last {
				_ = c.ws.Close(websocket.StatusNormalClosure, "organization gone")
				c.cancel()
				return
			}
		}
	}
}

// keepAlive checks the connection every 20 s; without an answer within the
// next 20 it counts as gone, so a vanished screen is gone within 40 s (G-024).
func (c *conn) keepAlive(ctx context.Context) {
	clk := c.l.s.Clock
	for {
		select {
		case <-ctx.Done():
			return
		case <-clk.After(pingEvery):
		}
		pingCtx, stop := context.WithCancel(ctx)
		answered := make(chan error, 1)
		go func() { answered <- c.ws.Ping(pingCtx) }()
		select {
		case <-ctx.Done():
			stop()
			return
		case err := <-answered:
			stop()
			if err != nil {
				c.cancel()
				_ = c.ws.CloseNow()
				return
			}
			c.l.pongs.Add(1)
		case <-clk.After(pongWithin):
			stop()
			c.cancel()
			_ = c.ws.CloseNow() // ends the read loop, which ends the watchings
			return
		}
	}
}

func (c *conn) lastSentLoop(ctx context.Context) {
	clk := c.l.s.Clock
	for {
		select {
		case <-ctx.Done():
			return
		case <-clk.After(lastSentEvery):
			c.sendLastSent()
		}
	}
}

// sendLastSent is queued behind every update sent so far, so its numbers never
// run ahead of what the screen was sent.
func (c *conn) sendLastSent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last < 0 {
		return
	}
	b, err := json.Marshal(contract.LiveLastSent{LastSent: contract.LastSentNumbers{
		Conversations: c.last, Feed: c.feedAt, FeedEdition: c.edition}})
	if err == nil {
		c.enqueue(outMessage{body: b})
	}
}

func (c *conn) readLoop(ctx context.Context, h *orgHub) {
	for {
		_, raw, err := c.ws.Read(ctx)
		if err != nil {
			return
		}
		var in contract.LiveIn
		if err := json.Unmarshal(raw, &in); err != nil {
			continue // a message this version does not know
		}
		switch m := in.Value.(type) {
		case contract.LiveResume:
			h.resume(c, m.Resume)
		case contract.LiveWatching:
			if err := c.l.watch(ctx, c, m.Watching); err != nil && ctx.Err() == nil {
				slog.Error("watching", "org", c.org, "err", err)
			}
		}
	}
}
