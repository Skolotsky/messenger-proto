//go:build fault_no_action_ids

package conversations

// rememberActionIDs off is the fault G-003 must catch: a resent action is
// handled as if it were new.
const rememberActionIDs = false
