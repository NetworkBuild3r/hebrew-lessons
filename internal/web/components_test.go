package web

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// -update rewrites the golden files: go test ./internal/web -run Component -update
// UPDATE_GOLDEN=1 does the same, for a shell that cannot pass flags.
var updateGolden = flag.Bool("update", false, "rewrite internal/web/testdata/components/*.golden.html")

func updating() bool { return *updateGolden || os.Getenv("UPDATE_GOLDEN") == "1" }

func renderComponent(t *testing.T, c templ.Component, children string) string {
	t.Helper()
	ctx := context.Background()
	if children != "" {
		ctx = templ.WithChildren(ctx, templ.Raw(children))
	}
	var buf bytes.Buffer
	if err := c.Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

type componentCase struct {
	name     string
	c        templ.Component
	children string
}

// componentCases is every component in every variant. AnswerOption has one
// case per Figma state, Button one per kind.
func componentCases() []componentCase {
	cases := []componentCase{
		{"button-primary", views.Button(views.ButtonPrimary), "Check"},
		{"button-quiet", views.Button(views.ButtonQuiet), "Skip"},
		{"button-unknown-kind-is-quiet", views.Button(views.ButtonKind("loud")), "Skip"},
		{"button-with-attrs", views.ButtonWith(views.ButtonPrimary, templ.Attributes{"type": "submit", "name": "rating", "value": "3"}), "Good"},
		{"progressbar-start", views.ProgressBar(0, 8, "Progress"), ""},
		{"progressbar-half", views.ProgressBar(4, 8, "Progress"), ""},
		{"progressbar-clamped", views.ProgressBar(12, 8, "Progress"), ""},
		{"audioorb", views.AudioOrb("Play the word", templ.Attributes{"data-src": "/audio/1.mp3"}), ""},
		{"chip", views.ChipLabel(), "m."},
		{"card", views.Card(), "<p>Body</p>"},
		{"sheet", views.Sheet("Feedback"), "<p>That's right.</p>"},
	}
	for _, s := range views.AnswerStates {
		cases = append(cases, componentCase{"answer-" + string(s), views.AnswerOption(s, "1"), `<bdi lang="he">×©Ö¸××œ×•Ö¹×</bdi>`})
	}
	cases = append(cases, componentCase{"answer-unknown-state-is-default", views.AnswerOption(views.AnswerState("maybe"), "2"), "Hello"})
	return cases
}

// TestComponentGolden is ac-3: every component renders to the committed HTML.
func TestComponentGolden(t *testing.T) {
	dir := filepath.Join("testdata", "components")
	want := map[string]bool{}
	for _, c := range componentCases() {
		c := c
		file := filepath.Join(dir, c.name+".golden.html")
		want[filepath.Base(file)] = true
		t.Run(c.name, func(t *testing.T) {
			got := renderComponent(t, c.c, c.children) + "\n"
			if strings.Contains(got, "style=") {
				t.Errorf("inline style in %s; style-src is 'self'", c.name)
			}
			if updating() {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("%v\nrun: go test ./internal/web -run TestComponentGolden -update", err)
			}
			if exp := strings.ReplaceAll(string(b), "\r\n", "\n"); got != exp {
				t.Errorf("%s differs from its golden file.\n got: %s\nwant: %s\nrewrite with: go test ./internal/web -run TestComponentGolden -update", c.name, got, exp)
			}
		})
	}
	if updating() {
		return
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.golden.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !want[filepath.Base(f)] {
			t.Errorf("%s has no test case; delete it", f)
		}
	}
}

// TestComponentAnswerOptionStates checks AnswerOption has exactly the four
// Figma states, and that each one is told apart by more than colour.
func TestComponentAnswerOptionStates(t *testing.T) {
	var names []string
	for _, s := range views.AnswerStates {
		names = append(names, string(s))
	}
	sort.Strings(names)
	if got := strings.Join(names, ","); got != "correct,default,selected,wrong" {
		t.Fatalf("states = %s, want exactly correct, default, selected, wrong", got)
	}
	goldens, err := filepath.Glob(filepath.Join("testdata", "components", "answer-*.golden.html"))
	if err != nil {
		t.Fatal(err)
	}
	var fixed int
	for _, g := range goldens {
		if !strings.Contains(g, "unknown") {
			fixed++
		}
	}
	if fixed != 4 {
		t.Errorf("%d answer goldens for the four states: %v", fixed, goldens)
	}
	for _, s := range views.AnswerStates {
		html := renderComponent(t, views.AnswerOption(s, "A"), "Label")
		for _, part := range []string{`data-state="` + string(s) + `"`, `class="c-answer__key"`, `>A<`, `class="c-answer__label"`, "Label"} {
			if !strings.Contains(html, part) {
				t.Errorf("%s: missing %s in %s", s, part, html)
			}
		}
		marked := strings.Contains(html, "c-answer__mark")
		if marked != (s == views.AnswerCorrect || s == views.AnswerWrong) {
			t.Errorf("%s: mark present = %v", s, marked)
		}
		if pressed := strings.Contains(html, `aria-pressed="true"`); pressed != (s == views.AnswerSelected) {
			t.Errorf("%s: aria-pressed = %v", s, pressed)
		}
	}
	if html := renderComponent(t, views.AnswerOption("bogus", "A"), "x"); !strings.Contains(html, `data-state="default"`) {
		t.Errorf("an unknown state must render as default: %s", html)
	}
	for _, k := range views.ButtonKinds {
		html := renderComponent(t, views.Button(k), "x")
		if !strings.Contains(html, `data-kind="`+string(k)+`"`) || !strings.Contains(html, `type="button"`) {
			t.Errorf("button %s: %s", k, html)
		}
	}
	if html := renderComponent(t, views.Button("bogus"), "x"); !strings.Contains(html, `data-kind="quiet"`) {
		t.Errorf("an unknown kind must render quiet, never a second primary: %s", html)
	}
}
