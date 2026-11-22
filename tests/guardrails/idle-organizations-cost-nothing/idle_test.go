// Package idle holds G-029 for delivery: nothing a worker asks on its timer gets
// slower because idle organizations exist. Housekeeping's questions (ticks,
// leases, abandoned replies, old events) join this test with T-053 and T-054.
package idle

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"contexia/internal/delivery"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
)

var bg = context.Background()

const (
	busy    = 10
	perBusy = 8 // rows one question may touch per busy organization
)

// addIdle makes organizations that were busy earlier: a log head, a place for
// every receiver, a note head and place, and nothing due.
func addIdle(t *testing.T, b *rig.Bench, from, to int) {
	t.Helper()
	for _, q := range []struct {
		pool string
		sql  string
	}{
		{"c", `INSERT INTO org_log (org, head, head_at) SELECT 'idle-' || n, 40, now() - interval '1 day' FROM generate_series($1::int, $2::int) n`},
		{"d", `INSERT INTO delivery_places (org, receiver, place, claimed_until)
			SELECT 'idle-' || n, r, 40, now() - interval '1 day' FROM generate_series($1::int, $2::int) n, (VALUES ('search'), ('feed')) v(r)`},
		{"f", `INSERT INTO note_heads (org, head) SELECT 'idle-' || n, 7 FROM generate_series($1::int, $2::int) n`},
		{"d", `INSERT INTO feed.note_places (org, place, claimed_until) SELECT 'idle-' || n, 7, now() - interval '1 day' FROM generate_series($1::int, $2::int) n`},
	} {
		pool := b.DB.Conversations
		switch q.pool {
		case "d":
			pool = b.DB.Del
		case "f":
			pool = b.DB.Feed
		}
		if _, err := pool.Exec(bg, q.sql, from, to); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{"ANALYZE org_log", "ANALYZE delivery_due", "ANALYZE delivery_places"} {
		if _, err := b.DB.Conversations.Exec(bg, q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{"ANALYZE note_heads", "ANALYZE note_due", "ANALYZE note_places"} {
		if _, err := b.DB.Feed.Exec(bg, q); err != nil {
			t.Fatal(err)
		}
	}
}

func addBusy(t *testing.T, b *rig.Bench) {
	t.Helper()
	for i := range busy {
		org := fmt.Sprintf("busy-%d", i)
		b.DB.AddOrg(t, org, 60)
		for range 3 {
			if _, err := b.DB.Save(bg, org); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := b.DB.SaveNote(bg, org, "clock"); err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{
			`INSERT INTO delivery_places (org, receiver) VALUES ($1, 'search'), ($1, 'feed') ON CONFLICT DO NOTHING`,
			`INSERT INTO feed.note_places (org) VALUES ($1) ON CONFLICT DO NOTHING`,
		} {
			if _, err := b.DB.Del.Exec(bg, q, org); err != nil {
				t.Fatal(err)
			}
		}
	}
}

type measured struct {
	millis float64
	read   float64 // rows the plan read from tables that hold every organization
	seq    []string
}

var wide = map[string]bool{"org_log": true, "delivery_places": true, "note_heads": true, "note_places": true}

func walk(node map[string]any, m *measured) {
	rel, _ := node["Relation Name"].(string)
	if wide[rel] {
		loops, _ := node["Actual Loops"].(float64)
		rows, _ := node["Actual Rows"].(float64)
		removed, _ := node["Rows Removed by Filter"].(float64)
		m.read += (rows + removed) * loops
		if t, _ := node["Node Type"].(string); t == "Seq Scan" {
			m.seq = append(m.seq, rel)
		}
	}
	if kids, ok := node["Plans"].([]any); ok {
		for _, k := range kids {
			walk(k.(map[string]any), m)
		}
	}
}

// ask runs a question under EXPLAIN ANALYZE and undoes whatever it did.
func ask(t *testing.T, b *rig.Bench, q delivery.Question) measured {
	t.Helper()
	var m measured
	tx, err := b.DB.Del.BeginTx(bg, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(bg)
	var raw string
	if err := tx.QueryRow(bg, "EXPLAIN (ANALYZE, FORMAT JSON) "+q.SQL, q.Args...).Scan(&raw); err != nil {
		t.Fatalf("%s: %v", q.Name, err)
	}
	var plans []map[string]any
	if err := json.Unmarshal([]byte(raw), &plans); err != nil {
		t.Fatal(err)
	}
	m.millis, _ = plans[0]["Execution Time"].(float64)
	walk(plans[0]["Plan"].(map[string]any), &m)
	return m
}

func measure(t *testing.T, b *rig.Bench, s *delivery.Service) map[string]measured {
	t.Helper()
	out := map[string]measured{}
	now := time.Now()
	for _, q := range s.Questions(now) {
		var times []float64
		var last measured
		for range 41 {
			last = ask(t, b, q)
			times = append(times, last.millis)
		}
		sort.Float64s(times)
		last.millis = times[len(times)/2]
		out[q.Name] = last
	}
	return out
}

// TestIdleOrganizationsCostNothing compares every timer question beside 1,000
// and beside 20,000 idle organizations, with ten busy ones. The service is built
// but not run, so the busy organizations stay due for the whole measurement.
func TestIdleOrganizationsCostNothing(t *testing.T) {
	b := rig.NewBench(t, nil)
	s := delivery.New(delivery.Config{DB: b.DB.Directory(), Search: b.Search.URL, Feed: b.Feed.URL, Sender: b.Sender.URL, Seed: 47, Worker: "w"})
	addBusy(t, b)

	addIdle(t, b, 1, 1000)
	small := measure(t, b, s)
	addIdle(t, b, 1001, 20000)
	large := measure(t, b, s)

	for name, a := range small {
		z := large[name]
		t.Logf("%-48s %7.3f ms -> %7.3f ms; rows read %.0f -> %.0f", name, a.millis, z.millis, a.read, z.read)
		if len(z.seq) > 0 {
			t.Errorf("%s reads every row of %v beside 20,000 idle organizations. Reach those tables per due row (G-029)", name, z.seq)
		}
		// A question touches each busy organization's rows a few times; what it
		// may not do is read in proportion to the idle ones, at either size.
		if limit := float64(perBusy * busy); a.read > limit || z.read > limit {
			t.Errorf("%s read %.0f and %.0f rows of the tables that hold every organization, with only %d busy. It must not read the idle ones (G-029)", name, a.read, z.read, busy)
		}
		if z.millis > a.millis*1.5 {
			t.Errorf("%s rose from %.3f ms to %.3f ms between 1,000 and 20,000 idle organizations, more than half (G-029)", name, a.millis, z.millis)
		}
	}
}
