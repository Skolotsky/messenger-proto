// Package stack starts the real programs for one test: each is its own process with
// its own login, on a throwaway database, joined only by their network addresses.
// Between them sit the test's meters (what each receiver was handed and when) and
// routers (which copy serves what).
//
// CONTEXIA_E2E_FEED says which feed runs: "real" (default), "standin", or the base
// address of a feed that is already running.
package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"contexia/tests/harness/router"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options says how many copies of each program run.
type Options struct {
	Conversations int
	Delivery      int
	Feeds         int
	// DeliveryFault, when set, builds delivery with its faults compiled in and
	// switches this one on (CONTEXIA_DELIVERY_FAULT).
	DeliveryFault string
	// RuleFault switches one of the feed rules' faults on in the stand-in feed
	// (rules.Fault*), to see a check fail. It has no effect on the real feed.
	RuleFault string
}

// Proc is one running program.
type Proc struct {
	Name string
	URL  string
	cmd  *exec.Cmd
	bin  string
	env  []string
	addr string
	log  *tail
	done chan struct{}
}

type tail struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *tail) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Write(p)
	if l.b.Len() > 64<<10 {
		l.b.Next(l.b.Len() - 32<<10)
	}
	return len(p), nil
}

func (l *tail) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// Stack is the composed system of one test.
type Stack struct {
	T      testing.TB
	DB     string
	Admin  *pgxpool.Pool
	Door   string // Conversations, through its router
	Search string
	Sender string
	Feed   string // the feed, through its router

	ConversationsRouter *router.Router
	FeedRouter          *router.Router
	SearchMeter         *Meter
	FeedMeter           *Meter
	SenderMeter         *Meter

	Conversations []*Proc
	Deliveries    []*Proc
	Feeds         []*Proc
	Others        []*Proc

	orgNames []string
}

var (
	buildMu sync.Mutex
	built   = map[string]string{}
)

func sourceRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func goBuild(t testing.TB, dir, out, tags string, pkgs ...string) {
	t.Helper()
	args := []string{"build", "-o", out}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	cmd := exec.Command("go", append(args, pkgs...)...)
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %v: %v\n%s", pkgs, err, b)
	}
}

// binaries builds the programs once per set of tags.
func binaries(t testing.TB, tags string) string {
	t.Helper()
	buildMu.Lock()
	defer buildMu.Unlock()
	if d, ok := built[tags]; ok {
		return d
	}
	d, err := os.MkdirTemp("", "contexia-e2e-bin")
	if err != nil {
		t.Fatal(err)
	}
	root := sourceRoot()
	goBuild(t, filepath.Join(root, "server"), d+"/", tags,
		"./cmd/conversations", "./cmd/delivery", "./cmd/search", "./cmd/sender", "./cmd/feed")
	goBuild(t, filepath.Join(root, "tests"), filepath.Join(d, "standinfeed"), tags, "./e2e/standinfeed")
	built[tags] = d
	return d
}

func pgURL(login, password, db string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:5432/%s", login, password, os.Getenv("CONTEXIA_PG_HOST"), db)
}

func login(part, db string) string { return pgURL(part, part+"_dev", db) }

func newDatabase(t testing.TB) (string, *pgxpool.Pool) {
	t.Helper()
	if os.Getenv("CONTEXIA_PG_HOST") == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through tests/e2e/run or ./scripts/verify")
	}
	ctx := context.Background()
	name := fmt.Sprintf("t057_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, pgURL("contexia_admin", "admin_dev", "contexia"))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	inNew, err := pgx.Connect(ctx, pgURL("contexia_admin", "admin_dev", name))
	if err != nil {
		t.Fatal(err)
	}
	defer inNew.Close(ctx)
	for _, q := range []string{
		"REVOKE ALL ON SCHEMA public FROM PUBLIC",
		"CREATE SCHEMA conversations AUTHORIZATION conversations",
		"CREATE SCHEMA feed AUTHORIZATION feed",
		"CREATE SCHEMA search AUTHORIZATION search",
		"CREATE SCHEMA sender AUTHORIZATION sender",
		"GRANT USAGE ON SCHEMA conversations TO delivery",
		"GRANT USAGE ON SCHEMA feed TO delivery",
	} {
		if _, err := inNew.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	pool, err := pgxpool.New(ctx, pgURL("contexia_admin", "admin_dev", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		c, err := pgx.Connect(ctx, pgURL("contexia_admin", "admin_dev", "contexia"))
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return name, pool
}

func freeAddr(t testing.TB) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func (s *Stack) start(name, bin string, env ...string) *Proc {
	s.T.Helper()
	addr := freeAddr(s.T)
	p := &Proc{Name: name, URL: "http://" + addr, bin: bin, addr: addr, log: &tail{},
		env: append(append(os.Environ(), "CONTEXIA_LISTEN="+addr), env...)}
	s.launch(p)
	s.T.Cleanup(func() { p.Kill() })
	return p
}

func (s *Stack) launch(p *Proc) {
	s.T.Helper()
	p.cmd = exec.Command(p.bin)
	p.cmd.Env = p.env
	p.cmd.Stdout, p.cmd.Stderr = p.log, p.log
	if err := p.cmd.Start(); err != nil {
		s.T.Fatalf("start %s: %v", p.Name, err)
	}
	p.done = make(chan struct{})
	go func(c *exec.Cmd, done chan struct{}) { _ = c.Wait(); close(done) }(p.cmd, p.done)
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := client.Get(p.URL + "/alive")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		select {
		case <-p.done:
			s.T.Fatalf("%s stopped at start:\n%s", p.Name, p.log.String())
		case <-time.After(25 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			s.T.Fatalf("%s did not answer within 30 s:\n%s", p.Name, p.log.String())
		}
	}
}

// Kill ends the program with no chance to finish anything.
func (p *Proc) Kill() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(syscall.SIGKILL)
	<-p.done
}

// Restart starts a killed program again on the same address.
func (s *Stack) Restart(p *Proc) { s.T.Helper(); s.launch(p) }

// Start builds and starts the system.
func Start(t testing.TB, o Options) *Stack {
	t.Helper()
	o.Conversations, o.Delivery, o.Feeds = max(o.Conversations, 1), max(o.Delivery, 1), max(o.Feeds, 1)
	tags := ""
	if o.DeliveryFault != "" {
		tags = "deliveryfaults"
	}
	if o.RuleFault != "" {
		if os.Getenv("CONTEXIA_E2E_FEED") != "standin" {
			t.Skip("a planted fault in the feed's rules needs the stand-in feed: tests/e2e/run standin")
		}
		tags = strings.TrimSpace(tags + " standinfaults")
	}
	bins := binaries(t, tags)
	db, admin := newDatabase(t)
	s := &Stack{T: t, DB: db, Admin: admin}
	t.Cleanup(func() {
		if t.Failed() {
			for _, name := range s.orgNames {
				if len(s.Deliveries) > 0 {
					_, out, _ := Call(http.MethodGet, s.Deliveries[0].URL+"/orgs/"+name+"/status", nil)
					t.Logf("delivery status of %s: %s", name, out)
				}
			}
			t.Logf("handed over: search %d, feed %d, sender %d", s.SearchMeter.Arrivals(), s.FeedMeter.Arrivals(), s.SenderMeter.Arrivals())
			for _, p := range s.all() {
				t.Logf("--- end of %s's log ---\n%s", p.Name, lastLines(p.log.String(), 12))
			}
		}
	})

	// Conversations migrates first: delivery's tables and grants come from it.
	selfAddr := freeAddr(t)
	_ = selfAddr
	s.Search = s.startOther("search", bins, login("search", db)).URL
	s.Sender = s.startOther("sender", bins, login("sender", db)).URL

	var feedURLs []string
	feed := os.Getenv("CONTEXIA_E2E_FEED")
	external := strings.HasPrefix(feed, "http")
	if external {
		feedURLs = []string{feed}
	}

	// Addresses that must exist before the programs that call them start.
	convFront := newLate()
	feedFront := newLate()
	searchMeter, feedMeter, senderMeter := NewMeter(s.Search), NewMeter(""), NewMeter(s.Sender)
	s.SearchMeter, s.FeedMeter, s.SenderMeter = searchMeter, feedMeter, senderMeter
	for _, h := range []*httptest.Server{convFront.srv, feedFront.srv} {
		t.Cleanup(h.Close)
	}
	serve := func(h http.Handler) string {
		srv := httptest.NewServer(h)
		t.Cleanup(srv.Close)
		return srv.URL
	}
	searchURL, feedDeliveryURL, senderURL := serve(searchMeter), serve(feedMeter), serve(senderMeter)
	s.Door, s.Feed = convFront.srv.URL, feedFront.srv.URL

	for i := range o.Conversations {
		env := []string{"CONTEXIA_DATABASE_URL=" + login("conversations", db), "CONTEXIA_FEED_URL=" + s.Feed,
			"CONTEXIA_SEARCH_URL=" + s.Search, "CONTEXIA_CONVERSATIONS_URL=" + s.Door}
		s.Conversations = append(s.Conversations, s.start(fmt.Sprintf("conversations-%d", i+1), filepath.Join(bins, "conversations"), env...))
	}
	if !external {
		bin := filepath.Join(bins, "feed")
		if feed == "standin" {
			bin = filepath.Join(bins, "standinfeed")
		}
		for i := range o.Feeds {
			p := s.start(fmt.Sprintf("feed-%d", i+1), bin, "CONTEXIA_DATABASE_URL="+login("feed", db), "CONTEXIA_E2E_RULE_FAULT="+o.RuleFault)
			if o.RuleFault != "" && !strings.Contains(p.log.String(), "rule fault on: "+o.RuleFault) {
				t.Fatalf("the planted fault %q is not active in the stand-in feed:\n%s", o.RuleFault, p.log.String())
			}
			s.Feeds = append(s.Feeds, p)
			feedURLs = append(feedURLs, p.URL)
		}
	}
	var err error
	if s.FeedRouter, err = router.New(router.Spread, feedURLs...); err != nil {
		t.Fatal(err)
	}
	feedFront.set(s.FeedRouter)
	feedMeter.SetNext(s.FeedRouter)
	var convURLs []string
	for _, p := range s.Conversations {
		convURLs = append(convURLs, p.URL)
	}
	if s.ConversationsRouter, err = router.New(router.ByOrg, convURLs...); err != nil {
		t.Fatal(err)
	}
	convFront.set(s.ConversationsRouter)
	s.requireFeedContract()

	for i := range o.Delivery {
		env := []string{"CONTEXIA_DATABASE_URL=" + login("delivery", db), "CONTEXIA_SEARCH_URL=" + searchURL,
			"CONTEXIA_FEED_URL=" + feedDeliveryURL, "CONTEXIA_SENDER_URL=" + senderURL, "CONTEXIA_CONVERSATIONS_URL=" + s.Door}
		if o.DeliveryFault != "" {
			env = append(env, "CONTEXIA_DELIVERY_FAULT="+o.DeliveryFault)
		}
		s.Deliveries = append(s.Deliveries, s.start(fmt.Sprintf("delivery-%d", i+1), filepath.Join(bins, "delivery"), env...))
	}
	return s
}

func (s *Stack) startOther(name, bins, dbURL string) *Proc {
	s.T.Helper()
	p := s.start(name, filepath.Join(bins, name), "CONTEXIA_DATABASE_URL="+dbURL)
	s.Others = append(s.Others, p)
	return p
}

// requireFeedContract stops at once, with the reason, if the feed being run does
// not answer its boundary at all, which is what the program did before T-052.
func (s *Stack) requireFeedContract() {
	s.T.Helper()
	resp, err := http.Get(s.Feed + "/orgs/contract-probe/place")
	if err != nil {
		s.T.Fatalf("the feed does not answer: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.T.Fatalf("the feed answered %d to GET /orgs/{org}/place. T-052 is not in this tree: merge it, or run on the stand-in with tests/e2e/run standin", resp.StatusCode)
	}
	var place map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&place); err != nil || place["through"] == nil {
		s.T.Fatalf("the feed's place is not as the contract says: %v %v", place, err)
	}
}

func (s *Stack) all() []*Proc {
	out := append([]*Proc{}, s.Conversations...)
	out = append(out, s.Deliveries...)
	out = append(out, s.Feeds...)
	return append(out, s.Others...)
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// late is an address that exists before what it forwards to does.
type late struct {
	srv *httptest.Server
	mu  sync.RWMutex
	h   http.Handler
}

func newLate() *late {
	l := &late{}
	l.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.mu.RLock()
		h := l.h
		l.mu.RUnlock()
		if h == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		h.ServeHTTP(w, r)
	}))
	return l
}

func (l *late) set(h http.Handler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.h = h
}

// Until waits for a condition, polling, and fails the test with what it waited for.
func Until(t testing.TB, limit time.Duration, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("not within %v: %s", limit, what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// failure stands in for a test while a check runs that is expected to fail.
type failure struct {
	testing.TB
	mu  sync.Mutex
	msg string
}

func (f *failure) record(m string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.msg == "" {
		f.msg = m
	}
}

func (f *failure) Helper()                   {}
func (f *failure) Fatal(a ...any)            { f.record(fmt.Sprint(a...)); runtime.Goexit() }
func (f *failure) Fatalf(s string, a ...any) { f.record(fmt.Sprintf(s, a...)); runtime.Goexit() }
func (f *failure) Error(a ...any)            { f.record(fmt.Sprint(a...)) }
func (f *failure) Errorf(s string, a ...any) { f.record(fmt.Sprintf(s, a...)) }
func (f *failure) FailNow()                  { f.record("failed"); runtime.Goexit() }
func (f *failure) Fail()                     { f.record("failed") }

// MustFail runs a check that a planted fault must break, and fails the test if the
// check passes or fails for another reason than the one named.
func MustFail(t *testing.T, because string, check func(testing.TB)) {
	t.Helper()
	f := &failure{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		check(f)
	}()
	<-done
	switch {
	case f.msg == "":
		t.Fatalf("the check passed with the fault planted, so it cannot be trusted (expected: %s)", because)
	case !strings.Contains(f.msg, because):
		t.Fatalf("the check failed for another reason than the planted fault.\nexpected: %s\ngot: %s", because, f.msg)
	}
	t.Logf("seen failing for the right reason: %s", f.msg)
}
