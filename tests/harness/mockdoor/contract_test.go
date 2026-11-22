package mockdoor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type liveUpdate struct {
	Stream        string
	Number, After int64
	Parts         []map[string]any
	TooFarBehind  string `json:"too_far_behind"`
}

func dial(t *testing.T, ctx context.Context, base, person string) *websocket.Conn {
	t.Helper()
	ws, _, err := websocket.Dial(ctx, strings.Replace(base, "http", "ws", 1)+"/orgs/acme/live?person="+person, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func waitClients(t *testing.T, ctx context.Context, d *Door, n int) {
	t.Helper()
	for {
		d.mu.Lock()
		have := len(d.clients)
		d.mu.Unlock()
		if have == n {
			return
		}
		if ctx.Err() != nil {
			t.Fatalf("%d connections, wanted %d", have, n)
		}
		time.Sleep(time.Millisecond)
	}
}

func getJSON(t *testing.T, target string, out any) {
	t.Helper()
	resp, err := http.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

func say(t *testing.T, base, as, conv, id string, mentions ...string) int64 {
	t.Helper()
	m, _ := json.Marshal(mentions)
	if mentions == nil {
		m = []byte("[]")
	}
	code, out := act(t, base, fmt.Sprintf(`{"id":"a_%s","as":%q,"kind":"send","message":{"id":%q,"conversation":%q,"thread":null,"text":"hello","mentions":%s,"level":null}}`, id, as, id, conv, m))
	if code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	return int64(out["number"].(float64))
}

func TestEachPersonHasTheirOwnChainOfNumbers(t *testing.T) {
	door := New(1, 400)
	s := httptest.NewServer(door.Handler())
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	door.mu.Lock()
	var outsider string
	for _, p := range door.people {
		if id := p["id"].(string); !contains(door.convs["#launch"]["people"], id) {
			outsider = id
			break
		}
	}
	door.mu.Unlock()
	if outsider == "" {
		door.mu.Lock()
		door.convs["#launch"]["people"] = []any{"anna", "lena"}
		outsider = "marco"
		door.mu.Unlock()
	}

	var start struct{ Number int64 }
	getJSON(t, s.URL+"/orgs/acme/people/"+outsider+"/start", &start)
	ws := dial(t, ctx, s.URL, outsider)
	defer ws.CloseNow()
	waitClients(t, ctx, door, 1)

	say(t, s.URL, "anna", "#launch", "m_hidden")
	var direct string
	code, out := act(t, s.URL, fmt.Sprintf(`{"id":"d1","as":"anna","kind":"open_direct","with":%q}`, outsider))
	if code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	direct = out["conversation"].(string)
	n := say(t, s.URL, "anna", direct, "m_seen")

	for {
		_, b, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "m_hidden") {
			t.Fatalf("a message of a channel they are not in reached them: %s", b)
		}
		var u liveUpdate
		_ = json.Unmarshal(b, &u)
		if u.Stream == "conversations" && strings.Contains(string(b), "m_seen") {
			if u.Number != n {
				t.Fatalf("number %d, wanted %d", u.Number, n)
			}
			if u.After <= start.Number-1 || u.After >= u.Number {
				t.Fatalf("after %d does not chain from %d to %d", u.After, start.Number, u.Number)
			}
			return
		}
	}
}

func TestResumeReplaysWhatWasMissedAndTooFarIsSaid(t *testing.T) {
	c := schemas(t)
	door := New(1, 400)
	s := httptest.NewServer(door.Handler())
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var start struct{ Number int64 }
	getJSON(t, s.URL+"/orgs/acme/people/marco/start", &start)
	first := say(t, s.URL, "anna", "#general", "m_r1")
	second := say(t, s.URL, "anna", "#general", "m_r2")

	ws := dial(t, ctx, s.URL, "marco")
	defer ws.CloseNow()
	waitClients(t, ctx, door, 1)
	if err := ws.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf(`{"resume":{"conversations":%d,"feed":null,"feed_edition":null}}`, start.Number))); err != nil {
		t.Fatal(err)
	}
	var got []liveUpdate
	for len(got) < 2 {
		_, b, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		check(t, c, "LiveOut", b)
		var u liveUpdate
		_ = json.Unmarshal(b, &u)
		got = append(got, u)
	}
	if got[0].Number != first || got[0].After != start.Number || got[1].Number != second || got[1].After != first {
		t.Fatalf("replay is not chained: %+v", got)
	}

	body, _ := json.Marshal(map[string]any{"Burst": map[string]any{"As": "anna", "Conversation": "#general", "Count": replayMax + 1}})
	resp, err := http.Post(s.URL+"/mock/control", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if err := ws.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf(`{"resume":{"conversations":%d,"feed":null,"feed_edition":null}}`, second))); err != nil {
		t.Fatal(err)
	}
	for {
		_, b, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var u liveUpdate
		_ = json.Unmarshal(b, &u)
		if u.TooFarBehind == "conversations" {
			check(t, c, "LiveOut", b)
			return
		}
	}
}

func TestActingImpliesReadAndReactionsReadOnlyUpToTheirMessage(t *testing.T) {
	door := New(1, 400)
	s := httptest.NewServer(door.Handler())
	defer s.Close()
	first := say(t, s.URL, "anna", "#general", "m_a", "marco")
	second := say(t, s.URL, "anna", "#general", "m_b")
	if code, _ := act(t, s.URL, `{"id":"r1","as":"marco","kind":"react","message":"m_a","emoji":"eyes","on":true}`); code != 200 {
		t.Fatal(code)
	}
	door.mu.Lock()
	at := door.reads[key("marco", "#general", "m_a")]
	door.mu.Unlock()
	if at != first {
		t.Fatalf("the reaction read up to %d, wanted %d", at, first)
	}
	if code, _ := act(t, s.URL, `{"id":"k1","as":"marco","kind":"keep","conversation":"#general","thread":null}`); code != 200 {
		t.Fatal(code)
	}
	door.mu.Lock()
	at = door.reads[key("marco", "#general", nil)]
	door.mu.Unlock()
	if at < second {
		t.Fatalf("keep read up to %d, wanted %d", at, second)
	}
}

func TestSameIDWithAnotherBodyIsRefused(t *testing.T) {
	s := httptest.NewServer(New(1, 400).Handler())
	defer s.Close()
	say(t, s.URL, "anna", "#general", "m_x")
	code, out := act(t, s.URL, `{"id":"a_m_x","as":"anna","kind":"send","message":{"id":"m_other","conversation":"#general","thread":null,"text":"different","mentions":[],"level":null}}`)
	if code != 409 || out["refused"] != "id reused" {
		t.Fatalf("%d %v", code, out)
	}
}

func TestAroundLimitAndTheThreadRoot(t *testing.T) {
	c := schemas(t)
	door := New(1, 400)
	s := httptest.NewServer(door.Handler())
	defer s.Close()
	root := "m_root"
	say(t, s.URL, "anna", "#general", root)
	var numbers []int64
	for i := 0; i < 12; i++ {
		code, out := act(t, s.URL, fmt.Sprintf(`{"id":"t%d","as":"lena","kind":"send","message":{"id":"m_t%d","conversation":"#general","thread":%q,"text":"reply","mentions":[],"level":null}}`, i, i, root))
		if code != 200 {
			t.Fatal(code)
		}
		numbers = append(numbers, int64(out["number"].(float64)))
	}
	target := fmt.Sprintf("%s/orgs/acme/conversations/%s/messages?as=marco&thread=%s&limit=4&around=%d", s.URL, url.PathEscape("#general"), root, numbers[6])
	resp, err := http.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	check(t, c, "MessagesAnswer", b)
	var page struct {
		Messages []struct {
			Message struct {
				ID     string
				Number int64
			}
		}
		HasBefore bool `json:"has_before"`
		HasAfter  bool `json:"has_after"`
	}
	_ = json.Unmarshal(b, &page)
	if len(page.Messages) != 5 || page.Messages[0].Message.ID != root || !page.HasBefore || !page.HasAfter {
		t.Fatalf("%+v", page)
	}
}

func TestAReplyInProgressAnswersItsTextAndClosed(t *testing.T) {
	c := schemas(t)
	door := New(1, 400)
	s := httptest.NewServer(door.Handler())
	defer s.Close()
	post := func(body string) {
		resp, err := http.Post(s.URL+"/mock/control", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	post(`{"Reply":{"Start":true,"Message":"m_p","Conversation":"#general","Author":"lena"}}`)
	post(`{"Reply":{"Message":"m_p","Piece":"héllo "}}`)
	post(`{"Reply":{"Message":"m_p","Piece":"there","Silent":true}}`)
	resp, err := http.Get(s.URL + "/orgs/acme/replies/m_p")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	check(t, c, "ReplyText", b)
	var r struct {
		At   int
		Text string
	}
	_ = json.Unmarshal(b, &r)
	if r.Text != "héllo there" || r.At != len([]rune(r.Text)) {
		t.Fatalf("%+v", r)
	}
	post(`{"Reply":{"Message":"m_p","Abandon":true}}`)
	resp, err = http.Get(s.URL + "/orgs/acme/replies/m_p")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("status %d after closing", resp.StatusCode)
	}
}

func TestARetiredOrganizationAnswers410AndAMissingOne404(t *testing.T) {
	c := schemas(t)
	door := New(1, 400)
	s := httptest.NewServer(door.Handler())
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws := dial(t, ctx, s.URL, "marco")
	defer ws.CloseNow()
	waitClients(t, ctx, door, 1)
	resp, err := http.Post(s.URL+"/mock/control", "application/json", strings.NewReader(`{"Retire":true}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	_, b, err := ws.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	check(t, c, "OrganizationGone", b)
	resp, err = http.Get(s.URL + "/orgs/acme/people/marco/feed")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 410 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	check(t, c, "OrganizationGone", body)
	var current struct{ Org string }
	getJSON(t, s.URL+"/demo/current", &current)
	if current.Org != "acme-2" {
		t.Fatalf("current is %q", current.Org)
	}
	resp, err = http.Get(s.URL + "/orgs/nobody/people/marco/feed")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d for an organization that never existed", resp.StatusCode)
	}
}
