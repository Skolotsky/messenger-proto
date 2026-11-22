package delivery

import (
	"testing"
	"time"
)

// D-035: the pause between retries grows and never passes 20 seconds.
func TestThePauseBetweenRetriesIsAtMost20Seconds(t *testing.T) {
	s := New(Config{Seed: 1})
	longest := time.Duration(0)
	for failures := 1; failures <= 40; failures++ {
		for range 200 {
			longest = max(longest, s.backoff(failures))
		}
	}
	if longest > 20*time.Second {
		t.Fatalf("a pause of %v; D-035 allows at most 20 s", longest)
	}
	if longest < 16*time.Second {
		t.Fatalf("the longest pause is %v: it should grow to about 20 s", longest)
	}
	if first := s.backoff(1); first > 1200*time.Millisecond {
		t.Fatalf("the first pause is %v, want about 1 s", first)
	}
}
