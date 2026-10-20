//go:build browser

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
)

// TestMotionInBrowser is SPEC-009 ac-3 and ac-4 in real Chromium. It stays out
// of go test ./... (no browser tag there), like the shots test. Run it with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run TestMotionInBrowser ./internal/web
//
// ac-3: with reduced motion emulated, a screen swap has only view-transition
// animations of 150ms or less, and none of them has a transform.
// ac-4: with motion on, the first tap on an answer option registers. Chromium
// blocks all taps for the length of a view transition, so a swap that brings
// in an answer option runs without one (operator decision, 2026-10-19).
func TestMotionInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	script, err := os.ReadFile("testdata/motion-browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/motion-browser.mjs", script, 0o644); err != nil {
		t.Fatal(err)
	}
	h := newMux(nil, CSRFFunc(func(*http.Request) string { return "t" }))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	cmd := exec.Command("node", "motion-browser.mjs")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ORIGIN="+srv.URL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s\n%s", err, out, stderr.String())
	}
	type anim struct {
		EndTime   float64 `json:"endTime"`
		Transform bool    `json:"transform"`
	}
	var got struct {
		Reduced struct {
			VT map[string]anim `json:"vt"`
		} `json:"reduced"`
		Full struct {
			VT map[string]anim `json:"vt"`
		} `json:"full"`
		Options struct {
			VT   map[string]anim `json:"vt"`
			Taps *int            `json:"taps"`
		} `json:"options"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("bad output %q: %v", out, err)
	}
	if len(got.Reduced.VT) == 0 {
		t.Fatal("ac-3: no view-transition animation seen under reduced motion")
	}
	for name, a := range got.Reduced.VT {
		if a.EndTime > 150 {
			t.Errorf("ac-3: %s ends at %vms under reduced motion; limit 150ms", name, a.EndTime)
		}
		if a.Transform {
			t.Errorf("ac-3: %s has a transform under reduced motion", name)
		}
	}
	if len(got.Full.VT) == 0 {
		t.Fatal("ac-4: no view transition on a plain swap with motion on; transitions are broken, not just skipped")
	}
	if len(got.Options.VT) != 0 {
		t.Errorf("ac-4: a view transition ran on a swap that brings in an answer option: %v", got.Options.VT)
	}
	if got.Options.Taps == nil || *got.Options.Taps != 1 {
		taps := -1
		if got.Options.Taps != nil {
			taps = *got.Options.Taps
		}
		t.Errorf("ac-4: taps = %d, want exactly 1 on the first tap on the new answer option", taps)
	}
}
