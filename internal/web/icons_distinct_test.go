package web

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// Every place a learner goes draws its own icon.
//
// The nav used to draw Learn with the Lessons book and Tutor with the My stuff
// person, so two pairs of destinations looked identical. This test reads the
// rendered nav and the Today card and fails when two of them share a glyph.

// navLinks pulls out each link in one nav block as {glyph, label}.
var (
	linkBlock = regexp.MustCompile(`(?s)<a [^>]*class="(?:dock__link|rail__link|today-mini__link)"[^>]*>.*?</a>`)
	glyphIn   = regexp.MustCompile(`#icon-([a-z-]+)"`)
	textIn    = regexp.MustCompile(`>([A-Z][^<>]{0,40})<`)
)

// linksOf returns glyph -> label for every nav-style link in the fragment.
func linksOf(fragment string) map[string]string {
	out := map[string]string{}
	for _, a := range linkBlock.FindAllString(fragment, -1) {
		g := glyphIn.FindStringSubmatch(a)
		if g == nil {
			continue
		}
		// The label is the first run of real text after the icon, never a tag.
		rest := a[strings.Index(a, "</svg>")+len("</svg>"):]
		label := textIn.FindStringSubmatch(rest)
		if label == nil || strings.TrimSpace(label[1]) == "" {
			continue
		}
		out[g[1]] = strings.TrimSpace(label[1])
	}
	return out
}

func TestEveryDestinationHasItsOwnIcon(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	// Both switches on, so Roots and Isaiah render as rail sections too.
	store, h := openShellFlags(t, fixedClock(now), nil, nil, Options{ShowRoots: true, ShowIsaiah: true})
	l := addLearner(t, store, "a@example.com", "sub-icons", now)
	body := getLearner(t, h, l, "/")

	dock := regexp.MustCompile(`(?s)<nav class="dock".*?</nav>`).FindString(body)
	if dock == "" {
		t.Fatal("Today has no dock")
	}
	seen := linksOf(dock)
	if len(seen) < 5 {
		t.Fatalf("only %d destinations found in the dock: %v", len(seen), seen)
	}
	// Every dock destination must also be in the extra sections, so the whole
	// set of places is checked for a repeated glyph.
	rail := regexp.MustCompile(`(?s)<nav class="rail-more".*?</nav>`).FindString(body)
	for glyph, label := range linksOf(rail) {
		if prev, dup := seen[glyph]; dup {
			t.Errorf("%q and %q both draw %q", prev, label, glyph)
		}
		seen[glyph] = label
	}
}

// The Today card's ways onward differ from each other and from the nav.
func TestTodayWaysOnwardDifferFromEachOther(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), dueEngine(), nil)
	l := addLearner(t, store, "a@example.com", "sub-ways", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	addCard(t, store, l.user.ID, addLexeme(t, store, helloHe, "hello", "shalom", "m", ""), "read", now.Add(-time.Hour), 1, 3, now.Add(-48*time.Hour))
	body := getLearner(t, h, l, "/")

	card := regexp.MustCompile(`(?s)<nav class="today-mini".*?</nav>`).FindString(body)
	if card == "" {
		t.Fatal("Today has no ways-onward card")
	}
	ways := linksOf(card)
	if len(ways) < 2 {
		t.Fatalf("%d ways onward, want at least 2", len(ways))
	}
	// The two ways onward must be told apart. A way that goes to the same place
	// as a nav destination keeps that destination's icon, which is consistency
	// rather than repetition, so only a different label sharing a glyph is a bug.
	glyphs := make([]string, 0, len(ways))
	for glyph := range ways {
		glyphs = append(glyphs, glyph)
	}
	if len(glyphs) == 2 && glyphs[0] == glyphs[1] {
		t.Fatalf("both ways onward draw %q", glyphs[0])
	}
	dock := regexp.MustCompile(`(?s)<nav class="dock".*?</nav>`).FindString(body)
	for glyph, label := range ways {
		if prev, dup := linksOf(dock)[glyph]; dup && prev != label {
			t.Errorf("the %q way onward repeats %q, which is the icon of %q", label, glyph, prev)
		}
	}
}

// The new glyphs are in the sprite, so a missing symbol cannot silently fall
// back to the help question mark.
func TestNewSectionIconsExist(t *testing.T) {
	for _, name := range []string{"tiles", "chat", "compass", "branch"} {
		if !views.HasIcon(name) {
			t.Errorf("sprite has no %q glyph", name)
		}
	}
}
