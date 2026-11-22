package router

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func named(name string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/live") {
			c, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer c.CloseNow()
			_ = c.Write(r.Context(), websocket.MessageText, []byte(name))
			_, _, _ = c.Read(r.Context())
			return
		}
		_, _ = io.WriteString(w, name)
	}))
}

func get(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestAnOrganizationStaysOnOneCopyUntilMoved(t *testing.T) {
	a, b := named("a"), named("b")
	defer a.Close()
	defer b.Close()
	r, err := New(ByOrg, a.URL, b.URL)
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(r)
	defer front.Close()
	first := get(t, front.URL+"/orgs/acme/x")
	for range 20 {
		if got := get(t, front.URL+"/orgs/acme/x"); got != first {
			t.Fatalf("one organization reached two copies: %s and %s", first, got)
		}
	}
	to := 1 - r.CopyFor("acme")
	r.Move("acme", to)
	if got := get(t, front.URL+"/orgs/acme/x"); got == first {
		t.Fatalf("after the move the organization is still served by %s", got)
	}
}

func TestMoveCutsALiveConnection(t *testing.T) {
	a, b := named("a"), named("b")
	defer a.Close()
	defer b.Close()
	r, err := New(ByOrg, a.URL, b.URL)
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(r)
	defer front.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(front.URL, "http")+"/orgs/acme/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	if _, _, err := ws.Read(ctx); err != nil {
		t.Fatal(err)
	}
	r.Move("acme", 1-r.CopyFor("acme"))
	if _, _, err := ws.Read(ctx); err == nil {
		t.Fatal("the live connection survived the move")
	} else if ctx.Err() != nil {
		t.Fatal("the move did not cut the live connection within 5 s")
	}
}

func TestSpreadUsesEveryLiveCopy(t *testing.T) {
	a, b := named("a"), named("b")
	defer a.Close()
	defer b.Close()
	r, err := New(Spread, a.URL, b.URL)
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(r)
	defer front.Close()
	seen := map[string]int{}
	for range 10 {
		seen[get(t, front.URL+"/changes")]++
	}
	if seen["a"] == 0 || seen["b"] == 0 {
		t.Fatalf("calls were not spread: %v", seen)
	}
	r.SetDown(0, true)
	for range 6 {
		if got := get(t, front.URL+"/changes"); got != "b" {
			t.Fatalf("a call reached the copy that is down: %s", got)
		}
	}
}
