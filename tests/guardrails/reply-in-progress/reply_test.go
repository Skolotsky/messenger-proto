// Package replyinprogress holds G-017: text of a reply in progress never reaches
// search, the feed or the notification sender; a reply with no piece for 60 s, or
// older than 10 minutes, is closed as "stopped" with the text so far.
package replyinprogress

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	r "contexia/tests/guardrails/agent-events/agentrig"
)

const secret = "unfinished-words"

type created struct {
	Message struct {
		Id       string   `json:"id"`
		Text     string   `json:"text"`
		Stopped  bool     `json:"stopped"`
		Mentions []string `json:"mentions"`
		Number   int64    `json:"number"`
	} `json:"message"`
	Number int64 `json:"number"`
}

func start(h *r.H, id string) string {
	h.T.Helper()
	s, a := h.Agent(r.Release, "POST", "/replies", map[string]any{"id": id, "conversation": r.Launch, "thread": r.Root, "because_of": nil})
	if s != 200 {
		h.T.Fatalf("starting a reply answered %d: %v", s, a)
	}
	return a["message"].(string)
}

func piece(h *r.H, id string, at int, text string) (int, map[string]any) {
	h.T.Helper()
	return h.Agent(r.Release, "POST", "/replies/"+id+"/pieces", map[string]any{"at": at, "text": text})
}

func finish(h *r.H, id, done string) (int, map[string]any) {
	h.T.Helper()
	return h.Agent(r.Release, "POST", "/replies/"+id+"/finish", map[string]any{"id": done, "level": nil, "mentions": []string{r.Marco}})
}

func messagesOf(h *r.H, message string) []created {
	h.T.Helper()
	var out []created
	for _, e := range h.Log() {
		if e.Kind != "message.created" || !strings.Contains(e.Body, message) {
			continue
		}
		var c created
		if err := json.Unmarshal([]byte(e.Body), &c); err != nil {
			h.T.Fatal(err)
		}
		if c.Message.Id == message {
			out = append(out, c)
		}
	}
	return out
}

func kindCount(h *r.H, kind string) int {
	n := 0
	for _, e := range h.Log() {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// offLogProblems: while a reply is written the log holds only a text-free start,
// the text is there for a screen to fetch, and the finished reply is one message.
func offLogProblems(h *r.H) []string {
	var p []string
	var seen []string
	h.Store.SetAgentPush(func(_ idsOrg, _ string, update liveOut) {
		b, _ := json.Marshal(update)
		seen = append(seen, string(b))
	})
	message := start(h, "a-1")
	for i, text := range []string{secret + " one ", "two "} {
		at := 0
		if i == 1 {
			at = len([]rune(secret + " one "))
		}
		if s, a := piece(h, "a-1", at, text); s != 200 {
			p = append(p, fmt.Sprintf("a piece answered %d: %v", s, a))
		}
	}
	h.Housekeep() // the text so far is saved, and must still not be on the log
	if held := h.LogHolding(secret); len(held) > 0 {
		p = append(p, fmt.Sprintf("the log holds text of a reply in progress, in %d entries, first %s", len(held), held[0].Kind))
	}
	if n := kindCount(h, "reply.started"); n != 1 {
		p = append(p, fmt.Sprintf("the log has %d reply.started, expected 1", n))
	}
	if n := len(messagesOf(h, message)); n != 0 {
		p = append(p, "an unfinished reply is a message on the log")
	}
	if len(seen) != 2 || !strings.Contains(seen[0], secret) {
		p = append(p, fmt.Sprintf("open screens were pushed %v", seen))
	}
	status, raw := h.Call("GET", fmt.Sprintf("/orgs/%s/replies/%s", r.Org, message), nil)
	var text struct {
		At   int    `json:"at"`
		Text string `json:"text"`
	}
	_ = json.Unmarshal(raw, &text)
	if whole := secret + " one two "; status != 200 || text.Text != whole || text.At != len([]rune(whole)) {
		p = append(p, fmt.Sprintf("the text so far answered %d %s", status, raw))
	}
	if s, a := finish(h, "a-1", "a-1-done"); s != 200 || a["message"] != message {
		p = append(p, fmt.Sprintf("finishing answered %d: %v", s, a))
		return p
	}
	if s, a := finish(h, "a-1", "a-1-done"); s != 200 || a["message"] != message {
		p = append(p, fmt.Sprintf("finishing again with the same ID answered %d: %v", s, a))
	}
	got := messagesOf(h, message)
	if len(got) != 1 || got[0].Message.Text != secret+" one two " || got[0].Message.Stopped {
		p = append(p, fmt.Sprintf("the finished reply is on the log as %+v", got))
	} else if got[0].Message.Number != got[0].Number {
		p = append(p, "the finished message does not have the number of its message.created")
	}
	if s, _ := piece(h, "a-1", 0, "more"); s != http.StatusConflict {
		p = append(p, fmt.Sprintf("a piece after finishing answered %d, expected 409 closed", s))
	}
	return p
}

func TestTextInProgressStaysOffTheLog(t *testing.T) {
	for _, p := range offLogProblems(r.New(t)) {
		t.Error(p)
	}
}

// piecesProblems: positions count code points, a repeat adds nothing, a gap is
// answered with what the door has.
func piecesProblems(h *r.H) []string {
	var p []string
	message := start(h, "a-1")
	steps := []struct {
		at     int
		text   string
		status int
		have   float64
	}{
		{0, "héllo🙂", 200, 0},
		{0, "héllo🙂", 200, 0}, // the same piece again
		{2, "llo🙂 w", 200, 0}, // overlaps from position 2, adds " w"
		{20, "far ahead", http.StatusConflict, 8},
		{8, "orld", 200, 0},
	}
	for _, c := range steps {
		s, a := piece(h, "a-1", c.at, c.text)
		if s != c.status {
			p = append(p, fmt.Sprintf("a piece at %d answered %d, expected %d", c.at, s, c.status))
		}
		if c.status == http.StatusConflict && a["have"] != c.have {
			p = append(p, fmt.Sprintf("a missing piece answered have %v, expected %v", a["have"], c.have))
		}
	}
	finish(h, "a-1", "a-1-done")
	got := messagesOf(h, message)
	if len(got) != 1 || got[0].Message.Text != "héllo🙂 world" {
		p = append(p, fmt.Sprintf("the pieces came to %+v, expected one message saying héllo🙂 world", got))
	}
	return p
}

func TestPiecesAreIdentifiedByPosition(t *testing.T) {
	for _, p := range piecesProblems(r.New(t)) {
		t.Error(p)
	}
}

// silenceProblems: 60 s with no new piece closes the reply as stopped with the
// text so far, searchable, with no mentions, and a repeat does not keep it open.
func silenceProblems(h *r.H) []string {
	var p []string
	message := start(h, "a-1")
	if s, _ := piece(h, "a-1", 0, "text so far"); s != 200 {
		p = append(p, "the piece was not taken")
	}
	h.Housekeep()
	h.Clock.Advance(59 * time.Second)
	piece(h, "a-1", 0, "text so far") // adds nothing, so it must not extend the time
	h.Housekeep()
	if n := len(messagesOf(h, message)); n != 0 {
		p = append(p, "the reply closed before 60 s of silence")
	}
	h.Clock.Advance(2 * time.Second)
	h.Housekeep()
	got := messagesOf(h, message)
	if len(got) != 1 {
		p = append(p, fmt.Sprintf("after 61 s of silence the log has %d messages for the reply, expected one marked stopped", len(got)))
		return p
	}
	if m := got[0].Message; !m.Stopped || m.Text != "text so far" || len(m.Mentions) != 0 {
		p = append(p, fmt.Sprintf("the stopped reply is %+v", m))
	}
	if s, a := piece(h, "a-1", 11, "late"); s != http.StatusConflict || a["refused"] != "closed" {
		p = append(p, fmt.Sprintf("a piece after closing answered %d %v", s, a))
	}
	if s, a := finish(h, "a-1", "a-1-done"); s != http.StatusConflict || a["refused"] != "closed" {
		p = append(p, fmt.Sprintf("finishing after closing answered %d %v", s, a))
	}
	if s, raw := h.Call("GET", fmt.Sprintf("/orgs/%s/replies/%s", r.Org, message), nil); s != http.StatusConflict {
		p = append(p, fmt.Sprintf("the text of a closed reply answered %d %s", s, raw))
	}
	return p
}

func TestSilenceClosesAReplyAsStopped(t *testing.T) {
	for _, p := range silenceProblems(r.New(t)) {
		t.Error(p)
	}
}

func TestAReplyOlderThanTenMinutesIsClosed(t *testing.T) {
	h := r.New(t)
	message := start(h, "a-1")
	at := 0
	for range 19 { // a piece every 30 s, for nine and a half minutes
		if s, a := piece(h, "a-1", at, "x"); s != 200 {
			t.Fatalf("a piece at %d answered %d: %v", at, s, a)
		}
		at++
		h.Housekeep()
		h.Clock.Advance(30 * time.Second)
	}
	h.Housekeep()
	if n := len(messagesOf(h, message)); n != 0 {
		t.Fatal("the reply closed before 10 minutes with pieces still coming")
	}
	if s, _ := piece(h, "a-1", at, "x"); s != 200 {
		t.Fatalf("a piece at 9:30 answered %d", s)
	}
	h.Clock.Advance(31 * time.Second)
	h.Housekeep()
	got := messagesOf(h, message)
	if len(got) != 1 || !got[0].Message.Stopped || len(got[0].Message.Text) != 20 {
		t.Fatalf("after 10 minutes the reply is %+v, expected one stopped message with 20 characters", got)
	}
}

func TestAReplyWithNoSavedTextLeavesNoMessage(t *testing.T) {
	h := r.New(t)
	message := start(h, "a-1")
	h.Clock.Advance(61 * time.Second)
	h.Housekeep()
	if n := len(messagesOf(h, message)); n != 0 {
		t.Errorf("a reply with no text left %d messages", n)
	}
	if n := kindCount(h, "reply.abandoned"); n != 1 {
		t.Errorf("the log has %d reply.abandoned, expected 1", n)
	}
}

// crashProblems: what was saved survives a crash, what came after it is lost,
// and the agent is told where to go on from.
func crashProblems(h *r.H) []string {
	var p []string
	message := start(h, "a-1")
	piece(h, "a-1", 0, "saved ")
	h.Housekeep()
	piece(h, "a-1", 6, "lost in the crash")
	h.Restart()
	s, a := piece(h, "a-1", 23, "next")
	if s != http.StatusConflict || a["have"] != float64(6) {
		p = append(p, fmt.Sprintf("after a crash a later piece answered %d %v, expected 409 have 6", s, a))
	}
	if s, a := piece(h, "a-1", 6, "again"); s != 200 {
		p = append(p, fmt.Sprintf("resuming from where it was told answered %d %v", s, a))
	}
	h.Housekeep()
	h.Restart()
	h.Clock.Advance(61 * time.Second)
	h.Housekeep()
	got := messagesOf(h, message)
	if len(got) != 1 || !got[0].Message.Stopped || got[0].Message.Text != "saved again" {
		p = append(p, fmt.Sprintf("after a crash and silence the reply is %+v, expected stopped with \"saved again\"", got))
	}
	return p
}

func TestACrashLosesAtMostWhatWasNotYetSaved(t *testing.T) {
	for _, p := range crashProblems(r.New(t)) {
		t.Error(p)
	}
}

func TestACrashBeforeAnyTextWasSavedLeavesNoMessage(t *testing.T) {
	h := r.New(t)
	message := start(h, "a-1")
	piece(h, "a-1", 0, "never saved")
	h.Restart()
	h.Clock.Advance(61 * time.Second)
	h.Housekeep()
	if n := len(messagesOf(h, message)); n != 0 {
		t.Errorf("a reply whose text was never saved left %d messages", n)
	}
	if n := kindCount(h, "reply.abandoned"); n != 1 {
		t.Errorf("the log has %d reply.abandoned, expected 1", n)
	}
}

func TestAFreshLoadShowsTheReplyInProgress(t *testing.T) {
	h := r.New(t)
	message := start(h, "a-1")
	piece(h, "a-1", 0, "so far")
	status, raw := h.Call("GET", fmt.Sprintf("/orgs/%s/conversations/%%23launch/messages?thread=%s", r.Org, r.Root), nil)
	var answer struct {
		InProgress []struct {
			Message, Text string
			At            int
		} `json:"in_progress"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil || status != 200 {
		t.Fatalf("the thread answered %d: %s", status, raw)
	}
	if len(answer.InProgress) != 1 || answer.InProgress[0].Message != message || answer.InProgress[0].Text != "so far" || answer.InProgress[0].At != 6 {
		t.Errorf("in_progress is %+v", answer.InProgress)
	}
}
