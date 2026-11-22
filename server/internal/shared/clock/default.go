//go:build !testclock

package clock

import "net/http"

// Default is the clock a program runs on: the machine's.
func Default() Clock { return Real{} }

// ControlHandler is nil outside test builds: nothing can move a real clock.
func ControlHandler() http.Handler { return nil }
