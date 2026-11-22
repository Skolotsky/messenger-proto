//go:build demofaults

package demo

import (
	"os"
	"slices"
	"strings"
)

// fault plants one broken rule for G-020's must-fail tests, named in
// CONTEXIA_DEMO_FAULT: vary_seed, skip_level, wipe_all, no_restart.
func fault(name string) bool {
	return slices.Contains(strings.Split(os.Getenv("CONTEXIA_DEMO_FAULT"), ","), name)
}
