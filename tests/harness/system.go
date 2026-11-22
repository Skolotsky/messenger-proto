package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// System is the composed system a guardrail test runs against.
type System struct {
	t       testing.TB
	project string
}

// Connect finds the system started by ./scripts/verify.
func Connect(t testing.TB) *System {
	t.Helper()
	p := os.Getenv("CONTEXIA_PROJECT")
	if p == "" {
		t.Fatal("CONTEXIA_PROJECT is not set. Run this test through ./scripts/verify")
	}
	return &System{t: t, project: p}
}

// URL is the address of a part from inside the system's network.
func (s *System) URL(part string) string { return "http://" + part + ":8080" }

func (s *System) container(part string) string {
	s.t.Helper()
	out, err := exec.Command("docker", "ps", "-aq",
		"--filter", "label=com.docker.compose.project="+s.project,
		"--filter", "label=com.docker.compose.service="+part).Output()
	if err != nil {
		s.t.Fatalf("find %s: %v", part, err)
	}
	id := strings.TrimSpace(string(out))
	if id == "" || strings.Contains(id, "\n") {
		s.t.Fatalf("expected one container for %s, got %q", part, id)
	}
	return id
}

func (s *System) docker(args ...string) {
	s.t.Helper()
	if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		s.t.Fatalf("docker %v: %v\n%s", args, err, out)
	}
}

// Stop shuts a part down cleanly.
func (s *System) Stop(part string) { s.docker("stop", "-t", "2", s.container(part)) }

// Kill ends a part with no chance to finish anything.
func (s *System) Kill(part string) { s.docker("kill", s.container(part)) }

// Freeze suspends a part: it keeps its connections and answers nothing.
func (s *System) Freeze(part string) { s.docker("pause", s.container(part)) }

// Thaw resumes a frozen part.
func (s *System) Thaw(part string) { s.docker("unpause", s.container(part)) }

// Start starts a stopped part and waits until it answers.
func (s *System) Start(part string) {
	s.t.Helper()
	s.docker("start", s.container(part))
	s.WaitAlive(part, 20*time.Second)
}

// Alive says whether a part answers right now.
func (s *System) Alive(part string) bool {
	c := http.Client{Timeout: time.Second}
	resp, err := c.Get(s.URL(part) + "/alive")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// WaitAlive fails the test if the part does not answer in time.
func (s *System) WaitAlive(part string, limit time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(limit)
	for !s.Alive(part) {
		if time.Now().After(deadline) {
			s.t.Fatalf("%s did not answer within %v", part, limit)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Parts are the programs that read the clock.
var Parts = []string{"conversations", "delivery", "feed", "search", "sender", "demo"}

// Advance moves the clock of every given part (all parts if none are given).
func (s *System) Advance(d time.Duration, parts ...string) {
	s.t.Helper()
	if len(parts) == 0 {
		parts = Parts
	}
	body, _ := json.Marshal(map[string]int64{"advance_ms": d.Milliseconds()})
	for _, p := range parts {
		resp, err := http.Post(s.URL(p)+"/test/clock", "application/json", bytes.NewReader(body))
		if err != nil {
			s.t.Fatalf("move the clock of %s: %v", p, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			s.t.Fatal(fmt.Errorf("move the clock of %s: status %d (is this a test build?)", p, resp.StatusCode))
		}
	}
}

// Now reads a part's clock.
func (s *System) Now(part string) time.Time {
	s.t.Helper()
	resp, err := http.Get(s.URL(part) + "/test/clock")
	if err != nil {
		s.t.Fatalf("read the clock of %s: %v", part, err)
	}
	defer resp.Body.Close()
	var out struct {
		Now time.Time `json:"now"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		s.t.Fatal(err)
	}
	return out.Now
}
