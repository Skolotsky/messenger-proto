//go:build fault_no_lease_check

package conversations

// Test-only fault: a start of watching is saved without confirming the lease.
const (
	hubSendsWhatItIsHanded = false
	watchingChecksLease    = false
	connectionEndsWatching = true
)
