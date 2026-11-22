//go:build demofaults

package rebuild

import (
	"testing"
	"time"
)

// Each planted fault breaks one rule of G-020; the check for it must then report it.

func TestFailsWhenResetsUseDifferentSeeds(t *testing.T) {
	t.Setenv("CONTEXIA_DEMO_FAULT", "vary_seed")
	if err := checkTwoResets(t, newSystem(t, 0)); err == nil {
		t.Fatal("two different histories passed as equal starting states")
	} else {
		t.Log("caught:", err)
	}
}

func TestFailsWhenResetDoesNotWaitForLevel(t *testing.T) {
	t.Setenv("CONTEXIA_DEMO_FAULT", "skip_level")
	if err := checkLevelBeforeCurrent(t, newSystem(t, 8*time.Second)); err == nil {
		t.Fatal("Reset moved current before the feed was level, and the check passed")
	} else {
		t.Log("caught:", err)
	}
}

func TestFailsWhenRebuildWipesEveryOrganization(t *testing.T) {
	t.Setenv("CONTEXIA_DEMO_FAULT", "wipe_all")
	if err := checkRebuild(t, newSystem(t, 0), false); err == nil {
		t.Fatal("a rebuild that wiped the other organization passed")
	} else {
		t.Log("caught:", err)
	}
}

func TestFailsWhenRebuildDoesNotRefill(t *testing.T) {
	t.Setenv("CONTEXIA_DEMO_FAULT", "no_restart")
	if err := checkRebuild(t, newSystem(t, 0), false); err == nil {
		t.Fatal("a rebuild that never refilled passed")
	} else {
		t.Log("caught:", err)
	}
}
