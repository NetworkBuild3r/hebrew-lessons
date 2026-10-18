package pages

import (
	"log"
	"net/http"
	"os"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/config"
	lpath "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/path"
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
// when HEBREW_DEV_WEB=1. path.New is the stand-in for learner/path.New.
// After PR #37 merges, import that package and pass the read-only *sql.DB.
// Unavailable returns ErrNotPersisted, and the page still lets the link through.
func MountLearning(mux *http.ServeMux) {
	if os.Getenv(DevEnv) != "1" {
		return
	}
	cfg, err := config.Load()
	if err != nil {
		log.Printf("learning path: config: %v", err)
		return
	}
	NewLearning(lpath.New(nil, cfg), lpath.Unavailable{}, cfg, lpath.DemoPlanID).Register(mux)
}
