//go:build standinfaults

package standins

import "sync/atomic"

var active atomic.Value

// SetFault switches off one rule, so a guardrail test can be seen to fail.
// An empty name restores every rule.
func SetFault(name string) { active.Store(name) }

func fault(name string) bool {
	v, _ := active.Load().(string)
	return v == name
}
