//go:build !fault_hub_handed && !fault_no_lease_check && !fault_no_watch_end

package conversations

// The rules of the live connection, on. Each has a test-only build that turns it
// off, for its guardrail to be seen failing (T-053).
const (
	hubSendsWhatItIsHanded = false
	watchingChecksLease    = true
	connectionEndsWatching = true
)
