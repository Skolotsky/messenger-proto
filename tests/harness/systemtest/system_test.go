// Package systemtest proves the harness can really control the running system.
package systemtest

import (
	"testing"
	"time"

	"contexia/tests/harness"
)

func TestStopStartKillFreeze(t *testing.T) {
	s := harness.Connect(t)
	const part = "sender"

	s.WaitAlive(part, 10*time.Second)
	s.Stop(part)
	if s.Alive(part) {
		t.Fatal("still answers after Stop")
	}
	s.Start(part)

	s.Kill(part)
	if s.Alive(part) {
		t.Fatal("still answers after Kill")
	}
	s.Start(part)

	s.Freeze(part)
	alive := s.Alive(part)
	s.Thaw(part) // before judging, so a failure does not leave the part frozen
	if alive {
		t.Fatal("answers while frozen")
	}
	s.WaitAlive(part, 5*time.Second)
}

func TestClockMovesOnlyWhenTold(t *testing.T) {
	s := harness.Connect(t)
	before := s.Now("feed")
	other := s.Now("search")
	s.Advance(7*24*time.Hour, "feed")
	if got := s.Now("feed").Sub(before); got != 7*24*time.Hour {
		t.Fatalf("feed moved by %v", got)
	}
	if got := s.Now("search"); !got.Equal(other) {
		t.Fatalf("search moved to %v", got)
	}
}
