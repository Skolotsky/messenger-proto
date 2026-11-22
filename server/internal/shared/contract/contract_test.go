package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const contracts = "../../../../contracts"

type operation struct {
	Contract string         `json:"contract"`
	Name     string         `json:"name"`
	Method   string         `json:"method"`
	Path     string         `json:"path"`
	Request  *string        `json:"request"`
	Answers  map[string]int `json:"answers"`
}

func load(t *testing.T) (map[string]any, *jsonschema.Compiler) {
	t.Helper()
	f, err := os.Open(filepath.Join(contracts, "schema/contexia.json"))
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
	return doc.(map[string]any)["$defs"].(map[string]any), c
}

func compile(t *testing.T, c *jsonschema.Compiler, name string) *jsonschema.Schema {
	t.Helper()
	s, err := c.Compile("contexia.json#/$defs/" + name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

func parse(t *testing.T, b []byte) any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func isObject(d any) bool { return d.(map[string]any)["type"] == "object" }

func examples(t *testing.T, name string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(contracts, "examples", name, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// Every example fits its schema, and every concrete type has one.
func TestEveryTypeHasAValidExample(t *testing.T) {
	defs, c := load(t)
	for name, d := range defs {
		if !isObject(d) {
			continue
		}
		files := examples(t, name)
		if len(files) == 0 {
			t.Errorf("%s has no example. Add contracts/examples/%s/example.json", name, name)
		}
		s := compile(t, c, name)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Validate(parse(t, b)); err != nil {
				t.Errorf("%s does not fit schema %s: %v", f, name, err)
			}
		}
	}
	dirs, _ := filepath.Glob(filepath.Join(contracts, "examples", "*"))
	for _, dir := range dirs {
		if _, ok := defs[filepath.Base(dir)]; !ok {
			t.Errorf("%s has no schema of that name", dir)
		}
	}
}

// The schema can refuse: an example without a required field, or with an
// unknown one, must fail.
func TestSchemasRefuseBrokenExamples(t *testing.T) {
	defs, c := load(t)
	for name, d := range defs {
		if !isObject(d) {
			continue
		}
		s := compile(t, c, name)
		b, err := os.ReadFile(filepath.Join(contracts, "examples", name, "example.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range d.(map[string]any)["required"].([]any) {
			v := parse(t, b).(map[string]any)
			delete(v, r.(string))
			if s.Validate(v) == nil {
				t.Errorf("%s accepted an example without %s", name, r)
			}
		}
		v := parse(t, b).(map[string]any)
		v["surely_not_a_field"] = 1
		if s.Validate(v) == nil {
			t.Errorf("%s accepted an unknown field", name)
		}
	}
}

// The Go types read every example and write it back with nothing lost.
func TestGoTypesRoundTripEveryExample(t *testing.T) {
	defs, _ := load(t)
	for name, d := range defs {
		if !isObject(d) {
			continue
		}
		decode, ok := Types[name]
		if !ok {
			t.Errorf("%s has no Go type. Add it to types.go and to Types", name)
			continue
		}
		for _, f := range examples(t, name) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			v, err := decode(b)
			if err != nil {
				t.Errorf("%s: %v", f, err)
				continue
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Errorf("%s: %v", f, err)
				continue
			}
			var want, got any
			_ = json.Unmarshal(b, &want)
			_ = json.Unmarshal(out, &got)
			if !reflect.DeepEqual(want, got) {
				t.Errorf("%s changed in a round trip:\n want %s\n got  %s", f, b, out)
			}
		}
	}
	for name := range Types {
		if d, ok := defs[name]; !ok || !isObject(d) {
			t.Errorf("Go type %s has no object schema", name)
		}
	}
}

// Every operation names real types, and every type is used by an operation.
func TestOperationsCoverEveryType(t *testing.T) {
	defs, _ := load(t)
	b, err := os.ReadFile(filepath.Join(contracts, "operations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ops []operation
	if err := json.Unmarshal(b, &ops); err != nil {
		t.Fatal(err)
	}
	seenOp := map[string]bool{}
	used := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if r, ok := x["$ref"].(string); ok {
				n := filepath.Base(r)
				if !used[n] {
					used[n] = true
					walk(defs[n])
				}
			}
			for _, c := range x {
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	mark := func(op operation, n string) {
		if _, ok := defs[n]; !ok {
			t.Errorf("%s/%s names %s, which is not in the schema", op.Contract, op.Name, n)
			return
		}
		if !used[n] {
			used[n] = true
			walk(defs[n])
		}
	}
	for _, op := range ops {
		key := fmt.Sprintf("%s/%s", op.Contract, op.Name)
		if seenOp[key] {
			t.Errorf("operation %s is listed twice", key)
		}
		seenOp[key] = true
		if _, err := os.Stat(filepath.Join(contracts, "../docs/contracts", op.Contract+".md")); err != nil {
			t.Errorf("%s: no such contract", key)
		}
		if op.Request != nil {
			mark(op, *op.Request)
		}
		if len(op.Answers) == 0 {
			t.Errorf("%s has no answers", key)
		}
		for a, status := range op.Answers {
			mark(op, a)
			if status < 200 || status > 599 {
				t.Errorf("%s: %s has status %d", key, a, status)
			}
		}
	}
	var unused []string
	for n := range defs {
		if !used[n] {
			unused = append(unused, n)
		}
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		t.Errorf("types no operation uses: %v. Add the operation to contracts/operations.json or remove the type", unused)
	}
}
