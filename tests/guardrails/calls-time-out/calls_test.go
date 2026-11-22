// Package callstimeout holds G-023: every call between parts gives up within
// 3 s, retries wait 1, 2, 4 … 60 s with random variation, kept once per
// receiver, and no other way of calling exists.
package callstimeout

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"contexia/internal/shared/clock"
	"contexia/tests/guardrails/receivers-independent/rig"
)

var bg = context.Background()

func TestAReceiverThatNeverAnswersStallsNobody(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "acme"
	b.DB.AddOrg(t, org, 60)
	b.Search.SetMode(rig.Off)
	b.Start(t, "copy-1")
	for range 30 {
		if _, err := b.DB.Save(bg, org); err != nil {
			t.Fatal(err)
		}
	}
	rig.Until(t, 5*time.Second, "the feed gets everything while search never answers", func() bool { return b.Feed.Place(org) >= 30 })

	var failures int
	rig.Until(t, 8*time.Second, "the call to search gives up and the outage is noted. A call without a limit never does", func() bool {
		if err := b.DB.Del.QueryRow(bg, `SELECT failures FROM delivery_receivers WHERE receiver = 'search'`).Scan(&failures); err != nil {
			t.Fatal(err)
		}
		return failures >= 1
	})
	rig.Until(t, 2*time.Second, "the receiver saw the caller hang up", func() bool { return b.Search.LongestHang.Load() > 0 })
	// The limit is 3 s; the margin is only for the hang-up to travel.
	if got := time.Duration(b.Search.LongestHang.Load()); got > 3*time.Second+250*time.Millisecond {
		t.Fatalf("a call stayed open %v, over the 3 s of G-023", got)
	}
}

// retryState reads the shared wait of a receiver.
func retryState(t *testing.T, b *rig.Bench, receiver string) (int, time.Time) {
	t.Helper()
	var failures int
	var at *time.Time
	if err := b.DB.Del.QueryRow(bg, `SELECT failures, retry_at FROM delivery_receivers WHERE receiver = $1`, receiver).Scan(&failures, &at); err != nil {
		return 0, time.Time{}
	}
	if at == nil {
		return failures, time.Time{}
	}
	return failures, *at
}

func TestRetriesGrowWithJitterAndAreSharedByEveryCopy(t *testing.T) {
	start := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	c := clock.NewManual(start)
	b := rig.NewBench(t, c)
	const org = "acme"
	b.DB.AddOrg(t, org, 60)
	b.Feed.SetMode(rig.Broken)
	for range 10 {
		if _, err := b.DB.Save(bg, org); err != nil {
			t.Fatal(err)
		}
	}
	// Three copies: more workers must not mean more retries.
	b.Start(t, "copy-1")
	b.Start(t, "copy-2")
	b.Start(t, "copy-3")

	wants := []time.Duration{1, 2, 4, 8, 16, 20, 20, 20, 20} // at most 20 s (D-035)
	var varied bool
	for i, want := range wants {
		want *= time.Second
		var at time.Time
		rig.Until(t, 5*time.Second, "the failed try is noted", func() bool {
			var f int
			f, at = retryState(t, b, "feed")
			return f == i+1
		})
		now := c.Now()
		wait := at.Sub(now)
		low, high := time.Duration(float64(want)*0.79), min(time.Duration(float64(want)*1.21), 20*time.Second)
		if wait < low || wait > high {
			t.Fatalf("try %d: the wait is %v, expected about %v (from %v to %v)", i+1, wait, want, low, high)
		}
		if wait != want {
			varied = true
		}
		calls := b.Feed.Calls.Load()
		// Just before the wait ends nobody calls.
		c.Set(at.Add(-time.Millisecond))
		quiet := time.Now().Add(120 * time.Millisecond)
		for time.Now().Before(quiet) {
			if got := b.Feed.Calls.Load(); got != calls {
				t.Fatalf("try %d: %d calls before the shared wait ended", i+1, got-calls)
			}
			time.Sleep(5 * time.Millisecond)
		}
		c.Set(at.Add(time.Millisecond))
		rig.Until(t, 5*time.Second, "a try is made when the wait ends", func() bool { return b.Feed.Calls.Load() > calls })
		// Let any other copy that would also try show itself.
		settle := time.Now().Add(150 * time.Millisecond)
		for time.Now().Before(settle) {
			time.Sleep(5 * time.Millisecond)
		}
		if got := b.Feed.Calls.Load() - calls; got != 1 {
			t.Fatalf("try %d: %d calls at the end of one wait, with three copies. The wait must be shared", i+1, got)
		}
	}
	if !varied {
		t.Fatal("no wait differed from the plain doubling: the random variation is missing")
	}

	// Back up: everything arrives and the state clears.
	b.Feed.SetMode(rig.Up)
	_, at := retryState(t, b, "feed")
	c.Set(at.Add(time.Second))
	rig.Until(t, 5*time.Second, "the feed gets everything after the outage", func() bool { return b.Feed.Place(org) >= 10 })
	rig.Until(t, 5*time.Second, "the outage is cleared", func() bool {
		f, _ := retryState(t, b, "feed")
		return f == 0
	})
}

// TestNoOtherClient is the lint rule of G-023: only the shared client calls out.
func TestNoOtherClient(t *testing.T) {
	root := filepath.Join("..", "..", "..", "server")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("server sources not found at %s: %v", root, err)
	}
	banned := []string{"http.Client{", "http.DefaultClient", "http.Get(", "http.Post(", "http.PostForm(", "http.Head("}
	var bad []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		if strings.Contains(filepath.ToSlash(path), "internal/shared/call/") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 1<<20), 1<<20)
		for line := 1; scan.Scan(); line++ {
			text := scan.Text()
			if line == 1 && strings.Contains(text, "go:build deliveryfaults") {
				return nil // the test-only build that plants this very fault
			}
			for _, b := range banned {
				if strings.Contains(text, b) {
					bad = append(bad, path+": "+strings.TrimSpace(text))
				}
			}
		}
		return scan.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Fatalf("calls between parts must go through internal/shared/call (G-023). Found:\n%s", strings.Join(bad, "\n"))
	}
}
