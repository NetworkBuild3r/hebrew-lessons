package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageNameSelfCheck(t *testing.T) {
	script := filepath.Join("..", "..", "scripts", "check-image-name.sh")
	out, err := exec.Command("sh", script, "--self-check").CombinedOutput()
	if err != nil {
		t.Fatalf("self-check: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "self-check ok") {
		t.Fatalf("output %s", out)
	}
}
