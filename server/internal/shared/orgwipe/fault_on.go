//go:build deletefaults

package orgwipe

import "sync/atomic"

var active atomic.Value

// SetFault breaks the delete, so G-031 can be seen to fail. Names:
//
//	leave-table:<name>  the delete skips that table
//	ignore-org          the delete removes every organization's rows
//
// An empty name restores the rule.
func SetFault(name string) { active.Store(name) }

func fault(name string) bool {
	v, _ := active.Load().(string)
	return v == name
}
