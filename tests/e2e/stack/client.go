package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
)

var client = http.Client{Timeout: 10 * time.Second}

// Call makes one request and answers its status and body.
func Call(method, address string, in any) (int, []byte, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, address, body)
	if err != nil {
		return 0, nil, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	return resp.StatusCode, out, err
}

// MustGet asks a question that must answer 200.
func (s *Stack) MustGet(address string, into any) []byte {
	s.T.Helper()
	status, out, err := Call(http.MethodGet, address, nil)
	if err != nil {
		s.T.Fatalf("GET %s: %v", address, err)
	}
	if status != http.StatusOK {
		s.T.Fatalf("GET %s: %d %s", address, status, out)
	}
	if into != nil {
		if err := json.Unmarshal(out, into); err != nil {
			s.T.Fatalf("GET %s: %v in %s", address, err, out)
		}
	}
	return out
}

// Org is a seeded organization, reached through the people door.
type Org struct {
	S      *Stack
	Name   string
	Head   int64 // the seal's number
	people []string
	next   atomic.Int64
}

// Seed creates an organization with the given people in the given channels.
func (s *Stack) Seed(name string, people, channels []string) *Org {
	s.T.Helper()
	post := func(path string, in any) []byte {
		status, out, err := Call(http.MethodPost, s.Door+"/orgs/"+name+path, in)
		if err != nil || status != http.StatusOK {
			s.T.Fatalf("seed %s%s: %d %v %s", name, path, status, err, out)
		}
		return out
	}
	var opened struct {
		Session string `json:"session"`
	}
	if err := json.Unmarshal(post("/seed/open", map[string]any{}), &opened); err != nil {
		s.T.Fatal(err)
	}
	const at = "2026-11-01T09:00:00.000Z"
	var changes []map[string]any
	n := int64(0)
	add := func(c map[string]any) {
		n++
		c["org"], c["number"], c["previous"], c["at"] = name, n, n-1, at
		changes = append(changes, c)
	}
	add(map[string]any{"kind": "seed.opened"})
	for _, p := range people {
		add(map[string]any{"kind": "person.joined", "person": p, "name": strings.ToUpper(p[:1]) + p[1:], "is_agent": strings.HasSuffix(p, "-agent")})
	}
	for _, c := range channels {
		add(map[string]any{"kind": "channel.created", "conversation": c, "name": strings.TrimPrefix(c, "#"), "people": people})
	}
	post("/seed/"+opened.Session, map[string]any{"changes": changes})
	var sealed struct {
		Through int64 `json:"through"`
	}
	if err := json.Unmarshal(post("/seed/"+opened.Session+"/seal", map[string]any{}), &sealed); err != nil {
		s.T.Fatal(err)
	}
	s.orgNames = append(s.orgNames, name)
	return &Org{S: s, Name: name, Head: sealed.Through, people: people}
}

// Address is an address inside the organization at the people door.
func (o *Org) Address(rest string) string { return o.S.Door + "/orgs/" + o.Name + rest }

// Act sends one action with a fresh ID and answers the status and body.
func (o *Org) Act(as, kind string, fields map[string]any) (int, []byte) {
	o.S.T.Helper()
	body := map[string]any{"id": fmt.Sprintf("act_%s_%d", kind, o.next.Add(1)), "as": as, "kind": kind}
	for k, v := range fields {
		body[k] = v
	}
	status, out, err := Call(http.MethodPost, o.Address("/people/actions"), body)
	if err != nil {
		o.S.T.Fatalf("%s by %s: %v", kind, as, err)
	}
	return status, out
}

// Do sends an action that must be saved and answers its number.
func (o *Org) Do(as, kind string, fields map[string]any) int64 {
	o.S.T.Helper()
	status, out := o.Act(as, kind, fields)
	if status != http.StatusOK {
		o.S.T.Fatalf("%s by %s: %d %s", kind, as, status, out)
	}
	var saved struct {
		Number int64 `json:"number"`
	}
	if err := json.Unmarshal(out, &saved); err != nil {
		o.S.T.Fatal(err)
	}
	return saved.Number
}

// Message is the message body of a send or an edit.
func Message(id, conversation, text string, thread *string, mentions ...string) map[string]any {
	if mentions == nil {
		mentions = []string{}
	}
	return map[string]any{"message": map[string]any{"id": id, "conversation": conversation, "thread": thread,
		"text": text, "mentions": mentions, "level": nil}}
}

// Edit is the body of an edit action.
func Edit(id, text string, mentions ...string) map[string]any {
	if mentions == nil {
		mentions = []string{}
	}
	return map[string]any{"message": id, "text": text, "mentions": mentions, "level": nil}
}

// Delete is the body of a delete action.
func Delete(id string) map[string]any { return map[string]any{"message": id} }

// Hit is one search result.
type Hit struct {
	Message string `json:"message"`
	Version int64  `json:"version"`
	Text    string `json:"text"`
}

// SearchAnswer is the door's search answer.
type SearchAnswer struct {
	Through int64   `json:"through"`
	Total   int64   `json:"total"`
	Results []Hit   `json:"results"`
	Notice  *string `json:"notice"`
}

// SearchFor asks through the people door.
func (o *Org) SearchFor(person, q string) SearchAnswer {
	o.S.T.Helper()
	var a struct {
		Answer *SearchAnswer `json:"answer"`
		Notice *string       `json:"notice"`
	}
	o.S.MustGet(o.Address("/people/"+person+"/search?q="+url.QueryEscape(q)), &a)
	if a.Answer == nil {
		return SearchAnswer{Notice: a.Notice}
	}
	a.Answer.Notice = a.Notice
	return *a.Answer
}

// Row is the part of a feed row the tests read.
type Row struct {
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Message      string  `json:"message"`
	Bucket       string  `json:"bucket"`
	Unread       bool    `json:"unread"`
	Why          string  `json:"why"`
}

// FeedAnswer is the door's feed answer.
type FeedAnswer struct {
	Edition *int64  `json:"edition"`
	Through *int64  `json:"through"`
	Number  *int64  `json:"number"`
	Rows    []Row   `json:"rows"`
	Notice  *string `json:"notice"`
}

// FeedFor asks through the people door.
func (o *Org) FeedFor(person string) FeedAnswer {
	o.S.T.Helper()
	var a FeedAnswer
	o.S.MustGet(o.Address("/people/"+person+"/feed"), &a)
	return a
}

// Notification is one entry on the sender's list.
type Notification struct {
	Name    string     `json:"name"`
	Person  string     `json:"person"`
	State   string     `json:"state"`
	At      *time.Time `json:"at"`
	Delayed bool       `json:"delayed"`
	Text    string     `json:"text"`
	Now     *string    `json:"now"`
}

// NotificationsFor reads the sender's list directly, as the notification app does.
func (o *Org) NotificationsFor(person string) []Notification {
	o.S.T.Helper()
	var a struct {
		Notifications []Notification `json:"notifications"`
	}
	o.S.MustGet(o.S.Sender+"/orgs/"+o.Name+"/notifications?person="+url.QueryEscape(person), &a)
	return a.Notifications
}

// HandOvers counts what the sender handed to its provider for a name.
func (o *Org) HandOvers(name string) int {
	o.S.T.Helper()
	var n int
	if err := o.S.Admin.QueryRow(context.Background(), `SELECT count(*) FROM sender.deliveries WHERE org = $1 AND name = $2`, o.Name, name).Scan(&n); err != nil {
		o.S.T.Fatal(err)
	}
	return n
}

// LogEntry is one line of the organization's change log.
type LogEntry struct {
	Number int64
	At     time.Time
	Kind   string
}

// Log reads the change log. The test's own admin login does this; no program does.
func (o *Org) Log() []LogEntry {
	o.S.T.Helper()
	rows, err := o.S.Admin.Query(context.Background(), `SELECT number, at, kind FROM conversations.change_log WHERE org = $1 ORDER BY number`, o.Name)
	if err != nil {
		o.S.T.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (LogEntry, error) {
		var e LogEntry
		return e, r.Scan(&e.Number, &e.At, &e.Kind)
	})
	if err != nil {
		o.S.T.Fatal(err)
	}
	return out
}

// LogBodies reads the raw entries, for a clean replay.
func (o *Org) LogBodies() [][]byte {
	o.S.T.Helper()
	rows, err := o.S.Admin.Query(context.Background(), `SELECT body::text FROM conversations.change_log WHERE org = $1 ORDER BY number`, o.Name)
	if err != nil {
		o.S.T.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) ([]byte, error) {
		var b string
		err := r.Scan(&b)
		return []byte(b), err
	})
	if err != nil {
		o.S.T.Fatal(err)
	}
	return out
}

// Screen is an open screen: it loads, follows the live connection by the contract's
// numbers, resumes after a cut, and remembers each message at its newest version.
type Screen struct {
	org    *Org
	person string

	mu       sync.Mutex
	last     int64
	messages map[string]ScreenMessage
	received map[int64]time.Time
	resumes  atomic.Int64
	frames   [][]byte
	feedAt   *int64
	edition  *int64

	cancel context.CancelFunc
	done   chan struct{}
}

// ScreenMessage is what a screen shows of one message.
type ScreenMessage struct {
	Version int64
	Text    string
	Deleted bool
}

// OpenScreen loads the person's view and opens the live connection.
func (o *Org) OpenScreen(person string) *Screen {
	o.S.T.Helper()
	var start struct {
		Number int64 `json:"number"`
	}
	o.S.MustGet(o.Address("/people/"+person+"/start"), &start)
	feed := o.FeedFor(person)
	ctx, cancel := context.WithCancel(context.Background())
	sc := &Screen{org: o, person: person, last: start.Number, messages: map[string]ScreenMessage{},
		received: map[int64]time.Time{}, cancel: cancel, done: make(chan struct{}), feedAt: feed.Number, edition: feed.Edition}
	first := make(chan error, 1)
	go sc.run(ctx, first)
	select {
	case err := <-first:
		if err != nil {
			cancel()
			o.S.T.Fatalf("the screen of %s could not connect: %v", person, err)
		}
	case <-time.After(10 * time.Second):
		cancel()
		o.S.T.Fatalf("the screen of %s did not connect in 10 s", person)
	}
	o.S.T.Cleanup(sc.Close)
	return sc
}

// Close ends the screen.
func (sc *Screen) Close() {
	sc.cancel()
	<-sc.done
}

func (sc *Screen) run(ctx context.Context, first chan<- error) {
	defer close(sc.done)
	pause := 100 * time.Millisecond
	address := "ws" + strings.TrimPrefix(sc.org.Address("/live?person="+sc.person), "http")
	for ctx.Err() == nil {
		ws, _, err := websocket.Dial(ctx, address, nil)
		if err == nil {
			ws.SetReadLimit(4 << 20)
			err = sc.follow(ctx, ws, first)
			first = nil
			pause = 100 * time.Millisecond
		} else if first != nil {
			first <- err
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pause):
		}
		pause = min(2*pause, time.Second)
	}
}

func (sc *Screen) resume(ctx context.Context, ws *websocket.Conn) error {
	sc.mu.Lock()
	n, feedAt, edition := sc.last, sc.feedAt, sc.edition
	sc.mu.Unlock()
	sc.resumes.Add(1)
	b, _ := json.Marshal(map[string]any{"resume": map[string]any{"conversations": n, "feed": feedAt, "feed_edition": edition}})
	return ws.Write(ctx, websocket.MessageText, b)
}

func (sc *Screen) follow(ctx context.Context, ws *websocket.Conn, first chan<- error) error {
	defer ws.CloseNow()
	if err := sc.resume(ctx, ws); err != nil {
		if first != nil {
			first <- err
		}
		return err
	}
	if first != nil {
		first <- nil
	}
	for {
		_, raw, err := ws.Read(ctx)
		if err != nil {
			return err
		}
		var u struct {
			Stream string `json:"stream"`
			Number int64  `json:"number"`
			After  int64  `json:"after"`
			Parts  []struct {
				Kind    string          `json:"kind"`
				Message json.RawMessage `json:"message"`
			} `json:"parts"`
			LastSent *struct {
				Conversations int64 `json:"conversations"`
			} `json:"last_sent"`
		}
		if json.Unmarshal(raw, &u) != nil {
			continue
		}
		sc.mu.Lock()
		sc.frames = append(sc.frames, raw)
		if u.Stream == "feed" && (sc.feedAt == nil || u.Number > *sc.feedAt) {
			n := u.Number
			sc.feedAt = &n
		}
		sc.mu.Unlock()
		if u.LastSent != nil {
			sc.mu.Lock()
			behind := u.LastSent.Conversations > sc.last
			sc.mu.Unlock()
			if behind {
				if err := sc.resume(ctx, ws); err != nil {
					return err
				}
			}
			continue
		}
		if u.Stream != "conversations" {
			continue
		}
		now := time.Now()
		sc.mu.Lock()
		switch {
		case u.Number <= sc.last:
			sc.mu.Unlock()
			continue
		case u.After != sc.last:
			sc.mu.Unlock()
			if err := sc.resume(ctx, ws); err != nil {
				return err
			}
			continue
		}
		sc.last = u.Number
		sc.received[u.Number] = now
		for _, p := range u.Parts {
			var m struct {
				ID      string `json:"id"`
				Version int64  `json:"version"`
				Text    string `json:"text"`
			}
			if json.Unmarshal(p.Message, &m) != nil || m.ID == "" {
				continue
			}
			if cur, ok := sc.messages[m.ID]; ok && (cur.Version >= m.Version || cur.Deleted) {
				continue
			}
			switch p.Kind {
			case "message.created", "message.changed":
				sc.messages[m.ID] = ScreenMessage{Version: m.Version, Text: m.Text}
			case "message.deleted":
				sc.messages[m.ID] = ScreenMessage{Version: m.Version, Deleted: true}
			}
		}
		sc.mu.Unlock()
	}
}

// ReceivedAt says when the screen applied an update number.
func (sc *Screen) ReceivedAt(number int64) (time.Time, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	at, ok := sc.received[number]
	return at, ok
}

// Last is the last number the screen applied.
func (sc *Screen) Last() int64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.last
}

// Messages is what the screen shows now.
func (sc *Screen) Messages() map[string]ScreenMessage {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	out := make(map[string]ScreenMessage, len(sc.messages))
	for k, v := range sc.messages {
		out[k] = v
	}
	return out
}

// Saw says whether anything that reached this screen holds the text.
func (sc *Screen) Saw(text string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	for _, f := range sc.frames {
		if bytes.Contains(f, []byte(text)) {
			return true
		}
	}
	return false
}

// Resumes counts how many times the screen asked to resume.
func (sc *Screen) Resumes() int64 { return sc.resumes.Load() }

// FreshMessages loads a channel and its threads from the door, every page, as a
// new screen would.
func (o *Org) FreshMessages(person, conversation string) map[string]ScreenMessage {
	o.S.T.Helper()
	out := map[string]ScreenMessage{}
	for _, root := range o.freshPage(person, conversation, "", out) {
		o.freshPage(person, conversation, root, out)
	}
	return out
}

// freshPage loads one channel or thread into out and answers the messages with replies.
func (o *Org) freshPage(person, conversation, thread string, out map[string]ScreenMessage) []string {
	o.S.T.Helper()
	var roots []string
	var before *int64
	for {
		q := url.Values{"as": {person}, "limit": {"100"}}
		if thread != "" {
			q.Set("thread", thread)
		}
		if before != nil {
			q.Set("before", fmt.Sprint(*before))
		}
		var a struct {
			Messages []struct {
				Message struct {
					ID      string `json:"id"`
					Number  int64  `json:"number"`
					Version int64  `json:"version"`
					Text    string `json:"text"`
					Deleted bool   `json:"deleted"`
				} `json:"message"`
				ThreadSummary *struct {
					Replies int64 `json:"replies"`
				} `json:"thread_summary"`
			} `json:"messages"`
			HasBefore bool `json:"has_before"`
		}
		o.S.MustGet(o.Address("/conversations/"+url.PathEscape(conversation)+"/messages?"+q.Encode()), &a)
		oldest := int64(0)
		for _, v := range a.Messages {
			m := v.Message
			out[m.ID] = ScreenMessage{Version: m.Version, Text: m.Text, Deleted: m.Deleted}
			if oldest == 0 || m.Number < oldest {
				oldest = m.Number
			}
			if v.ThreadSummary != nil && v.ThreadSummary.Replies > 0 {
				roots = append(roots, m.ID)
			}
		}
		if !a.HasBefore || len(a.Messages) == 0 {
			return roots
		}
		before = &oldest
	}
}
