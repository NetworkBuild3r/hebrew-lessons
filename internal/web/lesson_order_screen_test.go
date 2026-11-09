package web

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// INIT-029/SPEC-004 AC1–AC2, AC4–AC5: ask HTML has no chip translit/English,
// the answer row is rtl, keyboard markup is present, and a wrong reveal shows
// the right order. CSS flex-wrap is checked in views.TestOrderCSSWrapsChips.
func TestOrderAskScreenNoChipEnglishOrTranslit(t *testing.T) {
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	body := getAuthed(t, f.h, f.cookie, f.path)

	if !strings.Contains(body, `data-order-row`) {
		t.Fatalf("ask missing order row:\n%s", body)
	}
	if !strings.Contains(body, `dir="rtl"`) {
		t.Fatalf("ask missing rtl:\n%s", body)
	}
	row := regexp.MustCompile(`(?s)<div[^>]*data-order-row[^>]*>`).FindString(body)
	if row == "" || !strings.Contains(row, `dir="rtl"`) {
		t.Fatalf("order row is not rtl: %q", row)
	}
	if !strings.Contains(body, `aria-label="Your sentence"`) {
		t.Fatalf("row missing accessible name:\n%s", body)
	}
	if !strings.Contains(body, `role="listbox"`) {
		t.Fatalf("row missing listbox:\n%s", body)
	}
	if !strings.Contains(body, "/static/js/order.js") {
		t.Fatalf("ask missing order.js:\n%s", body)
	}
	if !strings.Contains(body, `data-order-chip`) {
		t.Fatalf("ask missing chips:\n%s", body)
	}

	bank := isolate(t, body, `data-order-bank`, `data-order-fields`)
	for _, banned := range []string{"shalom", "boker", "toda", "hello", "morning", "thanks", "gchip"} {
		if strings.Contains(strings.ToLower(bank), banned) {
			t.Fatalf("chip bank has %q:\n%s", banned, bank)
		}
	}
	if !strings.Contains(body, "hello morning thanks") {
		t.Fatalf("ask missing English cue:\n%s", body)
	}
	if !strings.Contains(body, "Put the words in order.") {
		t.Fatalf("ask missing prompt:\n%s", body)
	}
	// Noscript still posts order= positions.
	if !strings.Contains(body, `<select name="order"`) {
		t.Fatalf("noscript missing order selects:\n%s", body)
	}
}

// INIT-029/SPEC-004: wrong answer reveal shows the learner's row and the right order.
func TestOrderWrongRevealShowsRightOrder(t *testing.T) {
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	run, ask := f.openAsk(t)
	pos := wrongPermutation(ask.OrderChips)
	rec := postOrder(t, f, strconv.FormatInt(run.ID, 10), pos)
	if rec.Code != 200 {
		t.Fatalf("POST = %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "order-answer--wrong") {
		t.Fatalf("reveal missing wrong learner row:\n%s", body)
	}
	if !strings.Contains(body, "Right order") {
		t.Fatalf("reveal missing right-order label:\n%s", body)
	}
	for _, tok := range units.Tokenize(orderLine()) {
		if !strings.Contains(body, tok) {
			t.Fatalf("reveal missing right token %q:\n%s", tok, body)
		}
	}
	if !strings.Contains(body, "shalom") && !strings.Contains(body, "boker") {
		t.Fatalf("reveal missing readings:\n%s", body)
	}
	if !strings.Contains(body, "hello morning thanks") {
		t.Fatalf("reveal missing English:\n%s", body)
	}
	if !strings.Contains(body, "/audio/") {
		t.Fatalf("reveal missing clip:\n%s", body)
	}
	if !strings.Contains(body, `value="continue"`) {
		t.Fatalf("reveal missing Continue:\n%s", body)
	}

	// Reload still carries the learner's answer. INIT-029/SPEC-004.
	run2, err := f.store.PlayerRunFor(context.Background(), f.userID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	p := player{store: f.store}
	screen, err := p.screenFor(context.Background(), views.NewPage("en", "brian-csrf", "lesson"), f.lesson, run2, lessonQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(screen.Activity.OrderAnswer) != 3 {
		t.Fatalf("reload OrderAnswer = %v", screen.Activity.OrderAnswer)
	}
}

// INIT-029/SPEC-004 AC4: keyboard markup — roles, accessible name, and the key handler script.
func TestOrderKeyboardMarkup(t *testing.T) {
	f := setupOrderLesson(t, nil)
	body := getAuthed(t, f.h, f.cookie, f.path)
	for _, want := range []string{
		`role="listbox"`,
		`aria-label="Your sentence"`,
		`data-order-chip`,
		`/static/js/order.js`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("keyboard markup missing %q:\n%s", want, body)
		}
	}
	js := readStatic(t, "static/js/order.js")
	for _, want := range []string{"ArrowRight", "ArrowLeft", "Enter", "keydown"} {
		if !strings.Contains(js, want) {
			t.Fatalf("order.js missing key handling for %q", want)
		}
	}
}

func isolate(t *testing.T, body, startMark, endMark string) string {
	t.Helper()
	start := strings.Index(body, startMark)
	end := strings.Index(body, endMark)
	if start < 0 || end < start {
		t.Fatalf("cannot isolate %q…%q", startMark, endMark)
	}
	return body[start:end]
}
