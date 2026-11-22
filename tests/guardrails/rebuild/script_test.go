package rebuild

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"contexia/internal/shared/contract"
)

// line is what a change says happened, without numbers, IDs or times.
func line(raw []byte) (string, bool) {
	var c struct {
		Kind         string `json:"kind"`
		Person       string `json:"person"`
		Conversation string `json:"conversation"`
		Emoji        string `json:"emoji"`
		Level        string `json:"level"`
		Message      json.RawMessage
	}
	if json.Unmarshal(raw, &c) != nil {
		return "", false
	}
	switch c.Kind {
	case "reader.read", "clock.tick":
		return "", false // how much a reaction reads is the door's rule (D-035)
	}
	var m struct {
		Author       string  `json:"author"`
		Conversation string  `json:"conversation"`
		Text         string  `json:"text"`
		Level        *string `json:"level"`
	}
	_ = json.Unmarshal(c.Message, &m)
	conv := c.Conversation
	if conv == "" {
		conv = m.Conversation
	}
	level := c.Level
	if m.Level != nil {
		level = *m.Level
	}
	return strings.Join([]string{c.Kind, c.Person + m.Author, conv, m.Text, c.Emoji, level}, "|"), true
}

// TestTheTeamPlaysEverySituationThroughTheDoors plays all 16 on demand through the
// real people and agent doors. What lands in the log must say what the situation's
// script says: the same acts, by the same people, in the same conversations.
func TestTheTeamPlaysEverySituationThroughTheDoors(t *testing.T) {
	s := newSystem(t, 0)
	cur, err := s.demo.Reset(bg)
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(s.demo.Front(""))
	t.Cleanup(front.Close)
	var from int64
	if err := s.admin.QueryRow(bg, `SELECT head FROM conversations.org_log WHERE org = $1`, cur.Org).Scan(&from); err != nil {
		t.Fatal(err)
	}
	pace := 16.0
	if c := s.post(t, front.URL+"/orgs/"+cur.Org+"/script", contract.ScriptRequest{Do: "pace", Pace: &pace}, nil); c != 200 {
		t.Fatalf("pace: %d", c)
	}
	total := 0
	var want []string
	for i := range s.content.Situations {
		sit := s.content.Situations[i]
		total += len(sit.Steps)
		if c := s.post(t, front.URL+"/orgs/"+cur.Org+"/script", contract.ScriptRequest{Do: "play_situation", Situation: &sit.ID}, nil); c != 200 {
			t.Fatalf("play %s: %d", sit.ID, c)
		}
		compiled, err := s.content.Compile(sit, 1, time.Now(), fmt.Sprintf("x%d", i))
		if err != nil {
			t.Fatal(err)
		}
		for _, ch := range compiled {
			b, _ := json.Marshal(ch)
			if l, ok := line(b); ok {
				want = append(want, strings.ReplaceAll(l, s.content.Org.Org, cur.Org))
			}
		}
	}
	var state contract.ScriptState
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(front.URL + "/orgs/" + cur.Org + "/script")
		if err != nil {
			t.Fatal(err)
		}
		err = json.NewDecoder(resp.Body).Decode(&state)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Happened) >= total {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(state.Happened) != total {
		t.Fatalf("%d of %d steps happened", len(state.Happened), total)
	}
	for _, h := range state.Happened {
		if strings.HasPrefix(h.What, "could not") {
			t.Errorf("%s %s", h.Who, h.What)
		}
	}
	rows, err := s.admin.Query(bg, `SELECT body::text FROM conversations.change_log WHERE org = $1 AND number > $2 ORDER BY number`, cur.Org, from)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			t.Fatal(err)
		}
		if l, ok := line([]byte(body)); ok {
			got = append(got, l)
		}
	}
	if !slices.Equal(got, want) {
		for i := range min(len(got), len(want)) {
			if got[i] != want[i] {
				t.Fatalf("change %d differs:\n got %s\nwant %s", i, got[i], want[i])
			}
		}
		t.Fatalf("%d changes in the log, the scripts say %d", len(got), len(want))
	}
}
