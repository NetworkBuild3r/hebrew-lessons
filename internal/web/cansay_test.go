package web

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"github.com/a-h/templ"
)

func renderView(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestCanSaySectionRendersTheList(t *testing.T) {
	v := views.ProgressView{
		Page: views.Page{UI: views.UIFor("en", views.AddressMasculine)},
		CanSay: &views.CanSayPanel{
			TotalText: "2 things",
			AddedText: "1 this week",
			Items: []views.CanSayWord{
				{He: "שָׁמַע", Gloss: "he heard", Audio: "/audio/abc.mp3"},
				{He: "בֹּקֶר טוֹב", Gloss: "good morning"},
			},
			Next: []views.CanSayWord{{He: "כָּתַב", Gloss: "he wrote"}},
		},
	}
	body := renderView(t, views.Progress(v))
	for _, want := range []string{
		`<bdi class="cansay__he" lang="he" dir="rtl">שָׁמַע</bdi>`,
		`<bdi class="cansay__he" lang="he" dir="rtl">בֹּקֶר טוֹב</bdi>`,
		"2 things", "1 this week", "Almost there", "he wrote",
		`data-src="/audio/abc.mp3"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestCanSaySectionEmpty(t *testing.T) {
	v := views.ProgressView{
		Page:   views.Page{UI: views.UIFor("en", views.AddressMasculine)},
		CanSay: &views.CanSayPanel{Empty: true},
	}
	body := renderView(t, views.Progress(v))
	if !strings.Contains(body, "gather here as you learn them") {
		t.Fatal("empty state missing")
	}
	if strings.Contains(body, "cansay__total") {
		t.Fatal("empty state showed a count")
	}
}

func TestTodayCanSayLine(t *testing.T) {
	with := views.Home{UI: views.UIFor("en", views.AddressMasculine), Today: views.TodayView{CanSayLine: "You can now say 2 things · 1 this week"}}
	body := renderView(t, views.Today(with))
	if !strings.Contains(body, "You can now say 2 things · 1 this week") || !strings.Contains(body, `href="/progress"`) {
		t.Fatal("today line missing")
	}
	without := views.Home{UI: views.UIFor("en", views.AddressMasculine)}
	if strings.Contains(renderView(t, views.Today(without)), "You can now say") {
		t.Fatal("today showed a count with nothing known")
	}
}
