// Package mockdoor serves the people door and the live connection from memory,
// so the web app can be built before the real server exists. Its sorting into
// buckets is a deliberate simplification: the real rules live in the feed.
package mockdoor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"contexia/tests/harness/logwriter"
)

const (
	firstOrg  = "acme"
	replayMax = 500 // conversations-to-screen.md: further behind, the screen is told to load fresh
)

type msg = map[string]any

// update is one numbered update of one person's stream. Its parts are frozen
// when it is made, so a replay says what was true then. after is the number
// sent to this person before it, so a filtered stream's gaps can be told from a loss.
type update struct {
	number, after int64
	parts         json.RawMessage
}

type client struct {
	person string
	out    chan []byte
	held   [][]byte // updates kept back by the Hold control
	// watching holds the rows this connection said are open in front of the person.
	watching map[string]bool
}

type replying struct {
	conversation, author string
	thread               any
	text                 []rune
}

// Door is the mock of Conversations' door for people.
type Door struct {
	mu         sync.Mutex
	org        string
	generation int
	retired    map[string]bool
	number     int64
	seedNumber int64
	feedNum    int64
	edition    int64
	people     []map[string]any
	convs      map[string]map[string]any
	order      []string
	msgs       []msg
	byID       map[string]msg
	reads      map[string]int64
	kept       map[string]bool
	dials      map[string]string
	prefs      map[string]map[string]string
	answers    map[string]answer
	convLog    map[string][]update
	feedLog    map[string][]update
	rows       map[string]map[string]map[string]any
	histSeen   map[string]map[string]int
	inProgress map[string]*replying
	clients    map[*client]bool
	feedDown   bool
	down       bool
	refuseLive bool
	hold       bool
	dropNext   int
	refuse     string
}

type answer struct {
	status int
	body   any
	raw    string // the action as first received: the same ID with another body is refused
}

// New returns a door filled from a seeded log.
func New(seed int64, changes int) *Door {
	d := &Door{org: firstOrg, generation: 1, retired: map[string]bool{}, edition: 1, convs: map[string]map[string]any{}, byID: map[string]msg{}, reads: map[string]int64{},
		kept: map[string]bool{}, dials: map[string]string{}, prefs: map[string]map[string]string{}, answers: map[string]answer{},
		convLog: map[string][]update{}, feedLog: map[string][]update{}, rows: map[string]map[string]map[string]any{},
		histSeen: map[string]map[string]int{}, inProgress: map[string]*replying{}, clients: map[*client]bool{}}
	for _, e := range logwriter.Write(logwriter.Config{Orgs: []string{firstOrg}, Seed: seed, Changes: changes})[firstOrg] {
		var c map[string]any
		if err := json.Unmarshal(e.JSON, &c); err != nil {
			panic(err)
		}
		d.fold(c)
		d.number = e.Number
	}
	d.seedNumber = d.number
	return d
}

func (d *Door) fold(c map[string]any) {
	switch c["kind"] {
	case "person.joined":
		d.people = append(d.people, map[string]any{"id": c["person"], "name": title(c["name"].(string)), "is_agent": c["is_agent"], "line": nil, "status": nil})
	case "channel.created":
		d.addConv(c["conversation"].(string), "channel", c["name"].(string), c["people"].([]any))
	case "channel.people_added":
		conv := d.convs[c["conversation"].(string)]
		conv["people"] = append(conv["people"].([]any), c["people"].([]any)...)
	case "direct.created":
		d.addConv(c["conversation"].(string), "direct", c["conversation"].(string), c["people"].([]any))
	case "message.created":
		m := c["message"].(map[string]any)
		d.msgs = append(d.msgs, m)
		d.byID[m["id"].(string)] = m
	case "message.changed":
		m := c["message"].(map[string]any)
		old := d.byID[m["id"].(string)]
		for k, v := range m {
			old[k] = v
		}
	case "message.deleted":
		mm := c["message"].(map[string]any)
		tombstone(d.byID[mm["id"].(string)], mm["version"])
	case "reaction.added", "reaction.removed":
		if m := d.byID[c["message"].(string)]; m != nil {
			react(m, c["person"].(string), c["emoji"].(string), c["kind"] == "reaction.added")
		}
	case "reader.read":
		d.reads[key(c["person"].(string), c["conversation"].(string), c["thread"])] = int64(c["up_to"].(float64))
	}
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func tombstone(m msg, version any) {
	m["version"], m["deleted"], m["text"] = version, true, ""
	m["mentions"], m["reactions"] = []any{}, []any{}
}

func react(m msg, person, emoji string, on bool) {
	out := []any{}
	found := false
	for _, r := range m["reactions"].([]any) {
		rr := r.(map[string]any)
		people := []any{}
		for _, p := range rr["people"].([]any) {
			if p != person {
				people = append(people, p)
			}
		}
		if rr["emoji"] == emoji {
			found = true
			if on {
				people = append(people, person)
			}
		}
		if len(people) > 0 {
			out = append(out, map[string]any{"emoji": rr["emoji"], "people": people})
		}
	}
	if on && !found {
		out = append(out, map[string]any{"emoji": emoji, "people": []any{person}})
	}
	m["reactions"] = out
}

func (d *Door) addConv(id, kind, name string, people []any) {
	d.convs[id] = map[string]any{"conversation": id, "type": kind, "name": name, "people": people, "dial": "default"}
	d.order = append(d.order, id)
}

func key(person, conv string, thread any) string {
	return fmt.Sprintf("%s|%s|%v", person, conv, thread)
}

func num(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	}
	return 0
}

func contains(list any, s string) bool {
	items, _ := list.([]any)
	for _, i := range items {
		if i == s {
			return true
		}
	}
	return false
}

// rowsFor is the mock's plain sorting: direct messages in Focus, what mentions
// you in Needs you, other channel posts in For your information.
func (d *Door) rowsFor(person string) map[string]map[string]any {
	rows := map[string]map[string]any{}
	for _, m := range d.msgs {
		if m["deleted"] == true || m["author"] == person {
			continue
		}
		conv := m["conversation"].(string)
		c := d.convs[conv]
		if c == nil || !contains(c["people"], person) {
			continue
		}
		var thread any
		bucket, why := "fyi", title(m["author"].(string))+" posted in "+conv
		switch {
		case c["type"] == "direct":
			bucket, why = "focus", "Direct message from "+title(m["author"].(string))
		case contains(m["mentions"], person):
			bucket, why = "needs_you", title(m["author"].(string))+" mentioned you in "+conv
			thread = m["id"]
			if m["thread"] != nil {
				thread = m["thread"]
			}
		case m["thread"] != nil:
			continue
		}
		if d.isRead(person, m, thread) {
			continue
		}
		k := key(person, conv, thread)
		row := rows[k]
		if row == nil {
			row = map[string]any{"conversation": conv, "thread": thread, "first_new": m["number"], "new_items": int64(0)}
			rows[k] = row
		}
		kept := any(nil)
		if d.kept[k] {
			kept = "you"
		}
		row["message"], row["bucket"], row["unread"], row["kept"], row["why"] = m["id"], bucket, true, kept, why
		row["newest"], row["author"], row["preview"], row["at"] = m["number"], m["author"], m["text"], m["created_at"]
		row["new_items"] = row["new_items"].(int64) + 1
		row["reactions"], row["state"], row["fenced"] = m["reactions"], "none", false
		row["explanation"] = map[string]any{"what": why + ".", "decided_by": "The mock door's plain sorting.", "otherwise": "It would stay in the channel.", "change": nil}
	}
	return rows
}

// isRead applies docs/contracts/README.md: a root counts as read through its
// channel or its thread, a reply only through its thread.
func (d *Door) isRead(person string, m msg, thread any) bool {
	conv, n := m["conversation"].(string), num(m["number"])
	if thread == nil {
		return d.reads[key(person, conv, nil)] >= n
	}
	if m["thread"] == nil && d.reads[key(person, conv, nil)] >= n {
		return true
	}
	return d.reads[key(person, conv, thread)] >= n
}

func sortedRows(rows map[string]map[string]any) []any {
	list := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool { return num(list[i]["newest"]) > num(list[j]["newest"]) })
	out := make([]any, len(list))
	for i, r := range list {
		out[i] = r
	}
	return out
}

func counts(rows map[string]map[string]any) map[string]any {
	c := map[string]any{"focus": 0, "needs_you": 0, "fyi": false, "kept": 0}
	for _, r := range rows {
		switch r["bucket"] {
		case "focus":
			c["focus"] = c["focus"].(int) + 1
		case "needs_you":
			c["needs_you"] = c["needs_you"].(int) + 1
		case "fyi":
			c["fyi"] = true
		}
		if r["kept"] != nil {
			c["kept"] = c["kept"].(int) + 1
		}
	}
	return c
}

func lastOf(log []update, base int64) int64 {
	if len(log) > 0 {
		return log[len(log)-1].number
	}
	return base
}

func (d *Door) lastFeed(person string) int64 { return lastOf(d.feedLog[person], 0) }

// lastConv is the last number of the conversations stream for this person.
func (d *Door) lastConv(person string) int64 { return lastOf(d.convLog[person], d.seedNumber) }

// Handler serves the door and a /mock/control address for going wrong on purpose.
func (d *Door) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /alive", func(w http.ResponseWriter, _ *http.Request) {
		send(w, 200, map[string]string{"part": "mockdoor", "status": "alive"})
	})
	m.HandleFunc("POST /mock/control", d.control)
	m.HandleFunc("GET /demo/current", func(w http.ResponseWriter, _ *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		send(w, 200, map[string]any{"org": d.org, "name": "Acme"})
	})
	m.HandleFunc("POST /orgs/{org}/people/actions", d.guard(d.act))
	m.HandleFunc("GET /orgs/{org}/people/{person}/start", d.guard(d.start))
	m.HandleFunc("GET /orgs/{org}/conversations/{conversation}/messages", d.guard(d.messages))
	m.HandleFunc("GET /orgs/{org}/replies/{message}", d.guard(d.replyText))
	m.HandleFunc("GET /orgs/{org}/people/{person}/feed", d.guard(d.feed))
	m.HandleFunc("GET /orgs/{org}/people/{person}/history", d.guard(d.history))
	m.HandleFunc("GET /orgs/{org}/people/{person}/settings", d.guard(d.settings))
	m.HandleFunc("GET /orgs/{org}/people/{person}/preview", d.guard(func(w http.ResponseWriter, _ *http.Request) {
		send(w, 200, map[string]any{"last_7_days": map[string]any{"moved": 3, "from": "needs_you", "to": "fyi", "stay": 1}})
	}))
	m.HandleFunc("GET /orgs/{org}/people/{person}/search", d.guard(d.search))
	m.HandleFunc("GET /orgs/{org}/suggest", d.guard(d.suggest))
	m.HandleFunc("GET /orgs/{org}/live", d.live)
	return allowBrowser(m)
}

func allowBrowser(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (d *Door) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		down, current, gone := d.down, d.org, d.retired[r.PathValue("org")]
		d.mu.Unlock()
		if down {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		if gone {
			send(w, http.StatusGone, map[string]any{"organization_gone": true})
			return
		}
		if r.PathValue("org") != current {
			send(w, 404, map[string]string{"refused": "unknown organization"})
			return
		}
		next(w, r)
	}
}

// replyCommand drives a reply in progress, as the agent door would.
type replyCommand struct {
	Start, Finish, Abandon, Silent bool
	Message, Conversation, Author  string
	Thread                         *string
	Piece                          string
	At                             *int
}

func (d *Door) control(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Down, FeedDown, RefuseLive, Hold, Cut, Beat, NewEdition, Retire, Restore, Forget *bool
		Refuse, Release                                                                  *string
		DropNext                                                                         *int
		Burst                                                                            *struct {
			As, Conversation string
			Count            int
		}
		Reply       *replyCommand
		AgentStatus *struct {
			Agent, State string
			On           *string
		}
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if in.Down != nil {
		d.down = *in.Down
		if d.down {
			d.cutAll()
		}
	}
	if in.RefuseLive != nil {
		d.refuseLive = *in.RefuseLive
	}
	if in.Cut != nil && *in.Cut {
		d.cutAll()
	}
	if in.FeedDown != nil {
		was := d.feedDown
		d.feedDown = *in.FeedDown
		if was && !d.feedDown {
			d.pushFeed(nil)
		}
	}
	if in.Refuse != nil {
		d.refuse = *in.Refuse
	}
	if in.DropNext != nil {
		d.dropNext = *in.DropNext
	}
	if in.Hold != nil {
		d.hold = *in.Hold
	}
	if in.Release != nil {
		d.hold = false
		for c := range d.clients {
			held := c.held
			c.held = nil
			if *in.Release == "reverse" {
				for i := len(held) - 1; i >= 0; i-- {
					push(c, held[i])
				}
			} else {
				for _, b := range held {
					push(c, b)
				}
			}
		}
	}
	if in.Burst != nil {
		for i := 0; i < in.Burst.Count; i++ {
			id := fmt.Sprintf("burst_%d_%d", d.number, i)
			d.apply(map[string]any{"kind": "send", "message": map[string]any{"id": id, "conversation": in.Burst.Conversation,
				"thread": nil, "text": "Burst message " + id, "mentions": []any{}, "level": nil}}, in.Burst.As)
		}
	}
	if in.Reply != nil {
		d.replyControl(in.Reply)
	}
	if s := in.AgentStatus; s != nil {
		var on any
		if s.On != nil {
			on = *s.On
		}
		for _, p := range d.people {
			if p["id"] == s.Agent {
				p["status"] = map[string]any{"state": s.State, "on": on}
			}
		}
		b, _ := json.Marshal(map[string]any{"agent_status": map[string]any{"agent": s.Agent, "state": s.State, "on": on}})
		for c := range d.clients {
			push(c, b)
		}
	}
	if in.NewEdition != nil && *in.NewEdition {
		d.edition++
	}
	if in.Beat != nil && *in.Beat {
		for c := range d.clients {
			push(c, d.lastSent(c.person))
		}
	}
	if in.Retire != nil && *in.Retire {
		b, _ := json.Marshal(map[string]any{"organization_gone": true})
		for c := range d.clients {
			push(c, b)
		}
		d.cutAll()
		d.retired[d.org] = true
		d.generation++
		d.org = fmt.Sprintf("%s-%d", firstOrg, d.generation)
	}
	if in.Forget != nil && *in.Forget {
		d.retired = map[string]bool{} // deleted organizations answer 404
	}
	if in.Restore != nil && *in.Restore {
		d.org, d.generation, d.retired = firstOrg, 1, map[string]bool{}
	}
	send(w, 200, map[string]any{})
}

func (d *Door) replyControl(c *replyCommand) {
	var thread any
	if c.Thread != nil {
		thread = *c.Thread
	}
	switch {
	case c.Start:
		d.inProgress[c.Message] = &replying{conversation: c.Conversation, author: c.Author, thread: thread}
		d.number++
		d.publish([]any{map[string]any{"kind": "reply.started", "message": c.Message, "conversation": c.Conversation, "thread": thread, "author": c.Author}}, c.Conversation, nil, nil, nil)
	case c.Finish:
		rep := d.inProgress[c.Message]
		if rep == nil {
			return
		}
		delete(d.inProgress, c.Message)
		d.apply(map[string]any{"kind": "send", "message": map[string]any{"id": c.Message, "conversation": rep.conversation, "thread": rep.thread,
			"text": string(rep.text), "mentions": []any{}, "level": nil}}, rep.author)
	case c.Abandon:
		rep := d.inProgress[c.Message]
		if rep == nil {
			return
		}
		delete(d.inProgress, c.Message)
		d.number++
		d.publish([]any{map[string]any{"kind": "reply.abandoned", "message": c.Message, "conversation": rep.conversation, "thread": rep.thread}}, rep.conversation, nil, nil, nil)
	default:
		rep := d.inProgress[c.Message]
		if rep == nil {
			return
		}
		at := len(rep.text)
		if c.At != nil {
			at = *c.At
		}
		rep.text = append(rep.text[:min(at, len(rep.text))], []rune(c.Piece)...)
		if c.Silent {
			return // the piece is lost on its way: the screen sees a gap at the next one
		}
		b, _ := json.Marshal(map[string]any{"piece": map[string]any{"message": c.Message, "at": at, "text": c.Piece}})
		conv := d.convs[rep.conversation]
		for cl := range d.clients {
			if conv != nil && contains(conv["people"], cl.person) {
				push(cl, b)
			}
		}
	}
}

func (d *Door) cutAll() {
	for c := range d.clients {
		close(c.out)
		delete(d.clients, c)
	}
}

// push never blocks: a screen too slow for its buffer loses the update and heals by number.
func push(c *client, b []byte) {
	select {
	case c.out <- b:
	default:
	}
}

// deliver sends one numbered update to a person's connections, unless the
// controls lose it or hold it back.
func (d *Door) deliver(person string, b []byte) {
	for c := range d.clients {
		if c.person != person {
			continue
		}
		if d.dropNext > 0 {
			d.dropNext--
			continue
		}
		if d.hold {
			c.held = append(c.held, b)
			continue
		}
		push(c, b)
	}
}

func (d *Door) lastSent(person string) []byte {
	var feed, edition any = d.lastFeed(person), d.edition
	if d.feedDown {
		feed, edition = nil, nil
	}
	b, _ := json.Marshal(map[string]any{"last_sent": map[string]any{"conversations": d.lastConv(person), "feed": feed, "feed_edition": edition}})
	return b
}

func (d *Door) start(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	person := r.PathValue("person")
	var me map[string]any
	for _, p := range d.people {
		if p["id"] == person {
			me = p
		}
	}
	if me == nil {
		send(w, 404, map[string]string{"refused": "unknown person"})
		return
	}
	convs := []any{}
	for _, id := range d.order {
		c := d.convs[id]
		if !contains(c["people"], person) {
			continue
		}
		s := map[string]any{}
		for k, v := range c {
			s[k] = v
		}
		s["unread"] = d.unread(person, id)
		if level, ok := d.dials[key(person, id, nil)]; ok {
			s["dial"] = level
		}
		convs = append(convs, s)
	}
	prefs := []any{}
	names := make([]string, 0, len(d.prefs[person]))
	for k := range d.prefs[person] {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		prefs = append(prefs, map[string]any{"key": k, "value": d.prefs[person][k]})
	}
	send(w, 200, map[string]any{"org": d.org, "org_name": "Acme", "person": me, "people": d.people, "preferences": prefs,
		"conversations": convs, "number": d.lastConv(person)})
}

func (d *Door) unread(person, conv string) bool {
	for _, m := range d.msgs {
		if m["conversation"] == conv && m["thread"] == nil && m["deleted"] != true && m["author"] != person &&
			!d.isRead(person, m, nil) && !d.isRead(person, m, m["id"]) {
			return true
		}
	}
	return false
}

// threadUnread: a reply past the person's position in that thread.
func (d *Door) threadUnread(person, conv string, root any) bool {
	at := d.reads[key(person, conv, root)]
	for _, m := range d.msgs {
		if m["thread"] == root && m["conversation"] == conv && m["deleted"] != true && m["author"] != person && num(m["number"]) > at {
			return true
		}
	}
	return false
}

func (d *Door) messages(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	q := r.URL.Query()
	conv, thread, as := r.PathValue("conversation"), q.Get("thread"), q.Get("as")
	list := []msg{}
	for _, m := range d.msgs {
		if m["conversation"] != conv {
			continue
		}
		if thread == "" && m["thread"] != nil || thread != "" && m["id"] != thread && m["thread"] != thread {
			continue
		}
		if m["deleted"] == true && d.replies(m["id"].(string)) == 0 {
			continue
		}
		list = append(list, m)
	}
	from, to := 0, len(list)
	limit, _ := strconv.Atoi(q.Get("limit"))
	if around := q.Get("around"); around != "" {
		n, _ := strconv.ParseInt(around, 10, 64)
		if limit <= 0 {
			limit = 50
		}
		at := sort.Search(len(list), func(i int) bool { return num(list[i]["number"]) >= n })
		from = max(0, at-limit/2)
		to = min(len(list), from+limit)
	} else if limit > 0 {
		from = max(0, len(list)-limit)
	}
	window := list[from:to]
	if thread != "" && from > 0 {
		// A thread's page always starts with its root: the screen shows it above the replies.
		window = append([]msg{list[0]}, window...)
	}
	out := []any{}
	for _, m := range window {
		var summary any
		if n := d.replies(m["id"].(string)); n > 0 && m["thread"] == nil {
			summary = map[string]any{"replies": n, "last_reply_at": d.lastReply(m["id"].(string)), "people": d.repliers(m["id"].(string)),
				"unread": as != "" && d.threadUnread(as, conv, m["id"])}
		}
		out = append(out, map[string]any{"message": m, "thread_summary": summary})
	}
	open := []any{}
	ids := make([]string, 0, len(d.inProgress))
	for id := range d.inProgress {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rep := d.inProgress[id]
		if rep.conversation == conv && (thread == "" && rep.thread == nil || rep.thread == any(thread)) {
			open = append(open, replyView(id, rep))
		}
	}
	number, readTo := d.number, any(nil)
	if as != "" {
		number = d.lastConv(as)
		var row any
		if thread != "" {
			row = thread
		}
		if at, ok := d.reads[key(as, conv, row)]; ok {
			readTo = at
		}
	}
	send(w, 200, map[string]any{"number": number, "messages": out, "in_progress": open, "read_up_to": readTo, "has_before": from > 0, "has_after": to < len(list)})
}

func replyView(id string, rep *replying) map[string]any {
	return map[string]any{"message": id, "conversation": rep.conversation, "thread": rep.thread, "author": rep.author, "at": len(rep.text), "text": string(rep.text)}
}

func (d *Door) replyText(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	rep := d.inProgress[r.PathValue("message")]
	if rep == nil {
		send(w, 409, map[string]string{"refused": "closed"})
		return
	}
	send(w, 200, replyView(r.PathValue("message"), rep))
}

func (d *Door) replies(id string) int {
	n := 0
	for _, m := range d.msgs {
		if m["thread"] == id && m["deleted"] != true {
			n++
		}
	}
	return n
}

func (d *Door) lastReply(id string) any {
	var at any
	for _, m := range d.msgs {
		if m["thread"] == id && m["deleted"] != true {
			at = m["created_at"]
		}
	}
	return at
}

func (d *Door) repliers(id string) []any {
	seen := map[any]bool{}
	out := []any{}
	for _, m := range d.msgs {
		if m["thread"] == id && m["deleted"] != true && !seen[m["author"]] {
			seen[m["author"]] = true
			out = append(out, m["author"])
		}
	}
	return out
}

func (d *Door) feed(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	person := r.PathValue("person")
	if d.feedDown {
		send(w, 200, map[string]any{"edition": nil, "through": nil, "number": nil, "rows": []any{}, "counts": nil, "notice": "Catching up"})
		return
	}
	rows := d.rowsFor(person)
	if d.rows[person] == nil { // the first load is where this person's row log starts
		d.rows[person] = rows
		d.histSeen[person] = d.historyCounts(person)
	}
	send(w, 200, map[string]any{"edition": d.edition, "through": d.number, "number": d.lastFeed(person),
		"rows": sortedRows(rows), "counts": counts(rows), "notice": nil})
}

// historyEntries is one history tab for one person, newest first.
func (d *Door) historyEntries(person, tab string) ([]any, int) {
	entries := []any{}
	unread := 0
	for i := len(d.msgs) - 1; i >= 0 && len(entries) < 50; i-- {
		m := d.msgs[i]
		if m["deleted"] == true {
			continue
		}
		c := d.convs[m["conversation"].(string)]
		if tab == "reactions" {
			if m["author"] != person {
				continue
			}
			for _, r := range m["reactions"].([]any) {
				rr := r.(map[string]any)
				for _, by := range rr["people"].([]any) {
					if by != person {
						entries = append(entries, map[string]any{"message": m["id"], "conversation": m["conversation"], "thread": m["thread"], "author": m["author"],
							"preview": m["text"], "at": m["created_at"], "number": m["number"], "unread": false, "by": by, "emoji": rr["emoji"], "level": nil, "kept": nil})
					}
				}
			}
			continue
		}
		if m["author"] == person {
			continue
		}
		match := false
		switch tab {
		case "mentions":
			match = contains(m["mentions"], person)
		case "direct":
			match = c != nil && c["type"] == "direct" && contains(c["people"], person)
		case "threads":
			match = m["thread"] != nil
		}
		if !match {
			continue
		}
		rowThread := m["thread"]
		if rowThread == nil && c != nil && c["type"] != "direct" {
			rowThread = m["id"]
		}
		isUnread := !d.isRead(person, m, rowThread)
		if isUnread {
			unread++
		}
		var level any
		if rowThread != nil && tab == "threads" {
			if l, ok := d.dials[key(person, m["conversation"].(string), rowThread)]; ok {
				level = l
			}
		}
		entries = append(entries, map[string]any{"message": m["id"], "conversation": m["conversation"], "thread": rowThread, "author": m["author"],
			"preview": m["text"], "at": m["created_at"], "number": m["number"], "unread": isUnread, "by": nil, "emoji": nil, "level": level, "kept": nil})
	}
	return entries, unread
}

func (d *Door) historyCounts(person string) map[string]int {
	out := map[string]int{}
	for _, tab := range []string{"direct", "mentions", "threads", "reactions"} {
		_, out[tab] = d.historyEntries(person, tab)
	}
	return out
}

func (d *Door) history(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "mentions"
	}
	entries, unread := d.historyEntries(r.PathValue("person"), tab)
	send(w, 200, map[string]any{"tab": tab, "entries": entries, "unread": unread, "next": nil})
}

func (d *Door) settings(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	levels := []any{}
	prefix := r.PathValue("person") + "|"
	names := make([]string, 0, len(d.dials))
	for k := range d.dials {
		if strings.HasPrefix(k, prefix) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	for _, k := range names {
		parts := strings.SplitN(k, "|", 3)
		var thread any
		if parts[2] != "<nil>" {
			thread = parts[2]
		}
		levels = append(levels, map[string]any{"conversation": parts[1], "thread": thread, "level": d.dials[k]})
	}
	send(w, 200, map[string]any{"focus_people": []any{map[string]any{"person": "anna", "items_last_7_days": 6}}, "removed_people": []any{}, "auto_add": true,
		"focus_only": false, "release_on_act": true, "rules": []any{map[string]any{"type": "item_kind", "value": "flags"}}, "removed_rules": []any{}, "levels": levels})
}

func (d *Door) search(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	q := strings.ToLower(r.URL.Query().Get("q"))
	results := []any{}
	for i := len(d.msgs) - 1; i >= 0 && q != ""; i-- {
		m := d.msgs[i]
		text := m["text"].(string)
		at := strings.Index(strings.ToLower(text), q)
		if m["deleted"] == true || at < 0 {
			continue
		}
		from := len([]rune(text[:at]))
		results = append(results, map[string]any{"message": m["id"], "number": m["number"], "created_at": m["created_at"], "version": m["version"], "conversation": m["conversation"],
			"thread": m["thread"], "author": m["author"], "edited": num(m["version"]) > 1, "text": text, "match": [][2]int{{from, from + len([]rune(q))}}})
	}
	send(w, 200, map[string]any{"answer": map[string]any{"through": d.number, "total": len(results), "next": nil, "results": results}, "notice": nil})
}

func (d *Door) suggest(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	q := strings.ToLower(r.URL.Query().Get("q"))
	out := []any{}
	for _, p := range d.people {
		if strings.HasPrefix(strings.ToLower(p["name"].(string)), q) {
			out = append(out, p)
		}
	}
	send(w, 200, map[string]any{"people": out, "everyone": strings.HasPrefix("channel", q)})
}

func (d *Door) act(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	var a map[string]any
	if err == nil {
		err = json.Unmarshal(raw, &a)
	}
	if err != nil {
		send(w, 400, map[string]string{"refused": "unreadable"})
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	id, _ := a["id"].(string)
	as, _ := a["as"].(string)
	dedupe := as + "|" + id
	if prev, ok := d.answers[dedupe]; ok {
		if prev.raw != string(raw) {
			send(w, 409, map[string]string{"refused": "id reused"})
			return
		}
		send(w, prev.status, prev.body)
		return
	}
	status, body := d.apply(a, as)
	d.answers[dedupe] = answer{status, body, string(raw)}
	send(w, status, body)
}

// rowThread is the row a message belongs to for this reader (docs/contracts/README.md).
func (d *Door) rowThread(m msg, person string) any {
	if m["thread"] != nil {
		return m["thread"]
	}
	if c := d.convs[m["conversation"].(string)]; c != nil && c["type"] != "direct" && contains(m["mentions"], person) {
		return m["id"]
	}
	return nil
}

// rowNewest is the newest message of a row, which keep, done and a reply read up to.
func (d *Door) rowNewest(conv string, thread any) int64 {
	var newest int64
	for _, m := range d.msgs {
		if m["conversation"] != conv || m["deleted"] == true {
			continue
		}
		if thread == nil && m["thread"] == nil || thread != nil && (m["id"] == thread || m["thread"] == thread) {
			newest = max(newest, num(m["number"]))
		}
	}
	return newest
}

func (d *Door) isAgent(person string) bool {
	for _, p := range d.people {
		if p["id"] == person {
			return p["is_agent"] == true
		}
	}
	return false
}

// moveRead moves a read position forward and says so to that person's screens.
func (d *Door) moveRead(personal map[string][]any, person, conv string, thread any, upTo int64) {
	k := key(person, conv, thread)
	if upTo > d.reads[k] {
		d.reads[k] = upTo
		personal[person] = append(personal[person], map[string]any{"kind": "read.moved", "conversation": conv, "thread": thread, "up_to": upTo})
	}
}

func (d *Door) apply(a map[string]any, as string) (int, any) {
	if d.refuse != "" {
		reason := d.refuse
		d.refuse = ""
		return 409, map[string]any{"refused": reason}
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	var parts []any
	personal := map[string][]any{}
	personalFeed := map[string][]any{}
	extra := map[string]any{}
	touched, root := "", any(nil)
	var marks map[markKey]bool
	touch := func(conv string, thread any) {
		touched, root = conv, thread
		marks = d.marks(conv, thread)
	}
	switch a["kind"] {
	case "send":
		in := a["message"].(map[string]any)
		conv, _ := in["conversation"].(string)
		c := d.convs[conv]
		if c == nil {
			return 409, map[string]any{"refused": "unknown conversation"}
		}
		touch(conv, in["thread"])
		number := d.number + 1
		m := msg{"id": in["id"], "number": number, "version": int64(1), "conversation": conv, "thread": in["thread"], "author": as, "author_is_agent": d.isAgent(as),
			"participants": nil, "text": in["text"], "mentions": in["mentions"], "level": in["level"], "reactions": []any{}, "stopped": false, "created_at": now, "edited_at": nil, "deleted": false}
		if c["type"] == "direct" {
			m["participants"] = c["people"]
		}
		d.msgs = append(d.msgs, m)
		d.byID[m["id"].(string)] = m
		parts = []any{map[string]any{"kind": "message.created", "message": m}}
		if rootID, ok := in["thread"].(string); ok {
			parts = append(parts, map[string]any{"kind": "thread.summary", "conversation": conv, "thread": rootID, "replies": d.replies(rootID), "last_reply_at": now, "people": d.repliers(rootID)})
			d.moveRead(personal, as, conv, rootID, number) // a reply reads its whole row
		}
		// An arrival on a row someone is watching is read for them in the same step.
		for cl := range d.clients {
			if cl.watches(conv, in["thread"]) && cl.person != as {
				d.moveRead(personal, cl.person, conv, in["thread"], number)
			}
		}
	case "edit", "delete", "react":
		m := d.byID[fmt.Sprint(a["message"])]
		if m == nil || m["deleted"] == true {
			return 409, map[string]any{"refused": "message deleted"}
		}
		conv := m["conversation"].(string)
		threadOf := m["thread"]
		if threadOf == nil {
			threadOf = m["id"]
		}
		touch(conv, threadOf)
		switch a["kind"] {
		case "edit":
			m["version"], m["text"], m["mentions"], m["level"], m["edited_at"] = num(m["version"])+1, a["text"], a["mentions"], a["level"], now
			parts = []any{map[string]any{"kind": "message.changed", "message": m}}
		case "delete":
			tombstone(m, num(m["version"])+1)
			parts = []any{map[string]any{"kind": "message.deleted", "message": map[string]any{"id": m["id"], "version": m["version"], "conversation": m["conversation"], "thread": m["thread"]}}}
			if rootID, ok := m["thread"].(string); ok {
				parts = append(parts, map[string]any{"kind": "thread.summary", "conversation": conv, "thread": rootID, "replies": d.replies(rootID), "last_reply_at": d.lastReply(rootID), "people": d.repliers(rootID)})
			}
		case "react":
			react(m, as, a["emoji"].(string), a["on"] == true)
			parts = []any{map[string]any{"kind": "reaction.changed", "message": m["id"], "conversation": m["conversation"], "thread": m["thread"], "reactions": m["reactions"]}}
			// D-035: reacting reads only up to the message reacted to.
			d.moveRead(personal, as, conv, d.rowThread(m, as), num(m["number"]))
		}
	case "read":
		conv := a["conversation"].(string)
		touch(conv, a["thread"])
		d.moveRead(personal, as, conv, a["thread"], num(a["up_to"]))
	case "keep", "done":
		conv := a["conversation"].(string)
		touch(conv, a["thread"])
		d.kept[key(as, conv, a["thread"])] = a["kind"] == "keep"
		d.moveRead(personal, as, conv, a["thread"], d.rowNewest(conv, a["thread"])) // acting implies read
	case "open_direct":
		other := a["with"].(string)
		pair := []string{as, other}
		sort.Strings(pair)
		id := "dm:" + pair[0] + ":" + pair[1]
		if d.convs[id] == nil {
			d.addConv(id, "direct", id, []any{pair[0], pair[1]})
			s := map[string]any{}
			for k, v := range d.convs[id] {
				s[k] = v
			}
			s["unread"] = false
			parts = []any{map[string]any{"kind": "conversation.added", "conversation_summary": s}}
			touched = id
		}
		extra["conversation"] = id
	case "create_channel":
		id := "#" + a["name"].(string)
		if d.convs[id] != nil {
			return 409, map[string]any{"refused": "name taken"}
		}
		people, _ := a["people"].([]any)
		d.addConv(id, "channel", a["name"].(string), append([]any{as}, people...))
		s := map[string]any{}
		for k, v := range d.convs[id] {
			s[k] = v
		}
		s["unread"] = false
		parts = []any{map[string]any{"kind": "conversation.added", "conversation_summary": s}}
		touched = id
		extra["conversation"] = id
	case "dial":
		k := key(as, a["conversation"].(string), a["thread"])
		if a["level"] == "auto" {
			delete(d.dials, k)
		} else {
			d.dials[k] = a["level"].(string)
		}
		personalFeed[as] = append(personalFeed[as], map[string]any{"kind": "settings.changed"})
	case "preference":
		if d.prefs[as] == nil {
			d.prefs[as] = map[string]string{}
		}
		d.prefs[as][a["key"].(string)] = a["value"].(string)
		personal[as] = append(personal[as], map[string]any{"kind": "preference", "key": a["key"], "value": a["value"]})
	case "add_people", "focus_rule", "focus_person", "focus_auto_add", "focus_only", "release_setting", "profile_line":
	default:
		return 409, map[string]any{"refused": "unknown kind"}
	}
	d.number++
	d.publish(parts, touched, root, marks, personal)
	d.pushFeed(personalFeed)
	out := map[string]any{"saved": true, "number": d.number}
	for k, v := range extra {
		out[k] = v
	}
	return 200, out
}

type markKey struct{ person, conv, thread string }

// marks are the unread marks a change could move: the conversation's, and the thread's.
func (d *Door) marks(conv string, root any) map[markKey]bool {
	out := map[markKey]bool{}
	c := d.convs[conv]
	if c == nil {
		return out
	}
	people, _ := c["people"].([]any)
	for _, p := range people {
		person := p.(string)
		out[markKey{person, conv, ""}] = d.unread(person, conv)
		if id, ok := root.(string); ok {
			out[markKey{person, conv, id}] = d.threadUnread(person, conv, id)
		}
	}
	return out
}

// concerns says whether a part of this conversation is sent to this person.
func (d *Door) concerns(person, conv string) bool {
	c := d.convs[conv]
	return c != nil && contains(c["people"], person)
}

// publish makes one numbered update per person it concerns, with an unread
// part for every mark the change moved, and sends it.
func (d *Door) publish(parts []any, conv string, root any, before map[markKey]bool, personal map[string][]any) {
	moved := map[string][]any{}
	if before != nil {
		for k, was := range d.marks(conv, root) {
			if had, ok := before[k]; ok && had != was {
				var thread any
				if k.thread != "" {
					thread = k.thread
				}
				moved[k.person] = append(moved[k.person], map[string]any{"kind": "unread", "conversation": k.conv, "thread": thread, "unread": was})
			}
		}
	}
	for _, p := range d.people {
		person := p["id"].(string)
		list := []any{}
		if conv != "" && d.concerns(person, conv) {
			list = append(list, parts...)
		}
		list = append(list, personal[person]...)
		list = append(list, moved[person]...)
		if len(list) == 0 {
			continue
		}
		raw, _ := json.Marshal(list)
		u := update{number: d.number, after: d.lastConv(person), parts: raw}
		d.convLog[person] = append(d.convLog[person], u)
		d.deliver(person, conversationsJSON(u))
	}
}

func conversationsJSON(u update) []byte {
	b, _ := json.Marshal(map[string]any{"stream": "conversations", "number": u.number, "after": u.after, "parts": u.parts})
	return b
}

func feedJSON(u update, edition int64) []byte {
	b, _ := json.Marshal(map[string]any{"stream": "feed", "number": u.number, "after": u.after, "edition": edition, "parts": u.parts})
	return b
}

// pushFeed tells each person whose feed has been loaded how their rows changed,
// also while they are away, so that they can catch up by number.
func (d *Door) pushFeed(extra map[string][]any) {
	if d.feedDown {
		return
	}
	for _, p := range d.people {
		person := p["id"].(string)
		before := d.rows[person]
		if before == nil {
			continue
		}
		after := d.rowsFor(person)
		parts := append([]any{}, extra[person]...)
		for k, r := range after {
			was, _ := json.Marshal(before[k])
			now, _ := json.Marshal(r)
			if string(was) != string(now) {
				parts = append(parts, map[string]any{"kind": "row.changed", "row": r})
			}
		}
		for k, r := range before {
			if after[k] == nil {
				parts = append(parts, map[string]any{"kind": "row.removed", "row": map[string]any{"conversation": r["conversation"], "thread": r["thread"]}})
			}
		}
		d.rows[person] = after
		seen := d.histSeen[person]
		now := d.historyCounts(person)
		for _, tab := range []string{"direct", "mentions", "threads", "reactions"} {
			if seen[tab] != now[tab] {
				parts = append(parts, map[string]any{"kind": "history.changed", "tab": tab, "unread": now[tab]})
			}
		}
		d.histSeen[person] = now
		if len(parts) == 0 {
			continue
		}
		parts = append(parts, map[string]any{"kind": "counts", "counts": counts(after)})
		d.feedNum++
		raw, _ := json.Marshal(parts)
		u := update{number: d.feedNum, after: d.lastFeed(person), parts: raw}
		d.feedLog[person] = append(d.feedLog[person], u)
		d.deliver(person, feedJSON(u, d.edition))
	}
}

func (c *client) watches(conv string, thread any) bool {
	return c.watching[key("", conv, thread)]
}

func (d *Door) live(w http.ResponseWriter, r *http.Request) {
	person := r.URL.Query().Get("person")
	d.mu.Lock()
	blocked, current, gone := d.down || d.refuseLive, d.org, d.retired[r.PathValue("org")]
	d.mu.Unlock()
	if blocked || person == "" || (!gone && r.PathValue("org") != current) {
		http.Error(w, "no", http.StatusServiceUnavailable)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if gone {
		b, _ := json.Marshal(map[string]any{"organization_gone": true})
		_ = conn.Write(ctx, websocket.MessageText, b)
		_ = conn.Close(websocket.StatusNormalClosure, "organization gone")
		return
	}
	c := &client{person: person, out: make(chan []byte, 2048), watching: map[string]bool{}}
	d.mu.Lock()
	d.clients[c] = true
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.clients, c)
		d.mu.Unlock()
	}()
	go func() {
		defer cancel()
		for {
			_, b, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var in struct {
				Resume *struct {
					Conversations int64
					Feed          *int64
					FeedEdition   *int64 `json:"feed_edition"`
				}
				Watching *struct {
					Conversation string
					Thread       *string
					Open         bool
				}
			}
			if json.Unmarshal(b, &in) != nil {
				continue
			}
			if in.Resume != nil {
				d.replay(c, in.Resume.Conversations, in.Resume.Feed, in.Resume.FeedEdition)
			}
			if in.Watching != nil {
				var thread any
				if in.Watching.Thread != nil {
					thread = *in.Watching.Thread
				}
				d.mu.Lock()
				if in.Watching.Open {
					c.watching[key("", in.Watching.Conversation, thread)] = true
				} else {
					delete(c.watching, key("", in.Watching.Conversation, thread))
				}
				d.mu.Unlock()
			}
		}
	}()
	beat := time.NewTicker(5 * time.Second)
	defer beat.Stop()
	for {
		select {
		case b, ok := <-c.out:
			if !ok {
				return
			}
			if conn.Write(ctx, websocket.MessageText, b) != nil {
				return
			}
		case <-beat.C:
			d.mu.Lock()
			b := d.lastSent(person)
			d.mu.Unlock()
			if conn.Write(ctx, websocket.MessageText, b) != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// replay answers a resume: what the screen has not had, or that it is too far behind.
func (d *Door) replay(c *client, conversations int64, feed, edition *int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.clients[c] {
		return
	}
	behind := func(stream string) {
		b, _ := json.Marshal(map[string]any{"too_far_behind": stream})
		push(c, b)
	}
	missing := func(log []update, from int64) []update {
		at := sort.Search(len(log), func(i int) bool { return log[i].number > from })
		return log[at:]
	}
	if list := missing(d.convLog[c.person], conversations); len(list) > replayMax {
		behind("conversations")
	} else {
		for _, u := range list {
			push(c, conversationsJSON(u))
		}
	}
	if feed == nil || d.feedDown {
		return
	}
	if edition != nil && *edition != d.edition {
		behind("feed")
		return
	}
	if list := missing(d.feedLog[c.person], *feed); len(list) > replayMax {
		behind("feed")
	} else {
		for _, u := range list {
			push(c, feedJSON(u, d.edition))
		}
	}
}

// Addr formats a port for the command.
func Addr(port int) string { return ":" + strconv.Itoa(port) }
