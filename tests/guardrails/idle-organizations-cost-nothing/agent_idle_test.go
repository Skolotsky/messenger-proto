package idle

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
)

const agentBusy = 5

// agentRowsRead runs one housekeeping question as the program does, and counts
// the rows its plan read from the table that holds every organization.
func agentRowsRead(t *testing.T, db *rig.DB, q conversations.AgentQuestion) float64 {
	t.Helper()
	tx, err := db.Conversations.BeginTx(bg, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(bg)
	if _, err := tx.Exec(bg, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := tx.QueryRow(bg, "EXPLAIN (ANALYZE, FORMAT JSON) "+q.SQL, q.Args...).Scan(&raw); err != nil {
		t.Fatalf("%s: %v", q.Name, err)
	}
	var plans []map[string]any
	if err := json.Unmarshal([]byte(raw), &plans); err != nil {
		t.Fatal(err)
	}
	var read float64
	var visit func(map[string]any)
	visit = func(n map[string]any) {
		if n["Relation Name"] == "agent_org_due" {
			loops, _ := n["Actual Loops"].(float64)
			rows, _ := n["Actual Rows"].(float64)
			removed, _ := n["Rows Removed by Filter"].(float64)
			read += (rows + removed) * loops
		}
		if kids, ok := n["Plans"].([]any); ok {
			for _, k := range kids {
				visit(k.(map[string]any))
			}
		}
	}
	visit(plans[0]["Plan"].(map[string]any))
	return read
}

// agentIdleProblems holds the door's two timed questions to the organizations
// that have something due, beside 50 and beside 500 idle ones that were busy
// earlier and still have events kept.
func agentIdleProblems(t *testing.T) []string {
	db := rig.NewDB(t)
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	if _, err := db.Conversations.Exec(bg, `INSERT INTO agent_org_due (org, reply_due_at, events_due_at)
		SELECT 'busy-' || n, $1::timestamptz - interval '1 second', $1::timestamptz - interval '1 second' FROM generate_series(1, $2::int) n`, now, agentBusy); err != nil {
		t.Fatal(err)
	}
	addIdle := func(from, to int) {
		if _, err := db.Conversations.Exec(bg, `INSERT INTO agent_org_due (org, events_due_at)
			SELECT 'idle-' || n, $1::timestamptz + interval '3 days' FROM generate_series($2::int, $3::int) n`, now, from, to); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Conversations.Exec(bg, `ANALYZE agent_org_due`); err != nil {
			t.Fatal(err)
		}
	}
	var problems []string
	last := 0
	for _, idle := range []int{50, 500} {
		addIdle(last+1, idle)
		last = idle
		for _, q := range conversations.AgentQuestions(now) {
			read := agentRowsRead(t, db, q)
			t.Logf("%-34s beside %3d idle organizations: %.0f rows read", q.Name, idle, read)
			if read > agentBusy {
				problems = append(problems, fmt.Sprintf("%s read %.0f rows beside %d idle organizations, with %d due. It must not read the idle ones (G-029)", q.Name, read, idle, agentBusy))
			}
		}
	}
	return problems
}

func TestTheAgentDoorsTimedQuestionsSkipIdleOrganizations(t *testing.T) {
	for _, p := range agentIdleProblems(t) {
		t.Error(p)
	}
}
