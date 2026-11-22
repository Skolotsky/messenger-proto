package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"contexia/internal/shared/call"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
)

type answer int

const (
	applied    answer = iota
	behind            // 409 i_am_at
	staleReset        // 409 stale_reset
	refusedOne        // 422
	orgStopped        // 423
	down              // anything else, or 3 s of silence
)

// outcome is one receiver's answer in the terms the contracts give it.
type outcome struct {
	answer  answer
	iAmAt   int64
	edition int64
	wipe    int64
	hasWipe bool
	reason  string
}

// place is where a receiver says it stands.
type place struct {
	Reset, Through, Wipe, Edition int64
}

// client is the network side of one receiver.
type client struct {
	name string
	c    caller
}

// caller is the shared client's one method.
type caller interface {
	JSON(ctx context.Context, method, path string, in, out any) error
}

func newClient(name, base string) *client {
	return &client{name: name, c: newCall(base)}
}

func (r *client) place(ctx context.Context, org ids.Org) (place, error) {
	var p place
	var raw struct {
		Reset   int64 `json:"reset"`
		Through int64 `json:"through"`
		Wipe    int64 `json:"wipe"`
		Edition int64 `json:"edition"`
	}
	if err := r.c.JSON(ctx, http.MethodGet, "/orgs/"+string(org)+"/place", nil, &raw); err != nil {
		return p, err
	}
	return place{Reset: raw.Reset, Through: raw.Through, Wipe: raw.Wipe, Edition: raw.Edition}, nil
}

// alive asks whether the receiver answers at all.
func (r *client) alive(ctx context.Context) bool {
	return r.c.JSON(ctx, http.MethodGet, "/alive", nil, nil) == nil
}

// post delivers one body to path and classifies the answer.
func (r *client) post(ctx context.Context, path string, body any) outcome {
	var ok struct {
		Wipe *int64 `json:"wipe"`
	}
	err := r.c.JSON(ctx, http.MethodPost, path, body, &ok)
	if err == nil {
		o := outcome{answer: applied}
		if ok.Wipe != nil {
			o.wipe, o.hasWipe = *ok.Wipe, true
		}
		return o
	}
	var ref *call.Refused
	if !errors.As(err, &ref) {
		return outcome{answer: down, reason: err.Error()}
	}
	return classify(ref)
}

// classify reads a non-200 answer. The contracts list the answers; anything
// else counts as the receiver being down.
func classify(ref *call.Refused) outcome {
	var b struct {
		IAmAt   *int64  `json:"i_am_at"`
		Edition int64   `json:"edition"`
		Stale   *int64  `json:"stale_reset"`
		Stopped *string `json:"organization_stopped"`
		Refused string  `json:"refused"`
		Wipe    *int64  `json:"wipe"`
	}
	parsed := json.Unmarshal([]byte(ref.Body), &b) == nil
	switch {
	case ref.Status == http.StatusConflict && parsed && b.Stale != nil:
		return outcome{answer: staleReset}
	case ref.Status == http.StatusConflict && parsed && b.IAmAt != nil:
		return outcome{answer: behind, iAmAt: *b.IAmAt, edition: b.Edition}
	case ref.Status == http.StatusLocked && parsed && b.Stopped != nil:
		return outcome{answer: orgStopped}
	case ref.Status == http.StatusUnprocessableEntity:
		o := outcome{answer: refusedOne, reason: b.Refused}
		if o.reason == "" {
			o.reason = "refused"
		}
		if b.Wipe != nil {
			o.wipe, o.hasWipe = *b.Wipe, true
		}
		return o
	}
	return outcome{answer: down, reason: fmt.Sprintf("unexpected answer %d", ref.Status)}
}

// stamped returns body with the current reset number. A log entry keeps the
// number it was saved under; what a receiver must compare is the current one.
func stamped(body []byte, reset int64) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	r, err := json.Marshal(reset)
	if err != nil {
		return nil, err
	}
	m["reset"] = r
	return json.Marshal(m)
}

// forSearch turns a log entry into what search is sent. ok is false for a kind
// search is not sent; the place moves past it.
func forSearch(org ids.Org, reset, number int64, kind string, body []byte) (any, bool, error) {
	var in struct {
		Message json.RawMessage `json:"message"`
	}
	switch kind {
	case "message.created", "message.changed":
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, false, err
		}
		var m contract.SearchMessage
		if err := json.Unmarshal(in.Message, &m); err != nil {
			return nil, false, err
		}
		var extra struct {
			EditedAt *string `json:"edited_at"`
		}
		if err := json.Unmarshal(in.Message, &extra); err != nil {
			return nil, false, err
		}
		m.Edited = extra.EditedAt != nil
		if kind == "message.created" {
			return contract.SearchCreated{Kind: kind, Org: string(org), Reset: reset, Number: number, Message: m}, true, nil
		}
		return contract.SearchChanged{Kind: kind, Org: string(org), Reset: reset, Number: number, Message: m}, true, nil
	case "message.deleted":
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, false, err
		}
		var m contract.SearchDeleted
		if err := json.Unmarshal(in.Message, &m); err != nil {
			return nil, false, err
		}
		return contract.SearchDeletedChange{Kind: kind, Org: string(org), Reset: reset, Number: number, Message: m}, true, nil
	}
	return nil, false, nil
}
