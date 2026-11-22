//go:build agentfaults

package idle

import (
	"strings"
	"testing"

	"contexia/internal/conversations"
)

func TestFailsWhenTheAgentDoorReadsIdleOrganizations(t *testing.T) {
	conversations.SetAgentFault("scan-idle")
	defer conversations.SetAgentFault("")
	problems := agentIdleProblems(t)
	for _, p := range problems {
		if strings.Contains(p, "must not read the idle ones") {
			t.Logf("failed for the right reason: %s", p)
			return
		}
	}
	t.Fatalf("a question that reads every organization was not caught: %v", problems)
}
