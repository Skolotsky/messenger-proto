// Package changeandlog holds G-004: a change and its log entry are saved in one
// transaction, and no change exists without a log entry.
package changeandlog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	sql "contexia/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const start = "2026-11-01T09:00:00.000Z"

type fixture struct {
	ctx   context.Context
	pool  *pgxpool.Pool
	dir   orgdb.Single
	store *conversations.Store
	org   ids.Org
}

func connect(t *testing.T, login string) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://%s:%s_dev@%s:5432/contexia", login, login, host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool := connect(t, "conversations")
	if err := migrate.Apply(ctx, pool, sql.Files, "conversations"); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	f := &fixture{ctx: ctx, pool: pool, dir: orgdb.Single{Pool: pool}, org: ids.Org("g004-" + hex.EncodeToString(b))}
	f.store = &conversations.Store{Dir: f.dir, Clock: clock.Real{}}
	session, err := f.store.SeedOpen(ctx, f.org)
	if err != nil {
		t.Fatal(err)
	}
	head := func(n int64) (string, int64, int64, string) { return string(f.org), n, n - 1, start }
	o, n1, p1, at := head(1)
	o2, n2, p2, _ := head(2)
	o3, n3, p3, _ := head(3)
	batch := contract.SeedBatch{Changes: []contract.Change{
		{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: o, Number: n1, Previous: p1, At: at, Person: "anna", Name: "Anna", IsAgent: false}},
		{Value: contract.ChangeChannelCreated{Kind: "channel.created", Org: o2, Number: n2, Previous: p2, At: at, Conversation: "#launch", Name: "launch", People: []string{"anna"}}},
		{Value: contract.ChangeMessageCreated{Kind: "message.created", Org: o3, Number: n3, Previous: p3, At: at, Message: contract.Message{
			Id: "msg_1", Number: n3, Version: 1, Conversation: "#launch", Author: "anna", Text: "first", Mentions: []string{},
			Reactions: []contract.Reaction{}, CreatedAt: at}}},
	}}
	if _, err := f.store.SeedBatch(ctx, f.org, session, batch); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.SeedSeal(ctx, f.org, session); err != nil {
		t.Fatal(err)
	}
	return f
}

// missing is the check "every saved change is in the log".
func (f *fixture) missing(t *testing.T) []string {
	t.Helper()
	var out []string
	err := orgdb.InOrg(f.ctx, f.dir, f.org, func(tx pgx.Tx) error {
		rows, err := tx.Query(f.ctx, `
			SELECT 'message ' || m.id FROM messages m
			WHERE NOT EXISTS (SELECT 1 FROM change_log c WHERE c.org = m.org AND c.kind = 'message.created' AND c.body->'message'->>'id' = m.id)
			   OR (m.version > 1 AND NOT EXISTS (SELECT 1 FROM change_log c WHERE c.org = m.org
			        AND c.kind IN ('message.changed', 'message.deleted') AND c.body->'message'->>'id' = m.id
			        AND (c.body->'message'->>'version')::bigint = m.version))
			UNION ALL
			SELECT 'read ' || r.person || ' ' || r.conversation FROM reader_reads r
			WHERE NOT EXISTS (SELECT 1 FROM change_log c WHERE c.org = r.org AND c.kind = 'reader.read'
			        AND c.body->>'person' = r.person AND c.body->>'conversation' = r.conversation
			        AND c.body->>'thread' IS NOT DISTINCT FROM r.thread AND (c.body->>'up_to')::bigint = r.up_to)`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (f *fixture) ruleSwitch(t *testing.T, table string, on bool) {
	t.Helper()
	verb := "DISABLE"
	if on {
		verb = "ENABLE"
	}
	if _, err := f.pool.Exec(context.Background(), "ALTER TABLE conversations."+table+" "+verb+" TRIGGER require_log_entry"); err != nil {
		t.Fatal(err)
	}
}

func TestChangeWithoutLogEntryIsRefused(t *testing.T) {
	f := setup(t)
	for name, statement := range map[string]string{
		"messages":     `UPDATE messages SET text = 'changed behind the log' WHERE id = 'msg_1'`,
		"reader_reads": `INSERT INTO reader_reads (org, person, conversation, thread, up_to) VALUES (current_setting('app.org'), 'anna', '#launch', NULL, 3)`,
	} {
		t.Run(name, func(t *testing.T) {
			if ruleOff {
				f.ruleSwitch(t, name, false)
				t.Cleanup(func() { f.ruleSwitch(t, name, true) })
			}
			err := orgdb.InOrg(f.ctx, f.dir, f.org, func(tx pgx.Tx) error {
				_, err := tx.Exec(f.ctx, statement)
				return err
			})
			var pg *pgconn.PgError
			if !errors.As(err, &pg) || pg.Code != "CX004" {
				t.Fatalf("G-004 broken: a transaction that changed %s without a log entry was not refused at commit (got %v). Restore the require_log_entry trigger", name, err)
			}
		})
	}
	if got := f.missing(t); !ruleOff && len(got) != 0 {
		t.Fatalf("a refused transaction left changes behind: %v", got)
	}
}

func TestActionsSaveChangeAndLogTogether(t *testing.T) {
	f := setup(t)
	for i, body := range []string{
		`{"kind":"send","id":"a1","as":"anna","message":{"id":"msg_2","conversation":"#launch","thread":"msg_1","text":"a reply","mentions":[],"level":null}}`,
		`{"kind":"edit","id":"a2","as":"anna","message":"msg_2","text":"edited","mentions":[],"level":null}`,
		`{"kind":"react","id":"a3","as":"anna","message":"msg_1","emoji":"eyes","on":true}`,
		`{"kind":"read","id":"a4","as":"anna","conversation":"#launch","thread":null,"up_to":3}`,
		`{"kind":"delete","id":"a5","as":"anna","message":"msg_2"}`,
	} {
		status, answer, err := f.store.Act(f.ctx, f.org, []byte(body))
		if err != nil || status != 200 {
			t.Fatalf("action %d: %d %v %v", i, status, answer, err)
		}
	}
	if got := f.missing(t); len(got) != 0 {
		t.Fatalf("G-004 broken: saved but not in the log: %v", got)
	}
}

// Dying between the two steps of a deliberately split write, with the rule
// switched off, loses a change; the check must report it.
func TestDyingInASplitWriteIsReported(t *testing.T) {
	f := setup(t)
	f.ruleSwitch(t, "messages", false)
	t.Cleanup(func() { f.ruleSwitch(t, "messages", true) })
	err := orgdb.InOrg(f.ctx, f.dir, f.org, func(tx pgx.Tx) error {
		_, err := tx.Exec(f.ctx, `INSERT INTO messages (org, id, number, version, conversation, author, author_is_agent, text, mentions, created_at)
			VALUES (current_setting('app.org'), 'msg_lost', 999, 1, '#launch', 'anna', false, 'never logged', '{}', now())`)
		return err // the first step commits; the writer dies before the log entry
	})
	if err != nil {
		t.Fatal(err)
	}
	got := f.missing(t)
	if len(got) != 1 || !strings.Contains(got[0], "msg_lost") {
		t.Fatalf("the check \"every saved change is in the log\" must report msg_lost, got %v", got)
	}
}

func TestOnlyConversationsMayWriteFacts(t *testing.T) {
	setup(t)
	for _, login := range []string{"delivery", "feed", "search", "sender", "demo"} {
		pool := connect(t, login)
		_, err := pool.Exec(context.Background(), `UPDATE conversations.messages SET text = 'x'`)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "42501" {
			t.Errorf("%s must be refused writing messages, got %v", login, err)
		}
	}
}
