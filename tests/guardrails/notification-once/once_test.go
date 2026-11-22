// Package notificationonce holds G-010: every notification that is due and not
// withdrawn within its hold is sent exactly once.
package notificationonce

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"contexia/internal/shared/contract"
	"contexia/tests/guardrails/notification-once/rig"
	"contexia/tests/harness"
)

// scenario delivers a seeded burst through the troublemaker, with the sender
// off for 5 and for 11 minutes, wiped, and replayed under a new edition.
func scenario(t *testing.T, seed int64) []string {
	r := rig.New(t, "sender", seed)
	org := rig.NewOrg("g010")
	log := rig.Log(org, seed, 320)
	notes := rig.Notes(org, 1, log)
	want := rig.DueToSend(notes)
	if len(want) < 10 {
		t.Fatalf("the seeded log has only %d notifications; pick another seed", len(want))
	}
	r.TM.Set(harness.Trouble{Repeat: 0.2, LoseAnswer: 0.1, Late: 0.2, LateBy: 5 * time.Millisecond})

	a, b := len(notes)/4, len(notes)/2
	rig.DeliverNotes(t, r.Front, notes[:a])

	r.Mode(org, "off")
	if got := rig.Do(r.Front, "POST", "/notes", notes[a].Body, 200*time.Millisecond); !got.Outage {
		t.Errorf("a sender that is off answered %d", got.Status)
	}
	r.Clock.Advance(5 * time.Minute)
	r.Mode(org, "running")
	rig.DeliverNotes(t, r.Front, notes[a:b])

	r.Mode(org, "off")
	r.Clock.Advance(11 * time.Minute)
	r.Mode(org, "running")

	// Wiped mid-burst: refilled from the start of the log.
	r.Wipe(org)
	rig.DeliverNotes(t, r.Front, notes)

	// The feed is rebuilt: the same notes under a new edition.
	rig.DeliverNotes(t, r.Front, rig.Notes(org, 2, log))
	r.TM.Wait()
	return rig.OnceEach(rig.Deliveries(t, r.Dir, org), want)
}

func TestSentExactlyOnce(t *testing.T) {
	for _, bad := range scenario(t, 10) {
		t.Error(bad)
	}
}

func list(r *rig.Rig, org, person string) map[string]contract.NotificationEntry {
	r.T.Helper()
	var l contract.NotificationList
	r.Must("GET", "/orgs/"+org+"/notifications?person="+person, nil, &l)
	out := map[string]contract.NotificationEntry{}
	for _, e := range l.Notifications {
		out[e.Name] = e
	}
	return out
}

func TestDelayedMarkAndTheListSurvivesAWipe(t *testing.T) {
	r := rig.New(t, "sender", 1)
	org := rig.NewOrg("delayed")
	b := &rig.Builder{Org: org, Edition: 1}
	t0 := rig.Start
	onTime := b.Due("marco", "#a", 1, t0)
	late := b.Due("marco", "#b", 2, t0.Add(time.Second))
	b.Add("clock", "", t0.Add(5*time.Second+time.Millisecond), nil) // past the first hold only
	rig.DeliverNotes(t, r.Direct, b.Notes)

	r.Clock.Advance(10*time.Minute + 2*time.Second)
	more := &rig.Builder{Org: org, Edition: 1, Notes: b.Notes}
	more.Add("clock", "", t0.Add(7*time.Second), nil)
	rig.DeliverNotes(t, r.Direct, more.Notes)

	got := list(r, org, "marco")
	if e := got[onTime]; e.State != "sent" || e.Delayed || e.At == nil {
		t.Errorf("on time: %+v", e)
	}
	if e := got[late]; e.State != "sent" || !e.Delayed || e.At == nil {
		t.Errorf("late: %+v", e)
	}
	sentAt := *got[late].At

	r.Wipe(org)
	var place contract.SenderPlace
	r.Must("GET", "/orgs/"+org+"/place", nil, &place)
	if place.Through != 0 {
		t.Errorf("a wiped sender answers with its start, got %d", place.Through)
	}
	after := list(r, org, "marco")
	if e := after[late]; e.State != "sent" || !e.Delayed || e.At == nil || *e.At != sentAt {
		t.Errorf("the list of what was sent did not survive the wipe: %+v", e)
	}
	rig.DeliverNotes(t, r.Direct, more.Notes)
	if n := rig.Deliveries(t, r.Dir, org)[late]; n != 1 {
		t.Errorf("sent %d times after the refill", n)
	}
}

func TestEditions(t *testing.T) {
	r := rig.New(t, "sender", 2)
	org := rig.NewOrg("edition")
	first := &rig.Builder{Org: org, Edition: 3}
	dropped := first.Due("marco", "#a", 1, rig.Start)
	rig.DeliverNotes(t, r.Direct, first.Notes)

	second := &rig.Builder{Org: org, Edition: 4}
	second.Add("clock", "", rig.Start.Add(time.Minute), nil)
	rig.DeliverNotes(t, r.Direct, second.Notes)
	if _, kept := list(r, org, "marco")[dropped]; kept {
		t.Error("a higher edition must drop what is on hold")
	}
	if n := rig.Deliveries(t, r.Dir, org)[dropped]; n != 0 {
		t.Errorf("a dropped notification was sent %d times", n)
	}
	var place contract.SenderPlace
	r.Must("GET", "/orgs/"+org+"/place", nil, &place)
	if place.Edition != 4 || place.Through != 1 {
		t.Errorf("place after a new edition: %+v", place)
	}

	a := rig.Do(r.Direct, "POST", "/notes", first.Notes[0].Body, time.Second)
	var behind contract.NoteBehind
	_ = json.Unmarshal(a.Body, &behind)
	if a.Status != http.StatusConflict || behind.Edition != 4 || behind.IAmAt != 1 {
		t.Errorf("a lower edition: status %d: %s", a.Status, a.Body)
	}
}

func TestGapStaleResetRefusalAndOrganizations(t *testing.T) {
	r := rig.New(t, "sender", 3)
	org, other := rig.NewOrg("gaps"), rig.NewOrg("gaps")
	b := &rig.Builder{Org: org, Edition: 1}
	name := b.Due("marco", "#a", 1, rig.Start)
	b.Add("clock", "", rig.Start.Add(time.Second), nil)
	b.Add("clock", "", rig.Start.Add(2*time.Second), nil)

	a := rig.Do(r.Direct, "POST", "/notes", b.Notes[2].Body, time.Second)
	var behind contract.NoteBehind
	_ = json.Unmarshal(a.Body, &behind)
	if a.Status != http.StatusConflict || behind.IAmAt != 0 {
		t.Errorf("a gap: status %d: %s", a.Status, a.Body)
	}

	r.RefuseNumber(1, true)
	if a := rig.Do(r.Direct, "POST", "/notes", b.Notes[0].Body, time.Second); a.Status != http.StatusUnprocessableEntity {
		t.Errorf("a refused note: status %d", a.Status)
	}
	r.RefuseNumber(1, false)
	rig.DeliverNotes(t, r.Direct, b.Notes)

	repeat := rig.Do(r.Direct, "POST", "/notes", b.Notes[0].Body, time.Second)
	var taken contract.NoteTaken
	_ = json.Unmarshal(repeat.Body, &taken)
	if repeat.Status != http.StatusOK || taken.Already == nil || !*taken.Already {
		t.Errorf("a repeat: status %d: %s", repeat.Status, repeat.Body)
	}

	stale := map[string]any{}
	for k, v := range b.Notes[0].Body {
		stale[k] = v
	}
	newer := &rig.Builder{Org: org, Edition: 1, Notes: b.Notes}
	newer.Add("clock", "", rig.Start.Add(3*time.Second), map[string]any{"reset": 5})
	rig.DeliverNotes(t, r.Direct, newer.Notes[3:])
	a = rig.Do(r.Direct, "POST", "/notes", stale, time.Second)
	var sr contract.StaleReset
	_ = json.Unmarshal(a.Body, &sr)
	if a.Status != http.StatusConflict || sr.StaleReset != 5 {
		t.Errorf("a stale reset: status %d: %s", a.Status, a.Body)
	}

	foreign := map[string]any{}
	for k, v := range b.Notes[0].Body {
		foreign[k] = v
	}
	foreign["org"] = other
	if a := rig.Do(r.Direct, "POST", "/notes", foreign, time.Second); a.Status != http.StatusUnprocessableEntity {
		t.Errorf("a name of another organization: status %d", a.Status)
	}
	if got := list(r, other, "marco"); len(got) != 0 {
		t.Errorf("another organization's list: %+v", got)
	}
	if _, ok := list(r, org, "lena")[name]; ok {
		t.Error("the list shows another person's notification")
	}
}

func TestRebuildAfterARuleChangeListsHistory(t *testing.T) {
	r := rig.New(t, "sender", 4)
	org := rig.NewOrg("rules")
	b := &rig.Builder{Org: org, Edition: 2}
	name := b.Due("marco", "#a", 1, rig.Start)
	b.Add("withdrawn", name, rig.Start, map[string]any{"because": "rule_changed"})
	b.Add("clock", "", rig.Start.Add(time.Minute), nil)
	rig.DeliverNotes(t, r.Direct, b.Notes)
	if e := list(r, org, "marco")[name]; e.State != "history" {
		t.Errorf("listed as %q, want history", e.State)
	}
	if n := rig.Deliveries(t, r.Dir, org)[name]; n != 0 {
		t.Errorf("history was sent %d times", n)
	}
}

func TestEntryShowsTheCurrentTruth(t *testing.T) {
	r := rig.New(t, "sender", 5)
	org := rig.NewOrg("truth")
	b := &rig.Builder{Org: org, Edition: 1}
	edited := b.Due("marco", "#a", 1, rig.Start)
	read := b.Due("marco", "#b", 2, rig.Start)
	gone := b.Due("marco", "#c", 3, rig.Start)
	after := rig.Start.Add(6 * time.Second)
	b.Add("clock", "", after, nil)
	b.Add("changed", edited, after, map[string]any{"text": "new text"})
	b.Add("withdrawn", read, after, map[string]any{"because": "read"})
	b.Add("withdrawn", gone, after, map[string]any{"because": "deleted"})
	rig.DeliverNotes(t, r.Direct, b.Notes)
	got := list(r, org, "marco")
	if got[edited].Text != "new text" {
		t.Errorf("edited: %+v", got[edited])
	}
	if e := got[read]; e.State != "sent" || e.Now == nil || *e.Now != "read" {
		t.Errorf("read: %+v", e)
	}
	if e := got[gone]; e.State != "sent" || e.Now == nil || *e.Now != "deleted" || e.Text != "" {
		t.Errorf("deleted: %+v", e)
	}
}

func TestSenderModes(t *testing.T) {
	r := rig.New(t, "sender", 6)
	org, other := rig.NewOrg("modes"), rig.NewOrg("modes")
	b := &rig.Builder{Org: org, Edition: 1}
	b.Add("clock", "", rig.Start, nil)
	c := &rig.Builder{Org: other, Edition: 1}
	c.Add("clock", "", rig.Start, nil)

	r.Mode(org, "off")
	if a := rig.Do(r.Direct, "POST", "/notes", b.Notes[0].Body, 200*time.Millisecond); !a.Outage {
		t.Errorf("off answered %d", a.Status)
	}
	rig.DeliverNotes(t, r.Direct, c.Notes)
	r.Must("POST", "/control/mode", map[string]string{"mode": "off"}, nil)
	if a := rig.Do(r.Direct, "POST", "/notes", c.Notes[0].Body, 200*time.Millisecond); !a.Outage {
		t.Errorf("off for all: answered %d", a.Status)
	}
	r.Must("POST", "/control/mode", map[string]string{"mode": "running"}, nil)
	r.Must("POST", "/control/wipe", map[string]any{}, nil)
	var place contract.SenderPlace
	r.Must("GET", "/orgs/"+other+"/place", nil, &place)
	if place.Through != 0 {
		t.Errorf("wipe for all left place %d", place.Through)
	}
}

// The real program, killed mid-burst: each name still goes out once.
func TestKilledMidBurst(t *testing.T) {
	if os.Getenv("CONTEXIA_PROJECT") == "" {
		t.Skip("needs the composed system: ./scripts/verify runs this")
	}
	sys := harness.Connect(t)
	dir := rig.DB(t, "sender")
	org := rig.NewOrg("killed")
	notes := rig.Notes(org, 1, rig.Log(org, 11, 1200))
	want := rig.DueToSend(notes)
	base := sys.URL("sender")

	done := make(chan struct{})
	go func() {
		defer close(done)
		rig.DeliverNotes(t, base, notes)
	}()
	for _, share := range []int{5, 3} {
		target := int64(len(notes) / share)
		deadline := time.Now().Add(30 * time.Second)
		for {
			a := rig.Do(base, "GET", "/orgs/"+org+"/place", nil, time.Second)
			var p contract.SenderPlace
			if !a.Outage && json.Unmarshal(a.Body, &p) == nil && p.Through >= target {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the burst did not reach note %d", target)
			}
			time.Sleep(2 * time.Millisecond)
		}
		sys.Kill("sender")
		sys.Start("sender")
		var p contract.SenderPlace
		if a := rig.Do(base, "GET", "/orgs/"+org+"/place", nil, 2*time.Second); a.Outage || json.Unmarshal(a.Body, &p) != nil {
			t.Fatalf("the restarted sender does not answer its place: %+v", a)
		}
		if p.Through >= int64(len(notes)) {
			t.Fatal("the burst finished before the kill, so this run proved nothing. Lengthen the log")
		}
	}
	<-done
	for _, bad := range rig.OnceEach(rig.Deliveries(t, dir, org), want) {
		t.Error(bad)
	}
}
