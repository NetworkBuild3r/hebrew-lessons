package views

import "github.com/a-h/templ"

// Icons, brand mark, and illustrations (INIT-015/SPEC-010).
//
// Every icon is one <symbol> in /static/icons/sprite.svg: 24px grid, 2px
// round stroke, currentColor. Names here are the symbol ids without "icon-".

const (
	spritePath       = "/static/icons/sprite.svg"
	illustrationPath = "/static/illustrations/"
	brandGlyphHref   = "/static/brand/glyph.svg#glyph"

	// PlayWord names an icon-only link or button that plays a word.
	PlayWord = "Play the word"
)

// IconNames is every symbol in the sprite. The first group is the set the
// design names; the second group is the older ids the views still use, drawn
// with the same glyphs. The test suite keeps this list and the sprite equal.
var IconNames = []string{
	"home", "book", "layers", "chart", "user",
	"speaker", "replay", "slower", "play",
	"x", "check", "arrow-right", "arrow-left",
	"keyboard", "flag", "lamp", "spark",
	"theme", "help", "tip",
	// Older ids, kept so existing views and links keep working.
	"today", "lessons", "review", "progress", "sound", "next", "close",
}

var iconSet = func() map[string]bool {
	m := make(map[string]bool, len(IconNames))
	for _, n := range IconNames {
		m[n] = true
	}
	return m
}()

// HasIcon reports whether name is a symbol in the sprite.
func HasIcon(name string) bool { return iconSet[name] }

// iconRef is the sprite URL for name. An unknown name falls back to help, so
// a typo draws a visible glyph and can never request a missing symbol.
func iconRef(name string) string {
	if !iconSet[name] {
		name = "help"
	}
	return spritePath + "#icon-" + name
}

// Illustrations is every file in /static/illustrations/ that is a themed
// illustration: one <symbol id="art"> drawn with var(--token) fills.
var Illustrations = []string{
	"placement-intro",
	"results",
	"empty-review",
	"empty-words",
	"empty-progress",
	"band-up",
	"offline",
}

var illustrationSet = func() map[string]bool {
	m := make(map[string]bool, len(Illustrations))
	for _, n := range Illustrations {
		m[n] = true
	}
	return m
}()

// illustrationRef is the URL of one illustration symbol. An unknown name
// returns "" and the template draws nothing.
func illustrationRef(name string) string {
	if !illustrationSet[name] {
		return ""
	}
	return illustrationPath + name + ".svg#art"
}

// Brand head: favicon, manifest, and the icons stylesheet. One templ so
// Layout and SignIn share it.
const (
	faviconHref  = "/static/brand/mark.svg"
	manifestHref = "/static/brand/manifest.webmanifest"
	iconsCSSHref = "/static/css/icons.css"
)

func safeURL(s string) templ.SafeURL { return templ.SafeURL(s) }
