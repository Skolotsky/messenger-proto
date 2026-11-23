package rebuild

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"contexia/internal/demo"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
)

func listed(t *testing.T, d *demo.Service, org string) int64 {
	t.Helper()
	mux := http.NewServeMux()
	d.Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/organizations", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("the list answered %d: %s", rec.Code, rec.Body)
	}
	var list contract.OrganizationList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, e := range list.Organizations {
		if e.Org == org {
			return e.ApproxChanges
		}
	}
	t.Fatalf("%s is not listed", org)
	return 0
}

// The size follows the log, and a list still answers with the stored size when
// the log's head cannot be asked for.
func TestTheListedSizeFollowsTheLog(t *testing.T) {
	s := newSystem(t, 0)
	cur, err := s.demo.Reset(bg)
	if err != nil {
		t.Fatal(err)
	}
	seeded := listed(t, s.demo, cur.Org)
	s.act(t, cur.Org, "size-1")
	s.act(t, cur.Org, "size-2")
	if got := listed(t, s.demo, cur.Org); got != seeded+2 {
		t.Fatalf("listed size %d after two more changes on %d", got, seeded)
	}
	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()
	cut := demo.New(demo.Config{Pool: s.demoPool, Clock: clock.Real{}, Content: s.content,
		Conversations: gone.URL, Delivery: gone.URL, Search: gone.URL, Sender: gone.URL, Feed: gone.URL})
	start := time.Now()
	if got := listed(t, cut, cur.Org); got != seeded {
		t.Fatalf("with the log out of reach the list said %d, want the stored %d", got, seeded)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("the list took %s with the log out of reach", d)
	}
}

// A retired organization is reported as stopped by every receiver.
func TestARetiredOrganizationIsReportedStopped(t *testing.T) {
	s := newSystem(t, 0)
	first, err := s.demo.Reset(bg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.demo.Reset(bg); err != nil {
		t.Fatal(err)
	}
	for _, r := range s.status(t, first.Org).Receivers {
		if r.State != "stopped" {
			t.Fatalf("%s of the retired organization is %q", r.Name, r.State)
		}
	}
}
