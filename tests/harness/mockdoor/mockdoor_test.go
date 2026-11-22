package mockdoor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func schemas(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	f, err := os.Open("../../../contracts/schema/contexia.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("contexia.json", doc); err != nil {
		t.Fatal(err)
	}
	return c
}

func check(t *testing.T, c *jsonschema.Compiler, name string, b []byte) {
	t.Helper()
	s, err := c.Compile("contexia.json#/$defs/" + name)
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("%s: %v: %s", name, err, b)
	}
	if err := s.Validate(v); err != nil {
		t.Errorf("answer does not fit %s: %v\n%.400s", name, err, b)
	}
}

func TestEveryAnswerFitsItsSchema(t *testing.T) {
	c := schemas(t)
	s := httptest.NewServer(New(1, 400).Handler())
	defer s.Close()
	for path, name := range map[string]string{
		"/orgs/acme/people/marco/start":                                       "StartAnswer",
		"/orgs/acme/conversations/" + url.PathEscape("#launch") + "/messages": "MessagesAnswer",
		"/orgs/acme/people/marco/feed":                                        "ScreenFeedAnswer",
		"/orgs/acme/people/marco/history?tab=mentions":                        "HistoryAnswer",
		"/orgs/acme/people/marco/history?tab=direct":                          "HistoryAnswer",
		"/orgs/acme/people/marco/history?tab=threads":                         "HistoryAnswer",
		"/orgs/acme/people/marco/history?tab=reactions":                       "HistoryAnswer",
		"/orgs/acme/people/marco/settings":                                    "SettingsAnswer",
		"/orgs/acme/people/marco/preview?change=x":                            "PreviewAnswer",
		"/orgs/acme/people/marco/search?q=build":                              "ScreenSearchAnswer",
		"/orgs/acme/suggest?q=ma":                                             "SuggestAnswer",
	} {
		resp, err := http.Get(s.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("%s: %d %s", path, resp.StatusCode, b)
			continue
		}
		check(t, c, name, b)
	}
}

func act(t *testing.T, base, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(base+"/orgs/acme/people/actions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestLiveUpdatesAreNumberedChainedAndValid(t *testing.T) {
	c := schemas(t)
	door := New(1, 400)
	s := httptest.NewServer(door.Handler())
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, strings.Replace(s.URL, "http", "ws", 1)+"/orgs/acme/live?person=lena", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()

	for connected := false; !connected; {
		if ctx.Err() != nil {
			t.Fatal("the connection was never registered")
		}
		door.mu.Lock()
		connected = len(door.clients) == 1
		door.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		body := `{"id":"a` + string(rune('1'+i)) + `","as":"anna","kind":"send","message":{"id":"m` + string(rune('1'+i)) + `","conversation":"#launch","thread":null,"text":"hello","mentions":["lena"],"level":null}}`
		if code, out := act(t, s.URL, body); code != 200 || out["saved"] != true {
			t.Fatalf("%d %v", code, out)
		}
	}
	var last int64
	seen := 0
	for seen < 3 {
		_, b, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		check(t, c, "LiveOut", b)
		var u struct {
			Stream        string
			Number, After int64
		}
		_ = json.Unmarshal(b, &u)
		if u.Stream != "conversations" {
			continue
		}
		if seen > 0 && u.After != last {
			t.Fatalf("update %d says after %d, last was %d", u.Number, u.After, last)
		}
		last = u.Number
		seen++
	}
}

func TestSameIDIsSavedOnce(t *testing.T) {
	s := httptest.NewServer(New(1, 400).Handler())
	defer s.Close()
	body := `{"id":"a1","as":"anna","kind":"send","message":{"id":"m1","conversation":"#launch","thread":null,"text":"once","mentions":[],"level":null}}`
	_, first := act(t, s.URL, body)
	_, second := act(t, s.URL, body)
	if first["number"] != second["number"] {
		t.Fatalf("%v then %v", first, second)
	}
}

func TestGoingWrongOnPurpose(t *testing.T) {
	c := schemas(t)
	s := httptest.NewServer(New(1, 400).Handler())
	defer s.Close()
	control := func(body string) {
		resp, err := http.Post(s.URL+"/mock/control", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	control(`{"FeedDown":true}`)
	resp, err := http.Get(s.URL + "/orgs/acme/people/marco/feed")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	check(t, c, "ScreenFeedAnswer", b)
	if !strings.Contains(string(b), "Catching up") {
		t.Fatalf("no notice: %s", b)
	}
	control(`{"FeedDown":false,"Refuse":"message deleted"}`)
	if code, out := act(t, s.URL, `{"id":"r1","as":"anna","kind":"delete","message":"nope"}`); code != 409 || out["refused"] != "message deleted" {
		t.Fatalf("%d %v", code, out)
	}
	control(`{"Down":true}`)
	resp, err = http.Get(s.URL + "/orgs/acme/people/marco/start")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatalf("status %d while down", resp.StatusCode)
	}
}
