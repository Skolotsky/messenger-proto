package democontent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"contexia/internal/shared/contract"
)

// Options choose one history. The same options and content give the same bytes.
type Options struct {
	Seed uint64    // 0 means the organization's default seed
	End  time.Time // zero means the organization's end; must be a Tuesday at the same time of day
}

// History is the seeded log, ready to send to a seed session in batches.
type History struct {
	Changes []contract.Change
	Through int64
	Last    time.Time
}

// JSONL is the history with one change per line; this is what "byte for byte" compares.
func (h *History) JSONL() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, ch := range h.Changes {
		if err := enc.Encode(ch); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// LatestEnd is the newest valid end of history at or before now, so that a history
// can be shifted by whole weeks and keep its weekdays.
func (c *Content) LatestEnd(now time.Time) time.Time {
	end := c.Org.End
	for end.Add(7 * 24 * time.Hour).Before(now) {
		end = end.Add(7 * 24 * time.Hour)
	}
	for end.After(now) {
		end = end.Add(-7 * 24 * time.Hour)
	}
	return end
}

// planned is a message waiting for its place in the log.
type planned struct {
	at     time.Time
	author string
	conv   string
	root   *planned
	text   string
	level  string
	m      *msg
}

type event struct {
	at  time.Time
	seq int
	run func() error
}

type builder struct {
	c       *Content
	rng     *rand.Rand
	end     time.Time
	start   time.Time
	w       *writer
	events  []event
	posts   []*planned
	flagged []*planned
	replied map[string]bool // person|root id: has answered in that thread

	findingsLeft []string
	usedLines    map[string]int
	decks        map[string][]string
	ticket       int

	transcripts, transcriptsRead int
}

// fresh picks a line from a pool, at most twice each over the whole history;
// an empty answer means the pool is spent and the reply is simply not written.
func (b *builder) fresh(pool []string) string {
	start := b.rng.IntN(len(pool))
	for i := range pool {
		line := pool[(start+i)%len(pool)]
		if b.usedLines[line] < 2 {
			b.usedLines[line]++
			return line
		}
	}
	return ""
}

// Build generates the history.
func (c *Content) Build(o Options) (*History, error) {
	seed := o.Seed
	if seed == 0 {
		seed = c.Org.DefaultSeed
	}
	end := o.End
	if end.IsZero() {
		end = c.Org.End
	}
	end = end.UTC()
	if end.Weekday() != time.Tuesday || end.Sub(end.Truncate(24*time.Hour)) != c.Org.End.UTC().Sub(c.Org.End.UTC().Truncate(24*time.Hour)) {
		return nil, fmt.Errorf("end %s must be a Tuesday at %s", end, c.Org.End.UTC().Format("15:04"))
	}
	day := end.Truncate(24 * time.Hour)
	b := &builder{c: c, rng: rand.New(rand.NewPCG(seed, 0x4c61726b6d657265)), end: end,
		start: day.AddDate(0, 0, -15), replied: map[string]bool{}, findingsLeft: slices.Clone(findings), usedLines: map[string]int{}, decks: map[string][]string{}, ticket: 4100}
	b.w = newWriter(c, 0, b.start, "s")

	// The log itself says where seeded history starts (D-033). Sealing the
	// session writes its end.
	number, previous, stamp := b.w.next(b.start)
	b.w.add(contract.ChangeSeedOpened{Kind: "seed.opened", Org: c.Org.Org, Number: number, Previous: previous, At: stamp})
	b.opening()
	for _, ex := range c.Exchanges {
		b.placeExchange(ex)
	}
	b.agentTraffic()
	b.backgroundReactions()
	b.resolveFlags()
	b.keptRows()
	b.threadDials()
	b.visits()

	slices.SortStableFunc(b.events, func(x, y event) int {
		if d := x.at.Compare(y.at); d != 0 {
			return d
		}
		return x.seq - y.seq
	})
	for _, e := range b.events {
		if e.at.After(b.w.at) {
			b.w.at = e.at // acts below stamp the moment they were scheduled for
		}
		if err := e.run(); err != nil {
			return nil, err
		}
	}
	return &History{Changes: b.w.out, Through: b.w.number, Last: b.w.at}, nil
}

func (b *builder) schedule(at time.Time, run func() error) {
	if at.After(b.end) {
		return
	}
	b.events = append(b.events, event{at: at, seq: len(b.events), run: run})
}

// opening writes who exists, the channels, direct conversations and starting settings.
func (b *builder) opening() {
	w, c, at := b.w, b.c, b.start
	stamp := func() (int64, int64, string) { return w.next(at) }
	for _, p := range c.Org.People {
		n, prev, s := stamp()
		w.add(contract.ChangePersonJoined{Kind: "person.joined", Org: w.org, Number: n, Previous: prev, At: s, Person: p.ID, Name: p.Name, IsAgent: p.Agent})
	}
	for _, ch := range c.Org.Channels {
		n, prev, s := stamp()
		w.add(contract.ChangeChannelCreated{Kind: "channel.created", Org: w.org, Number: n, Previous: prev, At: s, Conversation: "#" + ch.Name, Name: ch.Name, People: slices.Clone(ch.People)})
	}
	for _, d := range c.Org.Directs {
		n, prev, s := stamp()
		people := []string{d[0], d[1]}
		slices.Sort(people)
		w.add(contract.ChangeDirectCreated{Kind: "direct.created", Org: w.org, Number: n, Previous: prev, At: s, Conversation: directID(d[0], d[1]), People: people})
	}
	for _, d := range c.Org.Dials {
		w.dial(at, d.Person, "#"+d.Channel, nil, d.Level)
	}
	for _, r := range c.Org.FocusRules {
		n, prev, s := stamp()
		w.add(contract.ChangeReaderFocusRule{Kind: "reader.focus_rule", Org: w.org, Number: n, Previous: prev, At: s, Person: r.Person,
			Rule: contract.FocusRule{Type: contract.RuleType(r.Type), Value: r.Value}, On: true})
	}
}

// days returns the midnights of the days in a phase.
func (b *builder) days(phase string) []time.Time {
	var offsets []int
	if m := dayPhase.FindStringSubmatch(phase); m != nil {
		n, _ := strconv.Atoi(m[1])
		return []time.Time{b.start.AddDate(0, 0, n)}
	}
	switch phase {
	case "early":
		offsets = []int{0, 1, 2, 3, 4}
	case "mid":
		offsets = []int{7, 8, 9, 10, 11}
	case "late":
		offsets = []int{14, 15}
	case "weekend":
		offsets = []int{5, 6, 12, 13}
	}
	out := make([]time.Time, len(offsets))
	for i, o := range offsets {
		out[i] = b.start.AddDate(0, 0, o)
	}
	return out
}

// workMoment is a believable moment in a day: busy mornings and afternoons, a dip at lunch.
func (b *builder) workMoment(day time.Time, weekend bool) time.Time {
	var hour float64
	switch r := b.rng.Float64(); {
	case weekend:
		hour = 10 + b.rng.Float64()*9
	case r < 0.47:
		hour = 8.75 + b.rng.Float64()*3.25
	case r < 0.57:
		hour = 12 + b.rng.Float64()*1.25
	case r < 0.95:
		hour = 13.25 + b.rng.Float64()*4
	default:
		hour = 17.25 + b.rng.Float64()*2.5
	}
	return day.Add(time.Duration(hour * float64(time.Hour))).Add(time.Duration(b.rng.IntN(60000)) * time.Millisecond)
}

// gap is the pause before the next message of an exchange.
func (b *builder) gap() time.Duration {
	minutes := 0.3 + b.rng.ExpFloat64()*5
	if minutes > 50 {
		minutes = 50
	}
	return time.Duration(minutes * float64(time.Minute))
}

func (b *builder) placeExchange(ex Exchange) {
	days := b.days(ex.Phase)
	day := days[b.rng.IntN(len(days))]
	weekend := day.Weekday() == time.Saturday || day.Weekday() == time.Sunday
	offsets := make([]time.Duration, len(ex.Lines))
	for i := 1; i < len(offsets); i++ {
		offsets[i] = offsets[i-1] + b.gap()
	}
	span := offsets[len(offsets)-1]
	at := b.workMoment(day, weekend)
	if day.Equal(b.end.Truncate(24 * time.Hour)) {
		// the last morning: spread over the hours that exist, so nothing piles up at the end
		first := day.Add(8*time.Hour + 40*time.Minute)
		room := b.end.Add(-time.Minute).Sub(first) - span
		if room < time.Minute {
			room = time.Minute
		}
		at = first.Add(time.Duration(b.rng.Int64N(int64(room))))
		if over := at.Add(span).Sub(b.end.Add(-time.Minute)); over > 0 {
			at = at.Add(-over)
		}
	}
	times := make([]time.Time, len(ex.Lines))
	for i := range times {
		times[i] = at.Add(offsets[i])
	}
	var root *planned
	for i, l := range ex.Lines {
		p := &planned{at: times[i], author: l.Author, conv: ex.Conversation, text: l.Text, level: l.Level}
		if l.Reply {
			p.root = root
			p.text = b.addressed(l, ex.Lines[i-1])
		} else {
			root = p
		}
		b.addPost(p)
		for _, r := range l.Reactions {
			for _, person := range r.People {
				b.reactLater(p, person, r.Emoji, time.Duration(30+b.rng.IntN(2400))*time.Second)
			}
		}
		if l.EditTo != "" {
			text := l.EditTo
			b.schedule(p.at.Add(time.Duration(40+b.rng.IntN(600))*time.Second), func() error {
				return b.w.edit(b.w.at, p.m, text, "same")
			})
		}
		if l.Delete {
			b.schedule(p.at.Add(time.Duration(60+b.rng.IntN(1500))*time.Second), func() error {
				b.w.remove(b.w.at, p.m)
				return nil
			})
		}
	}
}

// addressed sometimes opens a reply with the name of the person it answers, as people do
// in a busy thread. It never touches a line that was written with a mention or is edited later.
func (b *builder) addressed(l, before Line) string {
	if strings.Contains(l.Text, "@") || l.EditTo != "" || before.Author == l.Author || b.c.people[before.Author].Agent || b.c.people[l.Author].Agent || b.rng.Float64() >= 0.3 {
		return l.Text
	}
	name := strings.Fields(b.c.people[before.Author].Name)[0]
	return "@" + name + " " + lowerFirst(l.Text)
}

// lowerFirst lowers a sentence's first letter when its first word is an ordinary one,
// so that names and acronyms are never damaged.
func lowerFirst(s string) string {
	word, _, _ := strings.Cut(s, " ")
	if !slices.Contains(sentenceOpeners, strings.TrimRight(word, ".,")) {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

var sentenceOpeners = []string{"Yes", "No", "Good", "Thanks", "Agreed", "Fine", "That", "This", "It", "The", "We", "Can", "Do", "Does", "Is", "If", "Please", "Sure", "Done", "Not", "Only", "Then", "So", "Which", "What", "How", "Looks", "Noted"}

// addPost schedules a message and remembers it for reactions and flags.
func (b *builder) addPost(p *planned) {
	if p.at.After(b.end) {
		return
	}
	b.posts = append(b.posts, p)
	if p.level == "flag" {
		b.flagged = append(b.flagged, p)
	}
	if p.root != nil {
		b.replied[p.author+"|"+rootKey(p)] = true
	}
	b.schedule(p.at, func() error {
		var root *msg
		if p.root != nil {
			if p.root.m == nil {
				return fmt.Errorf("a reply in %s came before its root", p.conv)
			}
			root = p.root.m
		}
		m, err := b.w.post(p.at, p.author, p.conv, root, p.text, p.level)
		if err != nil {
			return fmt.Errorf("%s: %w", p.conv, err)
		}
		p.m = m
		return nil
	})
}

func rootKey(p *planned) string {
	if p.root != nil {
		return fmt.Sprintf("%p", p.root)
	}
	return fmt.Sprintf("%p", p)
}

func (b *builder) reactLater(p *planned, person, emoji string, after time.Duration) {
	b.schedule(p.at.Add(after), func() error {
		if p.m != nil {
			b.w.react(b.w.at, person, p.m, emoji)
		}
		return nil
	})
}

var (
	emojiPlain    = []string{"👍", "👍", "👍", "✅", "✅", "🙏", "💯", "❤️"}
	emojiQuestion = []string{"👀", "👀", "🤔", "👍"}
	emojiGood     = []string{"🎉", "🎉", "🚀", "❤️", "👏"}
	emojiFunny    = []string{"😂", "😂", "😅"}
	goodWords     = []string{"shipped", "live", "renewed", "signed", "fixed", "passed", "resolved", "welcome", "merged", "closed"}
	funnyWords    = []string{"coffee", "cake", "lunch", "dog", "pizza", "plant", "karaoke"}
)

func (b *builder) emojiFor(text string) string {
	low := strings.ToLower(text)
	pool := emojiPlain
	switch {
	case containsAny(low, funnyWords):
		pool = emojiFunny
	case containsAny(low, goodWords):
		pool = emojiGood
	case strings.Contains(low, "?"):
		pool = emojiQuestion
	}
	return pool[b.rng.IntN(len(pool))]
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// backgroundReactions gives messages the reactions a team leaves, a few taken back.
func (b *builder) backgroundReactions() {
	for _, p := range b.posts {
		chance := 0.24
		switch {
		case p.level == "info":
			chance = 0.05
		case b.c.people[p.author].Agent:
			chance = 0.14
		case p.root == nil && (p.conv == "#general" || p.conv == "#launch"):
			chance = 0.6
		}
		if b.rng.Float64() >= chance {
			continue
		}
		var pool []string
		for _, id := range sortedKeys(b.c.members[p.conv]) {
			if id != p.author && !b.c.people[id].Agent {
				pool = append(pool, id)
			}
		}
		count := 1
		for count < 7 && count < len(pool) && b.rng.Float64() < 0.42 {
			count++
		}
		emoji := b.emojiFor(p.text)
		for _, i := range b.rng.Perm(len(pool))[:min(count, len(pool))] {
			person := pool[i]
			if b.rng.Float64() < 0.2 {
				emoji = b.emojiFor(p.text)
			}
			e, after := emoji, time.Duration(20+b.rng.IntN(5400))*time.Second
			b.reactLater(p, person, e, after)
			if b.rng.Float64() < 0.03 {
				at := p.at.Add(after + time.Duration(30+b.rng.IntN(1800))*time.Second)
				b.schedule(at, func() error {
					if p.m != nil {
						b.w.unreact(b.w.at, person, p.m, e)
					}
					return nil
				})
			}
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// resolveFlags has people press Done on flags they will not answer, leaving at most
// two recent ones open per person, as someone who keeps up with their feed would (D-011).
func (b *builder) resolveFlags() {
	open := map[string]int{}
	flags := slices.Clone(b.flagged)
	slices.SortStableFunc(flags, func(x, y *planned) int { return y.at.Compare(x.at) })
	for _, p := range flags {
		named, err := b.c.mentionsIn(p.text)
		if err != nil {
			continue
		}
		for _, person := range named {
			if person == p.author || b.c.people[person].Agent || b.replied[person+"|"+rootKey(p)] {
				continue
			}
			if b.end.Sub(p.at) < 72*time.Hour && open[person] < 2 {
				open[person]++
				continue
			}
			at := p.at.Add(time.Duration(1+b.rng.IntN(20)) * time.Hour)
			if at.After(b.end) {
				at = b.end.Add(-time.Minute)
			}
			b.schedule(at, func() error {
				if p.m != nil && !p.m.deleted {
					b.w.done(b.w.at, person, p.m)
				}
				return nil
			})
		}
	}
}

// keptRows has a few people keep something to come back to, and finish some of it.
func (b *builder) keptRows() {
	for _, p := range b.posts {
		if p.root != nil || p.level == "info" || b.c.people[p.author].Agent || b.rng.Float64() >= 0.012 {
			continue
		}
		named, _ := b.c.mentionsIn(p.text)
		for _, person := range named {
			if person == p.author || b.c.people[person].Agent {
				continue
			}
			at := p.at.Add(time.Duration(5+b.rng.IntN(240)) * time.Minute)
			finish := at.Add(time.Duration(2+b.rng.IntN(40)) * time.Hour)
			b.schedule(at, func() error {
				if p.m != nil && !p.m.deleted {
					b.w.keep(b.w.at, person, p.m)
				}
				return nil
			})
			if b.rng.Float64() < 0.7 {
				b.schedule(finish, func() error {
					if p.m != nil && !p.m.deleted {
						b.w.done(b.w.at, person, p.m)
					}
					return nil
				})
			}
		}
	}
}

// threadDials has a few people follow, or turn off, a long thread.
func (b *builder) threadDials() {
	replies := map[*planned]int{}
	for _, p := range b.posts {
		if p.root != nil {
			replies[p.root]++
		}
	}
	levels := []string{"focus", "fyi", "off", "needs_you"}
	for _, p := range b.posts {
		if p.root != nil || replies[p] < 4 || strings.HasPrefix(p.conv, "dm:") || b.rng.Float64() >= 0.18 {
			continue
		}
		pool := sortedKeys(b.c.members[p.conv])
		person := pool[b.rng.IntN(len(pool))]
		if b.c.people[person].Agent {
			continue
		}
		level := levels[b.rng.IntN(len(levels))]
		at := p.at.Add(time.Duration(10+b.rng.IntN(120)) * time.Minute)
		b.schedule(at, func() error {
			if p.m != nil {
				b.w.dial(b.w.at, person, p.conv, &p.m.id, level)
			}
			return nil
		})
	}
}

// visits has people open the product a few times a day and read what they follow.
// The default viewer's last visit is Monday evening, so Tuesday morning is unread.
func (b *builder) visits() {
	convs := map[string][]string{}
	for _, ch := range b.c.Org.Channels {
		for _, p := range ch.People {
			convs[p] = append(convs[p], "#"+ch.Name)
		}
	}
	for _, d := range b.c.Org.Directs {
		id := directID(d[0], d[1])
		convs[d[0]] = append(convs[d[0]], id)
		convs[d[1]] = append(convs[d[1]], id)
	}
	for offset := 0; offset <= 15; offset++ {
		day := b.start.AddDate(0, 0, offset)
		weekend := day.Weekday() == time.Saturday || day.Weekday() == time.Sunday
		for _, p := range b.c.Org.People {
			if p.Agent || (weekend && b.rng.Float64() > 0.2) {
				continue
			}
			if offset == 15 && (p.ID == b.c.Org.DefaultViewer || b.rng.Float64() < 0.4) {
				continue
			}
			n := 2
			if b.rng.Float64() < 0.4 {
				n = 3
			}
			for range n {
				person, list := p.ID, convs[p.ID]
				at := b.workMoment(day, weekend)
				if offset == 14 && person == b.c.Org.DefaultViewer {
					at = day.Add(18*time.Hour + time.Duration(b.rng.IntN(1800))*time.Second)
				}
				skips := make([]bool, len(list))
				for i := range skips {
					skips[i] = b.rng.Float64() < 0.15
				}
				b.schedule(at, func() error {
					for i, conv := range list {
						if !skips[i] {
							b.w.read(b.w.at, person, conv, nil, b.w.top[conv])
						}
					}
					for _, th := range b.w.threads[person] {
						id := th.id
						b.w.read(b.w.at, person, th.conv, &id, b.w.inThread[id])
					}
					return nil
				})
			}
		}
	}
}
