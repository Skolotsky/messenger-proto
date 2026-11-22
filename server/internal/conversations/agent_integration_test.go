//go:build integration

package conversations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type agentBench struct {
	t      *testing.T
	store  *Store
	url    string
	org    ids.Org
	schema *jsonschema.Compiler
}

func seedAgents(t *testing.T, s *Store, org ids.Org) {
	t.Helper()
	ctx := context.Background()
	session, err := s.SeedOpen(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	n := int64(0)
	h := func() (string, int64, int64, string) { n++; return string(org), n, n - 1, seedAt }
	var list []contract.Change
	for _, p := range []struct {
		id    string
		agent bool
	}{{"anna", false}, {"marco", false}, {"release-agent", true}} {
		o, num, prev, at := h()
		list = append(list, contract.Change{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: o, Number: num, Previous: prev, At: at,
			Person: p.id, Name: p.id, IsAgent: p.agent}})
	}
	o, num, prev, at := h()
	list = append(list, contract.Change{Value: contract.ChangeChannelCreated{Kind: "channel.created", Org: o, Number: num, Previous: prev, At: at,
		Conversation: "#launch", Name: "launch", People: []string{"anna", "marco", "release-agent"}}})
	o, num, prev, at = h()
	list = append(list, contract.Change{Value: contract.ChangeMessageCreated{Kind: "message.created", Org: o, Number: num, Previous: prev, At: at, Message: contract.Message{
		Id: "msg_7Q2", Number: num, Version: 1, Conversation: "#launch", Author: "anna", Text: "root",
		Mentions: []string{}, Reactions: []contract.Reaction{}, CreatedAt: at}}})
	if _, err := s.SeedBatch(ctx, org, session, contract.SeedBatch{Changes: list}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SeedSeal(ctx, org, session); err != nil {
		t.Fatal(err)
	}
}

func newAgentBench(t *testing.T) *agentBench {
	t.Helper()
	pool := testPool(t)
	c := clock.NewManual(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC))
	b := &agentBench{t: t, org: newOrg("t054"), store: &Store{Dir: orgdb.Single{Pool: pool}, Clock: c}}
	seedAgents(t, b.store, b.org)
	mux := http.NewServeMux()
	b.store.Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	b.url = srv.URL
	path, err := filepath.Abs("../../../contracts/schema/contexia.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	b.schema = jsonschema.NewCompiler()
	if err := b.schema.AddResource("file:///contexia.json", doc); err != nil {
		t.Fatal(err)
	}
	return b
}

// call makes one request and holds its answer to the contract's type.
func (b *agentBench) call(method, tail string, body any, status int, answer string) map[string]any {
	b.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, b.url+"/orgs/"+string(b.org)+tail, reader)
	if err != nil {
		b.t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != status {
		b.t.Fatalf("%s %s: answered %d, expected %d: %s", method, tail, resp.StatusCode, status, raw)
	}
	s, err := b.schema.Compile("file:///contexia.json#/$defs/" + answer)
	if err != nil {
		b.t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
	if err != nil {
		b.t.Fatalf("%s %s: %v: %s", method, tail, err, raw)
	}
	if err := s.Validate(doc); err != nil {
		b.t.Errorf("%s %s: the answer is not a %s: %v\n%s", method, tail, answer, err, raw)
	}
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func (b *agentBench) person(kind, as string, id string, fields map[string]any) {
	b.t.Helper()
	body := map[string]any{"kind": kind, "id": id, "as": as}
	for k, v := range fields {
		body[k] = v
	}
	b.call("POST", "/people/actions", body, 200, "ActionSaved")
}

// Every operation of the agent door answers in the contract's form, and
// everything it wrote to the log matches the schema.
func TestEveryAgentOperationAnswersItsContract(t *testing.T) {
	b := newAgentBench(t)
	const me = "/agents/release-agent"
	b.person("send", "anna", "p1", map[string]any{"message": map[string]any{"id": "msg_m", "conversation": "#launch", "thread": nil,
		"text": "@release look", "mentions": []string{"release-agent"}, "level": nil}})
	events := b.call("GET", me+"/events?after=0", nil, 200, "AgentEvents")
	if n := len(events["events"].([]any)); n != 1 {
		t.Fatalf("%d events, expected 1", n)
	}
	b.call("POST", me+"/messages", map[string]any{"id": "a-91", "conversation": "#launch", "thread": "msg_7Q2", "text": "On it.", "level": "info", "because_of": 1}, 200, "AgentPosted")
	b.call("POST", me+"/messages", map[string]any{"id": "a-91", "conversation": "#launch", "thread": "msg_7Q2", "text": "other", "level": nil, "because_of": nil}, 409, "Refusal")
	b.call("POST", me+"/messages", map[string]any{"id": "a-x", "conversation": "#nowhere", "thread": nil, "text": "t", "level": nil, "because_of": nil}, 409, "Refusal")
	started := b.call("POST", me+"/replies", map[string]any{"id": "a-92", "conversation": "#launch", "thread": "msg_7Q2", "because_of": 1}, 200, "AgentPosted")
	b.call("POST", me+"/replies/a-92/pieces", map[string]any{"at": 0, "text": "Build 2.4.1 "}, 200, "Empty")
	b.call("POST", me+"/replies/a-92/pieces", map[string]any{"at": 50, "text": "x"}, 409, "PieceMissing")
	text := b.call("GET", "/replies/"+started["message"].(string), nil, 200, "ReplyText")
	if text["at"] != float64(12) {
		t.Errorf("the text so far says at %v, expected 12", text["at"])
	}
	finished := b.call("POST", me+"/replies/a-92/finish", map[string]any{"id": "a-92-done", "level": "flag", "mentions": []string{"marco"}}, 200, "AgentPosted")
	if finished["message"] != started["message"] {
		t.Errorf("the finished reply is %v, it started as %v", finished["message"], started["message"])
	}
	b.call("POST", me+"/replies/a-92/pieces", map[string]any{"at": 12, "text": "x"}, 409, "Refusal")
	b.call("GET", "/replies/"+started["message"].(string), nil, 409, "Refusal")
	direct := b.call("POST", me+"/direct", map[string]any{"with": "marco"}, 200, "AgentDirectAnswer")
	again := b.call("POST", me+"/direct", map[string]any{"with": "marco"}, 200, "AgentDirectAnswer")
	if direct["conversation"] != "dm:marco:release-agent" || again["conversation"] != direct["conversation"] {
		t.Errorf("the direct conversation is %v then %v", direct["conversation"], again["conversation"])
	}
	b.call("POST", me+"/direct", map[string]any{"with": "nobody"}, 409, "Refusal")
	b.call("POST", me+"/status", map[string]any{"state": "waiting", "on": "marco"}, 200, "Empty")
	b.call("POST", me+"/reactions", map[string]any{"id": "a-93", "message": "msg_7Q2", "emoji": "eyes", "on": true}, 200, "AgentPosted")
	b.call("POST", me+"/reactions", map[string]any{"id": "a-94", "message": "msg_nope", "emoji": "eyes", "on": true}, 409, "Refusal")
	b.call("PUT", me+"/subscriptions/%23launch", nil, 200, "Empty")
	b.call("DELETE", me+"/subscriptions/%23launch", nil, 200, "Empty")
	b.call("GET", me+"/conversations/%23launch/messages?around=5", nil, 200, "MessagesAnswer")

	start := b.call("GET", "/people/anna/start", nil, 200, "StartAnswer")
	found := false
	for _, v := range start["people"].([]any) {
		p := v.(map[string]any)
		if p["id"] == "release-agent" {
			status, _ := p["status"].(map[string]any)
			found = status["state"] == "waiting" && status["on"] == "marco"
		}
	}
	if !found {
		t.Errorf("a fresh load does not carry the agent's status: %v", start["people"])
	}

	schema := compileChange(t)
	f := &fixture{ctx: context.Background(), store: b.store, org: b.org}
	for _, entry := range f.log(t) {
		raw, _ := json.Marshal(entry)
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("log entry %v is not a Change: %v", entry["kind"], err)
		}
	}
}

func TestAnAgentReadsOnlyItsOwnDirectConversations(t *testing.T) {
	b := newAgentBench(t)
	b.person("open_direct", "anna", "p1", map[string]any{"with": "marco"})
	b.call("GET", "/agents/release-agent/conversations/"+"dm:anna:marco"+"/messages", nil, 409, "Refusal")
	b.call("GET", "/agents/anna/events", nil, 404, "Refusal") // a person is not an agent
}

func TestAnAgentsPushReachesTheLiveConnection(t *testing.T) {
	b := newAgentBench(t)
	var got []string
	b.store.SetAgentPush(func(org ids.Org, conversation string, update contract.LiveOut) {
		raw, _ := json.Marshal(update)
		got = append(got, fmt.Sprintf("%s|%s|%s", org, conversation, raw))
	})
	const me = "/agents/release-agent"
	started := b.call("POST", me+"/replies", map[string]any{"id": "a-1", "conversation": "#launch", "thread": nil, "because_of": nil}, 200, "AgentPosted")
	b.call("POST", me+"/replies/a-1/pieces", map[string]any{"at": 0, "text": "ready. "}, 200, "Empty")
	b.call("POST", me+"/replies/a-1/pieces", map[string]any{"at": 0, "text": "ready. "}, 200, "Empty") // a repeat is not pushed
	b.call("POST", me+"/status", map[string]any{"state": "working", "on": nil}, 200, "Empty")
	want := []string{
		fmt.Sprintf(`%s|#launch|{"piece":{"message":"%s","at":0,"text":"ready. "}}`, b.org, started["message"]),
		fmt.Sprintf(`%s||{"agent_status":{"agent":"release-agent","state":"working","on":null}}`, b.org),
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("pushed %v\nexpected %v", got, want)
	}
}

// One organization's agent, events and replies are invisible to another with
// the same names.
func TestAgentStateIsKeptApartByOrganization(t *testing.T) {
	a := newAgentBench(t)
	b := &agentBench{t: t, org: newOrg("t054"), store: a.store, url: a.url, schema: a.schema}
	seedAgents(t, b.store, b.org)
	a.person("send", "anna", "p1", map[string]any{"message": map[string]any{"id": "msg_same", "conversation": "#launch", "thread": nil,
		"text": "@release only in a", "mentions": []string{"release-agent"}, "level": nil}})
	if n := len(a.call("GET", "/agents/release-agent/events", nil, 200, "AgentEvents")["events"].([]any)); n != 1 {
		t.Fatalf("the first organization's agent has %d events", n)
	}
	if n := len(b.call("GET", "/agents/release-agent/events", nil, 200, "AgentEvents")["events"].([]any)); n != 0 {
		t.Errorf("the second organization's agent of the same name has %d events", n)
	}
	started := a.call("POST", "/agents/release-agent/replies", map[string]any{"id": "r-1", "conversation": "#launch", "thread": nil, "because_of": nil}, 200, "AgentPosted")
	a.call("POST", "/agents/release-agent/replies/r-1/pieces", map[string]any{"at": 0, "text": "only in a"}, 200, "Empty")
	b.call("POST", "/agents/release-agent/replies/r-1/pieces", map[string]any{"at": 0, "text": "x"}, 404, "Refusal")
	b.call("GET", "/replies/"+started["message"].(string), nil, 404, "Refusal")
}
