// Package retiredeleteretry holds the part of G-031 about a parked retry: a retry
// in flight holds a claim, so stop does not answer 200 until it has ended, and
// nothing can land after a finished delete.
package retiredeleteretry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/delivery"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
)

var bg = context.Background()

// gate lets calls through to a receiver, except one it can hold inside the call.
type gate struct {
	proxy   *httputil.ReverseProxy
	armed   atomic.Int64 // the number whose next plain delivery is held; 0 is none
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGate(t *testing.T, target string) (*gate, string) {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	g := &gate{proxy: httputil.NewSingleHostReverseProxy(u), entered: make(chan struct{}), release: make(chan struct{})}
	front := httptest.NewServer(g)
	t.Cleanup(func() {
		g.open()
		front.Close()
	})
	return g, front.URL
}

func (g *gate) open() { g.once.Do(func() { close(g.release) }) }

func (g *gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if want := g.armed.Load(); want != 0 && r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var h struct {
			Number int64 `json:"number"`
			Skip   bool  `json:"skip"`
		}
		if json.Unmarshal(body, &h) == nil && h.Number == want && !h.Skip && g.armed.CompareAndSwap(want, 0) {
			close(g.entered)
			<-g.release
		}
	}
	g.proxy.ServeHTTP(w, r)
}

func post(url string, limit time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(bg, limit)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func parkedCount(t *testing.T, ops, org string) int {
	t.Helper()
	resp, err := http.Get(ops + "/orgs/" + org + "/parked")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var l contract.ParkedList
	if err := json.NewDecoder(resp.Body).Decode(&l); err != nil {
		t.Fatal(err)
	}
	return len(l.Parked)
}

// stopDuringRetry parks one item, holds its retry inside the receiver's call,
// calls stop, and answers what stop said while the call was still in flight.
func stopDuringRetry(t *testing.T, receiver string) int {
	t.Helper()
	b := rig.NewBench(t, nil)
	org := fmt.Sprintf("retry-%d", time.Now().UnixNano())
	b.DB.AddOrg(t, org, 60)

	var number int64
	var refusing atomic.Bool
	refusing.Store(true)
	refuse := func(_ string, n int64) bool { return n == atomic.LoadInt64(&number) && refusing.Load() }
	searchURL, senderURL := b.Search.URL, b.Sender.URL
	var g *gate
	if receiver == "search" {
		g, searchURL = newGate(t, b.Search.URL)
		b.Search.Refuse(refuse)
		for {
			n, err := b.DB.Save(bg, org)
			if err != nil {
				t.Fatal(err)
			}
			if isMessage(t, b, org, n) {
				atomic.StoreInt64(&number, n)
				break
			}
		}
	} else {
		g, senderURL = newGate(t, b.Sender.URL)
		b.Sender.Refuse(refuse)
		atomic.StoreInt64(&number, 1)
		for range 3 {
			if _, err := b.DB.SaveNote(bg, org, "send"); err != nil {
				t.Fatal(err)
			}
		}
	}

	s := delivery.New(delivery.Config{DB: b.DB.Directory(), Clock: b.Clock, Search: searchURL, Feed: b.Feed.URL, Sender: senderURL, Seed: 47, Worker: "copy-1"})
	ctx, cancel := context.WithCancel(bg)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Run(ctx)
	}()
	t.Cleanup(func() {
		g.open()
		cancel()
		wg.Wait()
	})
	ops := httptest.NewServer(s.Routes())
	t.Cleanup(ops.Close)

	rig.Until(t, 15*time.Second, "the item is parked", func() bool { return parkedCount(t, ops.URL, org) == 1 })
	refusing.Store(false)
	g.armed.Store(atomic.LoadInt64(&number))

	retried := make(chan struct{})
	go func() {
		defer close(retried)
		_, _ = post(fmt.Sprintf("%s/orgs/%s/parked/%d/retry?receiver=%s", ops.URL, org, atomic.LoadInt64(&number), receiver), 10*time.Second)
	}()
	select {
	case <-g.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the retry never reached the receiver")
	}

	during, err := post(ops.URL+"/orgs/"+org+"/stop", 2900*time.Millisecond)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	select {
	case <-retried:
		t.Fatal("the retry ended while it was being held, so this run proved nothing")
	default:
	}

	g.open()
	<-retried
	rig.Until(t, 10*time.Second, "stop answers 200 once the retry has ended", func() bool {
		st, err := post(ops.URL+"/orgs/"+org+"/stop", 2900*time.Millisecond)
		return err == nil && st == http.StatusOK
	})
	return during
}

func isMessage(t *testing.T, b *rig.Bench, org string, n int64) bool {
	t.Helper()
	var kind string
	err := orgdb.InOrg(bg, orgdb.Single{Pool: b.DB.Conversations}, ids.Org(org), func(tx pgx.Tx) error {
		return tx.QueryRow(bg, `SELECT kind FROM change_log WHERE org = $1 AND number = $2`, org, n).Scan(&kind)
	})
	if err != nil {
		t.Fatal(err)
	}
	return kind == "message.created"
}

func TestStopWaitsForARetryInFlight(t *testing.T) {
	for _, receiver := range []string{"search", "sender"} {
		t.Run(receiver, func(t *testing.T) {
			if got := stopDuringRetry(t, receiver); got != http.StatusServiceUnavailable {
				t.Fatalf("stop answered %d while a retry was inside the receiver's call; it must answer 503 until the retry ends", got)
			}
		})
	}
}
