//go:build integration && fault_no_row_rules

package conversations

// rowRulesOff makes the row-rules test run its planted query with the rules
// off: it must then fail.
const rowRulesOff = true
