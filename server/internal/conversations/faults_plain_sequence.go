//go:build fault_plain_sequence

package conversations

// plainSequence numbers changes from a plain sequence with no lock, the fault
// G-005 must catch: a later number can commit before an earlier one.
const plainSequence = true

const faultSequenceSQL = `SELECT nextval('fault_log_numbers')`
