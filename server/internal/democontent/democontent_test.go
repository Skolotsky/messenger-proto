package democontent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"contexia/internal/shared/contract"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const contentDir = "../../../demo/content"

func load(t *testing.T) *Content {
	t.Helper()
	c, err := Load(os.DirFS(contentDir))
	if err != nil {
		t.Fatalf("demo/content does not load: %v", err)
	}
	return c
}

func build(t *testing.T, c *Content, o Options) *History {
	t.Helper()
	h, err := c.Build(o)
	if err != nil {
		t.Fatalf("building the history: %v", err)
	}
	return h
}

func changeSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open(filepath.Join("../../../contracts/schema/contexia.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := jsonschema.NewCompiler()
	sc.AssertFormat()
	if err := sc.AddResource("contexia.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := sc.Compile("contexia.json#/$defs/Change")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSameSeedSameBytes(t *testing.T) {
	a, err := build(t, load(t), Options{}).JSONL()
	if err != nil {
		t.Fatal(err)
	}
	b, err := build(t, load(t), Options{}).JSONL()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two builds with the same seed differ: something iterates a map or reads the clock")
	}
	other, err := build(t, load(t), Options{Seed: 7}).JSONL()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, other) {
		t.Fatal("another seed gave the same history: the seed is not used")
	}
	sum := sha256.Sum256(a)
	got := hex.EncodeToString(sum[:])
	if want := pinnedHistory; got != want {
		t.Fatalf("the default history changed: %s, pinned %s. If the content was changed on purpose, update pinnedHistory; otherwise a generator change broke reproducibility", got, want)
	}
}

// pinnedHistory is the SHA-256 of the default history's JSONL.
const pinnedHistory = "3911537f6d248f594d342d94ae3d9fd855e4a4a5d472b1695b2b8045ea0f7734"

func TestHistoryIsAValidLog(t *testing.T) {
	c := load(t)
	h := build(t, c, Options{})
	schema := changeSchema(t)
	checkLog(t, c, h.Changes, schema, 0)
}

// checkLog validates every change and the rules of a seed session.
func checkLog(t *testing.T, c *Content, changes []contract.Change, schema *jsonschema.Schema, after int64) {
	t.Helper()
	var last time.Time
	next := after + 1
	created := map[string]int64{}
	version := map[string]int64{}
	deleted := map[string]bool{}
	for i, ch := range changes {
		b, err := json.Marshal(ch)
		if err != nil {
			t.Fatal(err)
		}
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); err != nil {
			t.Fatalf("change %d does not fit the Change schema: %v\n%s", i, err, b)
		}
		var head struct {
			Kind     string `json:"kind"`
			Org      string `json:"org"`
			Number   int64  `json:"number"`
			Previous int64  `json:"previous"`
			At       string `json:"at"`
			Message  any    `json:"message"`
		}
		if err := json.Unmarshal(b, &head); err != nil {
			t.Fatal(err)
		}
		if head.Org != c.Org.Org {
			t.Fatalf("change %d belongs to %q", i, head.Org)
		}
		if head.Number != next || head.Previous != next-1 {
			t.Fatalf("numbering broken at change %d: number %d previous %d, wanted %d after %d", i, head.Number, head.Previous, next, next-1)
		}
		next++
		at, err := time.Parse(time.RFC3339Nano, head.At)
		if err != nil {
			t.Fatal(err)
		}
		if at.Before(last) {
			t.Fatalf("time goes backwards at number %d: %s after %s", head.Number, at, last)
		}
		last = at
		switch x := ch.Value.(type) {
		case contract.ChangeMessageCreated:
			if x.Message.Number != x.Number || x.Message.Version != 1 {
				t.Fatalf("message at %d has number %d version %d", x.Number, x.Message.Number, x.Message.Version)
			}
			if x.Message.Thread != nil {
				if _, ok := created[*x.Message.Thread]; !ok {
					t.Fatalf("reply %s before its root %s", x.Message.Id, *x.Message.Thread)
				}
			}
			created[x.Message.Id], version[x.Message.Id] = x.Number, 1
		case contract.ChangeMessageChanged:
			if created[x.Message.Id] != x.Message.Number || x.Message.Version != version[x.Message.Id]+1 || deleted[x.Message.Id] {
				t.Fatalf("edit of %s at %d is out of step", x.Message.Id, x.Number)
			}
			version[x.Message.Id] = x.Message.Version
		case contract.ChangeMessageDeleted:
			if x.Message.Version != version[x.Message.Id]+1 || deleted[x.Message.Id] {
				t.Fatalf("delete of %s at %d is out of step", x.Message.Id, x.Number)
			}
			version[x.Message.Id], deleted[x.Message.Id] = x.Message.Version, true
		case contract.ChangeReactionAdded:
			if _, ok := created[x.Message]; !ok || deleted[x.Message] {
				t.Fatalf("reaction at %d on a missing or deleted message", x.Number)
			}
		case contract.ChangeReaderRead:
			if x.UpTo <= 0 || x.UpTo > x.Number {
				t.Fatalf("read at %d up to %d", x.Number, x.UpTo)
			}
		}
	}
}

func TestCastAndChannels(t *testing.T) {
	c := load(t)
	people, agents := 0, 0
	for _, p := range c.Org.People {
		if p.Agent {
			agents++
		} else {
			people++
		}
	}
	if people != 30 || agents != 6 || len(c.Org.Channels) != 20 {
		t.Fatalf("D-010 asks for about 30 people, 6 agents, 20 channels; found %d, %d, %d", people, agents, len(c.Org.Channels))
	}
	if !slices.Contains(c.Org.Core, c.Org.DefaultViewer) {
		t.Fatal("the default viewer must be in the core cast (D-010)")
	}
}

// d010 is the list in D-010, one entry per situation it asks for.
var d010 = map[string]string{
	"a flag that names you, from a person":                          "flag-names-you-from-a-person",
	"a flag that names you, from an agent waiting for approval":     "flag-names-you-from-an-agent",
	"a flag that names nobody and closes when someone else answers": "flag-names-nobody",
	"a direct mention":                "direct-mention",
	"a mention added by a later edit": "mention-added-by-edit",
	"a mention removed by an edit":    "mention-removed-by-edit",
	"a message that reached your feed and is then edited to a different length":        "edited-to-another-length",
	"a message that reached your feed and is then deleted":                             "deleted-after-reaching-the-feed",
	"a pile of reactions on your message":                                              "pile-of-reactions",
	"a reaction taken back":                                                            "reaction-taken-back",
	"replies in a thread you started":                                                  "replies-in-a-thread-you-started",
	"replies in a thread you follow":                                                   "replies-in-a-thread-you-follow",
	"replies in a thread you turned off":                                               "replies-in-a-thread-you-turned-off",
	"a burst in the incidents channel, seen with the dial at Focus, Default and Muted": "incident-burst-at-three-dials",
	"an agent's progress updates as Info, its result, and its stop for a human":        "agent-progress-result-and-stop",
	"a courtesy mention marked Info":                                                   "courtesy-mention-info",
}

func TestEverySituationOfD010Exists(t *testing.T) {
	c := load(t)
	byID := map[string]Situation{}
	for _, s := range c.Situations {
		byID[s.ID] = s
	}
	for line, id := range d010 {
		s, ok := byID[id]
		if !ok {
			t.Errorf("D-010 asks for %q: no situation %q in demo/content/situations.json", line, id)
			continue
		}
		if !strings.HasPrefix(s.Covers, line) {
			t.Errorf("situation %s says it covers %q, D-010's line is %q", id, s.Covers, line)
		}
		named := 0
		for _, st := range s.Steps {
			named += len(st.Expect)
		}
		if named == 0 || s.Outcome == "" {
			t.Errorf("situation %s names no feed outcome", id)
		}
	}
}

func TestSituationsCompileToValidChanges(t *testing.T) {
	c := load(t)
	h := build(t, c, Options{})
	schema := changeSchema(t)
	for _, s := range c.Situations {
		out, err := c.Compile(s, h.Through, h.Last.Add(time.Minute), "p1")
		if err != nil {
			t.Fatal(err)
		}
		if len(out) == 0 {
			t.Fatalf("situation %s writes nothing", s.ID)
		}
		// Whole: a situation may build on what stands in the seeded history (D-038).
		checkLog(t, c, append(slices.Clip(h.Changes), out...), schema, 0)
	}
}

type stats struct {
	messages, replies, mentions, flags, infos, edits, deletes, reactions, removed, byAgent int
	texts                                                                                  []string
	perConversation                                                                        map[string]int
}

func measure(h *History) stats {
	s := stats{perConversation: map[string]int{}}
	for _, ch := range h.Changes {
		switch x := ch.Value.(type) {
		case contract.ChangeMessageCreated:
			m := x.Message
			s.messages++
			s.perConversation[m.Conversation]++
			s.texts = append(s.texts, m.Text)
			if m.Thread != nil {
				s.replies++
			}
			if len(m.Mentions) > 0 {
				s.mentions++
			}
			if m.Level != nil && *m.Level == "flag" {
				s.flags++
			}
			if m.Level != nil && *m.Level == "info" {
				s.infos++
			}
			if m.AuthorIsAgent {
				s.byAgent++
			}
		case contract.ChangeMessageChanged:
			s.edits++
		case contract.ChangeMessageDeleted:
			s.deletes++
		case contract.ChangeReactionAdded:
			s.reactions++
		case contract.ChangeReactionRemoved:
			s.removed++
		}
	}
	return s
}

func TestProportions(t *testing.T) {
	c := load(t)
	for _, seed := range []uint64{0, 7, 99} {
		s := measure(build(t, c, Options{Seed: seed}))
		t.Logf("seed %d: %+v", seed, struct{ M, R, Men, F, I, E, D, Rx, RxOff, Ag int }{s.messages, s.replies, s.mentions, s.flags, s.infos, s.edits, s.deletes, s.reactions, s.removed, s.byAgent})
		share := func(n int) float64 { return float64(n) / float64(s.messages) }
		for _, r := range []struct {
			name     string
			got      float64
			low, top float64
		}{
			{"replies", share(s.replies), 0.30, 0.60},
			{"messages with a mention", share(s.mentions), 0.06, 0.30},
			{"flags", share(s.flags), 0.01, 0.07},
			{"Info", share(s.infos), 0.10, 0.40},
			{"edits", share(s.edits), 0.015, 0.08},
			{"deletes", share(s.deletes), 0.002, 0.03},
			{"reactions per message", share(s.reactions), 0.25, 1.6},
			{"written by agents", share(s.byAgent), 0.20, 0.55},
		} {
			if r.got < r.low || r.got > r.top {
				t.Errorf("seed %d: %s at %.3f, believable is %.3f to %.3f", seed, r.name, r.got, r.low, r.top)
			}
		}
		if s.messages < 1500 {
			t.Errorf("seed %d: only %d messages; a busy team of 36 over two weeks needs at least 1,500", seed, s.messages)
		}
		for _, ch := range c.Org.Channels {
			if s.perConversation["#"+ch.Name] < 5 {
				t.Errorf("seed %d: #%s has only %d messages", seed, ch.Name, s.perConversation["#"+ch.Name])
			}
		}
	}
}

var forbidden = regexp.MustCompile(`(?i)lorem|ipsum|placeholder|\btodo\b|\bfoo\b|foo bar|test message|\{|\}|%!|\bapple\b|google|microsoft|amazon|\baws\b|slack|stripe|twilio|figma|notion|zoom\b|salesforce|hubspot|zendesk|github`)

func TestTextIsNotFiller(t *testing.T) {
	c := load(t)
	s := measure(build(t, c, Options{}))
	counts := map[string]int{}
	lengths := make([]int, 0, len(s.texts))
	for _, x := range s.texts {
		counts[x]++
		lengths = append(lengths, utf8.RuneCountInString(x))
		if m := forbidden.FindString(x); m != "" {
			t.Errorf("filler, an unresolved template or a real company name (%q) in: %s", m, x)
		}
	}
	for x, n := range counts {
		if n > 3 {
			t.Errorf("%d copies of: %s", n, x)
		}
	}
	if unique := float64(len(counts)) / float64(len(s.texts)); unique < 0.95 {
		t.Errorf("only %.1f%% of messages are distinct", unique*100)
	}
	slices.Sort(lengths)
	p10, p50, p90 := lengths[len(lengths)/10], lengths[len(lengths)/2], lengths[len(lengths)*9/10]
	t.Logf("length p10 %d, median %d, p90 %d", p10, p50, p90)
	if p10 > 45 || p90 < 110 || p90 < 2*p50 {
		t.Errorf("message lengths are too uniform: p10 %d, median %d, p90 %d", p10, p50, p90)
	}
}

func TestEndCanMoveByWholeWeeks(t *testing.T) {
	c := load(t)
	end := c.LatestEnd(time.Date(2026, 12, 3, 12, 0, 0, 0, time.UTC))
	if end.Weekday() != time.Tuesday || !end.Before(time.Date(2026, 12, 3, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("LatestEnd gave %s", end)
	}
	h := build(t, c, Options{End: end})
	if h.Last.After(end) {
		t.Fatalf("history ends %s, after %s", h.Last, end)
	}
	if _, err := c.Build(Options{End: end.Add(24 * time.Hour)}); err == nil {
		t.Fatal("an end that is not a Tuesday must be refused")
	}
}

// TestDump writes the history for reading by hand: DEMOCONTENT_DUMP=dir go test -run TestDump
func TestDump(t *testing.T) {
	dir := os.Getenv("DEMOCONTENT_DUMP")
	if dir == "" {
		t.Skip("set DEMOCONTENT_DUMP to write the history")
	}
	c := load(t)
	h := build(t, c, Options{})
	b, err := h.JSONL()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "history.jsonl"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	var sits bytes.Buffer
	for _, s := range c.Situations {
		out, err := c.Compile(s, h.Through, h.Last.Add(time.Minute), "p1")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&sits, "%s: %d changes\n", s.ID, len(out))
	}
	if err := os.WriteFile(filepath.Join(dir, "situations.txt"), sits.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
