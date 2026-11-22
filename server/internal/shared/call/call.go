// Package call is the one client for calls between parts (G-023). Every call
// gives up within Limit, including reading the answer, and fails in one of two
// stated ways: the other side is out, or it refused this request.
package call

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Limit is the longest any call between parts may take.
const Limit = 3 * time.Second

const maxAnswer = 16 << 20

// ErrOutage means the other side did not answer usefully: unreachable, timed
// out, or a server error. The caller waits and tries again.
var ErrOutage = errors.New("receiver unavailable")

// Refused means the other side answered and will not take this request.
type Refused struct {
	Status int
	Body   string
}

func (r *Refused) Error() string { return fmt.Sprintf("refused with %d: %s", r.Status, r.Body) }

// Client calls one other part.
type Client struct {
	base  string
	limit time.Duration
	http  *http.Client
}

// New returns a client for the part at base. A limit of zero, or above Limit,
// becomes Limit.
func New(base string, limit time.Duration) *Client {
	if limit <= 0 || limit > Limit {
		limit = Limit
	}
	return &Client{
		base:  strings.TrimRight(base, "/"),
		limit: limit,
		http:  &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

// JSON sends in (if any) and decodes the answer into out (if any).
func (c *Client) JSON(ctx context.Context, method, path string, in, out any) error {
	ctx, cancel := context.WithTimeout(ctx, c.limit)
	defer cancel()

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOutage, err)
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer+1))
	if err != nil {
		return fmt.Errorf("%w: reading answer: %v", ErrOutage, err)
	}
	switch {
	case resp.StatusCode >= 500, resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode == http.StatusRequestTimeout:
		return fmt.Errorf("%w: status %d", ErrOutage, resp.StatusCode)
	case resp.StatusCode >= 300:
		return &Refused{Status: resp.StatusCode, Body: string(answer[:min(len(answer), 500)])}
	}
	if len(answer) > maxAnswer {
		return fmt.Errorf("%w: answer larger than %d bytes", ErrOutage, maxAnswer)
	}
	if out != nil {
		if err := json.Unmarshal(answer, out); err != nil {
			return fmt.Errorf("%w: unreadable answer: %v", ErrOutage, err)
		}
	}
	return nil
}
