package normalize

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalRules(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "schema_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string          `json:"name"`
		Before json.RawMessage `json:"before"`
		After  json.RawMessage `json:"after"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 20 {
		t.Fatalf("expected a fixture per rule, got %d cases", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := Canonical(decode(t, tc.Before))
			want := decode(t, tc.After)
			if a, b := marshal(t, got), marshal(t, want); a != b {
				t.Fatalf("canonical mismatch\n got %s\nwant %s", a, b)
			}
			again := Canonical(decode(t, []byte(marshal(t, got))))
			if a, b := marshal(t, again), marshal(t, got); a != b {
				t.Fatalf("not idempotent\n once %s\n twice %s", b, a)
			}
		})
	}
}

func TestEnumOrderIsNotSorted(t *testing.T) {
	got := Canonical(decode(t, []byte(`{"type":"string","enum":["published","working"]}`)))
	m := got.(map[string]any)
	enum := m["enum"].([]any)
	if len(enum) != 2 || enum[0] != "published" || enum[1] != "working" {
		t.Fatalf("enum order changed: %#v", enum)
	}
	got = Canonical(decode(t, []byte(`{"anyOf":[{"type":"number","const":1},{"type":"number","const":0}]}`)))
	m = got.(map[string]any)
	enum = m["enum"].([]any)
	if len(enum) != 2 || !numEq(enum[0], 1) || !numEq(enum[1], 0) {
		t.Fatalf("const order changed: %#v", enum)
	}
}

func TestOnlyOneSafeBoundIsDropped(t *testing.T) {
	got := Canonical(decode(t, []byte(`{"type":"integer","minimum":0,"maximum":9007199254740991}`)))
	m := got.(map[string]any)
	if _, ok := m["maximum"]; ok {
		t.Fatal("safe maximum kept")
	}
	if !numEq(m["minimum"], 0) {
		t.Fatalf("real minimum lost: %#v", m["minimum"])
	}
}

func decode(t *testing.T, raw []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
