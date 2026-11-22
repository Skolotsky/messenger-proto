//go:build agentfaults

package replyinprogress

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

func TestFailsWhenTextReachesTheLog(t *testing.T) {
	mustReport(t, "text-on-log", "the log holds text of a reply in progress", offLogProblems)
}

func TestFailsWithoutTheTimeout(t *testing.T) {
	mustReport(t, "no-timeout", "after 61 s of silence", silenceProblems)
}

func TestFailsWhenARepeatAddsText(t *testing.T) {
	mustReport(t, "no-repeat-guard", "the pieces came to", piecesProblems)
}
