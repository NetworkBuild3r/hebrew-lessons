package views

import (
	"encoding/json"

	"github.com/a-h/templ"
)

// AppName is the pointed Hebrew wordmark. English sits beside mixed runs
// in its own bdi, not inside this string.
const AppName = "דֶּרֶךְ עִבְרִית"

// Page is the data the layout shell needs. CSRF is the token auth.CSRFToken
// returned for this request.
type Page struct {
	Title   string
	CSRF    string
	Current string
}

// HXHeaders is the JSON object htmx sends as request headers from <body hx-headers>.
func HXHeaders(token string) string {
	b, err := json.Marshal(map[string]string{"X-CSRF-Token": token})
	if err != nil {
		return "{}"
	}
	return string(b)
}

// IconHref points at one symbol in the vendored sprite. Unknown names fall
// back to help so a typo cannot request streak or xp.
func IconHref(name string) templ.SafeURL {
	switch name {
	case "today", "review", "progress", "theme", "next", "play", "sound", "help", "check":
		return templ.SafeURL("/static/icons/sprite.svg#icon-" + name)
	default:
		return templ.SafeURL("/static/icons/sprite.svg#icon-help")
	}
}

func iconClass(directional bool) string {
	if directional {
		return "icon icon-directional"
	}
	return "icon"
}

// themeHead is the only inline script: it applies a stored light/dark choice
// before paint, and the toggle writes that choice back to localStorage.
func themeHead() templ.Component {
	return templ.Raw(`<script>(function(){var k="hebrew-theme";try{var t=localStorage.getItem(k);if(t==="light"||t==="dark")document.documentElement.setAttribute("data-theme",t);}catch(e){}function dark(){var t=document.documentElement.getAttribute("data-theme");return t==="dark"||(t!=="light"&&matchMedia("(prefers-color-scheme: dark)").matches);}function paint(){var b=document.getElementById("theme-toggle");if(b)b.setAttribute("aria-pressed",dark()?"true":"false");}document.addEventListener("click",function(ev){var node=ev.target&&ev.target.closest?ev.target:ev.target&&ev.target.parentElement;var b=node&&node.closest&&node.closest("#theme-toggle");if(!b)return;var next=dark()?"light":"dark";document.documentElement.setAttribute("data-theme",next);try{localStorage.setItem(k,next);}catch(err){}paint();});document.addEventListener("DOMContentLoaded",paint);})();</script>`)
}
