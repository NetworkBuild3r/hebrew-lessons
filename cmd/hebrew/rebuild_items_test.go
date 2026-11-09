package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRebuildLessonItemsEmptyDatabase(t *testing.T) {
	path := opsDB(t)
	var stdout, stderr bytes.Buffer
	if err := rebuildLessonItemsCommand([]string{"--db", path}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "rebuilt 0\n") {
		t.Fatalf("stdout %q", stdout.String())
	}
	if err := rebuildLessonItemsCommand([]string{"--db", path}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
}
