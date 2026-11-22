package feedequalsreplay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"contexia/internal/feed/service"
	"contexia/internal/shared/clock"
	"contexia/tests/guardrails/feed-equals-replay/feedrig"
	"contexia/tests/guardrails/notification-once/rig"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// A rebuild made because the rules changed lists what it finds as history and
// sends none of it; what happens afterwards is sent as usual (D-034).
func TestRebuildAfterARuleChangeMarksHistory(t *testing.T) {
	r := feedrig.New(t)
	org := rig.NewOrg("g007h")
	log := feedrig.FromLog(rig.Log(org, 3, 320))
	cut := int64(240)
	rng := rand.New(rand.NewSource(3))
	c := r.Start(3)
	feedrig.Deliver(t, c.URL, log, cut, rng)
	sentBefore := 0
	for _, n := range r.Notes(org, feedrig.Place(t, c.URL, org).Edition) {
		if n.Kind == "send" {
			sentBefore++
		}
	}
	if sentBefore == 0 {
		t.Fatal("the log sent nothing before the rebuild, so this test proves nothing")
	}
	feedrig.Wipe(t, c.URL, org, true)
	feedrig.Deliver(t, c.URL, log, log[len(log)-1].Number, rng)
	edition := feedrig.Place(t, c.URL, org).Edition
	history, live := 0, 0
	truth := feedrig.Fold(t, org, log, cut, nil)
	before := map[string]bool{}
	for _, n := range truth.Notes {
		if n.Kind == "send" {
			before[n.Name] = true
		}
	}
	for _, n := range r.Notes(org, edition) {
		if n.Kind != "send" {
			continue
		}
		if before[n.Name] != n.History {
			t.Errorf("%s: history is %v; it was sent before the rebuild: %v", n.Name, n.History, before[n.Name])
		}
		if n.History {
			history++
		} else {
			live++
		}
	}
	if history != sentBefore || live == 0 {
		t.Fatalf("%d sends marked history (want %d) and %d sent as new (want some)", history, sentBefore, live)
	}
	// The mark survives a restart of the feed, and an ordinary rebuild after it.
	c.Close()
	c = r.Start(4)
	feedrig.Wipe(t, c.URL, org, false)
	feedrig.Deliver(t, c.URL, log, log[len(log)-1].Number, rng)
	marked := 0
	for _, n := range r.Notes(org, feedrig.Place(t, c.URL, org).Edition) {
		if n.Kind == "send" && n.History {
			marked++
		}
	}
	if marked != history {
		t.Fatalf("after an ordinary rebuild %d sends are history, want %d: a name listed as history must never be delivered", marked, history)
	}
}

// Mode, wipe and delete touch only the organization they name.
func TestControlTouchesOneOrganization(t *testing.T) {
	r := feedrig.New(t)
	a, b := rig.NewOrg("g007a"), rig.NewOrg("g007b")
	la, lb := feedrig.FromLog(rig.Log(a, 5, 120)), feedrig.FromLog(rig.Log(b, 5, 120))
	rng := rand.New(rand.NewSource(5))
	c := r.Start(5)
	feedrig.Deliver(t, c.URL, la, la[len(la)-1].Number, rng)
	feedrig.Deliver(t, c.URL, lb, lb[len(lb)-1].Number, rng)
	truthB := feedrig.Fold(t, b, lb, lb[len(lb)-1].Number, nil)

	if got := rig.Do(c.URL, "POST", "/orgs/"+a+"/control/mode", map[string]string{"mode": "off"}, time.Second); got.Status != 200 {
		t.Fatalf("mode off: %+v", got)
	}
	if got := rig.Do(c.URL, "GET", "/orgs/"+a+"/place", nil, 200*time.Millisecond); !got.Outage {
		t.Fatalf("an organization that is off answered: %d", got.Status)
	}
	if p := feedrig.Place(t, c.URL, b); p.Through != lb[len(lb)-1].Number {
		t.Fatalf("the other organization's place is %+v", p)
	}
	var mode struct {
		Mode string `json:"mode"`
	}
	feedrig.Get(t, c.URL, "/orgs/"+a+"/control/mode", &mode)
	if mode.Mode != "off" {
		t.Fatalf("mode reads %q", mode.Mode)
	}
	if got := rig.Do(c.URL, "POST", "/orgs/"+a+"/control/mode", map[string]string{"mode": "running"}, time.Second); got.Status != 200 {
		t.Fatalf("mode running: %+v", got)
	}

	feedrig.Wipe(t, c.URL, a, false)
	if p := feedrig.Place(t, c.URL, a); p.Through != 0 {
		t.Fatalf("after a wipe the place is %+v", p)
	}
	if bad := feedrig.Diff(t, c.URL, b, truthB); len(bad) > 0 {
		t.Fatalf("wiping one organization changed another: %s", bad[0])
	}
	var moved struct {
		NewEdition int64 `json:"new_edition"`
	}
	feedrig.Get(t, c.URL, "/orgs/"+a+"/updates?after=0&edition=1", &moved)
	if moved.NewEdition != 2 {
		t.Fatalf("updates for an old edition answered new_edition %d, want 2", moved.NewEdition)
	}
	if got := rig.Do(c.URL, "DELETE", "/orgs/"+a, nil, 2*time.Second); got.Status != 200 {
		t.Fatalf("delete: %+v", got)
	}
	if p := feedrig.Place(t, c.URL, a); p.Through != 0 || p.Edition != 1 {
		t.Fatalf("a deleted organization answers %+v and must hold nothing", p)
	}
	if bad := feedrig.Diff(t, c.URL, b, truthB); len(bad) > 0 {
		t.Fatalf("deleting one organization changed another: %s", bad[0])
	}
}

// Slow answers only after 10 s by the part's clock; "all" reaches every
// organization.
func TestSlowModeAndAllOrganizations(t *testing.T) {
	r := feedrig.New(t)
	manual := clock.NewManual(rig.Start)
	s := httptest.NewServer(service.New(service.Options{Dir: r.Dir, Clock: manual}).Routes())
	defer s.Close()
	a, b := rig.NewOrg("g007m"), rig.NewOrg("g007n")
	la, lb := feedrig.FromLog(rig.Log(a, 6, 60)), feedrig.FromLog(rig.Log(b, 6, 60))
	rng := rand.New(rand.NewSource(6))
	feedrig.Deliver(t, s.URL, la, la[len(la)-1].Number, rng)
	feedrig.Deliver(t, s.URL, lb, lb[len(lb)-1].Number, rng)

	if got := rig.Do(s.URL, "POST", "/orgs/"+a+"/control/mode", map[string]string{"mode": "slow"}, time.Second); got.Status != 200 {
		t.Fatalf("mode slow: %+v", got)
	}
	answered := make(chan rig.Answer, 1)
	go func() { answered <- rig.Do(s.URL, "GET", "/orgs/"+a+"/place", nil, 20*time.Second) }()
	for _, step := range []time.Duration{0, 9 * time.Second} {
		manual.Advance(step)
		select {
		case got := <-answered:
			t.Fatalf("slow mode answered after %v of its 10 s: %+v", step, got)
		case <-time.After(150 * time.Millisecond):
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for done := false; !done; {
		manual.Advance(time.Second)
		select {
		case got := <-answered:
			if got.Status != 200 {
				t.Fatalf("slow answered %+v", got)
			}
			done = true
		case <-time.After(20 * time.Millisecond):
			if time.Now().After(deadline) {
				t.Fatal("slow mode never answered as the clock moved")
			}
		}
	}
	if got := rig.Do(s.URL, "POST", "/control/mode", map[string]string{"mode": "off"}, time.Second); got.Status != 200 {
		t.Fatalf("mode off for all: %+v", got)
	}
	for _, org := range []string{a, b} {
		if got := rig.Do(s.URL, "GET", "/orgs/"+org+"/place", nil, 150*time.Millisecond); !got.Outage {
			t.Fatalf("%s answered while all are off: %d", org, got.Status)
		}
	}
	if got := rig.Do(s.URL, "POST", "/control/mode", map[string]string{"mode": "running"}, time.Second); got.Status != 200 {
		t.Fatalf("mode running for all: %+v", got)
	}
	if got := rig.Do(s.URL, "POST", "/control/wipe", map[string]any{}, 5*time.Second); got.Status != 200 {
		t.Fatalf("wipe all: %+v", got)
	}
	for _, org := range []string{a, b} {
		if p := feedrig.Place(t, s.URL, org); p.Through != 0 || p.Edition != 2 {
			t.Fatalf("%s after wipe all: %+v", org, p)
		}
	}
}

// Every answer of the door fits the schema both sides are generated from.
func TestAnswersFitTheirSchemas(t *testing.T) {
	f, err := os.Open("../../../contracts/schema/contexia.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	comp := jsonschema.NewCompiler()
	comp.AssertFormat()
	if err := comp.AddResource("contexia.json", doc); err != nil {
		t.Fatal(err)
	}
	r := feedrig.New(t)
	org := rig.NewOrg("g007s")
	log := feedrig.FromLog(rig.Log(org, 9, 200))
	c := r.Start(9)
	feedrig.Deliver(t, c.URL, log, log[len(log)-1].Number, rand.New(rand.NewSource(9)))
	edition := feedrig.Place(t, c.URL, org).Edition
	act, _ := json.Marshal(map[string]any{"kind": "focus_person", "about": "anna", "on": true})
	for path, name := range map[string]string{
		"/orgs/" + org + "/place":                                                       "FeedPlace",
		"/orgs/" + org + "/people/marco/rows":                                           "FeedRowsAnswer",
		"/orgs/" + org + "/people/marco/history?tab=mentions":                           "HistoryAnswer",
		"/orgs/" + org + "/people/marco/history?tab=direct":                             "HistoryAnswer",
		"/orgs/" + org + "/people/marco/history?tab=threads":                            "HistoryAnswer",
		"/orgs/" + org + "/people/anna/history?tab=reactions":                           "HistoryAnswer",
		"/orgs/" + org + "/people/marco/settings":                                       "SettingsAnswer",
		"/orgs/" + org + "/people/marco/preview?change=" + url.QueryEscape(string(act)): "PreviewAnswer",
		fmt.Sprintf("/orgs/%s/updates?after=0&edition=%d", org, edition):                "RowUpdatesPage",
		fmt.Sprintf("/orgs/%s/updates?after=0&edition=%d", org, edition+1):              "NewEdition",
		"/orgs/" + org + "/control/mode":                                                "ModeBody",
	} {
		s, err := comp.Compile("contexia.json#/$defs/" + name)
		if err != nil {
			t.Fatal(err)
		}
		a := rig.Do(c.URL, "GET", path, nil, 5*time.Second)
		if a.Outage || a.Status != 200 {
			t.Fatalf("GET %s: %+v", path, a)
		}
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(a.Body))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(v); err != nil {
			t.Errorf("%s does not fit %s: %v\n%.300s", strings.SplitN(path, "?", 2)[0], name, err, a.Body)
		}
	}
}
