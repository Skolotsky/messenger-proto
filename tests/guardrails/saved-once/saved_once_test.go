// Package savedonce holds G-003: an action a screen was told is saved, is saved
// exactly once, also when it is sent again with the same ID and when
// Conversations dies mid-burst. It builds the real program, kills it five times
// with SIGKILL during a burst, and resends one action in twenty.
package savedonce

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	kills       = 5
	workers     = 4
	acksPerKill = 40
	seed        = 46
)

type program struct {
	t    *testing.T
	bin  string
	env  []string
	url  string
	cmd  *exec.Cmd
	done chan struct{}
}

func (p *program) start() {
	p.t.Helper()
	p.cmd = exec.Command(p.bin)
	p.cmd.Env = p.env
	if err := p.cmd.Start(); err != nil {
		p.t.Fatal(err)
	}
	p.done = make(chan struct{})
	go func(c *exec.Cmd, done chan struct{}) { _ = c.Wait(); close(done) }(p.cmd, p.done)
	deadline := time.Now().Add(20 * time.Second)
	for {
		resp, err := http.Get(p.url + "/alive")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return
			}
		}
		if time.Now().After(deadline) {
			p.t.Fatal("Conversations did not answer within 20 s of starting")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (p *program) kill() {
	_ = p.cmd.Process.Kill()
	<-p.done
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func post(client *http.Client, url string, body []byte) (int, string, error) {
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(bytes.TrimSpace(b)), err
}

type action struct {
	id, message string
	body        []byte
}

func TestSavedExactlyOnceThroughKillsAndResends(t *testing.T) {
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	dbURL := fmt.Sprintf("postgres://conversations:conversations_dev@%s:5432/contexia", host)
	bin := filepath.Join(t.TempDir(), "conversations")
	build := exec.Command("go", "build", "-tags", programTags, "-o", bin, "contexia/cmd/conversations")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the program: %v\n%s", err, out)
	}
	port := freePort(t)
	p := &program{t: t, bin: bin, url: fmt.Sprintf("http://127.0.0.1:%d", port),
		env: append(os.Environ(), "CONTEXIA_DATABASE_URL="+dbURL, fmt.Sprintf("CONTEXIA_LISTEN=127.0.0.1:%d", port))}
	p.start()
	t.Cleanup(func() { p.kill() })

	b := make([]byte, 6)
	_, _ = rand.Read(b)
	org := "g003-" + hex.EncodeToString(b)
	client := &http.Client{Timeout: 3 * time.Second}
	base := p.url + "/orgs/" + org
	seedOrg(t, client, base, org)

	rng := mrand.New(mrand.NewSource(seed))
	var mu sync.Mutex
	acked := map[string]string{} // action ID → the first answer
	byID := map[string]action{}
	var waiting []action
	next := 0
	take := func() action {
		mu.Lock()
		defer mu.Unlock()
		if len(waiting) > 0 {
			a := waiting[0]
			waiting = waiting[1:]
			return a
		}
		next++
		id, msg := fmt.Sprintf("act_%05d", next), fmt.Sprintf("msg_%05d", next)
		body, _ := json.Marshal(map[string]any{"kind": "send", "id": id, "as": "anna", "message": map[string]any{
			"id": msg, "conversation": "#load", "thread": nil, "text": "burst " + msg, "mentions": []string{}, "level": nil}})
		a := action{id: id, message: msg, body: body}
		byID[id] = a
		return a
	}
	giveBack := func(a action) {
		mu.Lock()
		defer mu.Unlock()
		waiting = append(waiting, a)
	}
	var problems atomic.Int64
	report := func(format string, args ...any) {
		problems.Add(1)
		t.Errorf(format, args...)
	}
	// send posts once; an action with no answer goes back to wait, as a screen's would.
	send := func(a action) bool {
		status, answer, err := post(client, base+"/people/actions", a.body)
		if err != nil || status >= 500 {
			giveBack(a)
			return false
		}
		if status != 200 {
			report("action %s refused: %d %s", a.id, status, answer)
			return true
		}
		mu.Lock()
		first, seen := acked[a.id]
		if !seen {
			acked[a.id] = answer
		}
		resend := rng.Intn(20) == 0
		mu.Unlock()
		if seen && first != answer {
			report("G-003 broken: action %s answered %s, then %s", a.id, first, answer)
		}
		if resend {
			again, answer2, err := post(client, base+"/people/actions", a.body)
			if err == nil && again < 500 && (again != 200 || answer2 != answer) {
				report("G-003 broken: resent action %s answered %d %s, first answer was %s", a.id, again, answer2, answer)
			}
		}
		return true
	}

	for round := 1; round <= kills; round++ {
		mu.Lock()
		target := len(acked) + acksPerKill
		mu.Unlock()
		stop := make(chan struct{})
		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					if !send(take()) {
						return // the program is gone
					}
				}
			}()
		}
		deadline := time.Now().Add(20 * time.Second)
		for {
			mu.Lock()
			n := len(acked)
			mu.Unlock()
			if n >= target {
				break
			}
			if time.Now().After(deadline) {
				close(stop)
				wg.Wait()
				t.Fatalf("round %d: the burst did not reach %d saved actions", round, target)
			}
			time.Sleep(time.Millisecond)
		}
		p.kill() // mid-burst: requests are in flight
		close(stop)
		wg.Wait()
		p.start()
	}

	// Everything that never got an answer is sent again until it has one.
	for {
		mu.Lock()
		if len(waiting) == 0 {
			mu.Unlock()
			break
		}
		a := waiting[0]
		waiting = waiting[1:]
		mu.Unlock()
		if !send(a) {
			t.Fatalf("action %s got no answer from a running program", a.id)
		}
	}
	if problems.Load() > 0 {
		t.FailNow()
	}

	checkStore(t, dbURL, ids.Org(org), acked, byID)
}

func seedOrg(t *testing.T, client *http.Client, base, org string) {
	t.Helper()
	status, answer, err := post(client, base+"/seed/open", []byte(`{}`))
	if err != nil || status != 200 {
		t.Fatalf("open seed: %d %s %v", status, answer, err)
	}
	var opened struct {
		Session string `json:"session"`
	}
	if err := json.Unmarshal([]byte(answer), &opened); err != nil {
		t.Fatal(err)
	}
	at := "2026-11-01T09:00:00.000Z"
	batch, _ := json.Marshal(map[string]any{"changes": []map[string]any{
		{"kind": "person.joined", "org": org, "reset": 1, "number": 1, "previous": 0, "at": at, "person": "anna", "name": "Anna", "is_agent": false},
		{"kind": "channel.created", "org": org, "reset": 1, "number": 2, "previous": 1, "at": at, "conversation": "#load", "name": "load", "people": []string{"anna"}},
	}})
	if status, answer, err = post(client, base+"/seed/"+opened.Session, batch); err != nil || status != 200 {
		t.Fatalf("seed batch: %d %s %v", status, answer, err)
	}
	if status, answer, err = post(client, base+"/seed/"+opened.Session+"/seal", []byte(`{}`)); err != nil || status != 200 {
		t.Fatalf("seal: %d %s %v", status, answer, err)
	}
}

func checkStore(t *testing.T, dbURL string, org ids.Org, acked map[string]string, byID map[string]action) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	stored := map[string]int{}
	logged := map[string]int{}
	numbers := map[string]int64{}
	err = orgdb.InOrg(ctx, orgdb.Single{Pool: pool}, org, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM messages WHERE org = $1`, string(org))
		if err != nil {
			return err
		}
		list, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, id := range list {
			stored[id]++
		}
		rows, err = tx.Query(ctx, `SELECT body->'message'->>'id', number FROM change_log WHERE org = $1 AND kind = 'message.created'`, string(org))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var n int64
			if err := rows.Scan(&id, &n); err != nil {
				return err
			}
			logged[id]++
			numbers[id] = n
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(acked) < kills*acksPerKill {
		t.Fatalf("only %d actions were acknowledged", len(acked))
	}
	for id, answer := range acked {
		msg := byID[id].message
		if stored[msg] != 1 {
			t.Errorf("G-003 broken: acknowledged %s is in the store %d times", id, stored[msg])
		}
		if logged[msg] != 1 {
			t.Errorf("G-003 broken: acknowledged %s is in the log %d times", id, logged[msg])
		}
		var saved struct {
			Number int64 `json:"number"`
		}
		if err := json.Unmarshal([]byte(answer), &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Number != numbers[msg] {
			t.Errorf("G-003 broken: %s was answered number %d but its log entry is %d", id, saved.Number, numbers[msg])
		}
	}
	t.Logf("%d actions acknowledged through %d kills; each in the store and the log once", len(acked), kills)
}
