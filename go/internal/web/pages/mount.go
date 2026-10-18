package pages

import (
	"net/http"
	"os"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/path"
)

// DevEnv is the temporary switch for the unauthenticated guides mount.
// Sessions are not in this server yet. Leave it unset in any shared deploy.
const DevEnv = "HEBREW_DEV_WEB"

// Mount registers the guides pages on mux when HEBREW_DEV_WEB=1.
//
// cmd/hebrew calls this once, after the read-only database is open:
//
//	pages.Mount(mux, db)
//
// db is the handle from internal/db. The call is a no-op unless the flag is
// set, so the health check stays the only public route until then.
func Mount(mux *http.ServeMux, db Queryer) {
	if os.Getenv(DevEnv) != "1" {
		return
	}
	NewHandler(NewSQLSource(db)).Register(mux)
}

// MountLearning registers Today, the path map, and the gate acknowledgement
// when HEBREW_DEV_WEB=1. The reader and the gate are fixtures until
// go/internal/learner/path lands. The gate returns ErrNotPersisted, and the
// page still lets the link through.
func MountLearning(mux *http.ServeMux) {
	if os.Getenv(DevEnv) != "1" {
		return
	}
	demo := path.Demo()
	NewLearning(
		path.Mem{Snap: demo, Due: demo.Due},
		&path.MemGate{Err: path.ErrNotPersisted},
		path.DefaultConfig(),
		path.DemoPlanID,
	).Register(mux)
}
