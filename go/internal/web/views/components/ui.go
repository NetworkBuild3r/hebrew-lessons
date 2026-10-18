package components

import "context"

type uiKey struct{}

// UI is the interface language and theme for one Go page.
// Lang is en or he. Theme is light, dark, or auto.
type UI struct {
	Lang  string
	Theme string
	CSRF  string
	Next  string
}

// WithUI stores the page's interface choices on ctx for the templates.
func WithUI(ctx context.Context, ui UI) context.Context {
	return context.WithValue(ctx, uiKey{}, normalizeUI(ui))
}

// UIFrom reads the choices WithUI stored. A missing context is English and auto.
func UIFrom(ctx context.Context) UI {
	ui, _ := ctx.Value(uiKey{}).(UI)
	return normalizeUI(ui)
}

func normalizeUI(ui UI) UI {
	if ui.Lang != "he" {
		ui.Lang = "en"
	}
	switch ui.Theme {
	case "light", "dark", "auto":
	default:
		ui.Theme = "auto"
	}
	if ui.Next == "" {
		ui.Next = "/guides/"
	}
	return ui
}

// UIDir is ltr for English and rtl for Hebrew.
func UIDir(ctx context.Context) string {
	if UIFrom(ctx).Lang == "he" {
		return "rtl"
	}
	return "ltr"
}

// ResolvedTheme is the concrete light or dark value for data-theme.
// Auto starts as light; the pre-paint script follows the OS before paint.
func ResolvedTheme(choice string) string {
	if choice == "dark" {
		return "dark"
	}
	return "light"
}

// ThemeColor is the browser chrome color for a concrete theme.
func ThemeColor(choice string) string {
	if choice == "dark" {
		return "#0E121B"
	}
	return "#F7F5F0"
}

// Pressed is an aria-pressed value.
func Pressed(on bool) string {
	if on {
		return "true"
	}
	return "false"
}

// LangGroupLabel is the accessible name of the language control.
func LangGroupLabel(lang string) string {
	if lang == "he" {
		return "שפת הממשק"
	}
	return "Interface language"
}

// ThemeGroupLabel is the accessible name of the theme control.
func ThemeGroupLabel(lang string) string {
	if lang == "he" {
		return "ערכת נושא"
	}
	return "Theme"
}
