// Package searchequalstruth holds G-006: once level, the index holds the current
// text of every message that is not deleted, and nothing else.
package searchequalstruth

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"testing"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/standins"
	"contexia/tests/guardrails/notification-once/rig"
	"contexia/tests/harness"
)

const quick = 300 * time.Millisecond

// worker plays the search delivery worker: in order, retrying an outage,
// parking what is refused, and going back to the start when the wipe number
// changes.
type worker struct {
	r       *rig.Rig
	base    string
	wipe    int64
	changes []rig.Change
	parked  []rig.Change
}

func (w *worker) deliver(from, to int) {
	w.r.T.Helper()
	for i, tries := from, 0; i < to; {
		a := rig.Do(w.base, "POST", "/changes", w.changes[i].Body, 2*time.Second)
		switch {
		case a.Outage:
			if tries++; tries > 200 {
				w.r.T.Fatalf("change %d: no answer after %d tries", w.changes[i].Number, tries)
			}
		case a.Status == http.StatusUnprocessableEntity:
			w.parked = append(w.parked, w.changes[i])
			w.noteWipe(a.Body, &i)
			i++
		case a.Status == http.StatusOK:
			tries = 0
			if !w.noteWipe(a.Body, &i) {
				i++
			}
		default:
			w.r.T.Fatalf("change %d: status %d: %s", w.changes[i].Number, a.Status, a.Body)
		}
	}
}

// noteWipe sends the worker back to the start when the index was wiped.
func (w *worker) noteWipe(body []byte, i *int) bool {
	var a struct {
		Wipe int64 `json:"wipe"`
	}
	if err := json.Unmarshal(body, &a); err != nil {
		w.r.T.Fatal(err)
	}
	if a.Wipe == w.wipe {
		return false
	}
	w.wipe = a.Wipe
	*i = 0
	return true
}

// scenario runs G-006's sequence and returns every way the index ended up
// different from the truth.
func scenario(t *testing.T, seed int64) []string {
	r := rig.New(t, "search", seed)
	org := rig.NewOrg("g006")
	changes, truth := rig.SearchChanges(org, rig.Log(org, seed, 320))
	w := &worker{r: r, base: r.Front, changes: changes}
	var bad []string

	// Repeats, lost answers and late copies from the start.
	r.TM.Set(harness.Trouble{Repeat: 0.2, LoseAnswer: 0.1, Late: 0.2, LateBy: 5 * time.Millisecond})
	first := len(changes) * 4 / 10
	w.deliver(0, first)

	// The index stops for ten minutes of edits and deletes.
	r.Mode(org, "off")
	if a := rig.Do(r.Front, "POST", "/changes", changes[first].Body, quick); !a.Outage {
		bad = append(bad, fmt.Sprintf("an index that is off answered %d", a.Status))
	}
	r.Clock.Advance(10 * time.Minute)
	r.Mode(org, "running")
	second := len(changes) * 8 / 10
	began := time.Now()
	w.deliver(first, second)
	if took := time.Since(began); took > 30*time.Second {
		bad = append(bad, fmt.Sprintf("catching up took %v, the limit is 30 s", took))
	}

	// One delete is refused and parked; the index is then known not to be level.
	refused := -1
	for i := second; i < len(changes); i++ {
		if changes[i].Delete {
			refused = i
			break
		}
	}
	if refused < 0 {
		t.Fatal("the seeded log has no delete in its last part; pick another seed")
	}
	r.RefuseNumber(changes[refused].Number, true)
	w.deliver(second, len(changes))
	r.TM.Wait()
	if len(rig.Differences(r.Index(org), truth)) == 0 {
		bad = append(bad, "with a delete parked the index still equals the truth: the check cannot see a difference")
	}
	r.RefuseNumber(changes[refused].Number, false)
	for _, c := range w.parked {
		if a := rig.Do(r.Direct, "POST", "/changes", c.Body, 2*time.Second); a.Status != http.StatusOK {
			bad = append(bad, fmt.Sprintf("releasing change %d: status %d", c.Number, a.Status))
		}
	}
	bad = append(bad, rig.Differences(r.Index(org), truth)...)

	// Every change arrives once more, in a random order.
	shuffled := append([]rig.Change(nil), changes...)
	rand.New(rand.NewSource(seed)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	for _, c := range shuffled {
		if a := rig.Do(r.Direct, "POST", "/changes", c.Body, 2*time.Second); a.Status != http.StatusOK {
			bad = append(bad, fmt.Sprintf("late copy of %d: status %d", c.Number, a.Status))
		}
	}
	bad = append(bad, rig.Differences(r.Index(org), truth)...)

	// Wiped: a delivery still in flight sees the new number and the worker refills.
	r.Wipe(org)
	w.deliver(len(changes)-1, len(changes))
	if w.wipe != 1 {
		bad = append(bad, fmt.Sprintf("the worker saw wipe %d, expected 1", w.wipe))
	}
	w.deliver(0, len(changes))
	r.TM.Wait()
	bad = append(bad, rig.Differences(r.Index(org), truth)...)

	var place contract.SearchPlace
	r.Must("GET", "/orgs/"+org+"/place", nil, &place)
	if want := changes[len(changes)-1].Number; place.Through != want {
		bad = append(bad, fmt.Sprintf("through is %d, the last change is %d", place.Through, want))
	}
	return bad
}

func TestSearchEqualsTruth(t *testing.T) {
	for _, bad := range scenario(t, 6) {
		t.Error(bad)
	}
}

func message(org string, n, version int64, id, text string, thread *string, participants []string) map[string]any {
	kind := "message.created"
	edited := false
	if version > 1 {
		kind, edited = "message.changed", true
	}
	return map[string]any{"kind": kind, "org": org, "number": n + version - 1, "message": map[string]any{
		"id": id, "number": n, "created_at": "2026-11-22T09:00:00.000Z", "version": version, "conversation": "#launch", "thread": thread,
		"edited": edited, "participants": participants, "author": "anna", "text": text, "mentions": []string{}, "level": nil}}
}

func remove(org string, n, version int64, id string) map[string]any {
	return map[string]any{"kind": "message.deleted", "org": org, "number": n, "message": map[string]any{"id": id, "version": version}}
}

func search(r *rig.Rig, org, q, as, after string) contract.SearchAnswer {
	r.T.Helper()
	v := url.Values{"q": {q}, "as": {as}}
	if after != "" {
		v.Set("after", after)
	}
	var a contract.SearchAnswer
	r.Must("GET", "/orgs/"+org+"/query?"+v.Encode(), nil, &a)
	return a
}

func TestSearchContract(t *testing.T) {
	r := rig.New(t, "search", 1)
	org := rig.NewOrg("search")
	post := func(body map[string]any) rig.Answer {
		t.Helper()
		return rig.Do(r.Direct, "POST", "/changes", body, 2*time.Second)
	}
	mustApply := func(body map[string]any) {
		t.Helper()
		if a := post(body); a.Status != http.StatusOK {
			t.Fatalf("status %d: %s", a.Status, a.Body)
		}
	}

	t.Run("an older version and a repeat change nothing", func(t *testing.T) {
		mustApply(message(org, 1, 2, "m1", "new words", nil, nil))
		a := post(message(org, 1, 1, "m1", "old words", nil, nil))
		var ans contract.SearchApplied
		_ = json.Unmarshal(a.Body, &ans)
		if ans.Already == nil || !*ans.Already {
			t.Errorf("an older version should be answered as already had: %s", a.Body)
		}
		if got := search(r, org, "words", "lena", ""); got.Total != 1 || got.Results[0].Text != "new words" || !got.Results[0].Edited {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("a late edit cannot bring back a delete", func(t *testing.T) {
		mustApply(message(org, 5, 1, "m2", "gone soon", nil, nil))
		mustApply(remove(org, 8, 3, "m2"))
		mustApply(message(org, 5, 2, "m2", "gone soon edited", nil, nil))
		if got := search(r, org, "gone", "lena", ""); got.Total != 0 {
			t.Errorf("a deleted message was found: %+v", got)
		}
	})
	t.Run("marks count code points, a reply names its thread", func(t *testing.T) {
		root := "m1"
		mustApply(message(org, 10, 1, "m3", "😀 Pricing page", &root, nil))
		got := search(r, org, "pricing", "lena", "")
		if got.Total != 1 || len(got.Results[0].Match) != 1 || got.Results[0].Match[0] != (contract.Span{2, 9}) {
			t.Fatalf("got %+v", got)
		}
		if got.Results[0].Thread == nil || *got.Results[0].Thread != "m1" {
			t.Errorf("thread: %+v", got.Results[0].Thread)
		}
	})
	t.Run("a direct message is found only by its participants", func(t *testing.T) {
		mustApply(message(org, 20, 1, "m4", "private plan", nil, []string{"anna", "marco"}))
		if got := search(r, org, "private", "marco", ""); got.Total != 1 {
			t.Errorf("participant: %+v", got)
		}
		if got := search(r, org, "private", "lena", ""); got.Total != 0 || len(got.Results) != 0 {
			t.Errorf("not a participant: %+v", got)
		}
	})
	t.Run("the total is true across pages and through is the place", func(t *testing.T) {
		for i := int64(0); i < 25; i++ {
			mustApply(message(org, 100+i, 1, fmt.Sprintf("p%d", i), "paged item", nil, nil))
		}
		one := search(r, org, "paged", "lena", "")
		if one.Total != 25 || len(one.Results) != standins.PageSize || one.Next == nil || one.Through != 124 {
			t.Fatalf("first page: total %d, %d shown, through %d", one.Total, len(one.Results), one.Through)
		}
		two := search(r, org, "paged", "lena", *one.Next)
		if two.Total != 25 || len(two.Results) != 5 || two.Next != nil {
			t.Fatalf("second page: total %d, %d shown", two.Total, len(two.Results))
		}
	})
	t.Run("a refusal carries the wipe number", func(t *testing.T) {
		bad := message(org, 300, 1, "x", "text", nil, nil)
		bad["kind"] = "message.unknown"
		a := post(bad)
		var ans contract.SearchRefusal
		if err := json.Unmarshal(a.Body, &ans); err != nil || a.Status != http.StatusUnprocessableEntity || ans.Refused == "" {
			t.Errorf("status %d: %s", a.Status, a.Body)
		}
	})
	t.Run("a wipe touches one organization and raises its number", func(t *testing.T) {
		other := rig.NewOrg("search")
		mustApply(message(other, 1, 1, "o1", "other words", nil, nil))
		var before, after, otherPlace contract.SearchPlace
		r.Must("GET", "/orgs/"+org+"/place", nil, &before)
		r.Wipe(org)
		r.Must("GET", "/orgs/"+org+"/place", nil, &after)
		r.Must("GET", "/orgs/"+other+"/place", nil, &otherPlace)
		if after.Wipe != before.Wipe+1 || after.Through != 0 {
			t.Errorf("after a wipe: %+v (before %+v)", after, before)
		}
		if otherPlace.Wipe != 0 || search(r, other, "other", "lena", "").Total != 1 {
			t.Errorf("the other organization was touched: %+v", otherPlace)
		}
		if search(r, org, "paged", "lena", "").Total != 0 {
			t.Error("the wiped organization still answers")
		}
	})
}

func TestSearchModes(t *testing.T) {
	r := rig.New(t, "search", 2)
	org, other := rig.NewOrg("modes"), rig.NewOrg("modes")
	body := message(org, 1, 1, "m1", "words", nil, nil)
	otherBody := message(other, 1, 1, "m1", "words", nil, nil)

	r.Mode(org, "off")
	if a := rig.Do(r.Direct, "POST", "/changes", body, quick); !a.Outage {
		t.Errorf("off answered %d", a.Status)
	}
	if a := rig.Do(r.Direct, "POST", "/changes", otherBody, 2*time.Second); a.Status != http.StatusOK {
		t.Errorf("another organization should carry on: %d", a.Status)
	}
	var m contract.ModeBody
	r.Must("GET", "/orgs/"+org+"/control/mode", nil, &m)
	if m.Mode != "off" {
		t.Errorf("mode reads %q", m.Mode)
	}

	r.Mode(org, "slow")
	done := make(chan rig.Answer, 1)
	go func() { done <- rig.Do(r.Direct, "POST", "/changes", body, 3*time.Second) }()
	deadline := time.Now().Add(2500 * time.Millisecond)
	for answered := false; !answered; {
		select {
		case a := <-done:
			answered = true
			if a.Status != http.StatusOK {
				t.Errorf("slow should still answer once its time has passed: %+v", a)
			}
		default:
			if time.Now().After(deadline) {
				t.Fatal("slow never answered")
			}
			r.Clock.Advance(standins.SlowBy)
			time.Sleep(5 * time.Millisecond) // lets the request reach its wait
		}
	}

	r.Must("POST", "/control/mode", map[string]string{"mode": "off"}, nil)
	if a := rig.Do(r.Direct, "POST", "/changes", otherBody, quick); !a.Outage {
		t.Errorf("off for all: another organization answered %d", a.Status)
	}
	r.Must("POST", "/control/mode", map[string]string{"mode": "running"}, nil)
	r.Must("GET", "/orgs/"+org+"/control/mode", nil, &m)
	if m.Mode != "running" {
		t.Errorf("mode for all did not replace the organization's: %q", m.Mode)
	}
	r.Must("POST", "/control/wipe", map[string]any{}, nil)
	if got := search(r, other, "words", "lena", ""); got.Total != 0 {
		t.Errorf("wipe for all left %d", got.Total)
	}
}
