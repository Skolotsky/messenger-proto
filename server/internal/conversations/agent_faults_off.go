//go:build !agentfaults

package conversations

// agentFault is always false outside the test-only build.
func agentFault(string) bool { return false }
