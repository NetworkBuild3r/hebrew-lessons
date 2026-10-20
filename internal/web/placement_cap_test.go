package web

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The screen offers a new word PlaceMaxReplaceTries times and the engine allows MaxReplaces. They are
// one number; if they drift, the screen either offers a swap the engine refuses or hides one it allows.
func TestReplaceCapMatchesTheScreen(t *testing.T) {
	if placement.MaxReplaces != views.PlaceMaxReplaceTries {
		t.Fatalf("engine allows %d swaps, the screen offers %d", placement.MaxReplaces, views.PlaceMaxReplaceTries)
	}
}
