//go:build !demofaults

package demo

// fault is always false outside the test build that plants faults.
func fault(string) bool { return false }
