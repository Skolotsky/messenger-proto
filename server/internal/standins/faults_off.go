//go:build !standinfaults

package standins

// fault is always false outside the test-only build.
func fault(string) bool { return false }
