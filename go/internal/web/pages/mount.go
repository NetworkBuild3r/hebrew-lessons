package pages

import (
	"net/http"
	"os"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/library"
)

// DevEnv is the temporary switch for the unauthenticated guides mount.
// Sessions are not in this server yet. Leave it unset in any shared deploy.
const DevEnv = "HEBREW_DEV_WEB"

// Mount registers the guides and words pages on mux when HEBREW_DEV_WEB=1.
//
// cmd/hebrew calls this once, after the read-only database is open:
//
//	pages.Mount(mux, db)
//
// db is the handle from internal/db. Guides read it. Words do not: there is
// no library read in this module yet, so /words/ uses library.Demo until
// learner/library.List is wired here. The call is a no-op unless the flag is
// set, so the health check stays the only public route until then.
func Mount(mux *http.ServeMux, db Queryer) {
	if os.Getenv(DevEnv) != "1" {
		return
	}
	NewHandler(NewSQLSource(db)).Register(mux)
	NewWords(library.Demo{}).Register(mux)
}
