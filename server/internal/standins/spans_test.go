package standins

import (
	"reflect"
	"testing"

	"contexia/internal/shared/contract"
)

func TestSpansCountCodePoints(t *testing.T) {
	cases := []struct {
		name, text string
		terms      []string
		want       []contract.Span
	}{
		{"plain", "sign off the pricing page", []string{"pricing"}, []contract.Span{{13, 20}}},
		{"after an emoji", "😀 pricing", []string{"pricing"}, []contract.Span{{2, 9}}},
		{"after accents", "çéö pricing", []string{"PRICING"}, []contract.Span{{4, 11}}},
		{"every occurrence", "aXa", []string{"a"}, []contract.Span{{0, 1}, {2, 3}}},
		{"overlap joined", "abcd", []string{"abc", "bcd"}, []contract.Span{{0, 4}}},
		{"nothing", "abc", []string{"z"}, []contract.Span{}},
	}
	for _, c := range cases {
		if got := Spans(c.text, c.terms); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNextThroughAfterWipeFollowsTheReplay(t *testing.T) {
	high := int64(100)
	st := searchState{refillHigh: &high}
	// A copy from before the wipe arrives first; it must not end the refill.
	through, refill := nextThrough(st, 100)
	if through != 100 || refill == nil {
		t.Fatalf("stale copy: through %d, refill %v", through, refill)
	}
	// The worker restarts from the beginning: the place falls back with it.
	st = searchState{through: through, refillHigh: refill}
	through, refill = nextThrough(st, 3)
	if through != 3 || refill == nil {
		t.Fatalf("restart: through %d, refill %v", through, refill)
	}
	st = searchState{through: through, refillHigh: refill}
	through, refill = nextThrough(st, 100)
	if through != 100 || refill != nil {
		t.Fatalf("caught up: through %d, refill %v", through, refill)
	}
	// Level again: a late copy no longer moves the place back.
	if through, _ = nextThrough(searchState{through: 100}, 7); through != 100 {
		t.Fatalf("late copy moved the place to %d", through)
	}
}
