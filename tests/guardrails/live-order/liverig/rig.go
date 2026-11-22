// Package liverig is the bench for the live connection's guardrails: the real
// Conversations on PostgreSQL with a clock the test moves, and a model screen
// that keeps the contract's rule for applying updates.
package liverig

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/migrations"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Rig is one organization on a real database with one or more copies.
type Rig struct {
	T     *testing.T
	Ctx   context.Context
	Pool  *pgxpool.Pool
	Clock *clock.Manual
	Org   ids.Org
	n     int
	mu    sync.Mutex
}

// Copy is one running copy of Conversations.
type Copy struct {
	Store  *conversations.Store
	Live   *conversations.Live
	Server *httptest.Server
}

// New connects to the test database and seeds a small team.
func New(t *testing.T) *Rig {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://conversations:conversations_dev@%s:5432/contexia", host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(ctx, pool, migrations.Files, "conversations"); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	r := &Rig{T: t, Ctx: ctx, Pool: pool, Clock: clock.NewManual(time.Date(2026, 11, 22, 8, 0, 0, 0, time.UTC)), // before the composed system's test clock, so a sweep here never declares its copy dead
		Org: ids.Org("live-" + hex.EncodeToString(b))}
	r.seed()
	return r
}

func (r *Rig) seed() {
	s := &conversations.Store{Dir: orgdb.Single{Pool: r.Pool}, Clock: r.Clock}
	session, err := s.SeedOpen(r.Ctx, r.Org)
	if err != nil {
		r.T.Fatal(err)
	}
	at := "2026-11-01T09:00:00.000Z"
	n := int64(0)
	var list []contract.Change
	add := func(build func(num, prev int64) any) {
		n++
		list = append(list, contract.Change{Value: build(n, n-1)})
	}
	for _, p := range []string{"anna", "marco", "lena"} {
		add(func(num, prev int64) any {
			return contract.ChangePersonJoined{Kind: "person.joined", Org: string(r.Org), Number: num, Previous: prev, At: at, Person: p, Name: p}
		})
	}
	for _, ch := range []struct {
		name   string
		people []string
	}{{"launch", []string{"anna", "marco"}}, {"other", []string{"anna", "lena"}}} {
		add(func(num, prev int64) any {
			return contract.ChangeChannelCreated{Kind: "channel.created", Org: string(r.Org), Number: num, Previous: prev, At: at,
				Conversation: "#" + ch.name, Name: ch.name, People: ch.people}
		})
	}
	if _, err := s.SeedBatch(r.Ctx, r.Org, session, contract.SeedBatch{Changes: list}); err != nil {
		r.T.Fatal(err)
	}
	if _, err := s.SeedSeal(r.Ctx, r.Org, session); err != nil {
		r.T.Fatal(err)
	}
}

// NewCopy starts a copy with a registered lease. Its timers are not running:
// the test drives CheckPass, TickPass and SweepPass itself.
func (r *Rig) NewCopy(opt conversations.LiveOptions) *Copy {
	r.T.Helper()
	s := &conversations.Store{Dir: orgdb.Single{Pool: r.Pool}, Clock: r.Clock}
	l := conversations.NewLive(s, opt)
	if err := l.RegisterLease(r.Ctx); err != nil {
		r.T.Fatal(err)
	}
	mux := http.NewServeMux()
	s.Routes(mux)
	l.Routes(mux)
	srv := httptest.NewServer(mux)
	r.T.Cleanup(srv.CloseClientConnections)
	return &Copy{Store: s, Live: l, Server: srv}
}

// Act saves one action through a copy and returns its number.
func (r *Rig) Act(c *Copy, kind, as string, fields map[string]any) int64 {
	r.T.Helper()
	r.mu.Lock()
	r.n++
	id := fmt.Sprintf("act_%d", r.n)
	r.mu.Unlock()
	body := map[string]any{"kind": kind, "id": id, "as": as}
	for k, v := range fields {
		body[k] = v
	}
	raw, _ := json.Marshal(body)
	status, answer, err := c.Store.Act(r.Ctx, r.Org, raw)
	if err != nil {
		r.T.Fatal(err)
	}
	if status != http.StatusOK {
		r.T.Fatalf("%s refused: %d %v", kind, status, answer)
	}
	b, _ := json.Marshal(answer)
	var out struct {
		Number *int64 `json:"number"`
	}
	_ = json.Unmarshal(b, &out)
	if out.Number == nil {
		return 0
	}
	return *out.Number
}

// Say sends a top-level message and returns its number.
func (r *Rig) Say(c *Copy, as, conversation, text string) int64 {
	r.T.Helper()
	r.mu.Lock()
	r.n++
	id := fmt.Sprintf("msg_%d", r.n)
	r.mu.Unlock()
	return r.Act(c, "send", as, map[string]any{"message": map[string]any{
		"id": id, "conversation": conversation, "thread": nil, "text": text, "mentions": []string{}, "level": nil}})
}

// LogKinds counts the log's entries of the given kinds after a number.
func (r *Rig) LogKinds(after int64, kinds ...string) int {
	r.T.Helper()
	var n int
	if err := orgdb.InOrg(r.Ctx, orgdb.Single{Pool: r.Pool}, r.Org, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Ctx, `SELECT count(*) FROM change_log WHERE org = $1 AND number > $2 AND kind = ANY($3)`,
			string(r.Org), after, kinds).Scan(&n)
	}); err != nil {
		r.T.Fatal(err)
	}
	return n
}

// Head is the log's newest number.
func (r *Rig) Head() int64 {
	r.T.Helper()
	var n int64
	if err := r.Pool.QueryRow(r.Ctx, `SELECT head FROM org_log WHERE org = $1`, string(r.Org)).Scan(&n); err != nil {
		r.T.Fatal(err)
	}
	return n
}

// ReadUpTo is a person's read position on a channel.
func (r *Rig) ReadUpTo(person, conversation string) int64 {
	r.T.Helper()
	var at int64
	_ = orgdb.InOrg(r.Ctx, orgdb.Single{Pool: r.Pool}, r.Org, func(tx pgx.Tx) error {
		_ = tx.QueryRow(r.Ctx, `SELECT up_to FROM reader_reads WHERE org = $1 AND person = $2 AND conversation = $3 AND thread IS NULL`,
			string(r.Org), person, conversation).Scan(&at)
		return nil
	})
	return at
}

// Await waits for a condition something else is working towards.
func (r *Rig) Await(what string, cond func() bool) {
	r.T.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			r.T.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(3 * time.Millisecond)
	}
}

// AdvanceUntil moves the clock a second at a time until cond holds, and fails if
// that takes more than limit of the clock's time. It returns the time it took.
func (r *Rig) AdvanceUntil(what string, limit time.Duration, cond func() bool) time.Duration {
	r.T.Helper()
	var moved time.Duration
	for {
		wait := time.Now().Add(30 * time.Millisecond)
		for time.Now().Before(wait) {
			if cond() {
				return moved
			}
			time.Sleep(2 * time.Millisecond)
		}
		if moved >= limit {
			r.T.Fatalf("%s: still not true after %s on the clock", what, moved)
		}
		r.Clock.Advance(time.Second)
		moved += time.Second
	}
}

// State is what a screen shows, in a form two screens can be compared by.
type State struct {
	Number   int64
	Messages map[string]string // top-level message → "version:text"
	Unread   map[string]bool   // conversation → sidebar mark
}

// Fresh is what a freshly loaded screen of this person shows.
func (r *Rig) Fresh(c *Copy, person string) State {
	r.T.Helper()
	start, err := c.Store.Start(r.Ctx, r.Org, person)
	if err != nil {
		r.T.Fatal(err)
	}
	st := State{Number: start.Number, Messages: map[string]string{}, Unread: map[string]bool{}}
	for _, conv := range start.Conversations {
		st.Unread[conv.Conversation] = conv.Unread
		q := conversations.MessagesQuery{Conversation: conv.Conversation, Limit: 200, As: person}
		for {
			page, err := c.Store.Messages(r.Ctx, r.Org, q)
			if err != nil {
				r.T.Fatal(err)
			}
			for _, m := range page.Messages {
				st.Messages[m.Message.Id] = fmt.Sprintf("%d:%s", m.Message.Version, m.Message.Text)
			}
			if !page.HasBefore || len(page.Messages) == 0 {
				break
			}
			oldest := page.Messages[0].Message.Number
			q.Before = &oldest
		}
	}
	return st
}

// Diff says how two states differ, or "".
func Diff(a, b State) string {
	var out []string
	if a.Number != b.Number {
		out = append(out, fmt.Sprintf("number %d against %d", a.Number, b.Number))
	}
	for k, v := range a.Messages {
		if b.Messages[k] != v {
			out = append(out, fmt.Sprintf("message %s: %q against %q", k, v, b.Messages[k]))
		}
	}
	for k, v := range b.Messages {
		if _, ok := a.Messages[k]; !ok {
			out = append(out, fmt.Sprintf("message %s missing: fresh has %q", k, v))
		}
	}
	for k, v := range b.Unread {
		if a.Unread[k] != v {
			out = append(out, fmt.Sprintf("unread %s: %v against %v", k, a.Unread[k], v))
		}
	}
	slices.Sort(out)
	return strings.Join(out, "; ")
}

// Screen is a client keeping the contract: it applies an update only if its
// `after` is the number it holds, ignores a number it has, and on any other
// mismatch asks again from its own number.
type Screen struct {
	r      *Rig
	Person string
	ws     *websocket.Conn
	cancel context.CancelFunc

	mu       sync.Mutex
	state    State
	applied  []int64
	received []int64
	asked    int
	tooFar   []string
	feed     []contract.LiveFeedUpdate
	feedAt   *int64
	edition  *int64
	gone     bool
	closed   bool
	// Drop loses an update on the way, as a broken connection would.
	Drop func(number int64) bool
	// deaf stops answering anything, pings included.
	deaf chan struct{}
}

// Open loads a screen fresh and connects it.
func (r *Rig) Open(c *Copy, person string) *Screen {
	r.T.Helper()
	s := &Screen{r: r, Person: person, state: r.Fresh(c, person), deaf: make(chan struct{})}
	s.Connect(c)
	r.Await("the screen to be placed in its stream", func() bool { return c.Live.Following(r.Org, person) })
	return s
}

// OpenRetired connects to an organization that is already retired.
func (r *Rig) OpenRetired(c *Copy, person string) *Screen {
	r.T.Helper()
	s := &Screen{r: r, Person: person, state: State{Messages: map[string]string{}, Unread: map[string]bool{}}, deaf: make(chan struct{})}
	s.Connect(c)
	return s
}

// Connect opens the connection and resumes from the screen's number.
func (s *Screen) Connect(c *Copy) {
	s.r.T.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	url := "ws" + strings.TrimPrefix(c.Server.URL, "http") + "/orgs/" + string(s.r.Org) + "/live?person=" + s.Person
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		cancel()
		s.r.T.Fatal(err)
	}
	ws.SetReadLimit(1 << 22)
	s.mu.Lock()
	s.ws, s.cancel, s.closed = ws, cancel, false
	s.mu.Unlock()
	s.r.T.Cleanup(func() { cancel(); _ = ws.CloseNow() })
	go s.read(ctx, ws)
	s.resume(ctx, ws)
}

func (s *Screen) resume(ctx context.Context, ws *websocket.Conn) {
	s.mu.Lock()
	body := contract.LiveResume{Resume: contract.ResumeNumbers{Conversations: s.state.Number, Feed: s.feedAt, FeedEdition: s.edition}}
	s.asked++
	s.mu.Unlock()
	b, _ := json.Marshal(body)
	_ = ws.Write(ctx, websocket.MessageText, b)
}

// SetFeed gives the screen a position in the feed stream.
func (s *Screen) SetFeed(number, edition int64) {
	s.mu.Lock()
	s.feedAt, s.edition = &number, &edition
	ws := s.ws
	s.mu.Unlock()
	s.resume(context.Background(), ws)
}

// Say sends a message of the live protocol.
func (s *Screen) Say(v any) {
	b, _ := json.Marshal(v)
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if err := ws.Write(context.Background(), websocket.MessageText, b); err != nil {
		s.r.T.Fatalf("screen write: %v", err)
	}
}

// Watch says a row is open, or no longer.
func (s *Screen) Watch(conversation string, open bool) {
	s.Say(contract.LiveWatching{Watching: contract.WatchingNote{Conversation: conversation, Open: open}})
}

// Cut drops the connection without a goodbye.
func (s *Screen) Cut() {
	s.mu.Lock()
	ws, cancel := s.ws, s.cancel
	s.mu.Unlock()
	cancel()
	_ = ws.CloseNow()
	s.r.Await("the screen's connection to close", func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.closed
	})
}

// GoDeaf stops the screen reading, so it answers no ping: it has vanished.
func (s *Screen) GoDeaf() { close(s.deaf) }

func (s *Screen) read(ctx context.Context, ws *websocket.Conn) {
	defer func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
	}()
	for {
		// A deaf screen must stop calling Read, which is what answers pings, while
		// an already blocked Read is released by the next message or the close.
		select {
		case <-s.deaf:
			<-ctx.Done()
			return
		default:
		}
		_, raw, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		switch {
		case m["organization_gone"] != nil:
			s.mu.Lock()
			s.gone = true
			s.mu.Unlock()
		case m["too_far_behind"] != nil:
			var which string
			_ = json.Unmarshal(m["too_far_behind"], &which)
			s.mu.Lock()
			s.tooFar = append(s.tooFar, which)
			s.mu.Unlock()
		case m["last_sent"] != nil:
			var l contract.LastSentNumbers
			_ = json.Unmarshal(m["last_sent"], &l)
			s.mu.Lock()
			behind := l.Conversations != s.state.Number
			s.mu.Unlock()
			if behind {
				s.resume(ctx, ws)
			}
		case string(m["stream"]) == `"conversations"`:
			var u struct {
				Number int64             `json:"number"`
				After  int64             `json:"after"`
				Parts  []json.RawMessage `json:"parts"`
			}
			_ = json.Unmarshal(raw, &u)
			s.take(ctx, ws, u.Number, u.After, u.Parts)
		case string(m["stream"]) == `"feed"`:
			var loose struct {
				Number  int64 `json:"number"`
				After   int64 `json:"after"`
				Edition int64 `json:"edition"`
			}
			_ = json.Unmarshal(raw, &loose)
			u := contract.LiveFeedUpdate{Stream: "feed", Number: loose.Number, After: loose.After, Edition: loose.Edition}
			s.mu.Lock()
			s.feed = append(s.feed, u)
			n := u.Number
			s.feedAt = &n
			s.mu.Unlock()
		}
	}
}

func (s *Screen) take(ctx context.Context, ws *websocket.Conn, number, after int64, parts []json.RawMessage) {
	s.mu.Lock()
	s.received = append(s.received, number)
	if s.Drop != nil && s.Drop(number) {
		s.mu.Unlock()
		return
	}
	if number <= s.state.Number {
		s.mu.Unlock()
		return
	}
	if after != s.state.Number {
		s.mu.Unlock()
		s.resume(ctx, ws)
		return
	}
	for _, raw := range parts {
		s.apply(raw)
	}
	s.state.Number = number
	s.applied = append(s.applied, number)
	s.mu.Unlock()
}

func (s *Screen) apply(raw json.RawMessage) {
	var p struct {
		Kind    string          `json:"kind"`
		Message json.RawMessage `json:"message"`
		Conv    string          `json:"conversation"`
		Thread  *string         `json:"thread"`
		Unread  bool            `json:"unread"`
		Summary *struct {
			Conversation string `json:"conversation"`
			Unread       bool   `json:"unread"`
		} `json:"conversation_summary"`
	}
	_ = json.Unmarshal(raw, &p)
	switch p.Kind {
	case "message.created", "message.changed":
		var m contract.Message
		_ = json.Unmarshal(p.Message, &m)
		if m.Thread == nil {
			s.state.Messages[m.Id] = fmt.Sprintf("%d:%s", m.Version, m.Text)
		}
	case "message.deleted":
		var m contract.DeletedMessage
		_ = json.Unmarshal(p.Message, &m)
		delete(s.state.Messages, m.Id)
	case "unread":
		if p.Thread == nil {
			s.state.Unread[p.Conv] = p.Unread
		}
	case "conversation.added":
		if p.Summary != nil {
			s.state.Unread[p.Summary.Conversation] = p.Summary.Unread
		}
	}
}

// Now is what the screen shows.
func (s *Screen) Now() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := State{Number: s.state.Number, Messages: map[string]string{}, Unread: map[string]bool{}}
	for k, v := range s.state.Messages {
		out.Messages[k] = v
	}
	for k, v := range s.state.Unread {
		out.Unread[k] = v
	}
	return out
}

// Applied lists the numbers the screen applied, in order.
func (s *Screen) Applied() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.applied)
}

// Received lists every number that arrived, in order.
func (s *Screen) Received() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.received)
}

// Asked counts how often the screen asked to resume.
func (s *Screen) Asked() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.asked
}

// TooFar lists the streams the screen was told to load fresh.
func (s *Screen) TooFar() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.tooFar)
}

// FeedUpdates lists the feed updates that arrived.
func (s *Screen) FeedUpdates() []contract.LiveFeedUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.feed)
}

// Gone says whether the screen was told the organization is gone.
func (s *Screen) Gone() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gone
}

// Closed says whether the connection has ended.
func (s *Screen) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Reload loads the screen fresh, as after "too far behind", and resumes.
func (s *Screen) Reload(c *Copy) {
	s.r.T.Helper()
	fresh := s.r.Fresh(c, s.Person)
	s.mu.Lock()
	s.state = fresh
	ws := s.ws
	s.mu.Unlock()
	s.resume(context.Background(), ws)
}
