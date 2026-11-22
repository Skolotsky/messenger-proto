//go:build agentfaults

package agentlimits

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

func TestFailsWithoutTheChainCap(t *testing.T) {
	mustReport(t, "no-chain", "expected to stop at 3", chainProblems)
}

func TestFailsWithoutTheRateLimit(t *testing.T) {
	mustReport(t, "no-rate", "expected 20 accepted", rateProblems)
}
