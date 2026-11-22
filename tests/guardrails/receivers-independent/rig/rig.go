// Package rig is the delivery guardrails' test bench: a database of its own for
// each test, a log filled through the conversations and feed logins, recording
// receivers that keep the contract's answers, and delivery workers in-process.
package rig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/delivery"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	sqlfiles "contexia/migrations"
	"contexia/tests/harness/logwriter"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is a database of its own, with the parts' schemas and migrations.
type DB struct {
	Name                     string
	Conversations, Feed, Del *pgxpool.Pool
	mu                       sync.Mutex
	entries                  map[string][]logwriter.Entry
	next                     map[string]int
}

func url(login, password, db string) string {
	host := os.Getenv("CONTEXIA_PG_HOST")
	return fmt.Sprintf("postgres://%s:%s@%s:5432/%s", login, password, host, db)
}

func open(t testing.TB, u string, conns int32) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(u)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = conns
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// NewDB creates a throwaway database, so tests can run side by side and the
// running system's own delivery program never sees their organizations.
func NewDB(t testing.TB) *DB {
	t.Helper()
	if os.Getenv("CONTEXIA_PG_HOST") == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	ctx := context.Background()
	name := fmt.Sprintf("t047_%d_%d", time.Now().UnixNano(), rand.Intn(1000))
	admin, err := pgx.Connect(ctx, url("contexia_admin", "admin_dev", "contexia"))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, url("contexia_admin", "admin_dev", "contexia"))
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	inNew, err := pgx.Connect(ctx, url("contexia_admin", "admin_dev", name))
	if err != nil {
		t.Fatal(err)
	}
	defer inNew.Close(ctx)
	for _, q := range []string{
		"REVOKE ALL ON SCHEMA public FROM PUBLIC",
		"CREATE SCHEMA conversations AUTHORIZATION conversations",
		"CREATE SCHEMA feed AUTHORIZATION feed",
		"GRANT USAGE ON SCHEMA conversations TO delivery",
		"GRANT USAGE ON SCHEMA feed TO delivery",
	} {
		if _, err := inNew.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	d := &DB{
		Name:          name,
		Conversations: open(t, url("conversations", "conversations_dev", name), 12),
		Feed:          open(t, url("feed", "feed_dev", name), 6),
		Del:           open(t, url("delivery", "delivery_dev", name), 16),
		entries:       map[string][]logwriter.Entry{},
		next:          map[string]int{},
	}
	if err := migrate.Apply(ctx, d.Conversations, sqlfiles.Files, "conversations"); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(ctx, d.Feed, sqlfiles.Files, "feed"); err != nil {
		t.Fatal(err)
	}
	return d
}

// Directory is the delivery login's view of this database.
func (d *DB) Directory() delivery.Databases {
	return delivery.OneDatabase{Single: orgdb.Single{Pool: d.Del}}
}

// AddOrg creates an organization with an empty log and a script of n changes.
func (d *DB) AddOrg(t testing.TB, org string, n int) {
	t.Helper()
	ctx := context.Background()
	if _, err := d.Conversations.Exec(ctx, `INSERT INTO org_log (org) VALUES ($1)`, org); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Feed.Exec(ctx, `INSERT INTO note_heads (org) VALUES ($1)`, org); err != nil {
		t.Fatal(err)
	}
	script := logwriter.Write(logwriter.Config{Orgs: []string{org}, Seed: 47, Changes: max(n, 60)})
	d.mu.Lock()
	d.entries[org] = script[org]
	d.next[org] = 0
	d.mu.Unlock()
}

func (d *DB) nextEntry(org string) (logwriter.Entry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	i := d.next[org]
	if i >= len(d.entries[org]) {
		return logwriter.Entry{}, fmt.Errorf("the script of %s has only %d changes", org, i)
	}
	d.next[org] = i + 1
	return d.entries[org][i], nil
}

func patchAt(entry []byte, at time.Time) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(entry, &m); err != nil {
		return nil, err
	}
	stamp, _ := json.Marshal(at.UTC().Format("2006-01-02T15:04:05.000Z"))
	m["at"] = stamp
	return json.Marshal(m)
}

// Save writes the next change as Conversations does: number, entry and a due
// row per receiver in one transaction. It uses ON CONFLICT DO NOTHING, the
// weaker of the two ways a save can add its row, so a worker must cope with it.
func (d *DB) Save(ctx context.Context, org string) (int64, error) {
	return d.save(ctx, org, 0)
}

// SaveHolding is Save with the transaction kept open for hold before commit.
func (d *DB) SaveHolding(ctx context.Context, org string, hold time.Duration) (int64, error) {
	return d.save(ctx, org, hold)
}

func (d *DB) save(ctx context.Context, org string, hold time.Duration) (int64, error) {
	e, err := d.nextEntry(org)
	if err != nil {
		return 0, err
	}
	at := time.Now()
	body, err := patchAt(e.JSON, at)
	if err != nil {
		return 0, err
	}
	err = orgdb.InOrg(ctx, orgdb.Single{Pool: d.Conversations}, ids.Org(org), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE org_log SET head = $2, head_at = $3 WHERE org = $1`, org, e.Number, at); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO change_log (org, number, at, kind, body) VALUES ($1, $2, $3, $4, $5)`,
			org, e.Number, at, e.Kind, string(body)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO delivery_due (receiver, org) VALUES ('search', $1), ('feed', $1) ON CONFLICT DO NOTHING`, org); err != nil {
			return err
		}
		if hold > 0 {
			select {
			case <-time.After(hold):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return e.Number, nil
}

// SaveNote writes the next note of the feed's log.
func (d *DB) SaveNote(ctx context.Context, org, kind string) (int64, error) {
	var number int64
	err := orgdb.InOrg(ctx, orgdb.Single{Pool: d.Feed}, ids.Org(org), func(tx pgx.Tx) error {
		var edition int64
		if err := tx.QueryRow(ctx, `UPDATE note_heads SET head = head + 1 WHERE org = $1 RETURNING head, edition`, org).
			Scan(&number, &edition); err != nil {
			return err
		}
		at := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		note := map[string]any{"org": org, "number": number, "previous": number - 1, "edition": edition, "at": at, "kind": kind}
		name := fmt.Sprintf("%s / marco / #launch:msg_1 / %d", org, number)
		switch kind {
		case "due":
			note["name"], note["person"], note["bucket"], note["why"], note["text"] = name, "marco", "needs_you", "test", "text"
			note["row"] = map[string]any{"conversation": "#launch", "thread": "msg_1"}
		case "withdrawn":
			note["name"], note["because"] = name, "read"
		}
		b, _ := json.Marshal(note)
		if _, err := tx.Exec(ctx, `INSERT INTO note_log (org, edition, number, at, kind, body) VALUES ($1, $2, $3, now(), $4, $5)`,
			org, edition, number, kind, string(b)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO note_due (org) VALUES ($1) ON CONFLICT DO NOTHING`, org)
		return err
	})
	return number, err
}

// Mode is how a receiver behaves right now.
type Mode int32

// The ways a receiver fails. Slow and Off are the two the contracts name.
const (
	Up     Mode = iota
	Off         // accepts the call and never answers
	Slow        // answers after 10 s
	Broken      // answers 503
)

type key struct {
	org    string
	number int64
}

// Receiver records what it is sent and answers as its contract says.
type Receiver struct {
	Kind    string // search, feed or sender
	URL     string
	mode    atomic.Int32
	Calls   atomic.Int64
	PerItem atomic.Int64 // nanoseconds of work per item
	// LongestHang is the longest a caller stayed on a call that was never answered.
	LongestHang atomic.Int64

	mu       sync.Mutex
	places   map[string]int64
	editions map[string]int64
	wipes    map[string]int64
	stopped  map[string]bool
	refuse   func(org string, number int64) bool
	seen     map[key]int
	first    map[key]time.Time
	srv      *httptest.Server
	closing  chan struct{}
}

// NewReceiver starts a recording receiver of the given kind.
func NewReceiver(t testing.TB, kind string) *Receiver {
	r := &Receiver{Kind: kind, places: map[string]int64{}, editions: map[string]int64{},
		wipes: map[string]int64{}, stopped: map[string]bool{}, seen: map[key]int{}, first: map[key]time.Time{},
		closing: make(chan struct{})}
	r.srv = httptest.NewServer(r)
	r.URL = r.srv.URL
	t.Cleanup(func() {
		r.SetMode(Up)
		close(r.closing)
		r.srv.CloseClientConnections()
		r.srv.Close()
	})
	return r
}

// SetMode switches how the receiver behaves.
func (r *Receiver) SetMode(m Mode) { r.mode.Store(int32(m)) }

// Refuse sets which items are answered 422. nil refuses none.
func (r *Receiver) Refuse(f func(org string, number int64) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refuse = f
}

// StopOrg makes a feed answer 423 for org.
func (r *Receiver) StopOrg(org string, stopped bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped[org] = stopped
}

// Wipe empties the receiver for org: its place goes to 0 and its wipe number rises.
func (r *Receiver) Wipe(org string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wipes[org]++
	r.places[org] = 0
	for k := range r.seen {
		if k.org == org {
			delete(r.seen, k)
			delete(r.first, k)
		}
	}
}

// SetPlace makes the receiver stand at through for org, as one that lost or
// already holds changes would.
func (r *Receiver) SetPlace(org string, through int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.places[org] = through
}

func (r *Receiver) ServeHTTP(w http.ResponseWriter, q *http.Request) {
	// Read the body first: only then does the server notice a caller giving up.
	body, err := io.ReadAll(q.Body)
	if err != nil {
		return
	}
	if q.Method == http.MethodPost {
		r.Calls.Add(1)
	}
	switch Mode(r.mode.Load()) {
	case Off:
		began := time.Now()
		select {
		case <-q.Context().Done():
			if d := int64(time.Since(began)); d > r.LongestHang.Load() {
				r.LongestHang.Store(d)
			}
		case <-r.closing:
		}
		return
	case Slow:
		select {
		case <-time.After(10 * time.Second):
		case <-q.Context().Done():
			return
		case <-r.closing:
			return
		}
	case Broken:
		http.Error(w, "broken", http.StatusServiceUnavailable)
		return
	}
	switch {
	case q.Method == http.MethodGet && q.URL.Path == "/alive":
		writeJSON(w, 200, map[string]string{"status": "alive"})
	case q.Method == http.MethodGet && strings.HasSuffix(q.URL.Path, "/place"):
		org := strings.Split(q.URL.Path, "/")[2]
		r.mu.Lock()
		out := map[string]int64{"through": r.places[org], "wipe": r.wipes[org], "edition": max(r.editions[org], 1)}
		r.mu.Unlock()
		writeJSON(w, 200, out)
	case q.URL.Path == "/changes/batch":
		var in struct {
			Changes []json.RawMessage `json:"changes"`
		}
		if json.Unmarshal(body, &in) != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		status, out := 200, any(map[string]bool{"applied": true})
		for _, c := range in.Changes {
			if status, out = r.apply(c); status != 200 {
				break
			}
		}
		writeJSON(w, status, out)
	default:
		status, out := r.apply(body)
		writeJSON(w, status, out)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (r *Receiver) apply(raw []byte) (int, any) {
	var h struct {
		Org      string `json:"org"`
		Number   int64  `json:"number"`
		Previous int64  `json:"previous"`
		Edition  int64  `json:"edition"`
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		return 400, map[string]string{"refused": "unreadable"}
	}
	if d := r.PerItem.Load(); d > 0 {
		time.Sleep(time.Duration(d))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key{h.Org, h.Number}
	switch r.Kind {
	case "search":
		if r.refuse != nil && r.refuse(h.Org, h.Number) {
			return 422, map[string]any{"refused": "test refusal", "wipe": r.wipes[h.Org]}
		}
		r.record(k)
		if h.Number > r.places[h.Org] {
			r.places[h.Org] = h.Number
		}
		return 200, map[string]any{"applied": true, "wipe": r.wipes[h.Org]}
	case "feed":
		if r.stopped[h.Org] {
			return 423, map[string]string{"organization_stopped": h.Org}
		}
		if h.Number <= r.places[h.Org] {
			r.record(k)
			return 200, map[string]bool{"applied": true, "already": true}
		}
		if h.Previous != r.places[h.Org] {
			return 409, map[string]int64{"i_am_at": r.places[h.Org]}
		}
		r.record(k)
		r.places[h.Org] = h.Number
		return 200, map[string]bool{"applied": true}
	}
	current := max(r.editions[h.Org], 1)
	if h.Edition < current {
		return 409, map[string]int64{"i_am_at": r.places[h.Org], "edition": current}
	}
	if h.Edition > current {
		r.editions[h.Org], r.places[h.Org] = h.Edition, 0
		for s := range r.seen {
			if s.org == h.Org {
				delete(r.seen, s)
				delete(r.first, s)
			}
		}
	}
	if r.refuse != nil && r.refuse(h.Org, h.Number) {
		return 422, map[string]string{"refused": "test refusal"}
	}
	if h.Number <= r.places[h.Org] {
		r.record(k)
		return 200, map[string]any{"taken": true, "already": true}
	}
	if h.Previous != r.places[h.Org] {
		return 409, map[string]int64{"i_am_at": r.places[h.Org], "edition": current}
	}
	r.record(k)
	r.places[h.Org] = h.Number
	return 200, map[string]bool{"taken": true}
}

func (r *Receiver) record(k key) {
	r.seen[k]++
	if _, ok := r.first[k]; !ok {
		r.first[k] = time.Now()
	}
}

// Has says whether the item arrived.
func (r *Receiver) Has(org string, number int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.seen[key{org, number}] > 0
}

// Place is the receiver's own place for org.
func (r *Receiver) Place(org string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.places[org]
}

// Repeats counts items that arrived more than once.
func (r *Receiver) Repeats() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.seen {
		n += c - 1
	}
	return n
}

// Count is how many different items of org arrived.
func (r *Receiver) Count(org string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for k := range r.seen {
		if k.org == org {
			n++
		}
	}
	return n
}

// FirstSeen is when an item first arrived.
func (r *Receiver) FirstSeen(org string, number int64) (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	at, ok := r.first[key{org, number}]
	return at, ok
}

// Bench is one running set of workers with its three receivers.
type Bench struct {
	DB                   *DB
	Search, Feed, Sender *Receiver
	Nudges               *atomic.Int64
	Clock                clock.Clock
	conv                 *httptest.Server
	cancels              []context.CancelFunc
	waits                []*sync.WaitGroup
	Ops                  string
}

// NewBench builds receivers; Start runs a copy of the workers.
func NewBench(t testing.TB, c clock.Clock) *Bench {
	if c == nil {
		c = clock.Real{}
	}
	b := &Bench{DB: NewDB(t), Search: NewReceiver(t, "search"), Feed: NewReceiver(t, "feed"), Sender: NewReceiver(t, "sender"),
		Nudges: &atomic.Int64{}, Clock: c}
	b.conv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if strings.HasSuffix(q.URL.Path, "/nudge") {
			b.Nudges.Add(1)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(b.conv.Close)
	t.Cleanup(b.StopAll)
	return b
}

// Start runs one copy of the workers, and serves its operations at b.Ops.
func (b *Bench) Start(t testing.TB, name string) *delivery.Service {
	s := delivery.New(delivery.Config{
		DB: b.DB.Directory(), Clock: b.Clock,
		Search: b.Search.URL, Feed: b.Feed.URL, Sender: b.Sender.URL, Conversations: b.conv.URL,
		Seed: 47, Worker: name,
	})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Run(ctx)
	}()
	b.cancels = append(b.cancels, cancel)
	b.waits = append(b.waits, &wg)
	ops := httptest.NewServer(s.Routes())
	t.Cleanup(ops.Close)
	b.Ops = ops.URL
	return s
}

// StopCopy ends the copy started i-th, with no chance to finish its claim's work.
func (b *Bench) StopCopy(i int) {
	b.cancels[i]()
	b.waits[i].Wait()
}

// StopAll ends every copy.
func (b *Bench) StopAll() {
	for i := range b.cancels {
		b.StopCopy(i)
	}
}

// Until waits for a condition, failing with what was expected if it never
// comes. It observes; nothing in the system depends on its timing.
func Until(t testing.TB, limit time.Duration, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("not within %v: %s", limit, what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// P95 returns the 95th percentile.
func P95(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[(len(s)*95-1)/100]
}

// Pace calls f about perSecond times a second for the given time. It stops
// early when ctx ends, and reports the first error.
func Pace(ctx context.Context, perSecond int, d time.Duration, f func() error) error {
	tick := time.NewTicker(time.Second / time.Duration(perSecond))
	defer tick.Stop()
	end := time.After(d)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-end:
			return nil
		case <-tick.C:
			if err := f(); err != nil {
				return err
			}
		}
	}
}
