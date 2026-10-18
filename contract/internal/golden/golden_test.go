package golden

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestMarshalSortsKeys(t *testing.T) {
	got, err := Marshal(map[string]any{"b": 1, "a": map[string]any{"d": true, "c": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": {\n    \"c\": \"x\",\n    \"d\": true\n  },\n  \"b\": 1\n}\n"
	if string(got) != want {
		t.Fatalf("got %q", got)
	}
}

func TestModuleDependenciesAreSchemaOnly(t *testing.T) {
	raw, err := os.ReadFile(Root(t) + "/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	// The harness stays off the MCP SDK and the server module. The schema dump
	// is the one exception: pure-Go modernc.org/sqlite, no cgo.
	const allowed = "modernc.org/sqlite "
	inBlock := false
	for _, line := range strings.Split(string(raw), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "require (" {
			inBlock = true
			continue
		}
		if inBlock && trim == ")" {
			inBlock = false
			continue
		}
		if strings.HasPrefix(trim, "require ") {
			if !strings.Contains(trim, allowed) {
				t.Fatalf("direct require must be modernc.org/sqlite, found %q", trim)
			}
			continue
		}
		if inBlock && trim != "" && !strings.Contains(trim, "indirect") {
			t.Fatalf("block require is not indirect: %q", trim)
		}
	}
}

func TestNormalizeRawMasksAndIgnoresKeyOrder(t *testing.T) {
	compact := []byte(`{"id":"8ee73fd9-c223-4e01-8dcb-5f0a784eed97","z":1,"description":"keep 8ee73fd9-c223-4e01-8dcb-5f0a784eed97"}`)
	pretty := []byte("{\n  \"z\": 1,\n  \"description\": \"keep 8ee73fd9-c223-4e01-8dcb-5f0a784eed97\",\n  \"id\": \"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee\"\n}\n")
	gotA, err := NormalizeRaw(compact)
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := NormalizeRaw(pretty)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotA, gotB) {
		t.Fatalf("key order or a different uuid survived\n a %s\n b %s", gotA, gotB)
	}
	if !bytes.Contains(gotA, []byte(`"<uuid>"`)) {
		t.Fatalf("id was not masked: %s", gotA)
	}
	if !bytes.Contains(gotA, []byte("keep 8ee73fd9-c223-4e01-8dcb-5f0a784eed97")) {
		t.Fatalf("description was rewritten: %s", gotA)
	}
}

func TestUpdateEnv(t *testing.T) {
	t.Setenv("UPDATE_GOLDEN", "1")
	if !Update() {
		t.Fatal("UPDATE_GOLDEN=1 should enable updates")
	}
	t.Setenv("UPDATE_GOLDEN", "")
	if updateFlag {
		t.Skip("this process was started with -update")
	}
	if Update() {
		t.Fatal("empty UPDATE_GOLDEN should not enable updates")
	}
}
