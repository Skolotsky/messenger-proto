//go:build !fault_plain_sequence

package conversations

// plainSequence is the deliberate fault for G-005, off in every real build.
const plainSequence = false

const faultSequenceSQL = ""
