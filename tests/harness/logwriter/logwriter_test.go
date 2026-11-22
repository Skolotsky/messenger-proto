package logwriter

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"contexia/internal/shared/contract"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func changeSchema(t *testing.T) *jsonschema.Schema {
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
	s, err := c.Compile("contexia.json#/$defs/Change")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEveryEntryIsAValidChangeInOrder(t *testing.T) {
	s := changeSchema(t)
	log := Write(Config{Orgs: []string{"acme"}, Seed: 7, Changes: 3000, Reset: 2})["acme"]
	if len(log) < 3000 {
		t.Fatalf("wrote %d", len(log))
	}
	last := ""
	versions := map[string]int64{}
	deleted := map[string]bool{}
	for i, e := range log {
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(e.JSON))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(v); err != nil {
			t.Fatalf("entry %d (%s) breaks the schema: %v\n%s", i, e.Kind, err, e.JSON)
		}
		var c contract.Change
		if err := json.Unmarshal(e.JSON, &c); err != nil {
			t.Fatalf("entry %d does not decode: %v", i, err)
		}
		var head struct {
			Number, Previous, Reset int64
			At                      string
			Message                 json.RawMessage
		}
		_ = json.Unmarshal(e.JSON, &head)
		if head.Number != int64(i+1) || head.Previous != int64(i) || head.Reset != 2 {
			t.Fatalf("entry %d is numbered %d after %d", i, head.Number, head.Previous)
		}
		if head.At < last {
			t.Fatalf("entry %d goes back in time: %s after %s", i, head.At, last)
		}
		last = head.At
		if strings.HasPrefix(e.Kind, "message.") {
			var m struct {
				ID      string
				Number  int64
				Version int64
			}
			_ = json.Unmarshal(head.Message, &m)
			if deleted[m.ID] {
				t.Fatalf("entry %d changes deleted message %s", i, m.ID)
			}
			if m.Version <= versions[m.ID] {
				t.Fatalf("entry %d: version %d of %s after %d", i, m.Version, m.ID, versions[m.ID])
			}
			versions[m.ID] = m.Version
			if e.Kind == "message.created" && m.Number != e.Number {
				t.Fatalf("entry %d: message number %d", i, m.Number)
			}
			if e.Kind == "message.deleted" {
				deleted[m.ID] = true
			}
		}
	}
}

func TestEveryKindAppears(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Write(Config{Orgs: []string{"acme"}, Seed: 1})["acme"] {
		seen[e.Kind] = true
	}
	f, err := os.ReadFile("../../../contracts/schema/contexia.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Defs map[string]struct {
			OneOf []struct {
				Ref string `json:"$ref"`
			} `json:"oneOf"`
			Properties map[string]struct {
				Const string `json:"const"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(f, &doc); err != nil {
		t.Fatal(err)
	}
	for _, m := range doc.Defs["Change"].OneOf {
		kind := doc.Defs[strings.TrimPrefix(m.Ref, "#/$defs/")].Properties["kind"].Const
		if !seen[kind] {
			t.Errorf("the log writer never writes %s", kind)
		}
	}
}

func TestSameSeedSameBytes(t *testing.T) {
	c := Config{Orgs: []string{"acme"}, Seed: 11, Changes: 500}
	a, b := Write(c)["acme"], Write(c)["acme"]
	for i := range a {
		if !bytes.Equal(a[i].JSON, b[i].JSON) {
			t.Fatalf("entry %d differs", i)
		}
	}
	other := Write(Config{Orgs: []string{"acme"}, Seed: 12, Changes: 500})["acme"]
	same := true
	for i := range a {
		same = same && bytes.Equal(a[i].JSON, other[i].JSON)
	}
	if same {
		t.Fatal("a different seed wrote the same log")
	}
}

func TestTwoOrganizationsHaveTheSameNamesAndNumbers(t *testing.T) {
	logs := Write(Config{Orgs: []string{"acme", "globex"}, Seed: 3, Changes: 400})
	a, b := logs["acme"], logs["globex"]
	if len(a) != len(b) {
		t.Fatalf("%d against %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Number != b[i].Number || !bytes.Equal(bytes.Replace(a[i].JSON, []byte(`"org":"acme"`), []byte(`"org":"globex"`), 1), b[i].JSON) {
			t.Fatalf("entry %d differs beyond the organization", i)
		}
	}
}
