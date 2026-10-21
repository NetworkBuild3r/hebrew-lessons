package web

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// INIT-015/SPEC-016: the activity screens and the review card.
//
// The first group renders the screens from fixtures and checks them by acceptance
// criterion: ac-1 (one gold action), ac-2 (the sheet shows the right response),
// ac-3 (the flashcard grades), ac-4 (word match markup; the real-browser half is
// activity_browser_test.go) and ac-5 (placement shares no feedback component). The
// second group drives the review handlers with a real store, a real session mix and a
// real engine.

// ---- fixtures ---------------------------------------------------------------

const (
	helloHe   = "שָׁלוֹם"
	morningHe = "בֹּקֶר"
	eveningHe = "עֶרֶב"
	waterHe   = "מַיִם"
	chairHe   = "כִּסֵּא"
)

func clipID(c byte) string { return strings.Repeat(string(c), 64) }

// screen is one activity with the answers a test needs: right and wrong.
type screen struct {
	name         string
	item         activity.Item
	right, wrong *answerState
}

func screens() []screen {
	glosses := []string{"hello", "morning", "evening", "water"}
	pick := func(kind activity.Kind, prompt activity.Prompt) screen {
		it := activity.Item{Token: "tok-" + string(kind), Kind: kind, Skill: "listen", Prompt: prompt, Options: glosses}
		res := activity.Result{Feedback: true, Answer: "hello", Text: helloHe, Gloss: "hello", AudioID: clipID('a')}
		right, wrong := res, res
		right.Correct = true
		wrong.Confused = []activity.Confused{{Kind: fsrs.MissOther, Key: helloHe + "/" + morningHe, Word: helloHe, With: morningHe}}
		return screen{
			name: string(kind), item: it,
			right: &answerState{item: it, resp: activity.Response{Choice: 0}, res: right},
			wrong: &answerState{item: it, resp: activity.Response{Choice: 1}, res: wrong},
		}
	}
	typed := func(kind activity.Kind, prompt activity.Prompt) screen {
		it := activity.Item{Token: "tok-" + string(kind), Kind: kind, Skill: "recall", Prompt: prompt, Typed: true}
		res := activity.Result{Feedback: true, Answer: helloHe, Text: helloHe, Gloss: "hello", AudioID: clipID('a')}
		right, wrong := res, res
		right.Correct = true
		wrong.Confused = []activity.Confused{{Kind: fsrs.MissLetterPair, Key: "י/ו", Word: helloHe}}
		return screen{
			name: string(kind), item: it,
			right: &answerState{item: it, resp: activity.Response{Text: "שָׁלוֹם"}, text: "שָׁלוֹם", res: right},
			wrong: &answerState{item: it, resp: activity.Response{Text: "שליםם"}, text: "שליםם", res: wrong},
		}
	}
	match := func(kind activity.Kind) screen {
		words := []string{helloHe, morningHe, eveningHe, waterHe, chairHe}
		gl := []string{"hello", "morning", "evening", "water", "chair"}
		it := activity.Item{Token: "tok-" + string(kind), Kind: kind, Skill: "read", Right: gl}
		var right, wrong activity.Result
		right.Feedback, wrong.Feedback = true, true
		right.Correct = true
		for i := range words {
			if kind == activity.WordMatch {
				it.Left = append(it.Left, activity.Cue{Hebrew: words[i]})
			} else {
				it.Left = append(it.Left, activity.Cue{AudioID: clipID(byte('a' + i))})
			}
			right.Pairs = append(right.Pairs, activity.PairResult{Left: i, Chosen: i, Correct: true, Text: words[i], Gloss: gl[i], AudioID: clipID(byte('a' + i))})
			// The wrong answer swaps the first two.
			chosen, ok := i, true
			if i == 0 {
				chosen, ok = 1, false
			} else if i == 1 {
				chosen, ok = 0, false
			}
			wrong.Pairs = append(wrong.Pairs, activity.PairResult{Left: i, Chosen: chosen, Correct: ok, Text: words[i], Gloss: gl[i], AudioID: clipID(byte('a' + i))})
		}
		right.Answer, right.Text, right.Gloss, right.AudioID = "hello", helloHe, "hello", clipID('a')
		wrong.Answer, wrong.Text, wrong.Gloss, wrong.AudioID = "hello", helloHe, "hello", clipID('a')
		wrong.Confused = []activity.Confused{
			{Kind: fsrs.MissOther, Key: "x", Word: helloHe, With: morningHe},
			{Kind: fsrs.MissOther, Key: "y", Word: morningHe, With: helloHe},
		}
		return screen{
			name: string(kind), item: it,
			right: &answerState{item: it, resp: activity.Response{Pairs: []int{0, 1, 2, 3, 4}}, res: right},
			wrong: &answerState{item: it, resp: activity.Response{Pairs: []int{1, 0, 2, 3, 4}}, res: wrong},
		}
	}
	return []screen{
		pick(activity.ListenPick, activity.Prompt{AudioID: clipID('a')}),
		pick(activity.ReadPick, activity.Prompt{Hebrew: helloHe}),
		match(activity.WordMatch),
		match(activity.AudioMatch),
		typed(activity.TypeIt, activity.Prompt{Gloss: "hello"}),
		typed(activity.Dictation, activity.Prompt{AudioID: clipID('a')}),
	}
}

// renderAct renders the review body the handler would send for an item, with its
// answer when there is one. It goes through reviewView, as the handler does.
func renderAct(t *testing.T, it activity.Item, st *answerState) string {
	t.Helper()
	vm := ReviewCardVM{CardID: 7, Kind: "read", Index: 2, Total: 9, Item: &it, Answer: st}
	return renderComponent(t, views.ReviewBody(reviewView("en", "csrf-x", "lede", vm)), "")
}

func renderFlash(t *testing.T, kind string, revealed bool) string {
	t.Helper()
	card := views.ReviewCard{
		Lang: "en", CSRF: "csrf-x", CardID: 7, Kind: kind, Index: 2, Total: 9, Revealed: revealed,
		Pointed: helloHe, Unpointed: "שלום", Translit: "shalom", AudioURL: "/audio/" + clipID('a') + ".mp3",
		Gloss: "hello", Gender: "m",
	}
	return renderComponent(t, views.ReviewBody(card), "")
}

func sheetOf(t *testing.T, html string) string {
	t.Helper()
	i := strings.Index(html, `<section class="c-sheet"`)
	if i < 0 {
		t.Fatalf("no feedback sheet in %s", html)
	}
	j := strings.Index(html[i:], "</section>")
	if j < 0 {
		t.Fatal("unclosed feedback sheet")
	}
	return html[i : i+j]
}

const gold = `data-kind="primary"`

// ---- ac-1 -------------------------------------------------------------------

// TestActivityScreensHaveOneGoldAction is ac-1: every activity screen has exactly one
// gold action before the answer (Check) and after it (Continue), and the flashcard has
// Show answer before the reveal.
func TestActivityScreensHaveOneGoldAction(t *testing.T) {
	for _, sc := range screens() {
		cases := []struct {
			phase string
			st    *answerState
			label string
		}{
			{views.ActAsk, nil, "Check"},
			{views.ActFeedback, sc.right, "Continue"},
			{views.ActFeedback, sc.wrong, "Continue"},
		}
		for i, c := range cases {
			html := renderAct(t, sc.item, c.st)
			if n := strings.Count(html, gold); n != 1 {
				t.Errorf("%s case %d: %d gold actions, want 1", sc.name, i, n)
			}
			if !strings.Contains(html, `data-phase="`+c.phase+`"`) {
				t.Errorf("%s case %d: not in phase %s", sc.name, i, c.phase)
			}
			m := regexp.MustCompile(`(?s)<button[^>]*data-kind="primary"[^>]*>(.*?)</button>`).FindStringSubmatch(html)
			if m == nil || !strings.Contains(m[1], c.label) || !strings.Contains(m[0], "data-act-primary") {
				t.Errorf("%s case %d: the gold action is not %s: %v", sc.name, i, c.label, m)
			}
		}
	}
	if n := strings.Count(renderFlash(t, "read", false), gold); n != 1 {
		t.Errorf("flashcard before the reveal: %d gold actions, want 1", n)
	}
	// After the reveal the four grade buttons are the actions. They are neutral (ac-3),
	// as Figma 151:142 draws them, so no gold action competes with them.
	if n := strings.Count(renderFlash(t, "read", true), gold); n != 0 {
		t.Errorf("revealed flashcard: %d gold actions, want 0 (the grades are neutral)", n)
	}
}

// TestActivityFrameIsShared checks that every screen has the same frame: progress at the
// top, a prompt, and an answer area, in that order.
func TestActivityFrameIsShared(t *testing.T) {
	var htmls []string
	for _, sc := range screens() {
		htmls = append(htmls, renderAct(t, sc.item, nil))
	}
	htmls = append(htmls, renderFlash(t, "read", false), renderFlash(t, "recall", true))
	for i, html := range htmls {
		top := strings.Index(html, `class="act__top"`)
		stage := strings.Index(html, `class="act__stage"`)
		thumb := strings.Index(html, `class="act__thumb"`)
		if top < 0 || stage < top || thumb < stage {
			t.Errorf("screen %d: want progress, then prompt, then thumb zone (%d, %d, %d)", i, top, stage, thumb)
		}
		if !strings.Contains(html, `<progress class="c-progress"`) {
			t.Errorf("screen %d: no progress bar", i)
		}
	}
}

// TestPracticeResponsesCancelTheViewTransition is the first-tap rule. Chromium makes the
// page untappable for the length of a view transition, and motion.js cancels the
// transition for a response that holds a c-answer element or data-no-transition. Every
// response the review sends, for every screen and phase, has the attribute, and every
// answer option and tile uses c-answer.
func TestPracticeResponsesCancelTheViewTransition(t *testing.T) {
	var htmls []string
	for _, sc := range screens() {
		for _, st := range []*answerState{nil, sc.right, sc.wrong} {
			htmls = append(htmls, renderAct(t, sc.item, st))
		}
	}
	for _, kind := range []string{"read", "recall"} {
		htmls = append(htmls, renderFlash(t, kind, false), renderFlash(t, kind, true))
	}
	for i, html := range htmls {
		if !strings.Contains(html, "data-no-transition") {
			t.Errorf("response %d has no data-no-transition", i)
		}
	}
	// The same test motion.js applies.
	re := regexp.MustCompile(`class="[^"]*\bc-answer\b|data-no-transition`)
	for i, html := range htmls {
		if !re.MatchString(html) {
			t.Errorf("response %d would run a view transition", i)
		}
	}
	for _, sc := range screens() {
		html := renderAct(t, sc.item, nil)
		options := strings.Count(html, `data-pick="`) + strings.Count(html, `data-tile="`)
		if options > 0 && strings.Count(html, `class="c-answer"`) != options {
			t.Errorf("%s: %d options or tiles but %d c-answer buttons", sc.name, options, strings.Count(html, `class="c-answer"`))
		}
	}
	js := readStatic(t, "static/js/motion.js")
	if !strings.Contains(js, "c-answer") || !strings.Contains(js, "data-no-transition") {
		t.Error("motion.js no longer cancels the transition for c-answer or data-no-transition")
	}
}

// TestNothingAboutTheAnswerBeforeCheck: an unanswered item carries no answer. The
// options are not marked, the prompt shows only what its skill allows, and the pointed
// Hebrew of the word is not in the page.
func TestNothingAboutTheAnswerBeforeCheck(t *testing.T) {
	for _, sc := range screens() {
		html := renderAct(t, sc.item, nil)
		if strings.Contains(html, `data-state="correct"`) || strings.Contains(html, `data-state="wrong"`) {
			t.Errorf("%s: an option is marked before Check", sc.name)
		}
		if strings.Contains(html, `class="c-sheet"`) {
			t.Errorf("%s: the feedback sheet is up before Check", sc.name)
		}
		switch sc.item.Kind {
		case activity.ListenPick, activity.Dictation, activity.TypeIt:
			if strings.Contains(html, helloHe) {
				t.Errorf("%s: the pointed Hebrew is on a screen that must not show it", sc.name)
			}
		}
		if sc.item.Kind != activity.ReadPick && strings.Contains(html, `class="wb__he`) && sc.item.Kind != activity.WordMatch {
			t.Errorf("%s: shows a Hebrew word block", sc.name)
		}
	}
	// A listening or typing prompt names no Hebrew and no transliteration.
	listen := renderAct(t, screens()[0].item, nil)
	if strings.Contains(listen, "wb__tr") || strings.Contains(listen, `lang="he"`) {
		t.Errorf("listen_pick shows Hebrew or a transliteration: %s", listen)
	}
}

// ---- ac-2 -------------------------------------------------------------------

// TestWrongAnswerSheetShowsTheRightResponse is ac-2: after a wrong answer the sheet has
// the right gloss, the pointed Hebrew, and a playable clip, and for a confusion it says
// "You picked X. That's Y.", for typing it underlines the wrong letters.
func TestWrongAnswerSheetShowsTheRightResponse(t *testing.T) {
	for _, sc := range screens() {
		html := renderAct(t, sc.item, sc.wrong)
		sheet := sheetOf(t, html)
		if !strings.Contains(sheet, "feedback-wrong") {
			t.Errorf("%s: the sheet is not marked wrong", sc.name)
		}
		if !strings.Contains(sheet, ">hello<") {
			t.Errorf("%s: the sheet lacks the right gloss", sc.name)
		}
		if !strings.Contains(sheet, helloHe) {
			t.Errorf("%s: the sheet lacks the pointed Hebrew", sc.name)
		}
		clip := regexp.MustCompile(`<audio id="[^"]+" src="/audio/[a-f0-9]{64}\.mp3"`).MatchString(sheet)
		orb := regexp.MustCompile(`(?s)<button[^>]*class="c-orb"[^>]*>`).FindString(sheet)
		if !clip || !strings.Contains(orb, `data-src="/audio/`) || !strings.Contains(orb, "aria-label=") || !strings.Contains(orb, `data-audio="`) {
			t.Errorf("%s: the sheet has no playable clip (audio %v, orb %q)", sc.name, clip, orb)
		}
		if sc.item.Kind != activity.TypeIt && sc.item.Kind != activity.Dictation {
			if !strings.Contains(sheet, "You picked") || !strings.Contains(sheet, "That&#39;s") || !strings.Contains(sheet, morningHe) {
				t.Errorf("%s: the sheet lacks \"You picked X. That's Y.\": %s", sc.name, sheet)
			}
		}
		// The option or tile that was wrong and the one that was right are marked.
		typedKind := sc.item.Kind == activity.TypeIt || sc.item.Kind == activity.Dictation
		if !typedKind && (!strings.Contains(html, `data-state="correct"`) || !strings.Contains(html, `data-state="wrong"`)) {
			t.Errorf("%s: the options do not show Correct and Wrong", sc.name)
		}
		if strings.Contains(html, "data-act-primary") && strings.Count(html, "data-act-primary") != 1 {
			t.Errorf("%s: more than one primary after the answer", sc.name)
		}
	}
	typed := renderAct(t, screens()[4].item, screens()[4].wrong)
	if !strings.Contains(sheetOf(t, typed), `<u class="fb__bad">`) {
		t.Errorf("type_it: the wrong letters are not underlined: %s", sheetOf(t, typed))
	}
	// A right answer still shows the response and the clip, in the right colour class.
	for _, sc := range screens() {
		sheet := sheetOf(t, renderAct(t, sc.item, sc.right))
		if !strings.Contains(sheet, "feedback-right") || strings.Contains(sheet, "feedback-wrong") {
			t.Errorf("%s: a right answer is not marked right", sc.name)
		}
		if strings.Contains(sheet, "You picked") {
			t.Errorf("%s: a right answer names a confusion", sc.name)
		}
	}
}

// TestTypedAnswerIsEscaped: what the learner typed reaches the page only through templ.
func TestTypedAnswerIsEscaped(t *testing.T) {
	sc := screens()[4]
	evil := `<script>alert(1)</script>"'>`
	st := *sc.wrong
	st.text = evil
	st.resp = activity.Response{Text: evil}
	html := renderAct(t, sc.item, &st)
	if strings.Contains(html, "<script>alert") || strings.Contains(html, `"'>`) {
		t.Fatalf("typed text was echoed raw: %s", sheetOf(t, html))
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("typed text is not echoed escaped: %s", sheetOf(t, html))
	}
}

// ---- ac-3 -------------------------------------------------------------------

// neutralVars are the only var() names a self-grade button may use: surfaces, text,
// lines, and the sizes, never a colour of its own.
var neutralVars = map[string]bool{
	"surface": true, "surface-soft": true, "surface-raised": true, "paper": true,
	"text": true, "muted": true, "hairline": true, "hairline-strong": true,
	"space-1": true, "space-2": true, "space-3": true, "space-8": true, "size-tap": true,
	"radius-control": true, "fs-small": true, "lh-he-body": true,
}

// TestFlashcardGradesAreNeutral is ac-3: four buttons, one class, exactly the labels
// Forgot, Hard, Got it, Easy, and no colour in the style except neutral surfaces.
func TestFlashcardGradesAreNeutral(t *testing.T) {
	for _, kind := range []string{"read", "recall", "letter"} {
		html := renderFlash(t, kind, true)
		buttons := regexp.MustCompile(`(?s)<button[^>]*name="rating"[^>]*>(.*?)</button>`).FindAllStringSubmatch(html, -1)
		if len(buttons) != 4 {
			t.Fatalf("%s: %d grade buttons, want 4", kind, len(buttons))
		}
		var labels []string
		for i, b := range buttons {
			if !strings.Contains(b[0], `class="act-grade"`) {
				t.Errorf("%s: button %d is not class act-grade alone: %s", kind, i, b[0])
			}
			if strings.Contains(b[0], "data-tone") || strings.Contains(b[0], "style=") || strings.Contains(b[0], "data-state") {
				t.Errorf("%s: button %d carries its own tone: %s", kind, i, b[0])
			}
			if !strings.Contains(b[0], `value="`+strconv.Itoa(i+1)+`"`) {
				t.Errorf("%s: button %d does not grade %d", kind, i, i+1)
			}
			labels = append(labels, strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(b[1], "")))
		}
		if strings.Join(labels, "|") != "Forgot|Hard|Got it|Easy" {
			t.Errorf("%s: labels = %q, want Forgot, Hard, Got it, Easy", kind, labels)
		}
	}
	if fmt.Sprint(views.GradeLabels) != "[Forgot Hard Got it Easy]" {
		t.Errorf("GradeLabels = %v", views.GradeLabels)
	}
	// The style: every rule that names act-grade uses neutral roles only.
	rules := 0
	for _, r := range parseCSSRules(readStatic(t, "static/css/activity.css")) {
		if !strings.Contains(r.Selector, "act-grade") {
			continue
		}
		rules++
		if strings.Contains(r.Selector, "[data-") {
			t.Errorf("%s: a grade selector keys on a data attribute, so a grade could get its own tone", r.Selector)
		}
		for _, m := range regexp.MustCompile(`var\(\s*--([a-z0-9-]+)`).FindAllStringSubmatch(r.Body, -1) {
			if !neutralVars[m[1]] {
				t.Errorf("%s uses var(--%s), which is not a neutral surface", r.Selector, m[1])
			}
		}
		if res := lintColors("activity.css", r.Selector+"{"+r.Body+"}", nil); len(res.Violations) != 0 {
			t.Errorf("%v", res.Violations)
		}
	}
	if rules == 0 {
		t.Error("activity.css has no .act-grade rule")
	}
	// The old per-grade classes and the revealed card's old tonal button are gone.
	if html := renderFlash(t, "read", true); strings.Contains(html, "btn-tonal") || strings.Contains(html, `class="grade"`) {
		t.Error("the revealed card still uses an old button class")
	}
}

// ---- ac-4 (markup; the keyboard half runs in a browser) -----------------------

func TestMatchTilesHaveNamesAndKeys(t *testing.T) {
	for _, kind := range []activity.Kind{activity.WordMatch, activity.AudioMatch} {
		var sc screen
		for _, s := range screens() {
			if s.item.Kind == kind {
				sc = s
			}
		}
		for _, st := range []*answerState{nil, sc.right, sc.wrong} {
			html := renderAct(t, sc.item, st)
			tiles := regexp.MustCompile(`(?s)<button[^>]*data-tile="[lr]"[^>]*>`).FindAllString(html, -1)
			if len(tiles) != 10 {
				t.Fatalf("%s: %d tiles, want 10", kind, len(tiles))
			}
			keys := map[string]bool{}
			for _, tl := range tiles {
				name := regexp.MustCompile(`aria-label="([^"]*)"`).FindStringSubmatch(tl)
				if name == nil || strings.TrimSpace(name[1]) == "" {
					t.Errorf("%s: a tile has no accessible name: %s", kind, tl)
				}
				if !strings.Contains(tl, `type="button"`) {
					t.Errorf("%s: a tile is not a button: %s", kind, tl)
				}
				hk := regexp.MustCompile(`data-hotkey="([^"]*)"`).FindStringSubmatch(tl)
				if hk == nil || keys[hk[1]] {
					t.Errorf("%s: a tile has no key, or repeats one: %s", kind, tl)
				} else {
					keys[hk[1]] = true
				}
			}
		}
		html := renderAct(t, sc.item, nil)
		if kind == activity.WordMatch {
			if !strings.Contains(html, `<div class="act__col" role="group" dir="rtl" lang="he"`) {
				t.Errorf("the Hebrew column of word_match is not right to left: %s", html)
			}
		} else if strings.Contains(html, `dir="rtl"`) {
			t.Error("audio_match has no Hebrew column, so nothing should be right to left")
		}
		if strings.Count(html, `name="pair_`) != 5 && kind == activity.WordMatch {
			t.Errorf("word_match posts %d pair fields, want 5", strings.Count(html, `name="pair_`))
		}
		if !strings.Contains(html, "data-act-reset") {
			t.Errorf("%s has no way to start over", kind)
		}
	}
	// The keys of a pick option and a grade button are the keys the script listens for.
	html := renderAct(t, screens()[0].item, nil)
	for _, k := range []string{"1", "2", "3", "4"} {
		if !strings.Contains(html, `data-hotkey="`+k+`"`) {
			t.Errorf("pick option %s has no hotkey", k)
		}
	}
	js := readStatic(t, "static/js/activity.js")
	for _, want := range []string{`e.key === "Enter"`, `e.key === " "`, "data-hotkey", "data-act-primary"} {
		if !strings.Contains(js, want) {
			t.Errorf("activity.js does not handle %s", want)
		}
	}
}

// ---- ac-5 -------------------------------------------------------------------

// TestPlacementViewsShareNoFeedbackComponent is ac-5, at build time. Placement gives no
// right or wrong feedback until its review screen (und-1), so no placement view may
// import or name a component that can show it. The check reads the placement sources
// (the templ output and the Go beside it, and the handler) and lists every identifier
// they use. None may be declared by the practice screens (activity.go and
// activity_templ.go) or be the shared Sheet or AnswerOption, whose Correct and Wrong
// states are feedback. The placement templ may not call them either, and it imports
// nothing from the activity engine.
func TestPlacementViewsShareNoFeedbackComponent(t *testing.T) {
	forbidden := map[string]string{}
	// AnswerOption and its Default and Selected states are allowed (SPEC-012): a placement option needs
	// the c-answer class, and a Selected option is neutral. What placement may not reach is the Correct and
	// Wrong states, the list of all four states, and the feedback sheet.
	for _, name := range []string{"Sheet", "AnswerCorrect", "AnswerWrong", "AnswerStates"} {
		forbidden[name] = "a component with Correct and Wrong states"
	}
	fset := token.NewFileSet()
	for _, name := range []string{"views/activity.go", "views/activity_templ.go"} {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name != "_" {
					forbidden[d.Name.Name] = "declared by the practice screens"
				}
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					switch sp := sp.(type) {
					case *ast.TypeSpec:
						forbidden[sp.Name.Name] = "declared by the practice screens"
					case *ast.ValueSpec:
						for _, n := range sp.Names {
							if n.Name != "_" {
								forbidden[n.Name] = "declared by the practice screens"
							}
						}
					}
				}
			}
		}
	}
	if len(forbidden) < 30 {
		t.Fatalf("only %d feedback names found; the scan is broken", len(forbidden))
	}
	used := 0
	for _, name := range []string{"views/placement.go", "views/placement_templ.go", "placement.go", "placement_view.go"} {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasSuffix(path, "/internal/activity") || strings.Contains(path, "feedback") {
				t.Errorf("%s imports %s", name, path)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				used++
				if why, bad := forbidden[id.Name]; bad {
					t.Errorf("%s uses %s, %s", name, id.Name, why)
				}
			}
			return true
		})
	}
	if used < 500 {
		t.Fatalf("only %d identifiers scanned in the placement sources", used)
	}
	src, err := os.ReadFile("views/placement.templ")
	if err != nil {
		t.Fatal(err)
	}
	for name := range forbidden {
		if regexp.MustCompile(`@` + regexp.QuoteMeta(name) + `\b`).Match(src) {
			t.Errorf("placement.templ calls @%s", name)
		}
	}
	for _, class := range []string{"c-sheet", "c-answer__mark", "fb__", "feedback-right", "feedback-wrong", "act__", "act-grade", `data-state="correct"`, `data-state="wrong"`} {
		if strings.Contains(string(src), class) {
			t.Errorf("placement.templ uses %s", class)
		}
	}
	// The practice screens do not reach back into placement either.
	for _, name := range []string{"views/activity.templ", "views/activity.go"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if regexp.MustCompile(`@placement\w*\(|Placement`).Match(b) && name == "views/activity.templ" {
			t.Errorf("%s names a placement view", name)
		}
	}
}

// ---- styles -----------------------------------------------------------------

func TestActivityCSS(t *testing.T) {
	css := readStatic(t, "static/css/activity.css")
	tokens := readStatic(t, "static/css/tokens.css")
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`--([a-z0-9-]+)\s*:`).FindAllStringSubmatch(tokens, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\(--([a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		if !defined[m[1]] {
			t.Errorf("activity.css uses undefined token --%s", m[1])
		}
	}
	for _, bad := range []string{"transition", "animation", "@keyframes", "!important"} {
		if strings.Contains(cssComment.ReplaceAllString(css, " "), bad) {
			t.Errorf("activity.css contains %s: the motion belongs to motion.css", bad)
		}
	}
	// Green and red only inside feedback rules (also enforced by the shared colour lint).
	for _, r := range parseCSSRules(css) {
		if feedbackVar.MatchString(r.Body) && !inFeedbackRule(r.Selector) {
			t.Errorf("%s uses a feedback colour outside a feedback rule", r.Selector)
		}
	}
	if !strings.Contains(string(mustRead(t, "views/layout.templ")), `Asset("/static/css/activity.css")`) {
		t.Error("the layout does not link activity.css")
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ---- the review handlers, with a real store, mix and engine -------------------

type lexWord struct {
	id    int64
	he    string
	gloss string
	audio string
	card  int64
}

var actWords = []struct{ he, gloss, translit string }{
	{helloHe, "hello", "shalom"},
	{morningHe, "morning", "boker"},
	{eveningHe, "evening", "erev"},
	{waterHe, "water", "mayim"},
	{chairHe, "chair", "kise"},
	{"לֶחֶם", "bread", "lechem"},
	{"שֻׁלְחָן", "table", "shulchan"},
	{"בַּיִת", "house", "bayit"},
}

// openReview opens the app the way production builds it: every skill open, so the mix
// chooses a checked activity wherever the lexicon supports one.
func openReview(t *testing.T, clk clock.Clock, origin string) (*db.Store, http.Handler) {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin:  origin,
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"a@example.com", "b@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, NewOptions(nil, svc, Options{Store: store, Runs: store, SessionSecret: "spec-016-test-secret"})
}

// seedLexicon adds the words. voiced gives each a clip, glossed an English meaning.
// n limits how many of the eight are added.
func seedLexicon(t *testing.T, store *db.Store, now time.Time, n int, voiced, glossed bool) []lexWord {
	t.Helper()
	var out []lexWord
	for i := 0; i < n; i++ {
		w := actWords[i]
		audio := ""
		if voiced {
			audio = strings.Repeat(fmt.Sprintf("%02x", i+1), 32)
			if _, err := store.ExecContext(context.Background(), `
				INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
				VALUES (?, ?, 'voice', 'model', 3, ?)`, audio, w.he, db.FormatTime(now)); err != nil {
				t.Fatal(err)
			}
		}
		gloss := ""
		if glossed {
			gloss = w.gloss
		}
		id := addLexeme(t, store, w.he, gloss, w.translit, "m", audio)
		out = append(out, lexWord{id: id, he: w.he, gloss: w.gloss, audio: audio})
	}
	return out
}

// giveCards gives the learner a due card for each word, due in the order given.
func giveCards(t *testing.T, store *db.Store, userID int64, now time.Time, words []lexWord) {
	t.Helper()
	for i := range words {
		words[i].card = addCard(t, store, userID, words[i].id, fsrs.SkillRead, now.Add(-time.Duration(len(words)-i)*time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	}
}

func actPost(t *testing.T, h http.Handler, l learnerFix, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return actPostAs(t, h, l.cookie, l.csrf, path, form)
}

func actPostAs(t *testing.T, h http.Handler, cookie *http.Cookie, csrf, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("HX-Request", "true")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func answerForm(page string, kv ...string) url.Values {
	form := url.Values{}
	form.Set("card_id", regexp.MustCompile(`data-card="(\d+)"`).FindStringSubmatch(page)[1])
	form.Set("token", regexp.MustCompile(`name="token" value="([^"]+)"`).FindStringSubmatch(page)[1])
	for i := 0; i+1 < len(kv); i += 2 {
		form.Set(kv[i], kv[i+1])
	}
	return form
}

func nextForm(page string) url.Values {
	form := url.Values{}
	form.Set("card_id", regexp.MustCompile(`data-card="(\d+)"`).FindStringSubmatch(page)[1])
	return form
}

var (
	pickRe   = regexp.MustCompile(`(?s)<button[^>]*data-pick="(\d)"[^>]*>.*?<bdi lang="en" dir="ltr">([^<]*)</bdi>`)
	promptRe = regexp.MustCompile(`<audio id="act-prompt" src="/audio/([0-9a-f]{64})\.mp3"`)
	clipRe   = regexp.MustCompile(`id="act-clip-(\d)" src="/audio/([0-9a-f]{64})\.mp3"`)
	rightRe  = regexp.MustCompile(`(?s)<button[^>]*data-index="(\d)"[^>]*data-tile="r"[^>]*>.*?<bdi lang="en" dir="ltr">([^<]*)</bdi>`)
	glossRe  = regexp.MustCompile(`class="act__gloss" lang="en" dir="ltr">([^<]*)<`)
)

func glossByAudio(words []lexWord) map[string]lexWord {
	m := map[string]lexWord{}
	for _, w := range words {
		m[w.audio] = w
	}
	return m
}

func mustAttempts(t *testing.T, store *db.Store, userID, lexemeID int64) []db.Attempt {
	t.Helper()
	got, err := store.ListAttempts(context.Background(), userID, lexemeID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestReviewCheckedActivityFlow walks listen_pick and audio_match through the real
// handlers: the item has no answer, a right answer is checked and shown, a wrong one
// shows the right response and puts the visit back, Continue moves on, and the checked
// visit cannot be graded as a flashcard.
func TestReviewCheckedActivityFlow(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openReview(t, fixedClock(now), "https://learn.example")
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	words := seedLexicon(t, store, now, 8, true, true)
	giveCards(t, store, learner.user.ID, now, words)
	byAudio := glossByAudio(words)
	ctx := context.Background()

	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-activity") != "listen_pick" {
		t.Fatalf("the first visit is not a listen_pick: %s", cardAttrs(page))
	}
	for _, w := range words {
		if strings.Contains(page, w.he) {
			t.Fatalf("the pointed Hebrew of %s is on a listening screen", w.gloss)
		}
	}
	if strings.Contains(page, `data-state="correct"`) || strings.Contains(page, `data-state="wrong"`) || strings.Contains(page, `class="c-sheet"`) {
		t.Fatal("the answer is marked before Check")
	}
	for _, want := range []string{`name="_csrf"`, `name="token"`, `name="card_id"`, `action="/_hx/review/answer"`, "data-no-transition", `data-index="1"`} {
		if !strings.Contains(page, want) {
			t.Fatalf("the question lacks %s", want)
		}
	}
	if strings.Count(page, gold) != 1 {
		t.Fatalf("%d gold actions before the answer", strings.Count(page, gold))
	}
	first := byAudio[promptRe.FindStringSubmatch(page)[1]]
	options := pickRe.FindAllStringSubmatch(page, -1)
	if len(options) != 4 {
		t.Fatalf("%d options, want 4", len(options))
	}
	rightIx := -1
	for _, o := range options {
		if o[2] == first.gloss {
			rightIx, _ = strconv.Atoi(o[1])
		}
	}
	if rightIx < 0 {
		t.Fatalf("the right gloss %q is not among the options %v", first.gloss, options)
	}

	// Before any of that: a flashcard grade, or a reveal, of a checked visit is refused.
	card := nextForm(page)
	if rec := actPost(t, h, learner, "/_hx/review/reveal", url.Values{"card_id": card["card_id"]}); rec.Code != http.StatusBadRequest {
		t.Fatalf("reveal of a checked visit = %d, want 400", rec.Code)
	}
	if rec := actPost(t, h, learner, "/_hx/review/grade", url.Values{"card_id": card["card_id"], "rating": {"3"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("grade of a checked visit = %d, want 400", rec.Code)
	}
	// A post that does not fit is refused and leaves the item pending.
	if rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", "9")); rec.Code != http.StatusBadRequest {
		t.Fatalf("choice 9 = %d, want 400", rec.Code)
	}
	if rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page)); rec.Code != http.StatusBadRequest {
		t.Fatalf("no choice = %d, want 400", rec.Code)
	}
	bad := answerForm(page, "choice", strconv.Itoa(rightIx))
	bad.Set("token", "forged")
	if rec := actPost(t, h, learner, "/_hx/review/answer", bad); rec.Code != http.StatusBadRequest {
		t.Fatalf("a forged token = %d, want 400", rec.Code)
	}
	wrongCard := answerForm(page, "choice", strconv.Itoa(rightIx))
	wrongCard.Set("card_id", strconv.FormatInt(first.card+999, 10))
	if rec := actPost(t, h, learner, "/_hx/review/answer", wrongCard); rec.Code != http.StatusBadRequest {
		t.Fatalf("another card = %d, want 400", rec.Code)
	}
	noCSRF := answerForm(page, "choice", strconv.Itoa(rightIx))
	if rec := actPostAs(t, h, learner.cookie, "", "/_hx/review/answer", noCSRF); rec.Code == http.StatusOK {
		t.Fatalf("an answer without CSRF was accepted")
	}
	if n := len(mustAttempts(t, store, learner.user.ID, first.id)); n != 0 {
		t.Fatalf("refused posts wrote %d attempts", n)
	}

	// A right answer.
	rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", strconv.Itoa(rightIx)))
	if rec.Code != http.StatusOK {
		t.Fatalf("right answer = %d %s", rec.Code, rec.Body.String())
	}
	fb := rec.Body.String()
	sheet := sheetOf(t, fb)
	if attr(t, fb, "data-phase") != "feedback" || strings.Count(fb, `data-state="correct"`) != 1 || strings.Contains(fb, `data-state="wrong"`) {
		t.Fatalf("a right answer is not shown as Correct: %s", fb)
	}
	if !strings.Contains(sheet, first.he) || !strings.Contains(sheet, "feedback-right") || !strings.Contains(sheet, `data-src="/audio/`+first.audio+`.mp3"`) {
		t.Fatalf("the sheet lacks the pointed form or the clip: %s", sheet)
	}
	if strings.Count(fb, gold) != 1 || !strings.Contains(fb, "Continue") || !strings.Contains(fb, `action="/_hx/review/next"`) {
		t.Fatal("Continue is not the one gold action after the answer")
	}
	if !strings.Contains(fb, `data-answered="1"`) || !strings.Contains(fb, `data-correct="1"`) || !strings.Contains(fb, "data-no-transition") {
		t.Fatalf("counts or transition rule missing: %s", cardAttrs(fb))
	}
	attempts := mustAttempts(t, store, learner.user.ID, first.id)
	if len(attempts) != 1 || attempts[0].Activity != "listen_pick" || attempts[0].Source != "review" || !attempts[0].Correct || attempts[0].HintUsed {
		t.Fatalf("attempts = %+v", attempts)
	}
	// A reload shows the same feedback, and the item cannot be answered twice.
	if again := getLearner(t, h, learner, "/review"); attr(t, again, "data-phase") != "feedback" || !strings.Contains(again, first.he) {
		t.Fatal("a reload lost the feedback")
	}
	if rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", strconv.Itoa(rightIx))); rec.Code != http.StatusBadRequest {
		t.Fatalf("a second answer = %d, want 400", rec.Code)
	}
	if n := len(mustAttempts(t, store, learner.user.ID, first.id)); n != 1 {
		t.Fatalf("a second answer wrote an attempt: %d", n)
	}
	// Continue before an answer, or for another card, is refused.
	other := nextForm(fb)
	other.Set("card_id", strconv.FormatInt(first.card+999, 10))
	if rec := actPost(t, h, learner, "/_hx/review/next", other); rec.Code != http.StatusBadRequest {
		t.Fatalf("next for another card = %d, want 400", rec.Code)
	}

	// Continue.
	rec = actPost(t, h, learner, "/_hx/review/next", nextForm(fb))
	if rec.Code != http.StatusOK {
		t.Fatalf("next = %d", rec.Code)
	}
	page = rec.Body.String()
	if attr(t, page, "data-phase") != "ask" || attr(t, page, "data-index") != "2" || attr(t, page, "data-activity") != "listen_pick" {
		t.Fatalf("the second visit: %s", cardAttrs(page))
	}
	second := byAudio[promptRe.FindStringSubmatch(page)[1]]
	if second.id == first.id {
		t.Fatal("the same word came twice in a row")
	}
	total, _ := strconv.Atoi(attr(t, page, "data-total"))

	// A wrong answer shows the right response and puts the visit back once.
	options = pickRe.FindAllStringSubmatch(page, -1)
	wrongIx, pickedGloss := -1, ""
	for _, o := range options {
		if o[2] != second.gloss {
			wrongIx, _ = strconv.Atoi(o[1])
			pickedGloss = o[2]
			break
		}
	}
	rec = actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", strconv.Itoa(wrongIx)))
	if rec.Code != http.StatusOK {
		t.Fatalf("wrong answer = %d", rec.Code)
	}
	fb = rec.Body.String()
	sheet = sheetOf(t, fb)
	if strings.Count(fb, `data-state="wrong"`) != 1 || strings.Count(fb, `data-state="correct"`) != 1 {
		t.Fatalf("a wrong answer does not mark the pick Wrong and the answer Correct: %s", fb)
	}
	if !strings.Contains(sheet, "feedback-wrong") || !strings.Contains(sheet, ">"+second.gloss+"<") || !strings.Contains(sheet, second.he) || !strings.Contains(sheet, `data-src="/audio/`+second.audio+`.mp3"`) {
		t.Fatalf("the sheet lacks the right gloss, the pointed Hebrew, or the clip: %s", sheet)
	}
	if !strings.Contains(sheet, "You picked") || !strings.Contains(sheet, ">"+pickedGloss+"<") || !strings.Contains(sheet, "That&#39;s") {
		t.Fatalf("a confusion is not named: %s", sheet)
	}
	if got, _ := strconv.Atoi(attr(t, fb, "data-total")); got != total+1 {
		t.Fatalf("a miss did not put the visit back: total %d, was %d", got, total)
	}
	if !strings.Contains(fb, `data-answered="2"`) || !strings.Contains(fb, `data-correct="1"`) {
		t.Fatalf("counts after a miss: %s", cardAttrs(fb))
	}
	attempts = mustAttempts(t, store, learner.user.ID, second.id)
	if len(attempts) != 1 || attempts[0].Correct {
		t.Fatalf("attempts = %+v", attempts)
	}

	// The third visit through listening is audio_match (every third visit).
	rec = actPost(t, h, learner, "/_hx/review/next", nextForm(fb))
	page = rec.Body.String()
	if attr(t, page, "data-activity") != "audio_match" {
		t.Fatalf("the third visit is not an audio_match: %s", cardAttrs(page))
	}
	clips := clipRe.FindAllStringSubmatch(page, -1)
	glosses := rightRe.FindAllStringSubmatch(page, -1)
	if len(clips) != 4 || len(glosses) != 4 {
		t.Fatalf("audio_match has %d clips and %d glosses, want 4 and 4", len(clips), len(glosses))
	}
	form := answerForm(page)
	for _, c := range clips {
		want := byAudio[c[2]].gloss
		for _, g := range glosses {
			if g[2] == want {
				form.Set("pair_"+c[1], g[1])
			}
		}
	}
	rec = actPost(t, h, learner, "/_hx/review/answer", form)
	if rec.Code != http.StatusOK {
		t.Fatalf("pairs = %d %s", rec.Code, rec.Body.String())
	}
	fb = rec.Body.String()
	if strings.Count(fb, `data-state="correct"`) != 8 || strings.Contains(fb, `data-state="wrong"`) || !strings.Contains(fb, "Every pair is right") {
		t.Fatalf("all pairs right is not shown: %s", fb)
	}
	if strings.Count(fb, gold) != 1 {
		t.Fatal("audio_match feedback has no single gold action")
	}
	// Pairs that are not a permutation are refused.
	rec = actPost(t, h, learner, "/_hx/review/next", nextForm(fb))
	page = rec.Body.String()
	if attr(t, page, "data-activity") == "audio_match" {
		t.Fatal("two match items in a row")
	}
	_ = ctx
}

// TestAudioMatchRefusesBadPairs and the other bad-input cases of the match item.
func TestAudioMatchAnswerIsValidated(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openReview(t, fixedClock(now), "https://learn.example")
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	words := seedLexicon(t, store, now, 8, true, true)
	giveCards(t, store, learner.user.ID, now, words)
	page := getLearner(t, h, learner, "/review")
	for i := 0; i < 2; i++ {
		right := glossByAudio(words)[promptRe.FindStringSubmatch(page)[1]]
		ix := -1
		for _, o := range pickRe.FindAllStringSubmatch(page, -1) {
			if o[2] == right.gloss {
				ix, _ = strconv.Atoi(o[1])
			}
		}
		rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", strconv.Itoa(ix)))
		page = actPost(t, h, learner, "/_hx/review/next", nextForm(rec.Body.String())).Body.String()
	}
	if attr(t, page, "data-activity") != "audio_match" {
		t.Fatalf("not an audio_match: %s", cardAttrs(page))
	}
	for name, kv := range map[string][]string{
		"repeats a gloss": {"pair_0", "0", "pair_1", "0", "pair_2", "2", "pair_3", "3"},
		"out of range":    {"pair_0", "0", "pair_1", "1", "pair_2", "2", "pair_3", "7"},
		"missing a pair":  {"pair_0", "0", "pair_1", "1", "pair_2", "2"},
		"not a number":    {"pair_0", "a", "pair_1", "1", "pair_2", "2", "pair_3", "3"},
	} {
		if rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, kv...)); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, rec.Code)
		}
	}
	// "I don't know" is an answer: it shows the pairs and writes the attempts.
	rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "dont_know", "1"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Here is the answer") || strings.Contains(rec.Body.String(), `data-state="correct"`) {
		t.Fatalf("dont_know = %d %s", rec.Code, rec.Body.String())
	}
	if strings.Count(sheetOf(t, rec.Body.String()), `class="fb__pair"`) != 4 {
		t.Fatalf("the sheet does not spell out the four pairs: %s", sheetOf(t, rec.Body.String()))
	}
}

// TestReviewTypedActivities drives type_it and dictation through the handlers. The
// lexicon is chosen so only those two fit: words with an English meaning and no clip
// give type_it, words with a clip and no meaning give dictation.
func TestReviewTypedActivities(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for _, kind := range []string{"type_it", "dictation"} {
		t.Run(kind, func(t *testing.T) {
			store, h := openReview(t, fixedClock(now), "https://learn.example")
			learner := addLearner(t, store, "a@example.com", "sub-a", now)
			words := seedLexicon(t, store, now, 2, kind == "dictation", kind == "type_it")
			giveCards(t, store, learner.user.ID, now, words)
			page := getLearner(t, h, learner, "/review")
			if attr(t, page, "data-activity") != kind {
				t.Fatalf("served %s, want %s: %s", attr(t, page, "data-activity"), kind, cardAttrs(page))
			}
			for _, w := range words {
				if strings.Contains(page, w.he) {
					t.Fatalf("the pointed Hebrew of %s is on the question", w.gloss)
				}
			}
			for _, want := range []string{`id="act-answer"`, `name="text"`, `dir="rtl"`, `lang="he"`, `data-hk`, `data-no-transition`, `name="token"`} {
				if !strings.Contains(page, want) {
					t.Fatalf("the typing screen lacks %s", want)
				}
			}
			if strings.Count(page, gold) != 1 {
				t.Fatalf("%d gold actions before the answer", strings.Count(page, gold))
			}
			// Which word is asked.
			var asked lexWord
			if kind == "type_it" {
				g := glossRe.FindStringSubmatch(page)[1]
				for _, w := range words {
					if w.gloss == g {
						asked = w
					}
				}
			} else {
				asked = glossByAudio(words)[promptRe.FindStringSubmatch(page)[1]]
			}
			if asked.id == 0 {
				t.Fatal("cannot tell which word was asked")
			}

			// Empty text is refused; so is text that is too long.
			if rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "text", "  ")); rec.Code != http.StatusBadRequest {
				t.Fatalf("empty text = %d, want 400", rec.Code)
			}
			if rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "text", strings.Repeat("א", activity.MaxResponseBytes))); rec.Code != http.StatusBadRequest {
				t.Fatalf("long text = %d, want 400", rec.Code)
			}

			// Wrong: the sheet shows the pointed word, and the text comes back escaped.
			evil := `<script>alert(1)</script>`
			rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "text", evil))
			if rec.Code != http.StatusOK {
				t.Fatalf("wrong text = %d %s", rec.Code, rec.Body.String())
			}
			fb := rec.Body.String()
			sheet := sheetOf(t, fb)
			if strings.Contains(fb, "<script>alert") || !strings.Contains(sheet, "&lt;script&gt;alert(1)&lt;/script&gt;") {
				t.Fatalf("typed text is not escaped: %s", sheet)
			}
			if !strings.Contains(sheet, asked.he) || !strings.Contains(sheet, "feedback-wrong") || !strings.Contains(sheet, `<u class="fb__bad">`) || !strings.Contains(sheet, "You typed") {
				t.Fatalf("the sheet lacks the pointed word, or the underlined letters: %s", sheet)
			}
			if kind == "dictation" && !strings.Contains(sheet, `data-src="/audio/`+asked.audio+`.mp3"`) {
				t.Fatalf("dictation sheet has no clip: %s", sheet)
			}
			if strings.Contains(fb, `id="act-answer"`) {
				t.Fatal("the input is still on the feedback screen")
			}
			attempts := mustAttempts(t, store, learner.user.ID, asked.id)
			if len(attempts) != 1 || attempts[0].Activity != kind || attempts[0].Correct {
				t.Fatalf("attempts = %+v", attempts)
			}

			// The other word: one letter off is Almost, and the wrong letter is underlined.
			page = actPost(t, h, learner, "/_hx/review/next", nextForm(fb)).Body.String()
			if attr(t, page, "data-activity") != kind {
				t.Fatalf("second visit: %s", cardAttrs(page))
			}
			if kind == "type_it" {
				g := glossRe.FindStringSubmatch(page)[1]
				for _, w := range words {
					if w.gloss == g {
						asked = w
					}
				}
			} else {
				asked = glossByAudio(words)[promptRe.FindStringSubmatch(page)[1]]
			}
			plain := []rune(strings.Map(func(r rune) rune {
				if r >= 0x0591 && r <= 0x05C7 {
					return -1
				}
				return r
			}, asked.he))
			// One letter off: the last letter becomes another one.
			swap := 'ס'
			if plain[len(plain)-1] == swap {
				swap = 'ת'
			}
			plain[len(plain)-1] = swap
			rec = actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "text", string(plain)))
			fb = rec.Body.String()
			if rec.Code != http.StatusOK || !strings.Contains(fb, "Almost") || !strings.Contains(sheetOf(t, fb), `<u class="fb__bad">`+string(swap)+`</u>`) || !strings.Contains(sheetOf(t, fb), "feedback-right") {
				t.Fatalf("near miss = %d %s", rec.Code, sheetOf(t, fb))
			}
			// The queue puts the missed first word back after the second.
			page = actPost(t, h, learner, "/_hx/review/next", nextForm(fb)).Body.String()
			if attr(t, page, "data-retry") != "true" {
				t.Fatalf("the missed word did not come back: %s", cardAttrs(page))
			}
		})
	}
}

// TestReviewDictationHint: playing the prompt clip again before answering is a hint, so
// the post says hinted and the attempt is stored with the hint flag.
func TestReviewDictationHint(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openReview(t, fixedClock(now), "https://learn.example")
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	words := seedLexicon(t, store, now, 2, true, false)
	giveCards(t, store, learner.user.ID, now, words)
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-activity") != "dictation" || !strings.Contains(page, `name="hinted" value="0"`) || !strings.Contains(page, `data-hint="replay"`) || !strings.Contains(page, `data-autoplay`) {
		t.Fatalf("dictation screen: %s", cardAttrs(page))
	}
	asked := glossByAudio(words)[promptRe.FindStringSubmatch(page)[1]]
	plain := strings.Map(func(r rune) rune {
		if r >= 0x0591 && r <= 0x05C7 {
			return -1
		}
		return r
	}, asked.he)
	rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "text", plain, "hinted", "1"))
	if rec.Code != http.StatusOK || !strings.Contains(sheetOf(t, rec.Body.String()), "feedback-right") {
		t.Fatalf("hinted right answer = %d", rec.Code)
	}
	attempts := mustAttempts(t, store, learner.user.ID, asked.id)
	if len(attempts) != 1 || !attempts[0].Correct || !attempts[0].HintUsed {
		t.Fatalf("attempts = %+v", attempts)
	}
}

// TestReviewActivityIsPerLearner: the item belongs to the learner it was built for. Another
// learner cannot answer it with its token, with its card, or with their own card.
func TestReviewActivityIsPerLearner(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openReview(t, fixedClock(now), "https://learn.example")
	a := addLearner(t, store, "a@example.com", "sub-a", now)
	b := addLearner(t, store, "b@example.com", "sub-b", now)
	words := seedLexicon(t, store, now, 8, true, true)
	giveCards(t, store, a.user.ID, now, words)
	bWords := make([]lexWord, len(words))
	copy(bWords, words)
	giveCards(t, store, b.user.ID, now, bWords)
	pageA := getLearner(t, h, a, "/review")
	pageB := getLearner(t, h, b, "/review")
	tokenA := regexp.MustCompile(`name="token" value="([^"]+)"`).FindStringSubmatch(pageA)[1]
	tokenB := regexp.MustCompile(`name="token" value="([^"]+)"`).FindStringSubmatch(pageB)[1]
	if tokenA == tokenB {
		t.Fatal("two learners got one token")
	}
	steal := answerForm(pageB, "choice", "0")
	steal.Set("token", tokenA)
	if rec := actPost(t, h, b, "/_hx/review/answer", steal); rec.Code != http.StatusBadRequest {
		t.Fatalf("B answering with A's token = %d, want 400", rec.Code)
	}
	steal = answerForm(pageA, "choice", "0")
	if rec := actPost(t, h, b, "/_hx/review/answer", steal); rec.Code != http.StatusBadRequest {
		t.Fatalf("B answering A's card = %d, want 400", rec.Code)
	}
	// A's item is still pending and A can answer it.
	if rec := actPost(t, h, a, "/_hx/review/answer", answerForm(pageA, "choice", "0")); rec.Code != http.StatusOK {
		t.Fatalf("A's own answer = %d, want 200", rec.Code)
	}
	for _, w := range words {
		if n := len(mustAttempts(t, store, b.user.ID, w.id)); n != 0 {
			t.Fatalf("B has %d attempts from A's answer", n)
		}
	}
	// A signed-out post is sent to sign-in.
	signedOut := actPostAs(t, h, nil, "", "/_hx/review/answer", answerForm(pageA, "choice", "0"))
	if signedOut.Code == http.StatusOK {
		t.Fatalf("a signed-out answer = %d", signedOut.Code)
	}
}

// TestReviewItemExpiryBuildsAgain: an item that was not answered within its life is
// replaced by a new one, and the learner sees the new question.
func TestReviewItemExpiryBuildsAgain(t *testing.T) {
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := &stepClock{t: start}
	store, h := openReview(t, clk, "https://learn.example")
	learner := addLearner(t, store, "a@example.com", "sub-a", start)
	words := seedLexicon(t, store, start, 8, true, true)
	giveCards(t, store, learner.user.ID, start, words)
	page := getLearner(t, h, learner, "/review")
	token := regexp.MustCompile(`name="token" value="([^"]+)"`).FindStringSubmatch(page)[1]
	clk.Advance(activity.TokenTTL + time.Minute)
	rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", "0"))
	if rec.Code != http.StatusOK {
		t.Fatalf("answer after expiry = %d", rec.Code)
	}
	again := rec.Body.String()
	if attr(t, again, "data-phase") != "ask" || regexp.MustCompile(`name="token" value="([^"]+)"`).FindStringSubmatch(again)[1] == token {
		t.Fatalf("the expired item was not replaced: %s", cardAttrs(again))
	}
	if attr(t, again, "data-answered") != "0" {
		t.Fatal("an expired item counted as answered")
	}
}

// TestReviewNewWordIsExposed: a new card whose word was never shown records an exposure
// when it is served, before the first check.
func TestReviewNewWordIsExposed(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openReview(t, fixedClock(now), "https://learn.example")
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	words := seedLexicon(t, store, now, 8, true, true)
	// One new card, with no review, for the first word.
	addCard(t, store, learner.user.ID, words[0].id, fsrs.SkillRead, now.Add(-time.Hour), 1, 0, time.Time{})
	model := learner0(store, now)
	before, err := model.Word(context.Background(), learner.user.ID, words[0].id)
	if err != nil {
		t.Fatal(err)
	}
	if before.Exposures != 0 {
		t.Fatalf("exposures before = %d", before.Exposures)
	}
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-activity") == "" {
		t.Fatalf("not a checked activity: %s", cardAttrs(page))
	}
	after, err := model.Word(context.Background(), learner.user.ID, words[0].id)
	if err != nil {
		t.Fatal(err)
	}
	if after.Exposures != 1 {
		t.Fatalf("exposures after serving the new word = %d, want 1", after.Exposures)
	}
	// Serving the page again does not record it twice.
	getLearner(t, h, learner, "/review")
	if again, _ := model.Word(context.Background(), learner.user.ID, words[0].id); again.Exposures != 1 {
		t.Fatalf("a second GET recorded the exposure again: %d", again.Exposures)
	}
}

func learner0(store *db.Store, now time.Time) *learner.Model {
	return learner.New(store, fixedClock(now))
}
