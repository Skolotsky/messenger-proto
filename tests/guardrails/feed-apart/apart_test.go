// Package feedapart holds the second half of G-008: the feed's code holds no
// client for Conversations, and the real feed program is rebuilt from the log and
// answers its questions while Conversations is stopped.
package feedapart

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"contexia/tests/guardrails/feed-equals-replay/feedrig"
	"contexia/tests/guardrails/notification-once/rig"
	"contexia/tests/harness"
)

var forbidden = []*regexp.Regexp{
	regexp.MustCompile(`"contexia/internal/conversations`),
	regexp.MustCompile(`"contexia/internal/delivery`),
	regexp.MustCompile(`\bconversations\.[a-z_]+\b`),      // a table of its schema
	regexp.MustCompile(`https?://conversations`),          // its address
	regexp.MustCompile(`CONTEXIA_CONVERSATIONS`),          // its address, by setting
	regexp.MustCompile(`"contexia/internal/shared/call"`), // the feed calls nobody
}

// lint lists the lines of the feed's own code that could reach Conversations.
func lint(t *testing.T, roots ...string) []string {
	t.Helper()
	var bad []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				if code, _, ok := strings.Cut(line, " // "); ok {
					line = code // a comment may name a contract file
				}
				for _, re := range forbidden {
					if re.MatchString(line) {
						bad = append(bad, path+":"+itoa(i+1)+": "+strings.TrimSpace(line))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return bad
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

var feedCode = []string{"../../../server/internal/feed", "../../../server/cmd/feed"}

func TestFeedCodeHoldsNoClientForConversations(t *testing.T) {
	if bad := lint(t, feedCode...); len(bad) > 0 {
		t.Fatalf("the feed must never call Conversations or read its schema (G-008). Remove:\n%s", strings.Join(bad, "\n"))
	}
}

// The lint must be able to fail: a planted client in a scratch copy of the code.
func TestLintFailsOnAPlantedClient(t *testing.T) {
	dir := t.TempDir()
	planted := "package x\n\nimport _ \"contexia/internal/conversations\"\n\nconst q = `SELECT * FROM conversations.messages`\n"
	if err := os.WriteFile(filepath.Join(dir, "planted.go"), []byte(planted), 0o600); err != nil {
		t.Fatal(err)
	}
	if bad := lint(t, dir); len(bad) != 2 {
		t.Fatalf("the lint found %d of the 2 planted lines: %v", len(bad), bad)
	}
}

// The real path: Conversations saves a seeded log, the real delivery worker
// carries it, and the real feed ends equal to a clean fold of that same log.
func TestRealDeliveryReachesTheFeed(t *testing.T) {
	sys := harness.Connect(t)
	for _, part := range []string{"conversations", "delivery", "feed"} {
		sys.WaitAlive(part, 20*time.Second)
	}
	org := rig.NewOrg("g008d")
	base := sys.URL("conversations") + "/orgs/" + org
	var opened struct {
		Session string `json:"session"`
	}
	mustPost(t, base+"/seed/open", map[string]any{}, &opened)
	all := feedrig.FromLog(rig.Log(org, 12, 260))
	// Sealing writes seed.sealed itself, and the log writer's seal comes last in
	// its opening, so only what precedes it is sent.
	var batch []json.RawMessage
	var sent []feedrig.Change
	for _, c := range all {
		if c.Kind == "seed.sealed" {
			break
		}
		batch, sent = append(batch, c.Body), append(sent, c)
	}
	mustPost(t, base+"/seed/"+opened.Session, map[string]any{"changes": batch}, nil)
	var sealed struct {
		Through int64 `json:"through"`
	}
	mustPost(t, base+"/seed/"+opened.Session+"/seal", map[string]any{}, &sealed)
	feedURL := sys.URL("feed")
	deadline := time.Now().Add(20 * time.Second)
	for feedrig.Place(t, feedURL, org).Through < sealed.Through {
		if time.Now().After(deadline) {
			t.Fatalf("the worker did not bring the feed to %d: it is at %d", sealed.Through, feedrig.Place(t, feedURL, org).Through)
		}
		time.Sleep(20 * time.Millisecond) // waiting on a real worker, checked against a deadline
	}
	truth := feedrig.Fold(t, org, sent, sent[len(sent)-1].Number, nil)
	if bad := feedrig.Diff(t, feedURL, org, truth); len(bad) > 0 {
		t.Fatalf("after real delivery the feed differs from the log (%d): %s", len(bad), bad[0])
	}
}

func mustPost(t *testing.T, url string, in, out any) {
	t.Helper()
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s: %d %s", url, resp.StatusCode, answer)
	}
	if out != nil {
		if err := json.Unmarshal(answer, out); err != nil {
			t.Fatal(err)
		}
	}
}

// The real program, with Conversations stopped: fill it, wipe it, rebuild it
// from the log, and its answers are what they were.
func TestRebuiltAndAnsweringWithConversationsStopped(t *testing.T) {
	sys := harness.Connect(t)
	base := sys.URL("feed")
	sys.WaitAlive("feed", 20*time.Second)
	sys.Stop("conversations")
	defer sys.Start("conversations")
	if sys.Alive("conversations") {
		t.Fatal("Conversations still answers; the test proves nothing")
	}
	org := rig.NewOrg("g008")
	log := feedrig.FromLog(rig.Log(org, 8, 260))
	last := log[len(log)-1].Number
	rng := rand.New(rand.NewSource(8))
	feedrig.Deliver(t, base, log, last, rng)
	truth := feedrig.Fold(t, org, log, last, nil)
	if bad := feedrig.Diff(t, base, org, truth); len(bad) > 0 {
		t.Fatalf("filled with Conversations stopped, the feed differs from the log (%d): %s", len(bad), bad[0])
	}
	before := map[string]feedrig.Reader{}
	for _, p := range truth.E.Readers() {
		before[p] = feedrig.Ask(t, base, org, p)
	}
	edition := feedrig.Place(t, base, org).Edition
	feedrig.Wipe(t, base, org, false)
	if p := feedrig.Place(t, base, org); p.Through != 0 || p.Edition != edition+1 {
		t.Fatalf("after the wipe the place is %+v", p)
	}
	feedrig.Deliver(t, base, log, last, rng)
	for _, p := range truth.E.Readers() {
		if bad := feedrig.DiffReader(p, feedrig.Ask(t, base, org, p), before[p], nil); len(bad) > 0 {
			t.Fatalf("wiping and rebuilding changed what a reader sees (%d): %s", len(bad), bad[0])
		}
	}
}
