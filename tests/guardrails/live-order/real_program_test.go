package liveorder

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/tests/harness"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func post(t *testing.T, url string, body any) []byte {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s: %d %s", url, resp.StatusCode, out.String())
	}
	return out.Bytes()
}

// TestTheRealProgramServesTheLiveConnection runs through the composed program,
// so what main wires up is what is checked: a screen gets a saved message over
// the live address, housekeeping writes a tick when the clock moves, and retiring
// the organization closes the connection.
func TestTheRealProgramServesTheLiveConnection(t *testing.T) {
	sys := harness.Connect(t)
	base := sys.URL("conversations")
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	org := "real-" + hex.EncodeToString(b)

	var opened contract.SeedOpened
	if err := json.Unmarshal(post(t, base+"/orgs/"+org+"/seed/open", map[string]any{}), &opened); err != nil {
		t.Fatal(err)
	}
	at := "2026-11-01T09:00:00.000Z"
	post(t, base+"/orgs/"+org+"/seed/"+opened.Session, contract.SeedBatch{Changes: []contract.Change{
		{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: org, Number: 1, Previous: 0, At: at, Person: "anna", Name: "Anna"}},
		{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: org, Number: 2, Previous: 1, At: at, Person: "marco", Name: "Marco"}},
		{Value: contract.ChangeChannelCreated{Kind: "channel.created", Org: org, Number: 3, Previous: 2, At: at,
			Conversation: "#launch", Name: "launch", People: []string{"anna", "marco"}}},
	}})
	post(t, base+"/orgs/"+org+"/seed/"+opened.Session+"/seal", map[string]any{})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/orgs/"+org+"/live?person=marco", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	resume, _ := json.Marshal(contract.LiveResume{Resume: contract.ResumeNumbers{Conversations: 3}})
	if err := ws.Write(ctx, websocket.MessageText, resume); err != nil {
		t.Fatal(err)
	}
	post(t, base+"/orgs/"+org+"/people/actions", map[string]any{"id": "act_1", "as": "anna", "kind": "send",
		"message": map[string]any{"id": "msg_1", "conversation": "#launch", "thread": nil, "text": "over the wire", "mentions": []string{}, "level": nil}})
	_, raw, err := ws.Read(ctx)
	if err != nil {
		t.Fatalf("no update on the live connection: %v", err)
	}
	var update struct {
		Stream string `json:"stream"`
		After  int64  `json:"after"`
	}
	if err := json.Unmarshal(raw, &update); err != nil || update.Stream != "conversations" || update.After != 3 || !bytes.Contains(raw, []byte("over the wire")) {
		t.Fatalf("the first update should carry the message after number 3, got %s", raw)
	}

	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://conversations:conversations_dev@%s:5432/contexia", os.Getenv("CONTEXIA_PG_HOST")))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	deadline := time.Now().Add(15 * time.Second)
	for {
		sys.Advance(2*time.Second, "conversations")
		var ticks int
		if err := orgdb.InOrg(ctx, orgdb.Single{Pool: pool}, ids.Org(org), func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM change_log WHERE org = $1 AND kind = 'tick'`, org).Scan(&ticks)
		}); err != nil {
			t.Fatal(err)
		}
		if ticks > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the running program wrote no clock tick after a change")
		}
		time.Sleep(50 * time.Millisecond)
	}

	post(t, base+"/orgs/"+org+"/retire", map[string]any{})
	for {
		_, raw, err := ws.Read(ctx)
		if err != nil {
			t.Fatalf("the connection closed without organization_gone: %v", err)
		}
		if bytes.Contains(raw, []byte("organization_gone")) {
			break
		}
	}
	if _, _, err := ws.Read(ctx); err == nil {
		t.Fatal("the connection stayed open after organization_gone")
	}
}
