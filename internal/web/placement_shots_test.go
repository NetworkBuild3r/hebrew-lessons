//go:build browser

package web

import (
	"encoding/json"
	"os"
	"testing"
)

// TestPlacementScreenshots (SPEC-012 ac-7) saves a PNG of every placement screen at phone (390x844) and
// desktop (1440x900) size, in light and dark, into SHOTS_DIR. It needs the same browser setup as
// TestPlacementInBrowser (placement_browser_test.go):
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> SHOTS_DIR=<dir> go test -tags browser -run TestPlacementScreenshots ./internal/web
//
// It replaces the v2 deck shots, which covered a flashcard flow that no longer exists.
func TestPlacementScreenshots(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	shots := os.Getenv("SHOTS_DIR")
	if dir == "" || shots == "" {
		t.Skip("set MOTION_PW_DIR and SHOTS_DIR")
	}
	// Four captures (phone and desktop, light and dark) each need their own learner per state, because
	// walking a screen uses its state up.
	states := []string{"intro", "letters", "listening", "break", "offer", "results"}
	s := startPlacementServer(t, len(states)*4+4, false)
	all := map[string]map[string]placedUser{}
	for _, size := range []string{"phone", "desktop"} {
		for _, scheme := range []string{"light", "dark"} {
			set := map[string]placedUser{}
			for _, state := range states {
				set[state] = s.learner(state)
			}
			all[size+"-"+scheme] = set
		}
	}
	sets := map[string]string{}
	for k, set := range all {
		sets[k] = cookiesJSON(t, set)
	}
	b, _ := json.Marshal(sets)
	out := runNode(t, dir, "placement-shots.mjs", "ORIGIN="+s.origin, "SETS="+string(b), "OUT="+shots)
	t.Logf("%s", out)
}
