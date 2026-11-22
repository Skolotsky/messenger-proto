//go:build agentfaults

package conversations

import "sync/atomic"

var plantedAgentFault atomic.Value

// SetAgentFault switches one rule of the agent door off, so a guardrail test can
// be seen failing without it. Only in the build tagged agentfaults.
func SetAgentFault(name string) { plantedAgentFault.Store(name) }

func agentFault(name string) bool {
	v, _ := plantedAgentFault.Load().(string)
	return v == name
}
