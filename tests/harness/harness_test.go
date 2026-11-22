package harness

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func post(t *testing.T, url, body string) (int, error) {
	t.Helper()
	resp, err := http.Post(url+"/changes", "application/json", strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func through(t *testing.T, tr Trouble, n int, seed int64) (*Recorder, *Troublemaker, int) {
	t.Helper()
	rec := NewRecorder()
	target := httptest.NewServer(rec)
	t.Cleanup(target.Close)
	m := NewTroublemaker(target.URL, seed, tr)
	front := httptest.NewServer(m)
	t.Cleanup(front.Close)
	lost := 0
	for i := 1; i <= n; i++ {
		if _, err := post(t, front.URL, `{"org":"acme","number":`+itoa(i)+`}`); err != nil {
			lost++
		}
	}
	m.Wait()
	return rec, m, lost
}

func itoa(i int) string {
	var b strings.Builder
	if i >= 10 {
		b.WriteString(itoa(i / 10))
	}
	b.WriteByte(byte('0' + i%10))
	return b.String()
}

func TestWithoutTroubleEverythingArrivesOnce(t *testing.T) {
	rec, _, lost := through(t, Trouble{}, 50, 1)
	if lost != 0 || len(rec.Deliveries()) != 50 || rec.Repeats() != 0 {
		t.Fatalf("lost=%d got=%d repeats=%d", lost, len(rec.Deliveries()), rec.Repeats())
	}
}

func TestRepeatsReachTheReceiver(t *testing.T) {
	rec, m, _ := through(t, Trouble{Repeat: 1}, 20, 1)
	if rec.Repeats() != 20 || m.Repeats.Load() != 20 {
		t.Fatalf("receiver saw %d repeats, troublemaker made %d", rec.Repeats(), m.Repeats.Load())
	}
}

func TestLostAnswerIsDeliveredButCallerHearsNothing(t *testing.T) {
	rec, _, lost := through(t, Trouble{LoseAnswer: 1}, 5, 1)
	if lost != 5 {
		t.Fatalf("caller got an answer %d times", 5-lost)
	}
	if rec.Repeats() != 0 {
		// http.Client may retry an idempotent request; a POST it must not.
		t.Fatalf("repeats=%d", rec.Repeats())
	}
	if len(rec.Deliveries()) != 5 {
		t.Fatalf("receiver got %d", len(rec.Deliveries()))
	}
}

func TestLateCopiesArriveAfterTheOriginal(t *testing.T) {
	rec, m, _ := through(t, Trouble{Late: 1, LateBy: 40 * time.Millisecond}, 3, 1)
	if m.LateCopies.Load() != 3 || rec.Repeats() != 3 {
		t.Fatalf("late=%d repeats=%d", m.LateCopies.Load(), rec.Repeats())
	}
	got := rec.Deliveries()
	if got[0].Number != 1 || got[1].Number != 2 || got[2].Number != 3 {
		t.Fatalf("originals out of order: %v", got[:3])
	}
}

func TestSameSeedSameChoices(t *testing.T) {
	counts := func() (int64, int64) {
		_, m, _ := through(t, Trouble{Repeat: .3, Late: .3, LateBy: time.Millisecond}, 60, 42)
		return m.Repeats.Load(), m.LateCopies.Load()
	}
	a1, b1 := counts()
	a2, b2 := counts()
	if a1 != a2 || b1 != b2 || a1 == 0 || b1 == 0 {
		t.Fatalf("%d/%d against %d/%d", a1, b1, a2, b2)
	}
}

func TestRecorderAnswersAsTold(t *testing.T) {
	rec := NewRecorder()
	rec.AnswerWith(func(Delivery) (int, any) { return 422, map[string]string{"refused": "no"} })
	s := httptest.NewServer(rec)
	defer s.Close()
	if code, err := post(t, s.URL, `{"org":"acme","number":1}`); err != nil || code != 422 {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

func TestRecorderIsSafeUnderLoad(t *testing.T) {
	rec := NewRecorder()
	s := httptest.NewServer(rec)
	defer s.Close()
	var wg sync.WaitGroup
	for i := 1; i <= 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = post(t, s.URL, `{"org":"acme","number":`+itoa(i)+`}`)
		}(i)
	}
	wg.Wait()
	if len(rec.Deliveries()) != 40 || rec.Repeats() != 0 {
		t.Fatalf("got=%d repeats=%d", len(rec.Deliveries()), rec.Repeats())
	}
}
