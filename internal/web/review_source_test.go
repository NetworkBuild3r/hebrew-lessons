package web

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestReviewShowsTheSource(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"course", "Common words"},
		{"tutor", "From your tutor"},
		{"other", "From an earlier lesson"},
	} {
		it := activity.Item{
			Kind: activity.ReadPick, Token: "t",
			Prompt:  activity.Prompt{Hebrew: "שָׁלוֹם"},
			Options: []string{"hello", "water", "bread", "house"},
		}
		a := views.ActivityView{
			Kind: views.ActAsk, Phase: views.ActAsk, CSRF: "c", Lang: "en", CardID: 7, Token: it.Token,
			Lead: "Pick the meaning.", Hebrew: it.Prompt.Hebrew, Source: c.source,
			Options: []views.OptionView{{Text: "hello"}, {Text: "water"}, {Text: "bread"}, {Text: "house"}},
		}
		html := renderComponent(t, views.ActivityFrame(a), "")
		if !strings.Contains(html, `data-source="`+c.source+`"`) || !strings.Contains(html, c.want) {
			t.Fatalf("%s card missing its source line: %s", c.source, html)
		}
	}
}

func TestReviewSourceLineStaysOnTheFront(t *testing.T) {
	a := views.ActivityView{
		Kind: views.ActFeedback, CSRF: "c", Lang: "en", CardID: 7, Token: "t",
		Lead: "Pick the meaning.", Hebrew: "שָׁלוֹם", Source: "course",
		Feedback: &views.FeedbackView{Correct: true, Answer: "hello"},
		Options:  []views.OptionView{{Text: "hello", State: views.AnswerCorrect}},
	}
	html := renderComponent(t, views.ActivityFrame(a), "")
	if strings.Contains(html, `data-source=`) {
		t.Fatalf("the feedback screen still names the source: %s", html)
	}
}

func TestLetterDrillHasNoSourceLine(t *testing.T) {
	card := views.ReviewCard{
		Lang: "en", CSRF: "t", CardID: 3, Kind: "letter", Index: 1, Total: 5,
		Pointed: "ג", Pick: true, PickLetters: []string{"ג", "נ"},
		Sides: []views.DrillSide{{Hebrew: "ג", Name: "גימל", Sound: "ga"}},
	}
	html := renderComponent(t, views.ReviewBody(card), "")
	if strings.Contains(html, `data-source=`) {
		t.Fatalf("a letter drill names a source: %s", html)
	}
}
