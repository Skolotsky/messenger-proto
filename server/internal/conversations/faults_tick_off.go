//go:build !fault_tick_from_newest

package conversations

// tickFromNewest is the fault D-034 names: a tick timed from the newest entry, so
// a busy organization never gets one. Off in every real build.
const tickFromNewest = false
