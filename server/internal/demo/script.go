package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"contexia/internal/democontent"
	"contexia/internal/shared/call"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
)

const (
	keepHappened = 100
	showNext     = 12
	stepBudgetS  = 30 * time.Second // how long a step waits out an outage before it is reported
)

// play is one playing of one situation, with the names its steps gave to messages.
type play struct {
	sit     *democontent.Situation
	id      string
	next    int
	onCall  bool // asked for with "Play now": runs even while the script is paused
	aliases map[string]*sent
}

// sent is a message the team wrote.
type sent struct {
	id, conv string
	root     *sent
	level    *contract.Level
}

// team is the scripted team of one organization.
type team struct {
	s    *Service
	org  ids.Org
	stop context.CancelFunc
	wake chan struct{}

	mu       sync.Mutex
	running  bool
	pace     float64
	waited   float64 // seconds of the current delay already waited, at script speed
	onCall   []*play
	plan     []*play
	happened []contract.ScriptStep
	plays    int
	top      map[string]int64 // newest top-level number per conversation
	inThread map[string]int64 // newest number per thread
}

func (s *Service) teamFor(org ids.Org) *team {
	s.teamsMu.Lock()
	defer s.teamsMu.Unlock()
	if t, ok := s.teams[string(org)]; ok {
		return t
	}
	ctx, cancel := context.WithCancel(s.life)
	t := &team{s: s, org: org, stop: cancel, wake: make(chan struct{}, 1), pace: 1,
		top: map[string]int64{}, inThread: map[string]int64{}}
	t.plan = t.schedule()
	s.teams[string(org)] = t
	go t.loop(ctx)
	return t
}

// stopTeam ends an organization's script and says how it was running.
func (s *Service) stopTeam(org string) (running bool, pace float64) {
	s.teamsMu.Lock()
	t, ok := s.teams[org]
	delete(s.teams, org)
	s.teamsMu.Unlock()
	if !ok {
		return false, 1
	}
	t.mu.Lock()
	running, pace = t.running, t.pace
	t.running = false
	t.mu.Unlock()
	t.stop()
	return running, pace
}

func (t *team) schedule() []*play {
	var out []*play
	for i := range t.s.cfg.Content.Situations {
		out = append(out, t.newPlay(&t.s.cfg.Content.Situations[i], false))
	}
	return out
}

func (t *team) newPlay(sit *democontent.Situation, onCall bool) *play {
	t.plays++
	return &play{sit: sit, id: fmt.Sprintf("%s-%d", ids.New("p"), t.plays), onCall: onCall, aliases: map[string]*sent{}}
}

func (t *team) nudge() {
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

// command applies one control command and reports a refusal in words.
func (t *team) command(in contract.ScriptRequest) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	defer t.nudge()
	switch in.Do {
	case "start":
		if len(t.plan) == 0 {
			t.plan = t.schedule() // played through: start again from the first situation
		}
		t.running = true
	case "pause":
		t.running = false
	case "pace":
		if in.Pace == nil || *in.Pace <= 0 || *in.Pace > 16 {
			return "pace is a number above 0, up to 16"
		}
		t.pace = *in.Pace
	case "play_situation":
		if in.Situation == nil {
			return "no situation named"
		}
		list := t.s.cfg.Content.Situations
		i := slices.IndexFunc(list, func(x democontent.Situation) bool { return x.ID == *in.Situation })
		if i < 0 {
			return "unknown situation"
		}
		// It comes off the schedule unless it has already begun there, so it is not played twice in a row.
		t.plan = slices.DeleteFunc(t.plan, func(p *play) bool { return p.sit.ID == *in.Situation && p.next == 0 })
		t.onCall = append(t.onCall, t.newPlay(&list[i], true))
	default:
		return "unknown command"
	}
	return ""
}

// due is what plays next and how long before it, in script seconds.
func (t *team) due() (*play, float64) {
	var p *play
	switch {
	case len(t.onCall) > 0:
		p = t.onCall[0]
	case t.running && len(t.plan) > 0:
		p = t.plan[0]
	default:
		return nil, 0
	}
	delay := p.sit.Steps[p.next].AfterS
	if p.next == 0 && !p.onCall && t.plays > 0 && len(t.happened) > 0 {
		delay += t.s.cfg.Gap.Seconds()
	}
	return p, max(delay-t.waited, 0)
}

func (t *team) loop(ctx context.Context) {
	for ctx.Err() == nil {
		t.mu.Lock()
		p, delay := t.due()
		pace := t.pace
		t.mu.Unlock()
		if p == nil {
			select {
			case <-ctx.Done():
			case <-t.wake:
			}
			continue
		}
		if delay > 0 {
			started := t.s.cfg.Clock.Now()
			select {
			case <-ctx.Done():
				return
			case <-t.wake:
				// Anything may have changed. Keep what was already waited.
				t.mu.Lock()
				t.waited += t.s.cfg.Clock.Now().Sub(started).Seconds() * pace
				t.mu.Unlock()
				continue
			case <-t.s.cfg.Clock.After(time.Duration(delay / pace * float64(time.Second))):
			}
		}
		t.mu.Lock()
		if q, _ := t.due(); q != p { // paused or overtaken while the timer ran
			t.mu.Unlock()
			continue
		}
		step := p.sit.Steps[p.next]
		index := p.next
		t.waited = 0
		t.mu.Unlock()

		err := t.perform(ctx, p, index, step)
		if ctx.Err() != nil {
			return
		}
		t.mu.Lock()
		line := contract.ScriptStep{At: t.s.cfg.Clock.Now().UTC().Format(time.RFC3339), Who: step.Who, What: describe(step), Situation: &p.sit.ID}
		if err != nil {
			line.What = "could not " + line.What + " (" + err.Error() + ")"
			slog.Warn("demo: step failed", "org", t.org, "situation", p.sit.ID, "step", index, "err", err)
		}
		t.happened = append(t.happened, line)
		if len(t.happened) > keepHappened {
			t.happened = slices.Delete(t.happened, 0, len(t.happened)-keepHappened)
		}
		p.next++
		if p.next >= len(p.sit.Steps) {
			if len(t.onCall) > 0 && t.onCall[0] == p {
				t.onCall = t.onCall[1:]
			} else if len(t.plan) > 0 && t.plan[0] == p {
				t.plan = t.plan[1:]
				if len(t.plan) == 0 {
					t.running = false
				}
			}
		}
		t.mu.Unlock()
	}
}

func describe(st democontent.Step) string {
	quote := st.Text
	if len(quote) > 60 {
		quote = strings.ToValidUTF8(quote[:60], "") + "…"
	}
	switch st.Do {
	case "post":
		return "posts in " + st.In + ": " + quote
	case "reply":
		return "replies: " + quote
	case "edit":
		return "edits a message: " + quote
	case "delete":
		return "deletes a message"
	case "react":
		return "reacts with " + st.Emoji
	case "unreact":
		return "takes back a " + st.Emoji + " reaction"
	case "read":
		return "reads the conversation"
	case "keep":
		return "keeps a row"
	case "done":
		return "marks a row done"
	case "dial":
		return "sets " + st.In + " to " + st.Dial
	}
	return st.Do
}

// state is the script's answer to the control panel.
func (t *team) state() contract.ScriptState {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := contract.ScriptState{Running: t.running, Pace: t.pace, Happened: []contract.ScriptStep{}, Next: []contract.ScriptStep{}, Situations: []contract.Situation{}}
	for i := len(t.happened) - 1; i >= 0; i-- {
		out.Happened = append(out.Happened, t.happened[i])
	}
	for _, sit := range t.s.cfg.Content.Situations {
		out.Situations = append(out.Situations, contract.Situation{Id: sit.ID, Title: sit.Title, Outcome: sit.Outcome})
	}
	at := t.s.cfg.Clock.Now()
	offset := -t.waited
	for _, list := range [][]*play{t.onCall, t.plan} {
		for _, p := range list {
			for i := p.next; i < len(p.sit.Steps) && len(out.Next) < showNext; i++ {
				st := p.sit.Steps[i]
				offset += st.AfterS
				if i == 0 && !p.onCall && len(t.happened) > 0 {
					offset += t.s.cfg.Gap.Seconds()
				}
				when := at.Add(time.Duration(max(offset, 0) / t.pace * float64(time.Second)))
				id := p.sit.ID
				out.Next = append(out.Next, contract.ScriptStep{At: when.UTC().Format(time.RFC3339), Who: st.Who, What: describe(st), Situation: &id})
			}
		}
	}
	return out
}

var mentionPattern = regexp.MustCompile(`@(\p{L}+)`)

// mentions resolves @Firstname as the seeded history does.
func (t *team) mentions(text string) []string {
	out := []string{}
	for _, m := range mentionPattern.FindAllStringSubmatch(text, -1) {
		for _, p := range t.s.cfg.Content.Org.People {
			if strings.EqualFold(strings.Fields(p.Name)[0], m[1]) && !slices.Contains(out, p.ID) {
				out = append(out, p.ID)
			}
		}
	}
	return out
}

func (t *team) isAgent(id string) bool {
	return slices.ContainsFunc(t.s.cfg.Content.Org.People, func(p democontent.Person) bool { return p.ID == id && p.Agent })
}

func levelOf(text string) *contract.Level {
	if text == "" || text == "none" || text == "same" {
		return nil
	}
	l := contract.Level(text)
	return &l
}

// perform acts one step through the door a real client would use. The ID is fixed
// per step of this play, so repeating after a lost answer saves it once.
func (t *team) perform(ctx context.Context, p *play, index int, st democontent.Step) error {
	id := fmt.Sprintf("%s-%d", p.id, index)
	target := p.aliases[st.To]
	var thread *string
	if target != nil {
		root := target
		if target.root != nil {
			root = target.root
		}
		thread = &root.id
	}
	switch st.Do {
	case "post", "reply":
		conv := st.In
		var root *sent
		var threadID *string
		if st.Do == "reply" {
			if target == nil {
				return errors.New("nothing to reply to")
			}
			conv, root, threadID = target.conv, target, thread
			if target.root != nil {
				root = target.root
			}
		}
		m := &sent{conv: conv, root: root, level: levelOf(st.Level)}
		var number int64
		var err error
		if t.isAgent(st.Who) {
			m.id, number, err = t.agentPost(ctx, st, id, conv, threadID, m.level)
		} else {
			m.id = "msg_" + strings.ReplaceAll(id, "-", "_")
			number, err = t.act(ctx, contract.ActionSend{Kind: "send", Id: id, As: st.Who, Message: contract.NewMessage{
				Id: m.id, Conversation: conv, Thread: threadID, Text: st.Text, Mentions: t.mentions(st.Text), Level: m.level}})
		}
		if err != nil {
			return err
		}
		t.mu.Lock()
		if root == nil {
			t.top[conv] = number
			t.inThread[m.id] = number
		} else {
			t.inThread[root.id] = number
		}
		t.mu.Unlock()
		if st.As != "" {
			p.aliases[st.As] = m
		}
		return nil
	}
	if st.Do != "dial" && target == nil {
		return fmt.Errorf("no message named %q", st.To)
	}
	switch st.Do {
	case "edit":
		level := target.level
		if st.Level != "" && st.Level != "same" {
			level = levelOf(st.Level)
		}
		if _, err := t.act(ctx, contract.ActionEdit{Kind: "edit", Id: id, As: st.Who, Message: target.id, Text: st.Text, Mentions: t.mentions(st.Text), Level: level}); err != nil {
			return err
		}
		target.level = level
		return nil
	case "delete":
		_, err := t.act(ctx, contract.ActionDelete{Kind: "delete", Id: id, As: st.Who, Message: target.id})
		return err
	case "react", "unreact":
		on := st.Do == "react"
		if t.isAgent(st.Who) {
			return t.agentCall(ctx, st.Who, "/reactions", contract.AgentReaction{Id: id, Message: target.id, Emoji: st.Emoji, On: on}, nil)
		}
		_, err := t.act(ctx, contract.ActionReact{Kind: "react", Id: id, As: st.Who, Message: target.id, Emoji: st.Emoji, On: on})
		return err
	case "read":
		row := rowOf(target)
		t.mu.Lock()
		upTo := t.top[target.conv]
		if row != nil {
			upTo = t.inThread[*row]
		}
		t.mu.Unlock()
		_, err := t.act(ctx, contract.ActionRead{Kind: "read", Id: id, As: st.Who, Conversation: target.conv, Thread: row, UpTo: upTo})
		return err
	case "keep":
		_, err := t.act(ctx, contract.ActionKeep{Kind: "keep", Id: id, As: st.Who, Conversation: target.conv, Thread: rowOf(target)})
		return err
	case "done":
		_, err := t.act(ctx, contract.ActionDone{Kind: "done", Id: id, As: st.Who, Conversation: target.conv, Thread: rowOf(target)})
		return err
	case "dial":
		var dialThread *string
		if target != nil {
			dialThread = &target.id
		}
		_, err := t.act(ctx, contract.ActionDial{Kind: "dial", Id: id, As: st.Who, Conversation: st.In, Thread: dialThread, Level: contract.DialLevel(st.Dial)})
		return err
	}
	return fmt.Errorf("unknown act %q", st.Do)
}

// rowOf is the row Keep, Done and read name: the thread, or the direct conversation.
func rowOf(m *sent) *string {
	if strings.HasPrefix(m.conv, "dm:") {
		return nil
	}
	if m.root != nil {
		return &m.root.id
	}
	return &m.id
}

func (t *team) act(ctx context.Context, action any) (int64, error) {
	var saved contract.ActionSaved
	path := "/orgs/" + string(t.org) + "/people/actions"
	err := t.s.retrying(ctx, stepBudgetS, func() error {
		return t.s.conv.JSON(ctx, http.MethodPost, path, action, &saved)
	})
	if err != nil {
		return 0, plain(err)
	}
	if saved.Number == nil {
		return 0, nil
	}
	return *saved.Number, nil
}

func (t *team) agentPost(ctx context.Context, st democontent.Step, id, conv string, thread *string, level *contract.Level) (string, int64, error) {
	var posted contract.AgentPosted
	err := t.agentCall(ctx, st.Who, "/messages", contract.AgentPost{Id: id, Conversation: conv, Thread: thread, Text: st.Text, Level: level, Mentions: t.mentions(st.Text)}, &posted)
	return posted.Message, posted.Number, err
}

func (t *team) agentCall(ctx context.Context, agent, path string, in, out any) error {
	full := "/orgs/" + string(t.org) + "/agents/" + agent + path
	err := t.s.retrying(ctx, stepBudgetS, func() error {
		return t.s.conv.JSON(ctx, http.MethodPost, full, in, out)
	})
	return plain(err)
}

// plain words a door's answer for the "what happened" list.
func plain(err error) error {
	var refused *call.Refused
	if !errors.As(err, &refused) {
		return err
	}
	var body contract.Refusal
	if json.Unmarshal([]byte(refused.Body), &body) == nil && body.Refused != "" {
		return errors.New(body.Refused)
	}
	return err
}

func (s *Service) handleScript(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamOr(w, r)
	if ok {
		answer(w, http.StatusOK, t.state())
	}
}

func (s *Service) handleCommand(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamOr(w, r)
	if !ok {
		return
	}
	var in contract.ScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		answer(w, http.StatusBadRequest, contract.Refusal{Refused: "unreadable body"})
		return
	}
	if why := t.command(in); why != "" {
		answer(w, http.StatusBadRequest, contract.Refusal{Refused: why})
		return
	}
	answer(w, http.StatusOK, t.state())
}

// teamOr finds the team of the current organization, or answers why there is none.
func (s *Service) teamOr(w http.ResponseWriter, r *http.Request) (*team, bool) {
	org, ok := orgParam(w, r, "org")
	if !ok {
		return nil, false
	}
	state, err := s.stateOf(r.Context(), org)
	switch {
	case err != nil:
		notAvailable(w, err)
	case state == "retired":
		answer(w, http.StatusGone, contract.OrganizationGone{OrganizationGone: true})
	case state != "current":
		answer(w, http.StatusNotFound, contract.Refusal{Refused: "not the current organization"})
	default:
		return s.teamFor(org), true
	}
	return nil, false
}
