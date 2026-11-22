// Package democontent loads the seeded organization from demo/content and turns
// it into ordered changes. It is data and a generator; it runs no service.
// The format is described in demo/content/README.md.
package democontent

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Person is one account, a person or an agent.
type Person struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Agent bool   `json:"agent,omitempty"`
}

// Channel is a channel and the accounts in it, with groups expanded.
type Channel struct {
	Name   string   `json:"name"`
	People []string `json:"people"`
}

// Dial is a starting channel dial for one person.
type Dial struct {
	Person  string `json:"person"`
	Channel string `json:"channel"`
	Level   string `json:"level"`
}

// Rule is a starting Focus rule for one person.
type Rule struct {
	Person string `json:"person"`
	Type   string `json:"type"`
	Value  string `json:"value"`
}

// Organization is organization.json.
type Organization struct {
	Org           string              `json:"org"`
	Name          string              `json:"name"`
	About         string              `json:"about"`
	DefaultSeed   uint64              `json:"default_seed"`
	End           time.Time           `json:"end"`
	DefaultViewer string              `json:"default_viewer"`
	Core          []string            `json:"core"`
	People        []Person            `json:"people"`
	Groups        map[string][]string `json:"groups"`
	Channels      []Channel           `json:"channels"`
	Directs       [][2]string         `json:"directs"`
	Dials         []Dial              `json:"dials"`
	FocusRules    []Rule              `json:"focus_rules"`
}

// Reacted is a reaction written by hand on a story line.
type Reacted struct {
	Emoji  string
	People []string
}

// Line is one message of a hand-written exchange.
type Line struct {
	Author    string
	Text      string
	Reply     bool
	Level     string
	Reactions []Reacted
	EditTo    string
	Delete    bool
	Source    string
}

// Exchange is a hand-written conversation placed somewhere in its phase.
type Exchange struct {
	Phase        string
	Conversation string
	Lines        []Line
}

// Content is everything in demo/content, checked.
type Content struct {
	Org          Organization
	Exchanges    []Exchange
	AgentReplies map[string][]string
	Situations   []Situation

	people  map[string]Person
	byName  map[string]string
	members map[string]map[string]bool
}

// Phases a story exchange can belong to; d0 to d15 also name one day of the history.
var phases = []string{"early", "mid", "late", "weekend"}

var dayPhase = regexp.MustCompile(`^d([0-9]|1[0-5])$`)

// Load reads and checks the content. Pass os.DirFS("demo/content") or an embedded copy.
func Load(fsys fs.FS) (*Content, error) {
	c := &Content{people: map[string]Person{}, byName: map[string]string{}, members: map[string]map[string]bool{}}
	if err := readJSON(fsys, "organization.json", &c.Org); err != nil {
		return nil, err
	}
	if err := c.index(); err != nil {
		return nil, err
	}
	if err := readJSON(fsys, "agent-replies.json", &c.AgentReplies); err != nil {
		return nil, err
	}
	for agent, replies := range c.AgentReplies {
		if p, ok := c.people[agent]; !ok || !p.Agent {
			return nil, fmt.Errorf("agent-replies.json: %q is not an agent", agent)
		}
		for _, r := range replies {
			if _, err := c.mentionsIn(r); err != nil {
				return nil, fmt.Errorf("agent-replies.json: %v", err)
			}
		}
	}
	files, err := fs.Glob(fsys, "story/*.txt")
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		ex, err := c.parseStory(f, string(b))
		if err != nil {
			return nil, err
		}
		c.Exchanges = append(c.Exchanges, ex...)
	}
	if err := readJSON(fsys, "situations.json", &c.Situations); err != nil {
		return nil, err
	}
	for i := range c.Situations {
		if err := c.checkSituation(&c.Situations[i]); err != nil {
			return nil, fmt.Errorf("situations.json: %s: %w", c.Situations[i].ID, err)
		}
	}
	return c, nil
}

func readJSON(fsys fs.FS, name string, into any) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func (c *Content) index() error {
	for _, p := range c.Org.People {
		if _, dup := c.people[p.ID]; dup {
			return fmt.Errorf("person %q twice", p.ID)
		}
		c.people[p.ID] = p
		first := strings.ToLower(strings.Fields(p.Name)[0])
		if other, dup := c.byName[first]; dup {
			return fmt.Errorf("%q and %q share the name used in mentions", p.ID, other)
		}
		c.byName[first] = p.ID
	}
	for i := range c.Org.Channels {
		ch := &c.Org.Channels[i]
		var people []string
		for _, e := range ch.People {
			if g, ok := strings.CutPrefix(e, "group:"); ok {
				list, ok := c.Org.Groups[g]
				if !ok {
					return fmt.Errorf("channel %s: no group %q", ch.Name, g)
				}
				people = append(people, list...)
			} else {
				people = append(people, e)
			}
		}
		set := map[string]bool{}
		ch.People = ch.People[:0]
		for _, p := range people {
			if _, ok := c.people[p]; !ok {
				return fmt.Errorf("channel %s: unknown person %q", ch.Name, p)
			}
			if !set[p] {
				set[p] = true
				ch.People = append(ch.People, p)
			}
		}
		c.members["#"+ch.Name] = set
	}
	for _, d := range c.Org.Directs {
		for _, p := range d {
			if _, ok := c.people[p]; !ok {
				return fmt.Errorf("direct %v: unknown person %q", d, p)
			}
		}
		c.members[directID(d[0], d[1])] = map[string]bool{d[0]: true, d[1]: true}
	}
	for _, d := range c.Org.Dials {
		if !c.members["#"+d.Channel][d.Person] {
			return fmt.Errorf("dial: %s is not in #%s", d.Person, d.Channel)
		}
	}
	return nil
}

func directID(a, b string) string {
	if b < a {
		a, b = b, a
	}
	return "dm:" + a + ":" + b
}

var mentionPattern = regexp.MustCompile(`@(\p{L}+)`)

// mentionsIn resolves every @Name in a text to account IDs, in order, without repeats.
func (c *Content) mentionsIn(text string) ([]string, error) {
	out := []string{}
	for _, m := range mentionPattern.FindAllStringSubmatch(text, -1) {
		id, ok := c.byName[strings.ToLower(m[1])]
		if !ok {
			return nil, fmt.Errorf("@%s is nobody in %q", m[1], text)
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

var (
	storyAnnotation = regexp.MustCompile(`\s*\{([^{}]*)\}\s*$`)
	storyLine       = regexp.MustCompile(`^(>\s*)?([a-z]+):\s+(.+)$`)
)

func (c *Content) parseStory(file, body string) ([]Exchange, error) {
	base, _, _ := strings.Cut(path.Base(file), ".") // "launch.more.txt" is also #launch
	var out []Exchange
	var cur *Exchange
	for n, raw := range strings.Split(body, "\n") {
		src := fmt.Sprintf("%s:%d", file, n+1)
		line := strings.TrimRight(raw, " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "==="); ok {
			f := strings.Fields(rest)
			if len(f) == 0 || !(slices.Contains(phases, f[0]) || dayPhase.MatchString(f[0])) {
				return nil, fmt.Errorf("%s: a header is === early|mid|late|weekend|d0..d15", src)
			}
			conv := "#" + base
			if len(f) == 3 {
				conv = directID(f[1], f[2])
			} else if len(f) != 1 {
				return nil, fmt.Errorf("%s: a header takes a phase and, for a direct conversation, two people", src)
			}
			if c.members[conv] == nil {
				return nil, fmt.Errorf("%s: no conversation %s", src, conv)
			}
			out = append(out, Exchange{Phase: f[0], Conversation: conv})
			cur = &out[len(out)-1]
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("%s: text before the first === header", src)
		}
		l, err := c.parseLine(line, src, cur)
		if err != nil {
			return nil, err
		}
		cur.Lines = append(cur.Lines, l)
	}
	for _, e := range out {
		if len(e.Lines) == 0 || e.Lines[0].Reply {
			return nil, fmt.Errorf("%s: an exchange starts with a top-level message", file)
		}
	}
	return out, nil
}

func (c *Content) parseLine(line, src string, ex *Exchange) (Line, error) {
	var l Line
	l.Source = src
	for {
		m := storyAnnotation.FindStringSubmatchIndex(line)
		if m == nil {
			break
		}
		a := strings.TrimSpace(line[m[2]:m[3]])
		line = line[:m[0]]
		switch {
		case a == "flag" || a == "info":
			l.Level = a
		case a == "delete":
			l.Delete = true
		case strings.HasPrefix(a, "edit:"):
			l.EditTo = strings.TrimSpace(strings.TrimPrefix(a, "edit:"))
		case strings.HasPrefix(a, "+"):
			f := strings.Fields(a[1:])
			if len(f) != 2 {
				return l, fmt.Errorf("%s: a reaction is {+emoji a,b}", src)
			}
			r := Reacted{Emoji: f[0], People: strings.Split(f[1], ",")}
			for _, p := range r.People {
				if !c.members[ex.Conversation][p] {
					return l, fmt.Errorf("%s: %q cannot react in %s", src, p, ex.Conversation)
				}
			}
			l.Reactions = append(l.Reactions, r)
		default:
			return l, fmt.Errorf("%s: unknown annotation {%s}", src, a)
		}
	}
	m := storyLine.FindStringSubmatch(line)
	if m == nil {
		return l, fmt.Errorf("%s: expected \"author: text\" or \"> author: text\"", src)
	}
	l.Reply, l.Author, l.Text = m[1] != "", m[2], m[3]
	if !c.members[ex.Conversation][l.Author] {
		return l, fmt.Errorf("%s: %q is not in %s", src, l.Author, ex.Conversation)
	}
	for _, t := range []string{l.Text, l.EditTo} {
		ids, err := c.mentionsIn(t)
		if err != nil {
			return l, fmt.Errorf("%s: %v", src, err)
		}
		for _, id := range ids {
			if !c.members[ex.Conversation][id] {
				return l, fmt.Errorf("%s: mentions %s, who is not in %s", src, id, ex.Conversation)
			}
		}
	}
	return l, nil
}
