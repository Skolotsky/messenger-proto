//go:build !deletefaults

package orgwipe

// fault is always false outside the test-only build.
func fault(string) bool { return false }
