package democontent

import (
	"fmt"
	"slices"
	"time"

	"contexia/internal/shared/contract"
)

// Where a row can stand for a person after a step.
var whereValues = []string{"focus_new", "focus_kept", "needs_you", "fyi", "history", "nothing"}

// Expect names a feed outcome: for this person, the row holding this message is here.
type Expect struct {
	For   string `json:"for"`
	Of    string `json:"of"`
	Where string `json:"where"`
	Why   string `json:"why"`
}

// Step is one act of a situation, a delay after the previous one.
type Step struct {
	AfterS float64  `json:"after_s"`
	Who    string   `json:"who"`
	Do     string   `json:"do"` // post reply edit delete react unreact read keep done dial
	As     string   `json:"as,omitempty"`
	In     string   `json:"in,omitempty"`
	To     string   `json:"to,omitempty"`
	Text   string   `json:"text,omitempty"`
	Level  string   `json:"level,omitempty"`
	Emoji  string   `json:"emoji,omitempty"`
	Dial   string   `json:"dial,omitempty"`
	Expect []Expect `json:"expect,omitempty"`
}

// Situation is a replayable script from D-010 and the outcome it should produce.
type Situation struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Outcome string `json:"outcome"`
	Covers  string `json:"covers"`
	Viewer  string `json:"viewer"`
	Steps   []Step `json:"steps"`
}

func (c *Content) checkSituation(s *Situation) error {
	if s.Title == "" || s.Outcome == "" || s.Covers == "" {
		return fmt.Errorf("needs a title, an outcome and the line of D-010 it covers")
	}
	if _, ok := c.people[s.Viewer]; !ok {
		return fmt.Errorf("unknown viewer %q", s.Viewer)
	}
	seen := map[string]bool{}
	expects := 0
	for i, st := range s.Steps {
		if _, ok := c.people[st.Who]; !ok {
			return fmt.Errorf("step %d: unknown person %q", i, st.Who)
		}
		needsTo := slices.Contains([]string{"reply", "edit", "delete", "react", "unreact", "read", "keep", "done"}, st.Do)
		switch {
		case st.Do == "post" || st.Do == "dial":
			if c.members[st.In] == nil {
				return fmt.Errorf("step %d: no conversation %q", i, st.In)
			}
			if st.To != "" && !seen[st.To] {
				return fmt.Errorf("step %d: %q is not an earlier message", i, st.To)
			}
		case !needsTo:
			return fmt.Errorf("step %d: unknown act %q", i, st.Do)
		case !seen[st.To]:
			return fmt.Errorf("step %d: %q is not an earlier message", i, st.To)
		}
		if st.As != "" {
			seen[st.As] = true
		}
		for _, e := range st.Expect {
			expects++
			if _, ok := c.people[e.For]; !ok {
				return fmt.Errorf("step %d: expects for unknown person %q", i, e.For)
			}
			if !seen[e.Of] {
				return fmt.Errorf("step %d: expects about unknown message %q", i, e.Of)
			}
			if !slices.Contains(whereValues, e.Where) {
				return fmt.Errorf("step %d: unknown outcome %q", i, e.Where)
			}
			if e.Why == "" {
				return fmt.Errorf("step %d: an outcome says why, citing the decision", i)
			}
		}
	}
	if expects == 0 {
		return fmt.Errorf("names no feed outcome")
	}
	return nil
}

// Compile turns a situation into the changes it writes when played after the given
// number and time. Each play gets its own message IDs through idPrefix.
func (c *Content) Compile(s Situation, afterNumber int64, at time.Time, idPrefix string) ([]contract.Change, error) {
	w := newWriter(c, afterNumber, at, idPrefix)
	byAlias := map[string]*msg{}
	now := at
	for i, st := range s.Steps {
		now = now.Add(time.Duration(st.AfterS * float64(time.Second)))
		target := byAlias[st.To]
		var m *msg
		var err error
		switch st.Do {
		case "post":
			m, err = w.post(now, st.Who, st.In, nil, st.Text, st.Level)
		case "reply":
			root := target
			if target.root != nil {
				root = target.root
			}
			m, err = w.post(now, st.Who, target.conv, root, st.Text, st.Level)
		case "edit":
			level := st.Level
			if level == "" {
				level = "same"
			}
			err = w.edit(now, target, st.Text, level)
		case "delete":
			w.remove(now, target)
		case "react":
			w.react(now, st.Who, target, st.Emoji)
		case "unreact":
			w.unreact(now, st.Who, target, st.Emoji)
		case "read":
			row := w.keepRow(target)
			w.read(now, st.Who, target.conv, row, w.newest(target.conv, row))
		case "keep":
			w.keep(now, st.Who, target)
		case "done":
			w.done(now, st.Who, target)
		case "dial":
			var thread *string
			if target != nil {
				thread = &target.id
			}
			w.dial(now, st.Who, st.In, thread, st.Dial)
		}
		if err != nil {
			return nil, fmt.Errorf("%s step %d: %w", s.ID, i, err)
		}
		if st.As != "" && m != nil {
			byAlias[st.As] = m
		}
	}
	return w.out, nil
}
