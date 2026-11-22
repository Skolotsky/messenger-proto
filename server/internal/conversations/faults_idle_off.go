//go:build !fault_idle_scan

package conversations

// ticksDueWhere reaches the due organizations through the partial index (G-029).
const ticksDueWhere = `tick_due_at IS NOT NULL AND tick_due_at <= $1`
