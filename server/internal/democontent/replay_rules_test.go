package democontent

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"contexia/internal/feed/rules"
	"contexia/internal/shared/contract"
)

// whereNow says where the row holding a message stands for a person, in the
// words of the situations' expectations.
func whereNow(e *rules.Engine, person, id string) string {
	for _, r := range e.State(person).Rows {
		// The row holding a message is its thread's row, also when the row's
		// items are the replies; a retracted item still marks its row (D-004).
		holds := r.Key == "th:"+id
		for _, it := range r.Items {
			holds = holds || it.Msg == id
		}
		if !holds {
			continue
		}
		switch r.Where {
		case "Focus/new":
			return "focus_new"
		case "Focus/kept":
			return "focus_kept"
		case "Needs you":
			return "needs_you"
		case "FYI":
			return "fyi"
		case "Earlier":
			return "history"
		}
		return r.Where
	}
	return "nothing"
}

// Every situation of D-010 names the outcome it should produce; the real rules
// must produce it, on top of the real seeded history.
func TestSituationsProduceTheirOutcomesInTheRealRules(t *testing.T) {
	c, err := Load(os.DirFS("../../../demo/content"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := c.Build(Options{})
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for i, s := range c.Situations {
		t.Run(s.ID, func(t *testing.T) {
			e := rules.New("larkmere")
			for _, ch := range h.Changes {
				fx, err := e.Apply(ch, rules.Options{})
				if err != nil {
					t.Fatalf("seeded history does not apply: %v", err)
				}
				if len(fx.Notes) != 0 {
					t.Fatalf("seeded history wrote notes: %v", fx.Notes)
				}
			}
			// As sealing the session does.
			through := h.Through + 1
			seal := contract.ChangeSeedSealed{Kind: "seed.sealed", Org: c.Org.Org, Number: through, Previous: h.Through,
				At: h.Last.UTC().Format("2006-01-02T15:04:05.000Z")}
			if _, err := e.Apply(contract.Change{Value: seal}, rules.Options{}); err != nil {
				t.Fatal(err)
			}
			prefix := fmt.Sprintf("replay%d_", i)
			applied := 0
			ids := map[string]string{}
			for _, st := range s.Before {
				if st.Do == "post" {
					ids[st.As] = BeforeID(s.ID, st.As)
				}
			}
			for n, st := range s.Steps {
				partial := s
				partial.Steps = s.Steps[:n+1]
				changes, err := c.Compile(partial, through, h.Last, prefix)
				if err != nil {
					t.Fatal(err)
				}
				for _, ch := range changes[applied:] {
					if _, err := e.Apply(ch, rules.Options{}); err != nil {
						t.Fatalf("step %d does not apply: %v", n, err)
					}
					if m, ok := ch.Value.(contract.ChangeMessageCreated); ok && st.As != "" {
						ids[st.As] = m.Message.Id
					}
				}
				applied = len(changes)
				for _, x := range st.Expect {
					id, ok := ids[x.Of]
					if !ok {
						t.Fatalf("step %d expects %q, which no step created", n, x.Of)
					}
					checked++
					if got := whereNow(e, x.For, id); got != x.Where {
						t.Errorf("step %d (%s by %s): for %s, %q is in %s, expected %s. %s",
							n, st.Do, st.Who, x.For, x.Of, got, x.Where, strings.TrimSpace(x.Why))
					}
				}
			}
		})
	}
	if !t.Failed() && checked == 0 {
		t.Fatal("no expectation was checked")
	}
}
