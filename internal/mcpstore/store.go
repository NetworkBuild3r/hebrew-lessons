package mcpstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	sdauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// defaultActor matches the static bearer actor mcp writes when the
// principal has no name.
const defaultActor = "mcp"

// defaultStruggleTop matches get_context when topN is omitted.
const defaultStruggleTop = 30

// defaultSearchLimit matches lookup when limit is omitted.
const defaultSearchLimit = 8

// nearMissDistance is how far an unpointed spelling can be and still
// be offered as a suggestion. One or two edits is a typo. Farther
// spellings stay out of the list.
const nearMissDistance = 2

// Store implements the authoring interfaces on *db.Store.
type Store struct {
	db     *db.Store
	voices audio.Voices
}

// New wraps store. A nil store fails on the first call.
func New(store *db.Store) *Store {
	return &Store{db: store}
}

// WithVoices sets the speaker table that validate_unit uses to find a dialogue
// line's clip in its speaker's voice (INIT-015/SPEC-026). It returns s. Without
// it a speaker b line has no clip, and a unit with a dialogue fails audio.missing.
func (s *Store) WithVoices(v audio.Voices) *Store {
	s.voices = v
	return s
}

var (
	_ mcp.CourseReader    = (*Store)(nil)
	_ mcp.LexiconReader   = (*Store)(nil)
	_ mcp.UnitWriter      = (*Store)(nil)
	_ mcp.LexiconWriter   = (*Store)(nil)
	_ units.Lookup        = (*Store)(nil)
	_ units.SpeakerLookup = (*Store)(nil)
)

func (s *Store) ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return errors.New("mcpstore: database is not open")
	}
	return nil
}

func (s *Store) conn(ctx context.Context) (*sql.DB, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.DB()
}

func (s *Store) now() time.Time {
	if s != nil && s.db != nil && s.db.Clock() != nil {
		return s.db.Clock().Now()
	}
	return time.Now()
}

// actorFrom reads the principal the bearer middleware stored on ctx.
// The static token's actor is "mcp". OAuth stores the user id string.
func actorFrom(ctx context.Context) string {
	info := sdauth.TokenInfoFromContext(ctx)
	if info == nil || info.UserID == "" {
		return defaultActor
	}
	return info.UserID
}
