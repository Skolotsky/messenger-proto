package call

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGivesUpOnServerThatAcceptsAndNeverAnswers(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	assertGivesUp(t, "http://"+l.Addr().String())
}

func TestGivesUpWhenAnswerStallsHalfway(t *testing.T) {
	release := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"partial":`))
		w.(http.Flusher).Flush()
		<-release
	}))
	defer s.Close()
	defer close(release)
	assertGivesUp(t, s.URL)
}

func assertGivesUp(t *testing.T, url string) {
	t.Helper()
	start := time.Now()
	err := New(url, 100*time.Millisecond).JSON(context.Background(), "GET", "/x", nil, &map[string]any{})
	if !errors.Is(err, ErrOutage) {
		t.Fatalf("want an outage, got %v", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("took %v", took)
	}
}

func TestLimitCannotBeRaised(t *testing.T) {
	if got := New("http://x", time.Hour).limit; got != Limit {
		t.Fatalf("limit = %v", got)
	}
	if got := New("http://x", 0).limit; got != Limit {
		t.Fatalf("limit = %v", got)
	}
}

func TestAnswersAreClassified(t *testing.T) {
	for status, wantOutage := range map[int]bool{500: true, 503: true, 429: true, 408: true, 400: false, 404: false, 409: false, 302: false} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
		err := New(s.URL, 0).JSON(context.Background(), "POST", "/x", map[string]int{"a": 1}, nil)
		s.Close()
		var refused *Refused
		if wantOutage && !errors.Is(err, ErrOutage) {
			t.Errorf("%d: want outage, got %v", status, err)
		}
		if !wantOutage && !errors.As(err, &refused) {
			t.Errorf("%d: want refusal, got %v", status, err)
		}
	}
}

func TestDecodesAnswer(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"through":7}`)) }))
	defer s.Close()
	var out struct {
		Through int `json:"through"`
	}
	if err := New(s.URL, 0).JSON(context.Background(), "GET", "/x", nil, &out); err != nil || out.Through != 7 {
		t.Fatalf("out=%v err=%v", out, err)
	}
}
