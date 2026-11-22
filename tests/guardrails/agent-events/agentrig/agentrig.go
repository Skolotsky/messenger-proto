// Package agentrig is the bench of the agent door's guardrails (G-016, G-017,
// G-018): Conversations' real routes on a database of their own, a clock the
// test moves, and a restart that loses everything the program held in memory.
package agentrig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The cast of every test.
const (
	Org     = ids.Org("acme")
	Anna    = "anna"
	Marco   = "marco"
	Release = "release-agent"
	Review  = "review-agent"
	Root    = "msg_root"
	Launch  = "#launch"
)

// H is one running door.
type H struct {
	T     testing.TB
	DB    *rig.DB
	Pool  *pgxpool.Pool
	Clock *clock.Manual
	Store *conversations.Store
	URL   string

	mux  atomic.Pointer[http.ServeMux]
	next int
}

// New seeds the organization and serves the door.
func New(t testing.TB) *H {
	t.Helper()
	db := rig.NewDB(t)
	h := &H{T: t, DB: db, Pool: db.Conversations, Clock: clock.NewManual(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC))}
	h.serve()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.mux.Load().ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	h.URL = srv.URL
	h.seed()
	return h
}

func (h *H) serve() {
	h.Store = &conversations.Store{Dir: orgdb.Single{Pool: h.Pool}, Clock: h.Clock}
	mux := http.NewServeMux()
	h.Store.Routes(mux)
	h.mux.Store(mux)
}

// Restart is a crash: a new program over the same database, with nothing that
// the old one held in memory.
func (h *H) Restart() { h.serve() }

func (h *H) seed() {
	h.T.Helper()
	ctx := context.Background()
	session, err := h.Store.SeedOpen(ctx, Org)
	if err != nil {
		h.T.Fatal(err)
	}
	const at = "2026-11-01T09:00:00.000Z"
	n := int64(0)
	head := func() (string, int64, int64, string) { n++; return string(Org), n, n - 1, at }
	var list []contract.Change
	for _, p := range []struct {
		id    string
		agent bool
	}{{Anna, false}, {Marco, false}, {Release, true}, {Review, true}} {
		o, num, prev, stamp := head()
		list = append(list, contract.Change{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: o, Number: num, Previous: prev, At: stamp,
			Person: p.id, Name: p.id, IsAgent: p.agent}})
	}
	o, num, prev, stamp := head()
	list = append(list, contract.Change{Value: contract.ChangeChannelCreated{Kind: "channel.created", Org: o, Number: num, Previous: prev, At: stamp,
		Conversation: Launch, Name: "launch", People: []string{Anna, Marco, Release, Review}}})
	o, num, prev, stamp = head()
	list = append(list, contract.Change{Value: contract.ChangeMessageCreated{Kind: "message.created", Org: o, Number: num, Previous: prev, At: stamp,
		Message: contract.Message{Id: Root, Number: num, Version: 1, Conversation: Launch, Author: Anna, Text: "the thread's root",
			Mentions: []string{}, Reactions: []contract.Reaction{}, CreatedAt: stamp}}})
	if _, err := h.Store.SeedBatch(ctx, Org, session, contract.SeedBatch{Changes: list}); err != nil {
		h.T.Fatal(err)
	}
	if _, err := h.Store.SeedSeal(ctx, Org, session); err != nil {
		h.T.Fatal(err)
	}
}

// Call makes one request and returns its status and body.
func (h *H) Call(method, path string, body any) (int, []byte) {
	h.T.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.T.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.URL+path, reader)
	if err != nil {
		h.T.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.T.Fatal(err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		h.T.Fatal(err)
	}
	return resp.StatusCode, out
}

// Agent calls the agent door; tail starts with "/".
func (h *H) Agent(agent, method, tail string, body any) (int, map[string]any) {
	h.T.Helper()
	status, raw := h.Call(method, fmt.Sprintf("/orgs/%s/agents/%s%s", Org, agent, tail), body)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return status, out
}

// Sub is the address of a channel in a subscription call.
func Sub(conversation string) string { return "/subscriptions/" + url.PathEscape(conversation) }

// Act is one action of the people door, with a fresh ID.
func (h *H) Act(kind, as string, fields map[string]any) map[string]any {
	h.T.Helper()
	h.next++
	body := map[string]any{"kind": kind, "id": fmt.Sprintf("p-%d", h.next), "as": as}
	for k, v := range fields {
		body[k] = v
	}
	status, raw := h.Call("POST", fmt.Sprintf("/orgs/%s/people/actions", Org), body)
	if status != http.StatusOK {
		h.T.Fatalf("%s by %s answered %d: %s", kind, as, status, raw)
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		h.T.Fatal(err)
	}
	return out
}

// Say is a person's message with a fresh ID; it returns the message's ID.
func (h *H) Say(as, conversation string, thread any, text string, mentions ...string) string {
	h.T.Helper()
	h.next++
	id := fmt.Sprintf("msg_p%d", h.next)
	if mentions == nil {
		mentions = []string{}
	}
	h.Act("send", as, map[string]any{"message": map[string]any{"id": id, "conversation": conversation, "thread": thread,
		"text": text, "mentions": mentions, "level": nil}})
	return id
}

// Event is one event as an agent reads it.
type Event struct {
	Kind       string `json:"kind"`
	Number     int64  `json:"number"`
	AgentChain int64  `json:"agent_chain"`
	By         string `json:"by"`
	Emoji      string `json:"emoji"`
	Added      bool   `json:"added"`
	Message    struct {
		Id     string  `json:"id"`
		Text   string  `json:"text"`
		Thread *string `json:"thread"`
		Author string  `json:"author"`
	} `json:"message"`
}

// Events reads every event after a number, following continue_from.
func (h *H) Events(agent string, after int64) []Event {
	h.T.Helper()
	var all []Event
	for {
		status, raw := h.Call("GET", fmt.Sprintf("/orgs/%s/agents/%s/events?after=%d", Org, agent, after), nil)
		if status != http.StatusOK {
			h.T.Fatalf("events of %s after %d answered %d: %s", agent, after, status, raw)
		}
		var page struct {
			ContinueFrom *int64  `json:"continue_from"`
			Events       []Event `json:"events"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			h.T.Fatal(err)
		}
		all = append(all, page.Events...)
		if page.ContinueFrom == nil {
			return all
		}
		after = *page.ContinueFrom
	}
}

// Housekeep runs one pass of what the program does every second.
func (h *H) Housekeep() {
	h.T.Helper()
	if err := h.Store.AgentHousekeep(context.Background(), []*pgxpool.Pool{h.Pool}); err != nil {
		h.T.Fatal(err)
	}
}

// Entry is one row of the change log.
type Entry struct {
	Number int64
	Kind   string
	Body   string
}

// Log is the organization's change log after the seed.
func (h *H) Log() []Entry {
	h.T.Helper()
	var out []Entry
	err := orgdb.InOrg(context.Background(), orgdb.Single{Pool: h.Pool}, Org, func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT number, kind, body::text FROM change_log WHERE org = $1 AND number > 6 ORDER BY number`, string(Org))
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Entry, error) {
			var e Entry
			err := row.Scan(&e.Number, &e.Kind, &e.Body)
			return e, err
		})
		return err
	})
	if err != nil {
		h.T.Fatal(err)
	}
	return out
}

// LogHolding lists log entries whose body contains the text.
func (h *H) LogHolding(text string) []Entry {
	h.T.Helper()
	var out []Entry
	for _, e := range h.Log() {
		if strings.Contains(e.Body, text) {
			out = append(out, e)
		}
	}
	return out
}

// Retire makes the organization retired through its own route.
func (h *H) Retire() {
	h.T.Helper()
	if status, raw := h.Call("POST", fmt.Sprintf("/orgs/%s/retire", Org), nil); status != http.StatusOK {
		h.T.Fatalf("retire answered %d: %s", status, raw)
	}
}

// Delete removes the retired organization, delivery's rows first as the real
// delete does.
func (h *H) Delete() {
	h.T.Helper()
	if _, err := h.DB.Del.Exec(context.Background(), `DELETE FROM delivery_due WHERE org = $1`, string(Org)); err != nil {
		h.T.Fatal(err)
	}
	// As delivery's own delete does: Conversations refuses until this is set (T-064).
	if _, err := h.DB.Del.Exec(context.Background(), `UPDATE org_log SET delivery_cleared_at = now() WHERE org = $1`, string(Org)); err != nil {
		h.T.Fatal(err)
	}
	if status, raw := h.Call("DELETE", fmt.Sprintf("/orgs/%s", Org), nil); status != http.StatusOK {
		h.T.Fatalf("delete answered %d: %s", status, raw)
	}
}
