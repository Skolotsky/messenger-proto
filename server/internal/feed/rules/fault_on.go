//go:build g030fault

package rules

// crossConversationFault makes a row's bucket depend on how much the reader
// received from other conversations before it. Only for seeing G-030 fail:
//
//	TOOL_DIR=tests ./scripts/tool go test -tags g030fault ./guardrails/conversations-independent/
const crossConversationFault = true
