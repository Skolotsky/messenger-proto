//go:build fault_no_action_ids

package savedonce

// programTags builds the program with its action IDs forgotten: this test must fail.
const programTags = "fault_no_action_ids"
