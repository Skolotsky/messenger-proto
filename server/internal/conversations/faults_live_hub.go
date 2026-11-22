//go:build fault_hub_handed

package conversations

// Test-only fault: the hub sends what the saving handlers hand it.
const (
	hubSendsWhatItIsHanded = true
	watchingChecksLease    = true
	connectionEndsWatching = true
)
