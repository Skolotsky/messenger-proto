package harness

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

// Delivery is one request a Recorder received.
type Delivery struct {
	Path   string
	Org    string
	Number int64
	Body   []byte
}

// Recorder is a receiver that remembers what it got, and answers as told.
type Recorder struct {
	mu     sync.Mutex
	got    []Delivery
	answer func(Delivery) (int, any)
}

// NewRecorder returns a receiver answering 200 {"applied": true}.
func NewRecorder() *Recorder { return &Recorder{} }

// AnswerWith sets how the recorder answers. nil means 200.
func (r *Recorder) AnswerWith(f func(Delivery) (int, any)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.answer = f
}

// ServeHTTP records one delivery.
func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var head struct {
		Org    string `json:"org"`
		Number int64  `json:"number"`
	}
	_ = json.Unmarshal(body, &head)
	d := Delivery{Path: req.URL.Path, Org: head.Org, Number: head.Number, Body: body}
	r.mu.Lock()
	r.got = append(r.got, d)
	answer := r.answer
	r.mu.Unlock()
	status, out := http.StatusOK, any(map[string]bool{"applied": true})
	if answer != nil {
		status, out = answer(d)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

// Deliveries returns everything received, in order of arrival.
func (r *Recorder) Deliveries() []Delivery {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Delivery(nil), r.got...)
}

// Repeats counts deliveries of an organization and number already seen.
func (r *Recorder) Repeats() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	type key struct {
		org string
		n   int64
	}
	seen := map[key]bool{}
	repeats := 0
	for _, d := range r.got {
		k := key{d.Org, d.Number}
		if seen[k] {
			repeats++
		}
		seen[k] = true
	}
	return repeats
}
