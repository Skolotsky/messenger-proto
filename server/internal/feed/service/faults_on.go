//go:build feedfaults

package service

import "sync/atomic"

var planted atomic.Value

// SetFault switches on one planted fault, in the test-only build.
func SetFault(name string) { planted.Store(name) }

func fault(name string) bool {
	v, _ := planted.Load().(string)
	return v == name
}
