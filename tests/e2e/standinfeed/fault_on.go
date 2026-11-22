//go:build standinfaults

package main

import (
	"log/slog"
	"os"

	"contexia/internal/feed/rules"
)

// A planted fault in the rules, so a test can see its check fail.
func init() {
	if f := os.Getenv("CONTEXIA_E2E_RULE_FAULT"); f != "" {
		rules.SetFault(f)
		slog.Info("rule fault on: " + f)
	}
}
