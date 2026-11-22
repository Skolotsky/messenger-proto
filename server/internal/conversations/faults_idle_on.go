//go:build fault_idle_scan

package conversations

// Test-only fault for G-029: the same question, written so it reads every row.
const ticksDueWhere = `coalesce(tick_due_at, 'infinity') <= $1`
