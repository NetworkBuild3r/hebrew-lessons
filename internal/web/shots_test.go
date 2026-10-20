//go:build browser

package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// SPEC-024: visual and usability regression checks. TestShots opens every learning screen of SPEC-012
// (placement), SPEC-016 (activities and dialogue), SPEC-017 (the shell) and SPEC-018 (Progress) in real
// Chromium, in light and dark, at 360, 390, 1024 and 1440 px wide, saves a PNG of each into
// testdata/shots/, and checks the page's DOM. Run it with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run 'Shots' ./internal/web
//
// It is skipped, with the reason, when MOTION_PW_DIR is not set or node is not installed. Optional:
// SHOTS_DIR (default testdata/shots), SHOTS_PARALLEL (screens walked at once, default 3) and SHOTS_ONLY
// (comma-separated combination keys such as "light-390,dark-1440,reduce-360", a partial run for working on
// one screen; the full matrix is then not required).
//
// The checks (testdata/shots.mjs reads the page; judge below decides):
//
//   - axe-core 4.10.2 (vendored in testdata/axe), WCAG 2.0, 2.1 and 2.2 A and AA. A serious or critical
//     violation fails. Moderate and minor ones are listed in the index, not failed (SPEC-024 ac-2).
//   - One gold action. Gold is a visible Kind=Primary button (.c-button[data-kind=primary], .btn-primary),
//     the gold orb (.c-orb[data-gold]), or any other button or link painted with --lamp; inside an open
//     modal dialog only the dialog counts. A screen never has more than one gold element. A screen of
//     kind "one" has exactly one, a Primary button. An "orb" screen (the placement screens where the
//     learner chooses by ear) has exactly one, the orb, and no gold button: SPEC-012 ac-1 was recorded
//     as a deviation there. A "none" screen has no gold at all: either it has no main action (Progress,
//     Lessons, Me), or its choices are deliberately neutral (see goldNone below).
//   - Feedback colours. --go, --stop, --go-soft and --stop-soft may be painted only on a feedback surface:
//     an element that is, or is inside, [data-state=correct|wrong], .feedback--right|wrong or
//     .feedback-right|wrong. Text, backgrounds, borders, outlines, fills, strokes, shadows, gradients and
//     the ::before and ::after of every other visible element are read.
//   - Targets. Every visible interactive element (links, buttons, inputs, summaries, ARIA widgets and
//     anything with a tabindex) is at least 44x44 CSS px (the app's standard; WCAG 2.2 AA asks 24).
//   - No horizontal scroll. The page's scrollWidth never exceeds the screen, and no element sits past the
//     screen edge unless a scroller or clip inside the page holds it.
//   - Reduced motion. With prefers-reduced-motion: reduce, no animation or transition is still running
//     once the screen has settled (document.getAnimations()). The app's reduced motion is "subtle, not
//     none": fades of 120ms or less are allowed, so they must have ended; a looping or long one is a
//     failure. This pass runs at all four widths in light only: nothing in the motion rules depends on
//     the colour scheme.
//
// The index (testdata/shots/index.html) lists every screen with its checks, and shows each capture next
// to the matching Figma frame export in testdata/figma/ (file NJ8HfTPCQgrFty2VVQBMhY, page 139:2).
// A screen with no frame there says "no frame". The comparison is by eye; nothing diffs pixels.
//
// testdata/shots/ is ignored by git (about 700 PNGs); the test regenerates all of it. The Figma exports
// and the vendored axe-core are committed.

const (
	axeVersion = "4.10.2"
	axeSHA256  = "b511cd9dec01c76f4b2ad1723b66b6db37d4c2eb4ed199076e1829d9ee7b75e3"
)

type shotSize struct{ W, H int }

var shotSizes = []shotSize{{360, 740}, {390, 844}, {1024, 768}, {1440, 900}}

// shotScreen is one screen of the matrix. Gold is the screen's rule: "one", "orb" or "none".
type shotScreen struct {
	ID, Group, Title, Gold string
}

// shotScreens lists every screen the matrix must produce. The script's step names are these IDs.
func shotScreens() []shotScreen {
	var out []shotScreen
	add := func(group, gold string, ids ...string) {
		for _, id := range ids {
			out = append(out, shotScreen{ID: id, Group: group, Gold: gold, Title: strings.ReplaceAll(id, "-", " ")})
		}
	}
	// goldNone: screens with no gold, and why.
	//   placement-confirm        the leave dialog offers two neutral buttons on purpose (placement.templ, plConfirm).
	//   placement-reading[-picked]  a reading item has no audio and so no orb; the answers are the actions.
	//   activity-flash-*-back    the four self-grade buttons share one neutral style (north-star section 9).
	//   shell-lesson-ask         the answers are the actions; Continue comes with the result.
	//   shell-signin             the one action is Google's button, which Google's branding rules keep neutral.
	//   shell-today-idle, shell-lessons, shell-review-empty, shell-me, progress*: no main action.
	// SPEC-012: placement.
	add("placement", "one", "placement-intro", "placement-resume", "placement-break", "placement-offer", "placement-typing", "placement-results", "placement-review")
	add("placement", "orb", "placement-letters", "placement-listening", "placement-listening-picked")
	add("placement", "none", "placement-confirm", "placement-reading", "placement-reading-picked")
	// SPEC-016: every activity kind, unanswered, answered wrong and answered right, and the flashcards.
	for _, s := range screens() {
		for _, state := range []string{"ask", "wrong", "right"} {
			add("activity", "one", "activity-"+string(s.item.Kind)+"-"+state)
		}
	}
	add("activity", "one", "activity-flash-front", "activity-flash-recall-front")
	add("activity", "none", "activity-flash-back", "activity-flash-recall-back")
	add("dialogue", "one", "dialogue", "dialogue-playing")
	// SPEC-017: the shell. Today in each state, the destinations, the lesson frame, the report path, sign-in.
	add("shell", "one", "shell-today", "shell-today-start", "shell-today-resume", "shell-today-continue", "shell-today-checkin", "shell-today-next",
		"shell-review", "shell-lesson-story", "shell-lesson-result", "shell-lesson-report", "shell-report-page")
	add("shell", "none", "shell-signin", "shell-today-idle", "shell-lessons", "shell-review-empty", "shell-me", "shell-lesson-ask")
	// SPEC-018: Progress.
	add("progress", "none", "progress", "progress-read", "progress-recall", "progress-spell", "progress-table-open", "progress-hebrew", "progress-no-snapshots", "progress-one-snapshot", "progress-empty")
	return out
}

// shotFrame is a Figma frame exported to testdata/figma/<screen>-<form>-<mode>.png.
type shotFrame struct {
	Screen, Form, Mode, Node, Name string
}

// shotFrames is every frame of page 139:2 that matches a screen. A screen not listed has no frame.
// The frames are exported with Figma's get_screenshot at 1x (390 or 1440 wide).
var shotFrames = []shotFrame{
	{"placement-intro", "phone", "light", "142:3", "Placement / 1 Intro"},
	{"placement-listening", "phone", "light", "142:47", "Placement / 2 Listening"},
	{"placement-listening", "phone", "dark", "147:327", "Placement / 2 Listening · Dark"},
	{"placement-listening-picked", "phone", "light", "144:25", "Placement / 3 Listening · picked"},
	{"placement-listening-picked", "phone", "dark", "147:440", "Placement / 3 Listening · picked · Dark"},
	{"placement-reading", "phone", "light", "144:53", "Placement / 4 Reading"},
	{"placement-typing", "phone", "light", "145:62", "Placement / 5 Typing"},
	{"placement-results", "phone", "light", "145:149", "Placement / 6 Results"},
	{"placement-results", "phone", "dark", "147:507", "Placement / 6 Results · Dark"},
	{"placement-review", "phone", "light", "158:129", "Placement / 7 Review misses"},
	{"placement-review", "phone", "dark", "158:194", "Placement / 7 Review misses · Dark"},
	{"shell-today", "phone", "light", "149:102", "App / Phone · Today"},
	{"shell-today", "desktop", "light", "153:124", "App / Desktop · Today"},
	{"shell-lesson-result", "phone", "light", "149:178", "App / Phone · Lesson"},
	{"shell-lesson-result", "desktop", "light", "155:153", "App / Desktop · Lesson"},
	{"activity-flash-front", "phone", "dark", "151:119", "App / Phone · Review card"},
	{"activity-flash-back", "phone", "dark", "151:142", "App / Phone · Review card · revealed"},
	{"progress", "phone", "light", "182:204", "App / Phone · Progress"},
	{"progress", "desktop", "light", "183:204", "App / Desktop · Progress"},
}

func frameFile(f shotFrame) string { return "figma/" + f.Screen + "-" + f.Form + "-" + f.Mode + ".png" }

// ---- what the script returns ----

type shotAxeNode struct {
	Target string `json:"target"`
	HTML   string `json:"html"`
}
type shotAxeViolation struct {
	ID     string        `json:"id"`
	Impact string        `json:"impact"`
	Help   string        `json:"help"`
	Nodes  []shotAxeNode `json:"nodes"`
}
type shotRecord struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Width   int    `json:"width"`
	Scheme  string `json:"scheme"`
	Motion  string `json:"motion"`
	File    string `json:"file"`
	Targets int    `json:"targets"`
	Gold    struct {
		Primary int      `json:"primary"`
		Orb     int      `json:"orb"`
		Painted int      `json:"painted"`
		Total   int      `json:"total"`
		Items   []string `json:"items"`
	} `json:"gold"`
	Stray    []string `json:"stray"`
	StrayN   int      `json:"strayN"`
	Small    []string `json:"small"`
	SmallN   int      `json:"smallN"`
	Sideways float64  `json:"sideways"`
	Pushed   []string `json:"pushed"`
	Reach    []string `json:"reach"`
	PushedN  int      `json:"pushedN"`
	Running  []string `json:"running"`
	Vertical float64  `json:"vertical"`
	Modal    bool     `json:"modal"`
	Axe      *struct {
		Violations []shotAxeViolation `json:"violations"`
		Incomplete []struct {
			ID     string   `json:"id"`
			Impact string   `json:"impact"`
			N      int      `json:"n"`
			Why    []string `json:"why"`
		} `json:"incomplete"`
	} `json:"axe"`
}

type shotResults struct {
	Records  []shotRecord `json:"records"`
	Problems []string     `json:"problems"`
}

// shotFinding is one thing a check objects to. Check is "axe:<rule id>", "gold", "feedback", "targets",
// "scroll" or "motion".
type shotFinding struct {
	Check, Screen, Key string
	Width              int
	Scheme             string
	Detail             string
}

func (f shotFinding) String() string {
	return fmt.Sprintf("%s: %s at %dpx %s: %s", f.Check, f.Screen, f.Width, f.Scheme, f.Detail)
}

func more(shown []string, total int) []string {
	if total > len(shown) {
		return append(shown, fmt.Sprintf("(and %d more)", total-len(shown)))
	}
	return shown
}

// judge applies the rules to one record. The axe findings are the serious and critical violations only;
// axeNotes returns the rest.
func judge(s shotScreen, r shotRecord) []shotFinding {
	var out []shotFinding
	add := func(check, detail string) {
		out = append(out, shotFinding{Check: check, Screen: s.ID, Key: r.Key, Width: r.Width, Scheme: r.Scheme, Detail: detail})
	}
	g := r.Gold
	switch {
	case g.Total > 1:
		add("gold", fmt.Sprintf("%d gold actions (want at most one): %s", g.Total, strings.Join(g.Items, "; ")))
	case s.Gold == "one" && !(g.Primary == 1 && g.Total == 1):
		add("gold", fmt.Sprintf("want exactly one Primary button, found %d primary, %d orb, %d other painted gold: %s", g.Primary, g.Orb, g.Painted, strings.Join(g.Items, "; ")))
	case s.Gold == "orb" && !(g.Orb == 1 && g.Primary == 0 && g.Total == 1):
		add("gold", fmt.Sprintf("a pick-by-ear screen wants exactly one gold orb and no gold button, found %d primary, %d orb, %d other painted gold: %s", g.Primary, g.Orb, g.Painted, strings.Join(g.Items, "; ")))
	case s.Gold == "none" && g.Total != 0:
		add("gold", fmt.Sprintf("this screen is meant to have no gold, but shows: %s", strings.Join(g.Items, "; ")))
	}
	for _, d := range more(r.Stray, r.StrayN) {
		add("feedback", d)
	}
	for _, d := range more(r.Small, r.SmallN) {
		add("targets", d)
	}
	if r.Sideways > 0 {
		add("scroll", fmt.Sprintf("the page is %vpx wider than the screen; reaching past the edge: %s", r.Sideways, strings.Join(r.Reach, "; ")))
	}
	for _, d := range more(r.Pushed, r.PushedN) {
		add("scroll", "past the screen edge: "+d)
	}
	if r.Motion == "reduce" {
		for _, d := range r.Running {
			add("motion", "still running: "+d)
		}
	}
	if r.Axe != nil {
		for _, v := range r.Axe.Violations {
			if v.Impact != "serious" && v.Impact != "critical" {
				continue
			}
			for _, n := range v.Nodes {
				add("axe:"+v.ID, fmt.Sprintf("[%s] %s: %s | %s", v.Impact, v.Help, n.Target, n.HTML))
			}
		}
	}
	return out
}

// axeNotes are the moderate and minor axe findings, and the checks axe could not decide, for the index.
func axeNotes(r shotRecord) []string {
	if r.Axe == nil {
		return nil
	}
	var out []string
	for _, v := range r.Axe.Violations {
		if v.Impact == "serious" || v.Impact == "critical" {
			continue
		}
		out = append(out, fmt.Sprintf("%s (%s) on %d element(s)", v.ID, v.Impact, len(v.Nodes)))
	}
	for _, i := range r.Axe.Incomplete {
		out = append(out, fmt.Sprintf("%s: axe could not decide on %d element(s): %s", i.ID, i.N, strings.Join(i.Why, ", ")))
	}
	return out
}

// ---- exceptions ----

// shotException is a named, narrow allowance for a real defect that is a design call or too large to fix
// inside this spec. It names the check, the screen, the widths, and a piece of the offending detail, and it
// says how many offending items it covers in one capture. The test fails when an exception covers nothing
// (the defect is gone: delete the exception) and when a capture has more offenders than it allows (the
// defect grew).
type shotException struct {
	Check   string
	Screens []string
	Widths  []int // empty: every width
	Match   string
	// MinW and MinH, for a targets exception, are the least size the target may have and still be covered:
	// a smaller one is a new defect.
	MinW, MinH float64
	Max        int // offenders allowed in one capture
	Reason     string
}

// shotExceptions is every allowance. Each is a decision recorded in a spec, not a convenience.
var shotExceptions = []shotException{
	{
		Check:   "targets",
		Screens: []string{"activity-type_it-ask", "activity-dictation-ask", "placement-typing"},
		Widths:  []int{360, 390},
		Match:   "button.hk-key",
		MinW:    28, MinH: 44, Max: 40,
		Reason: "Hebrew keyboard keys (SPEC-011 aud-1, decided): ten or more keys in a row cannot be 44px wide on a 360 or 390px screen, so a key is 44px tall and at least 28px wide, with the cap inside it. Every key is still 44px tall, and from 1024px the keys are 44px wide.",
	},
}

var targetSize = regexp.MustCompile(`(\d+(?:\.\d+)?)x(\d+(?:\.\d+)?)$`)

func (e shotException) covers(f shotFinding) bool {
	if e.Check != f.Check || !strings.Contains(f.Detail, e.Match) {
		return false
	}
	onScreen := false
	for _, s := range e.Screens {
		onScreen = onScreen || s == f.Screen
	}
	if !onScreen {
		return false
	}
	if e.MinW > 0 || e.MinH > 0 {
		m := targetSize.FindStringSubmatch(f.Detail)
		if m == nil {
			return false
		}
		w, _ := strconv.ParseFloat(m[1], 64)
		h, _ := strconv.ParseFloat(m[2], 64)
		if w < e.MinW || h < e.MinH {
			return false
		}
	}
	if len(e.Widths) == 0 {
		return true
	}
	for _, w := range e.Widths {
		if w == f.Width {
			return true
		}
	}
	return false
}

// applyExceptions returns the findings no exception covers, and the faults of the exceptions themselves.
func applyExceptions(findings []shotFinding, ex []shotException) (left []shotFinding, faults []string) {
	used := make([]int, len(ex))
	perCapture := make([]map[string]int, len(ex))
	for i := range perCapture {
		perCapture[i] = map[string]int{}
	}
	for _, f := range findings {
		covered := false
		for i, e := range ex {
			if e.covers(f) {
				covered = true
				used[i]++
				perCapture[i][f.Screen+" at "+f.Key]++
				break
			}
		}
		if !covered {
			left = append(left, f)
		}
	}
	for i, e := range ex {
		if e.Reason == "" || e.Match == "" || e.Max < 1 {
			faults = append(faults, fmt.Sprintf("exception %d (%s %v) must have a Match, a Max of at least 1 and a Reason", i, e.Check, e.Screens))
		}
		if used[i] == 0 {
			faults = append(faults, fmt.Sprintf("exception for %s on %v (%q) covers nothing any more: delete it", e.Check, e.Screens, e.Match))
		}
		for key, n := range perCapture[i] {
			if n > e.Max {
				faults = append(faults, fmt.Sprintf("exception for %s on %v (%q) allows %d per capture but %s has %d: the defect grew", e.Check, e.Screens, e.Match, e.Max, key, n))
			}
		}
	}
	return left, faults
}

// The exception mechanism has to fail when it should, and that needs no browser.
func TestShotsExceptions(t *testing.T) {
	f := func(screen, key, detail string) shotFinding {
		return shotFinding{Check: "targets", Screen: screen, Key: key, Width: 360, Scheme: "light", Detail: detail}
	}
	ex := []shotException{{Check: "targets", Screens: []string{"a"}, Match: "btn", Max: 1, Reason: "r"}}
	left, faults := applyExceptions([]shotFinding{f("a", "k1", "btn 30x30"), f("b", "k1", "btn 30x30")}, ex)
	if len(left) != 1 || left[0].Screen != "b" || len(faults) != 0 {
		t.Errorf("a covered finding must go and an uncovered one stay: left=%v faults=%v", left, faults)
	}
	if _, faults := applyExceptions(nil, ex); len(faults) != 1 || !strings.Contains(faults[0], "covers nothing") {
		t.Errorf("an exception that covers nothing must be a fault: %v", faults)
	}
	if _, faults := applyExceptions([]shotFinding{f("a", "k1", "btn 1"), f("a", "k1", "btn 2")}, ex); len(faults) != 1 || !strings.Contains(faults[0], "grew") {
		t.Errorf("an exception that is exceeded must be a fault: %v", faults)
	}
	if _, faults := applyExceptions([]shotFinding{f("a", "k1", "btn 1")}, []shotException{{Check: "targets", Screens: []string{"a"}, Match: "btn"}}); len(faults) == 0 {
		t.Error("an exception without a Max and a Reason must be a fault")
	}
	// A size floor: a key narrower than the decided 28px is a new defect, not covered.
	sized := []shotException{{Check: "targets", Screens: []string{"a"}, Match: "key", MinW: 28, MinH: 44, Max: 5, Reason: "r"}}
	if left, _ := applyExceptions([]shotFinding{f("a", "k1", "key 28.4x44"), f("a", "k1", "key 20x44"), f("a", "k1", "key 30x30")}, sized); len(left) != 2 {
		t.Errorf("only the key that meets the floor is covered: %v", left)
	}
	// The gold rule, on made-up records.
	var r shotRecord
	r.Gold.Primary, r.Gold.Total = 1, 1
	if got := judge(shotScreen{ID: "x", Gold: "one"}, r); len(got) != 0 {
		t.Errorf("one primary on a one screen is fine: %v", got)
	}
	r.Gold.Primary, r.Gold.Total = 2, 2
	if got := judge(shotScreen{ID: "x", Gold: "one"}, r); len(got) != 1 || got[0].Check != "gold" {
		t.Errorf("two primaries must fail: %v", got)
	}
	r.Gold.Primary, r.Gold.Orb, r.Gold.Total = 0, 1, 1
	if got := judge(shotScreen{ID: "x", Gold: "orb"}, r); len(got) != 0 {
		t.Errorf("one orb on a pick-by-ear screen is fine: %v", got)
	}
	if got := judge(shotScreen{ID: "x", Gold: "one"}, r); len(got) != 1 {
		t.Errorf("an orb alone is not a primary action on a normal screen: %v", got)
	}
}

// ---- the run ----

type shotCombo struct {
	Key       string            `json:"key"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Scheme    string            `json:"scheme"`
	Motion    string            `json:"motion"`
	Placement map[string]string `json:"placement"`
	LessonA   string            `json:"lessonA"`
	LessonB   string            `json:"lessonB"`
}

func shotCombos() []shotCombo {
	var out []shotCombo
	for _, sz := range shotSizes {
		for _, scheme := range []string{"light", "dark"} {
			out = append(out, shotCombo{Key: scheme + "-" + strconv.Itoa(sz.W), Width: sz.W, Height: sz.H, Scheme: scheme, Motion: "no-preference"})
		}
	}
	for _, sz := range shotSizes {
		out = append(out, shotCombo{Key: "reduce-" + strconv.Itoa(sz.W), Width: sz.W, Height: sz.H, Scheme: "light", Motion: "reduce"})
	}
	return out
}

func TestShots(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("TestShots needs headless Chromium through Playwright: set MOTION_PW_DIR to a directory with node_modules/playwright (and Chromium installed for it)")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("TestShots needs node on the PATH to drive Playwright")
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "playwright")); err != nil {
		t.Skipf("TestShots: no node_modules/playwright in MOTION_PW_DIR (%s)", dir)
	}

	axePath, err := filepath.Abs(filepath.Join("testdata", "axe", "axe.min.js"))
	if err != nil {
		t.Fatal(err)
	}
	axe, err := os.ReadFile(axePath)
	if err != nil {
		t.Fatalf("the vendored axe-core is missing: %v", err)
	}
	if sum := sha256.Sum256(axe); hex.EncodeToString(sum[:]) != axeSHA256 {
		t.Fatalf("testdata/axe/axe.min.js is not axe-core %s (sha256 %x); see testdata/axe/NOTICE.txt", axeVersion, sum)
	}
	for _, f := range shotFrames {
		if _, err := os.Stat(filepath.Join("testdata", frameFile(f))); err != nil {
			t.Fatalf("Figma export %s is missing: %v", frameFile(f), err)
		}
	}

	shots := os.Getenv("SHOTS_DIR")
	if shots == "" {
		shots = filepath.Join("testdata", "shots")
	}
	shots, err = filepath.Abs(shots)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(shots, 0o755); err != nil {
		t.Fatal(err)
	}
	// Regenerate everything: old captures of a screen that no longer exists must not stay.
	old, _ := filepath.Glob(filepath.Join(shots, "*.png"))
	for _, f := range append(old, filepath.Join(shots, "index.html"), filepath.Join(shots, "results.json")) {
		_ = os.Remove(f)
	}

	combos := shotCombos()
	if only := os.Getenv("SHOTS_ONLY"); only != "" {
		want := map[string]bool{}
		for _, k := range strings.Split(only, ",") {
			want[strings.TrimSpace(k)] = true
		}
		var keep []shotCombo
		for _, c := range combos {
			if want[c.Key] {
				keep = append(keep, c)
			}
		}
		combos = keep
		if len(combos) == 0 {
			t.Fatalf("SHOTS_ONLY=%q names no combination", only)
		}
		t.Logf("partial run: %d of the combinations", len(combos))
	}

	// One server for each part of the app; each combination has its own learners where a walk uses them up.
	pl := startPlacementServer(t, len(combos)*6+2, false)
	for i := range combos {
		combos[i].Placement = map[string]string{}
		for _, state := range []string{"intro", "letters", "listening", "break", "offer", "results"} {
			combos[i].Placement[state] = pl.learner(state).cookie.Value
		}
	}
	shellOrigin, shellCookies := startShellServerN(t, len(combos)*2)
	for i := range combos {
		combos[i].LessonA = "lesson" + strconv.Itoa(2*i)
		combos[i].LessonB = "lesson" + strconv.Itoa(2*i+1)
	}
	actOrigin, actSession := startActivityServer(t)
	progOrigin, progCookies := startProgressServer(t)

	dlgSrv := httptest.NewUnstartedServer(nil)
	dlgOrigin := "http://" + dlgSrv.Listener.Addr().String()
	dlgAudio := t.TempDir()
	dlg := newDialogueEnvAt(t, true, dlgOrigin, dlgAudio)
	for _, id := range dlg.ids {
		// Long enough that a line is still playing while the checks run.
		if err := os.WriteFile(filepath.Join(dlgAudio, id+".mp3"), silentMP3(600), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dlgSrv.Config.Handler = dlg.h
	dlgSrv.Start()
	t.Cleanup(dlgSrv.Close)

	type actScreen struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var acts []actScreen
	for _, s := range screens() {
		for _, state := range []string{"ask", "wrong", "right"} {
			acts = append(acts, actScreen{"activity-" + string(s.item.Kind) + "-" + state, string(s.item.Kind) + "." + state})
		}
	}
	for _, n := range []string{"flash.front", "flash.back", "flash-recall.front", "flash-recall.back"} {
		acts = append(acts, actScreen{"activity-" + strings.ReplaceAll(n, ".", "-"), n})
	}

	var onlyScreens []string
	if v := os.Getenv("SHOTS_SCREENS"); v != "" {
		onlyScreens = strings.Split(v, ",")
		t.Logf("partial run: screens starting with %v", onlyScreens)
	}
	parallel := 3
	if p, err := strconv.Atoi(os.Getenv("SHOTS_PARALLEL")); err == nil && p > 0 {
		parallel = p
	}
	cfg := map[string]any{
		"axe": axePath, "out": shots, "parallel": parallel, "combos": combos, "only": onlyScreens,
		"placement": map[string]any{"origin": pl.origin},
		"activity":  map[string]any{"origin": actOrigin, "session": actSession, "screens": acts},
		"dialogue":  map[string]any{"origin": dlgOrigin, "session": dlg.cookie.Value, "unit": dlg.unitID},
		"shell":     map[string]any{"origin": shellOrigin, "cookies": shellCookies},
		"progress":  map[string]any{"origin": progOrigin, "cookies": progCookies},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "shots-config.json")
	if err := os.WriteFile(cfgPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(cfgPath) })
	out := runNode(t, dir, "shots.mjs", "CONFIG="+cfgPath)
	t.Logf("script: %s", strings.TrimSpace(string(out)))

	rawRes, err := os.ReadFile(filepath.Join(shots, "results.json"))
	if err != nil {
		t.Fatal(err)
	}
	var res shotResults
	if err := json.Unmarshal(rawRes, &res); err != nil {
		t.Fatal(err)
	}

	// ac-1: every screen, in every combination.
	screensList := shotScreens()
	have := map[string]shotRecord{}
	for _, r := range res.Records {
		have[r.ID+"|"+r.Key] = r
	}
	var findings []shotFinding
	byID := map[string]shotScreen{}
	for _, s := range screensList {
		byID[s.ID] = s
	}
	for _, r := range res.Records {
		s, ok := byID[r.ID]
		if !ok {
			t.Errorf("the script captured %q, which is not in shotScreens", r.ID)
			continue
		}
		findings = append(findings, judge(s, r)...)
	}
	for _, p := range res.Problems {
		t.Errorf("script: %s", p)
	}
	for _, s := range screensList {
		wanted := len(onlyScreens) == 0
		for _, p := range onlyScreens {
			wanted = wanted || strings.HasPrefix(s.ID, p)
		}
		for _, c := range combos {
			if !wanted {
				continue
			}
			if _, ok := have[s.ID+"|"+c.Key]; !ok {
				t.Errorf("MISSING capture: %s at %dpx %s (%s)", s.ID, c.Width, c.Scheme, c.Key)
			}
		}
	}
	left, faults := applyExceptions(findings, shotExceptions)
	if os.Getenv("SHOTS_ONLY") != "" || len(onlyScreens) > 0 {
		// A partial run cannot tell that an exception is unused.
		faults = nil
	}

	if err := writeShotsIndex(shots, screensList, combos, res, left); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d captures of %d screens; index: %s", len(res.Records), len(screensList), filepath.Join(shots, "index.html"))

	sort.SliceStable(left, func(i, j int) bool {
		if left[i].Check != left[j].Check {
			return left[i].Check < left[j].Check
		}
		return left[i].Screen < left[j].Screen
	})
	for _, f := range left {
		t.Errorf("FAIL %s", f)
	}
	for _, f := range faults {
		t.Errorf("FAIL %s", f)
	}
}

// ---- the index ----

type idxCell struct {
	Key, File string
	Fail      int
	Notes     []string
}
type idxFrame struct {
	Label, Mode, Figma, Capture string
	Has                         bool
	Name, Node                  string
}
type idxScreen struct {
	shotScreen
	Cells  [][]idxCell // rows: widths; cells: light, dark, reduced motion
	Frames []idxFrame
	Fails  int
	Notes  []string
}
type idxData struct {
	Screens   []idxScreen
	Widths    []int
	Total     int
	Fails     int
	Captures  int
	Groups    []string
	WithFrame int
	NoFrame   []string
	Rule      []string
	Exception []shotException
}

func writeShotsIndex(dir string, list []shotScreen, combos []shotCombo, res shotResults, failing []shotFinding) error {
	fails := map[string]int{}
	for _, f := range failing {
		fails[f.Screen+"|"+f.Key]++
	}
	recs := map[string]shotRecord{}
	for _, r := range res.Records {
		recs[r.ID+"|"+r.Key] = r
	}
	frames := map[string]shotFrame{}
	for _, f := range shotFrames {
		frames[f.Screen+"|"+f.Form+"|"+f.Mode] = f
	}
	ran := map[string]bool{}
	for _, c := range combos {
		ran[c.Key] = true
	}
	var data idxData
	for _, sz := range shotSizes {
		data.Widths = append(data.Widths, sz.W)
	}
	data.Captures = len(res.Records)
	data.Exception = shotExceptions
	data.Rule = []string{
		"axe-core " + axeVersion + ", WCAG 2.0/2.1/2.2 A and AA: no serious or critical violation (moderate and minor are listed as notes).",
		"One gold action: at most one gold element; exactly one Primary button on a screen with a main action; the orb alone, and no gold button, on a placement pick-by-ear screen; no gold at all on a screen whose choices are neutral by design or that has no main action.",
		"Green and red (--go, --stop and their soft forms) only on feedback surfaces.",
		"Every interactive target at least 44 x 44 CSS px.",
		"No horizontal scroll, and nothing cut off at the screen edge.",
		"With prefers-reduced-motion: reduce, nothing is still animating once the screen has settled.",
	}
	seenGroup := map[string]bool{}
	for _, s := range list {
		is := idxScreen{shotScreen: s}
		for _, sz := range shotSizes {
			var row []idxCell
			for _, key := range []string{"light-", "dark-", "reduce-"} {
				k := key + strconv.Itoa(sz.W)
				cell := idxCell{Key: k}
				if r, ok := recs[s.ID+"|"+k]; ok {
					cell.File = r.File
					cell.Fail = fails[s.ID+"|"+k]
					cell.Notes = axeNotes(r)
					is.Fails += cell.Fail
					for _, n := range cell.Notes {
						is.Notes = append(is.Notes, fmt.Sprintf("%s %dpx: %s", strings.Split(k, "-")[0], sz.W, n))
					}
				} else if !ran[k] {
					cell.Key = "skipped"
				} else {
					cell.Key = "missing"
					cell.Fail = 1
					is.Fails++
				}
				row = append(row, cell)
			}
			is.Cells = append(is.Cells, row)
		}
		for _, form := range []struct{ name, label string }{{"phone", "Phone, 390 px"}, {"desktop", "Desktop, 1440 px"}} {
			w := "390"
			if form.name == "desktop" {
				w = "1440"
			}
			for _, mode := range []string{"light", "dark"} {
				fr := idxFrame{Label: form.label, Mode: mode, Capture: s.ID + "." + mode + "-" + w + ".png"}
				if f, ok := frames[s.ID+"|"+form.name+"|"+mode]; ok {
					fr.Has, fr.Figma, fr.Name, fr.Node = true, frameFile(f), f.Name, f.Node
				}
				is.Frames = append(is.Frames, fr)
			}
		}
		data.Fails += is.Fails
		data.Screens = append(data.Screens, is)
		if !seenGroup[s.Group] {
			seenGroup[s.Group] = true
			data.Groups = append(data.Groups, s.Group)
		}
	}
	data.Total = len(list)
	have := map[string]bool{}
	for _, f := range shotFrames {
		have[f.Screen] = true
	}
	for _, s := range list {
		if have[s.ID] {
			data.WithFrame++
		} else {
			data.NoFrame = append(data.NoFrame, s.ID)
		}
	}
	f, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()
	return shotsIndexTmpl.Execute(f, data)
}

var shotsIndexTmpl = template.Must(template.New("index").Funcs(template.FuncMap{
	"figma": func(p string) template.URL { return template.URL("../" + p) },
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Screen captures and checks (INIT-015/SPEC-024)</title>
<style>
body{font:15px/1.5 system-ui,sans-serif;margin:24px;color:#151a2e;background:#f5f0e7}
h1{margin:0 0 4px}h2{margin:40px 0 4px;padding-top:16px;border-top:2px solid #ccc;font-size:19px}h3{margin:18px 0 6px;font-size:14px;color:#555b70}
.sum{padding:10px 14px;background:#fffdf9;border:1px solid #ddd;border-radius:8px;display:inline-block}
.ok{color:#0e7a55}.bad{color:#b3363f;font-weight:700}.muted{color:#555b70}
table{border-collapse:collapse;font-size:13px}td,th{border:1px solid #ddd;padding:2px 8px;text-align:center}
td.left,th.left{text-align:left}
.pairs{display:flex;flex-wrap:wrap;gap:20px;align-items:flex-start}
figure{margin:0;display:block}figcaption{font-size:12px;color:#555b70;max-width:300px}
.pair{display:flex;gap:10px;padding:10px;background:#fffdf9;border:1px solid #ddd;border-radius:8px;align-items:flex-start}
.pair img{display:block;max-width:300px;max-height:640px;border:1px solid #ccc;background:#fff}
.pair.wide img{max-width:560px}
.noframe{width:300px;height:120px;border:2px dashed #aaa;display:flex;align-items:center;justify-content:center;color:#777;font-weight:700}
.thumbs{display:flex;flex-wrap:wrap;gap:10px}.thumbs a{font-size:11px;color:#555b70;text-decoration:none;text-align:center}
.thumbs img{display:block;height:170px;border:1px solid #ccc;margin-bottom:2px}
nav a{margin-right:12px}
details{margin:6px 0}
</style></head><body>
<h1>Screen captures and checks</h1>
<p class="muted">INIT-015/SPEC-024. {{.Total}} screens, {{.Captures}} captures: every screen in light and dark at {{range .Widths}}{{.}} {{end}}px, plus a reduced-motion pass in light. Regenerated by <code>go test -tags browser -run Shots ./internal/web</code>. Figma frames come from file NJ8HfTPCQgrFty2VVQBMhY, page 139:2. The comparison is by eye; nothing diffs pixels.</p>
<p class="sum">{{if .Fails}}<span class="bad">{{.Fails}} failing check(s)</span>{{else}}<span class="ok">Every check passes on every capture.</span>{{end}}</p>
<h3>Figma coverage</h3>
<p>{{.WithFrame}} of {{.Total}} screens have at least one matching frame on the Figma page. These have <b>no frame</b>: {{range .NoFrame}}{{.}}, {{end}}and no screen has a frame at 360 or 1024 px. Only some have a dark frame.</p>
<h3>The rules, on every capture</h3>
<ul>{{range .Rule}}<li>{{.}}</li>{{end}}</ul>
{{if .Exception}}<h3>Named exceptions</h3><ul>{{range .Exception}}<li><b>{{.Check}}</b> on {{.Screens}} at {{.Widths}} px ({{.Match}}), at most {{.Max}} per capture: {{.Reason}}</li>{{end}}</ul>{{else}}<p class="muted">Named exceptions: none.</p>{{end}}
<nav>{{range .Screens}}<a href="#{{.ID}}" {{if .Fails}}class="bad"{{end}}>{{.ID}}</a> {{end}}</nav>
{{range .Screens}}
<h2 id="{{.ID}}">{{.ID}} <small class="muted">· {{.Group}} · gold rule: {{.Gold}}</small> {{if .Fails}}<span class="bad">{{.Fails}} failing</span>{{else}}<span class="ok">passes</span>{{end}}</h2>
<h3>Capture next to the Figma frame</h3>
<div class="pairs">
{{range .Frames}}
<div class="pair {{if eq .Label "Desktop, 1440 px"}}wide{{end}}">
 <figure>{{if .Has}}<img loading="lazy" src="{{figma .Figma}}" alt="Figma {{.Name}}"><figcaption>Figma: {{.Name}} ({{.Node}})</figcaption>{{else}}<div class="noframe">no frame</div><figcaption>Figma: no {{.Mode}} {{.Label}} frame</figcaption>{{end}}</figure>
 <figure><img loading="lazy" src="{{.Capture}}" alt="Capture {{.Capture}}"><figcaption>App, {{.Mode}}, {{.Label}}</figcaption></figure>
</div>
{{end}}
</div>
<h3>Every capture</h3>
<div class="thumbs">{{range .Cells}}{{range .}}{{if .File}}<a href="{{.File}}"><img loading="lazy" src="{{.File}}" alt="{{.File}}">{{.Key}}{{if .Fail}} <span class="bad">({{.Fail}} fail)</span>{{end}}</a>{{end}}{{end}}{{end}}</div>
<h3>Checks</h3>
<table><tr><th class="left">width</th><th>light</th><th>dark</th><th>reduced motion</th></tr>
{{range $i, $row := .Cells}}<tr><td class="left">{{index $.Widths $i}}</td>{{range $row}}<td>{{if eq .Key "skipped"}}<span class="muted">not run</span>{{else if eq .Key "missing"}}<span class="bad">missing</span>{{else if .Fail}}<span class="bad">{{.Fail}} fail</span>{{else}}<span class="ok">pass</span>{{end}}</td>{{end}}</tr>{{end}}
</table>
{{if .Notes}}<details><summary>axe notes (moderate, minor, needs review): {{len .Notes}}</summary><ul>{{range .Notes}}<li>{{.}}</li>{{end}}</ul></details>{{end}}
{{end}}
</body></html>
`))
