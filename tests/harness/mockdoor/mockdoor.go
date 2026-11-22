// Package mockdoor serves the people door and the live connection from memory,
// so the web app can be built before the real server exists. Its sorting into
// buckets is a deliberate simplification: the real rules live in the feed.
package mockdoor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"contexia/tests/harness/logwriter"
)

const org = "acme"

type msg = map[string]any

type update struct {
	number int64
	parts  []any
}

type client struct {
	person string
	out    chan []byte
}

// Door is the mock of Conversations' door for people.
type Door struct {
	mu       sync.Mutex
	number   int64
	feedNum  int64
	reset    int64
	edition  int64
	people   []map[string]any
	convs    map[string]map[string]any
	order    []string
	msgs     []msg
	byID     map[string]msg
	reads    map[string]int64
	kept     map[string]bool
	answers  map[string]answer
	convLog  []update
	feedLog  map[string][]update
	rows     map[string]map[string]map[string]any
	clients  map[*client]bool
	feedDown bool
	down     bool
	refuse   string
}

type answer struct {
	status int
	body   any
}

// New returns a door filled from a seeded log.
func New(seed int64, changes int) *Door {
	d := &Door{reset: 1, edition: 1, convs: map[string]map[string]any{}, byID: map[string]msg{}, reads: map[string]int64{},
		kept: map[string]bool{}, answers: map[string]answer{}, feedLog: map[string][]update{},
		rows: map[string]map[string]map[string]any{}, clients: map[*client]bool{}}
	for _, e := range logwriter.Write(logwriter.Config{Orgs: []string{org}, Seed: seed, Changes: changes, Reset: 1})[org] {
		var c map[string]any
		if err := json.Unmarshal(e.JSON, &c); err != nil {
			panic(err)
		}
		d.fold(c)
		d.number = e.Number
	}
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
		if d.reads[key(person, conv, thread)] >= num(m["number"]) {
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

func (d *Door) lastFeed(person string) int64 {
	if l := d.feedLog[person]; len(l) > 0 {
		return l[len(l)-1].number
	}
	return 0
}

// Handler serves the door and a /mock/control address for going wrong on purpose.
func (d *Door) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /alive", func(w http.ResponseWriter, _ *http.Request) {
		send(w, 200, map[string]string{"part": "mockdoor", "status": "alive"})
	})
	m.HandleFunc("POST /mock/control", d.control)
	m.HandleFunc("POST /orgs/{org}/people/actions", d.guard(d.act))
	m.HandleFunc("GET /orgs/{org}/people/{person}/start", d.guard(d.start))
	m.HandleFunc("GET /orgs/{org}/conversations/{conversation}/messages", d.guard(d.messages))
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
		down := d.down
		d.mu.Unlock()
		if down {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		if r.PathValue("org") != org {
			send(w, 404, map[string]string{"refused": "unknown organization"})
			return
		}
		next(w, r)
	}
}

func (d *Door) control(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Down, FeedDown *bool
		Refuse         *string
		Reset          bool
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
			for c := range d.clients {
				close(c.out)
				delete(d.clients, c)
			}
		}
	}
	if in.FeedDown != nil {
		d.feedDown = *in.FeedDown
	}
	if in.Refuse != nil {
		d.refuse = *in.Refuse
	}
	if in.Reset {
		d.reset++
		d.broadcast(map[string]any{"reset": d.reset})
	}
	send(w, 200, map[string]any{})
}

func (d *Door) broadcast(v any) {
	b, _ := json.Marshal(v)
	for c := range d.clients {
		select {
		case c.out <- b:
		default:
		}
	}
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
		convs = append(convs, s)
	}
	send(w, 200, map[string]any{"org": org, "org_name": "Acme", "person": me, "people": d.people, "preferences": []any{},
		"conversations": convs, "reset": d.reset, "number": d.number})
}

func (d *Door) unread(person, conv string) bool {
	for _, m := range d.msgs {
		if m["conversation"] == conv && m["thread"] == nil && m["deleted"] != true && m["author"] != person && num(m["number"]) > d.reads[key(person, conv, nil)] {
			return true
		}
	}
	return false
}

func (d *Door) messages(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	conv, thread := r.PathValue("conversation"), r.URL.Query().Get("thread")
	out := []any{}
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
		var summary any
		if n := d.replies(m["id"].(string)); n > 0 && m["thread"] == nil {
			summary = map[string]any{"replies": n, "last_reply_at": d.lastReply(m["id"].(string)), "people": d.repliers(m["id"].(string)), "unread": false}
		}
		out = append(out, map[string]any{"message": m, "thread_summary": summary})
	}
	send(w, 200, map[string]any{"number": d.number, "messages": out, "in_progress": []any{}, "read_up_to": nil, "has_before": false, "has_after": false})
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
		send(w, 200, map[string]any{"reset": d.reset, "edition": nil, "through": nil, "number": nil, "rows": []any{}, "counts": nil, "notice": "Catching up"})
		return
	}
	rows := d.rowsFor(person)
	d.rows[person] = rows
	send(w, 200, map[string]any{"reset": d.reset, "edition": d.edition, "through": d.number, "number": d.lastFeed(person),
		"rows": sortedRows(rows), "counts": counts(rows), "notice": nil})
}

func (d *Door) history(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	person, tab := r.PathValue("person"), r.URL.Query().Get("tab")
	if tab == "" {
		tab = "mentions"
	}
	entries := []any{}
	unread := 0
	for i := len(d.msgs) - 1; i >= 0 && len(entries) < 50; i-- {
		m := d.msgs[i]
		if m["deleted"] == true || m["author"] == person {
			continue
		}
		c := d.convs[m["conversation"].(string)]
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
		isUnread := d.reads[key(person, m["conversation"].(string), m["thread"])] < num(m["number"])
		if isUnread {
			unread++
		}
		entries = append(entries, map[string]any{"message": m["id"], "conversation": m["conversation"], "thread": m["thread"], "author": m["author"],
			"preview": m["text"], "at": m["created_at"], "number": m["number"], "unread": isUnread, "by": nil, "emoji": nil, "level": nil, "kept": nil})
	}
	send(w, 200, map[string]any{"tab": tab, "entries": entries, "unread": unread, "next": nil})
}

func (d *Door) settings(w http.ResponseWriter, _ *http.Request) {
	send(w, 200, map[string]any{"focus_people": []any{map[string]any{"person": "anna", "items_last_7_days": 6}}, "removed_people": []any{}, "auto_add": true,
		"focus_only": false, "release_on_act": true, "rules": []any{map[string]any{"type": "item_kind", "value": "flags"}}, "removed_rules": []any{}, "levels": []any{}})
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
	var a map[string]any
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		send(w, 400, map[string]string{"refused": "unreadable"})
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	id, _ := a["id"].(string)
	as, _ := a["as"].(string)
	dedupe := as + "|" + id
	if prev, ok := d.answers[dedupe]; ok {
		send(w, prev.status, prev.body)
		return
	}
	status, body := d.apply(a, as)
	d.answers[dedupe] = answer{status, body}
	send(w, status, body)
}

func (d *Door) apply(a map[string]any, as string) (int, any) {
	if d.refuse != "" {
		reason := d.refuse
		d.refuse = ""
		return 409, map[string]any{"refused": reason}
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	var parts []any
	extra := map[string]any{}
	switch a["kind"] {
	case "send":
		in := a["message"].(map[string]any)
		number := d.number + 1
		m := msg{"id": in["id"], "number": number, "version": int64(1), "conversation": in["conversation"], "thread": in["thread"], "author": as, "author_is_agent": false,
			"participants": nil, "text": in["text"], "mentions": in["mentions"], "level": in["level"], "reactions": []any{}, "stopped": false, "created_at": now, "edited_at": nil, "deleted": false}
		if c := d.convs[in["conversation"].(string)]; c == nil {
			return 409, map[string]any{"refused": "unknown conversation"}
		} else if c["type"] == "direct" {
			m["participants"] = c["people"]
		}
		d.msgs = append(d.msgs, m)
		d.byID[m["id"].(string)] = m
		parts = []any{map[string]any{"kind": "message.created", "message": m}}
		if root, ok := in["thread"].(string); ok {
			parts = append(parts, map[string]any{"kind": "thread.summary", "conversation": in["conversation"], "thread": root, "replies": d.replies(root), "last_reply_at": now, "people": d.repliers(root)})
		}
	case "edit", "delete", "react":
		m := d.byID[fmt.Sprint(a["message"])]
		if m == nil || m["deleted"] == true {
			return 409, map[string]any{"refused": "message deleted"}
		}
		switch a["kind"] {
		case "edit":
			m["version"], m["text"], m["mentions"], m["level"], m["edited_at"] = num(m["version"])+1, a["text"], a["mentions"], a["level"], now
			parts = []any{map[string]any{"kind": "message.changed", "message": m}}
		case "delete":
			tombstone(m, num(m["version"])+1)
			parts = []any{map[string]any{"kind": "message.deleted", "message": map[string]any{"id": m["id"], "version": m["version"], "conversation": m["conversation"], "thread": m["thread"]}}}
		case "react":
			react(m, as, a["emoji"].(string), a["on"] == true)
			parts = []any{map[string]any{"kind": "reaction.changed", "message": m["id"], "conversation": m["conversation"], "thread": m["thread"], "reactions": m["reactions"]}}
		}
	case "read":
		k := key(as, a["conversation"].(string), a["thread"])
		if up := num(a["up_to"]); up > d.reads[k] {
			d.reads[k] = up
			parts = []any{map[string]any{"kind": "read.moved", "conversation": a["conversation"], "thread": a["thread"], "up_to": up}}
		}
	case "keep", "done":
		d.kept[key(as, a["conversation"].(string), a["thread"])] = a["kind"] == "keep"
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
		extra["conversation"] = id
	case "add_people", "dial", "focus_rule", "focus_person", "focus_auto_add", "focus_only", "release_setting", "preference", "profile_line":
	default:
		return 409, map[string]any{"refused": "unknown kind"}
	}
	d.number++
	if parts != nil {
		prev := int64(0)
		if len(d.convLog) > 0 {
			prev = d.convLog[len(d.convLog)-1].number
		}
		d.convLog = append(d.convLog, update{d.number, parts})
		d.broadcast(map[string]any{"stream": "conversations", "number": d.number, "after": prev, "parts": parts})
	}
	d.pushFeed()
	out := map[string]any{"saved": true, "number": d.number}
	for k, v := range extra {
		out[k] = v
	}
	return 200, out
}

// pushFeed tells each connected person how their rows changed.
func (d *Door) pushFeed() {
	if d.feedDown {
		return
	}
	done := map[string]bool{}
	for c := range d.clients {
		if done[c.person] {
			continue
		}
		done[c.person] = true
		before, after := d.rows[c.person], d.rowsFor(c.person)
		parts := []any{}
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
		d.rows[c.person] = after
		if len(parts) == 0 {
			continue
		}
		parts = append(parts, map[string]any{"kind": "counts", "counts": counts(after)})
		d.feedNum++
		prev := d.lastFeed(c.person)
		d.feedLog[c.person] = append(d.feedLog[c.person], update{d.feedNum, parts})
		b, _ := json.Marshal(map[string]any{"stream": "feed", "number": d.feedNum, "after": prev, "edition": d.edition, "parts": parts})
		for other := range d.clients {
			if other.person == c.person {
				select {
				case other.out <- b:
				default:
				}
			}
		}
	}
}

func (d *Door) live(w http.ResponseWriter, r *http.Request) {
	person := r.URL.Query().Get("person")
	d.mu.Lock()
	down := d.down
	d.mu.Unlock()
	if down || person == "" || r.PathValue("org") != org {
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
	c := &client{person: person, out: make(chan []byte, 1024)}
	d.mu.Lock()
	d.clients[c] = true
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		if d.clients[c] {
			delete(d.clients, c)
		}
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
					Reset         int64
					Conversations int64
					Feed          *int64
				}
			}
			if json.Unmarshal(b, &in) == nil && in.Resume != nil {
				d.replay(c, in.Resume.Reset, in.Resume.Conversations, in.Resume.Feed)
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
			last := int64(0)
			if len(d.convLog) > 0 {
				last = d.convLog[len(d.convLog)-1].number
			}
			b, _ := json.Marshal(map[string]any{"last_sent": map[string]any{"conversations": last, "feed": d.lastFeed(person), "feed_edition": d.edition}})
			d.mu.Unlock()
			if conn.Write(ctx, websocket.MessageText, b) != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *Door) replay(c *client, reset, conversations int64, feed *int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	push := func(v any) {
		b, _ := json.Marshal(v)
		select {
		case c.out <- b:
		default:
		}
	}
	if reset != d.reset {
		push(map[string]any{"reset": d.reset})
		return
	}
	prev := conversations
	for _, u := range d.convLog {
		if u.number > conversations {
			push(map[string]any{"stream": "conversations", "number": u.number, "after": prev, "parts": u.parts})
			prev = u.number
		}
	}
	if feed != nil {
		prev = *feed
		for _, u := range d.feedLog[c.person] {
			if u.number > *feed {
				push(map[string]any{"stream": "feed", "number": u.number, "after": prev, "edition": d.edition, "parts": u.parts})
				prev = u.number
			}
		}
	}
}

// Addr formats a port for the command.
func Addr(port int) string { return ":" + strconv.Itoa(port) }
