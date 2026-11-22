//go:build standinfaults

package rules

import (
	"sync/atomic"
	"time"
)

var (
	active  atomic.Value
	arrival atomic.Value
)

// SetFault switches off one rule, so a guardrail test can be seen to fail.
// An empty name restores every rule.
func SetFault(name string) { active.Store(name) }

// SetArrival is the machine clock FaultHoldArrival reads.
func SetArrival(t time.Time) { arrival.Store(t) }

func fault(name string) bool {
	v, _ := active.Load().(string)
	return v == name
}

func arrivalNow() time.Time {
	t, _ := arrival.Load().(time.Time)
	return t
}
