package demo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"contexia/internal/democontent"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
)

// door is a fake Conversations that records each action and can fail the next ones.
type door struct {
	mu     sync.Mutex
	ids    []string
	seen   map[string]int
	number int64
	fail   int
}

func (d *door) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/status") { // a status has no ID and is not a step
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return
	}
	body, _ := io.ReadAll(r.Body)
	var head struct {
		Id string `json:"id"`
	}
	_ = json.Unmarshal(body, &head)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[head.Id]++
	if d.fail > 0 {
		d.fail--
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if d.seen[head.Id] == 1 || !containsID(d.ids, head.Id) {
		d.ids = append(d.ids, head.Id)
	}
	d.number++
	w.Header().Set("Content-Type", "application/json")
	if strings.Contains(r.URL.Path, "/agents/") {
		_ = json.NewEncoder(w).Encode(contract.AgentPosted{Message: "msg_" + head.Id, Number: d.number})
		return
	}
	n := d.number
	_ = json.NewEncoder(w).Encode(contract.ActionSaved{Saved: true, Number: &n})
}

func containsID(list []string, id string) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func (d *door) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.ids)
}

func testTeam(t *testing.T) (*team, *door, *clock.Manual) {
	t.Helper()
	content, err := democontent.Load(os.DirFS("../../../demo/content"))
	if err != nil {
		t.Fatal(err)
	}
	d := &door{seen: map[string]int{}}
	srv := httptest.NewServer(d)
	t.Cleanup(srv.Close)
	manual := clock.NewManual(time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC))
	s := New(Config{Clock: manual, Content: content, Conversations: srv.URL, Delivery: srv.URL, Search: srv.URL, Sender: srv.URL, Feed: srv.URL, Gap: time.Second})
	t.Cleanup(s.end)
	return s.teamFor(ids.Org("larkmere-1")), d, manual
}

// until moves the clock in small steps until ok, so a waiting step always sees it.
func until(t *testing.T, c *clock.Manual, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("never happened: %s", what)
		}
		c.Advance(250 * time.Millisecond)
		time.Sleep(2 * time.Millisecond)
	}
}

func TestNothingPlaysUntilStarted(t *testing.T) {
	team, d, c := testTeam(t)
	for range 40 {
		c.Advance(time.Second)
		time.Sleep(time.Millisecond)
	}
	if n := d.count(); n != 0 {
		t.Fatalf("%d actions before start", n)
	}
	if st := team.state(); st.Running || len(st.Next) == 0 || len(st.Situations) != 16 {
		t.Fatalf("state before start: running %v, %d next, %d situations", st.Running, len(st.Next), len(st.Situations))
	}
}

func TestPauseHoldsAndStartResumes(t *testing.T) {
	team, d, c := testTeam(t)
	team.command(contract.ScriptRequest{Do: "start"})
	until(t, c, "first step", func() bool { return d.count() >= 1 })
	team.command(contract.ScriptRequest{Do: "pause"})
	time.Sleep(20 * time.Millisecond) // a step already sent may finish
	held := d.count()
	for range 200 {
		c.Advance(time.Second)
		time.Sleep(200 * time.Microsecond)
	}
	if n := d.count(); n != held {
		t.Fatalf("%d actions while paused, was %d", n, held)
	}
	team.command(contract.ScriptRequest{Do: "start"})
	until(t, c, "resume", func() bool { return d.count() > held })
}

func TestPlayNowWorksWhilePausedAndComesOffTheSchedule(t *testing.T) {
	team, d, c := testTeam(t)
	last := team.s.cfg.Content.Situations[len(team.s.cfg.Content.Situations)-1]
	if why := team.command(contract.ScriptRequest{Do: "play_situation", Situation: &last.ID}); why != "" {
		t.Fatal(why)
	}
	until(t, c, "the situation to finish", func() bool { return d.count() >= len(last.Steps) })
	st := team.state()
	if st.Running {
		t.Fatal("playing one situation started the script")
	}
	if got := len(st.Happened); got != len(last.Steps) {
		t.Fatalf("%d steps happened, want %d", got, len(last.Steps))
	}
	team.mu.Lock()
	defer team.mu.Unlock()
	for _, p := range team.plan {
		if p.sit.ID == last.ID {
			t.Fatal("a situation played on demand is still on the schedule")
		}
	}
}

func TestAStepIsRepeatedWithTheSameIDAfterAnOutage(t *testing.T) {
	team, d, c := testTeam(t)
	d.mu.Lock()
	d.fail = 2
	d.mu.Unlock()
	team.command(contract.ScriptRequest{Do: "start"})
	until(t, c, "the first step to get through", func() bool { return d.count() >= 1 })
	d.mu.Lock()
	defer d.mu.Unlock()
	if tries := d.seen[d.ids[0]]; tries != 3 {
		t.Fatalf("the first action arrived %d times with its ID, want 3", tries)
	}
}

func TestPaceShortensTheWait(t *testing.T) {
	team, d, c := testTeam(t)
	sit := team.s.cfg.Content.Situations[0]
	var wait float64
	for _, st := range sit.Steps {
		wait += st.AfterS
	}
	if wait == 0 {
		t.Skip("the first situation has no delays")
	}
	pace := 4.0
	team.command(contract.ScriptRequest{Do: "pace", Pace: &pace})
	team.command(contract.ScriptRequest{Do: "play_situation", Situation: &sit.ID})
	began := c.Now()
	until(t, c, "the situation to finish", func() bool { return d.count() >= len(sit.Steps) })
	if took := c.Now().Sub(began).Seconds(); took > wait/pace+2 {
		t.Fatalf("took %.1f script seconds at pace 4, the delays add up to %.1f", took, wait)
	}
}

func TestPaceRefusesNonsense(t *testing.T) {
	team, _, _ := testTeam(t)
	bad := 0.0
	if why := team.command(contract.ScriptRequest{Do: "pace", Pace: &bad}); why == "" {
		t.Fatal("a pace of zero was accepted")
	}
}
