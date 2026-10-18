package pages

import (
	_ "embed"
	"net/http"
)

// htmx 2.0.4, vendored from https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
// Served by this process only. It is not a file under shared/.

//go:embed vendor/htmx-2.0.4.min.js
var htmxJS []byte

func htmx(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(htmxJS)
}
