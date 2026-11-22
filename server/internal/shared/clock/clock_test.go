package clock

import (
	"testing"
	"time"
)

func TestManualMovesOnlyWhenTold(t *testing.T) {
	start := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	c := NewManual(start)
	fired := c.After(5 * time.Second)

	c.Advance(4 * time.Second)
	select {
	case <-fired:
		t.Fatal("fired before its time")
	default:
	}
	c.Advance(time.Second)
	select {
	case got := <-fired:
		if !got.Equal(start.Add(5 * time.Second)) {
			t.Fatalf("fired at %v", got)
		}
	default:
		t.Fatal("did not fire at its time")
	}
}

func TestManualCanStepBackwards(t *testing.T) {
	start := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	c := NewManual(start)
	fired := c.After(time.Second)
	c.Set(start.Add(-time.Hour))
	if !c.Now().Equal(start.Add(-time.Hour)) {
		t.Fatalf("now = %v", c.Now())
	}
	select {
	case <-fired:
		t.Fatal("a backward step must not fire a waiter")
	default:
	}
}
