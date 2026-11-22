//go:build !deliveryfaults

package delivery

import "contexia/internal/shared/call"

// fault is always false outside the test-only build with the tag deliveryfaults.
func fault(string) bool { return false }

// newCall is the one client for calls between parts (G-023).
func newCall(base string) caller { return call.New(base, 0) }
