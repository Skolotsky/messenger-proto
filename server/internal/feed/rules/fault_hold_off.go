//go:build !standinfaults

package rules

import "time"

// fault is always false outside the test-only build.
func fault(string) bool { return false }

func arrivalNow() time.Time { return time.Time{} }
