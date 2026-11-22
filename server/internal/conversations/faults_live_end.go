//go:build fault_no_watch_end

package conversations

// Test-only fault: a connection that is gone leaves its watchings open.
const (
	hubSendsWhatItIsHanded = false
	watchingChecksLease    = true
	connectionEndsWatching = false
)
