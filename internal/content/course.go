package content

import (
	"context"
	"log/slog"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
)

// EnsureCourse adds ready course words that the lexicon does not already have.
// A stored lexeme is left as it is. An empty course list writes nothing.
func EnsureCourse(ctx context.Context, store *db.Store) error {
	if store == nil {
		return nil
	}
	ready, err := speech.CourseReady()
	if err != nil {
		return err
	}
	rows := make([]db.CourseRow, 0, len(ready))
	for _, w := range ready {
		rows = append(rows, db.CourseRow{
			Order: w.Order, Rank: w.Rank, Lemma: w.Lemma, Pointed: w.Pointed,
			Translit: w.Translit, Gloss: w.Gloss, POS: w.POS, Gender: w.Gender,
			Root: w.Root, Binyan: w.Binyan,
		})
	}
	sync, err := store.SyncCourse(ctx, rows)
	if err != nil {
		return err
	}
	if sync.Added+sync.Linked+sync.Moved > 0 {
		if err := store.AuditContent(ctx, "course", "sync_course", "shared"); err != nil {
			return err
		}
	}
	slog.Info("course", "added", sync.Added, "linked", sync.Linked, "skipped", sync.Skipped, "moved", sync.Moved)
	return nil
}
