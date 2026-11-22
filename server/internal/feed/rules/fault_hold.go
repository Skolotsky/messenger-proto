package rules

// The rules G-010 and G-011 must see fail. Each can be switched off only in the
// build tagged standinfaults (fault_hold_on.go).
const (
	FaultHoldArrival   = "hold-from-arrival"  // count the hold from when a change arrived
	FaultAnyChange     = "hold-any-change"    // any change releases, not only a tick
	FaultReadKeepsHold = "hold-read-keeps-it" // what ended the row's claim leaves the entry on hold
)
