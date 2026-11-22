//go:build deliveryfaults

package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"contexia/internal/shared/call"
)

// fault switches one rule off, to see its guardrail test fail. Names, comma
// separated, in CONTEXIA_DELIVERY_FAULT:
//
//	shared-place   the feed may not pass search's place (G-012)
//	no-parking     a refused change is retried forever (G-014)
//	no-pause       three refusals in a row do not pause (G-014)
//	notes-pass     a refused note is parked and stepped over (G-014)
//	no-time-limit  calls have no limit (G-023)
//	retry-storm    every worker retries at once, with no shared wait (G-023)
//	arrival-order  organizations are served one at a time, oldest first (G-026)
//	no-claim       a claim keeps nobody out (G-027)
//	scan-all       work is found by reading every organization's place (G-029)
//	eager-remove   the due row is removed with no later recheck (lost wake-up)
func fault(name string) bool {
	for _, f := range strings.Split(os.Getenv("CONTEXIA_DELIVERY_FAULT"), ",") {
		if f == name {
			return true
		}
	}
	return false
}

// newCall is the shared client, except with the fault no-time-limit.
func newCall(base string) caller {
	if fault("no-time-limit") {
		return unlimited{base: strings.TrimRight(base, "/")}
	}
	return call.New(base, 0)
}

// unlimited is the planted fault: a client that waits as long as it takes.
type unlimited struct{ base string }

func (u unlimited) JSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.base+path, body)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", call.ErrOutage, err)
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: status %d", call.ErrOutage, resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return &call.Refused{Status: resp.StatusCode, Body: string(answer)}
	}
	if out != nil {
		return json.Unmarshal(answer, out)
	}
	return nil
}
