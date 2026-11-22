//go:build agentfaults

package agentevents

import (
	"strings"
	"testing"

	"contexia/internal/conversations"
	r "contexia/tests/guardrails/agent-events/agentrig"
)

func mustReport(t *testing.T, fault, want string, check func(*r.H) []string) {
	t.Helper()
	conversations.SetAgentFault(fault)
	defer conversations.SetAgentFault("")
	problems := check(r.New(t))
	for _, p := range problems {
		if strings.Contains(p, want) {
			t.Logf("failed for the right reason: %s", p)
			return
		}
	}
	t.Fatalf("with %q planted the check did not report %q: %v", fault, want, problems)
}

func TestFailsWhenAnEventIsDropped(t *testing.T) {
	mustReport(t, "drop-replies", "the agent was told", toldProblems)
}

func TestFailsWithoutTooFarBehind(t *testing.T) {
	mustReport(t, "no-too-far", "expected 410 too_far_behind", tooFarProblems)
}
