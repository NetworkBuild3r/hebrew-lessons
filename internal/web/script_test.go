package web

import (
	"os"
	"testing"
)

func TestShellJSBudget(t *testing.T) {
	b, err := os.ReadFile("static/js/shell.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 2048 {
		t.Fatalf("shell.js is %d bytes; the motion and tile allowance is 2048", len(b))
	}
	if len(b) == 0 {
		t.Fatal("shell.js is empty")
	}
	body := get(t, New(nil, nil), "/static/js/shell.js")
	if body != string(b) {
		t.Fatal("served shell.js does not match the file")
	}
	if len(body) > 2048 {
		t.Fatalf("served shell.js is %d bytes", len(body))
	}
}
