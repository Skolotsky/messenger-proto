//go:build testclock

package clock

import (
	"encoding/json"
	"net/http"
	"time"
)

var testClock = NewManual(time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC))

// Default is, in a test build, a clock the test harness moves.
func Default() Clock { return testClock }

// ControlHandler moves the clock: POST {"advance_ms": 5000} or {"set": "<RFC 3339>"}.
func ControlHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var in struct {
				AdvanceMS int64      `json:"advance_ms"`
				Set       *time.Time `json:"set"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if in.Set != nil {
				testClock.Set(*in.Set)
			}
			if in.AdvanceMS != 0 {
				testClock.Advance(time.Duration(in.AdvanceMS) * time.Millisecond)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]time.Time{"now": testClock.Now()})
	})
}
