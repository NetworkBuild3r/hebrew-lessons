package golden

import (
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

func TestModuleHasNoDependencies(t *testing.T) {
	raw, err := os.ReadFile(Root(t) + "/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "require") {
			t.Fatalf("contract module must stay stdlib-only, found %q", line)
		}
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
