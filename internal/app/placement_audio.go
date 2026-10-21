package app

import (
	"context"
	"log/slog"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// logMissingPlacementAudio starts a background count of placement-manifest
// clips that have no audio_assets row. NewApp does not wait for it. The line
// is a count: it never includes clip text, a letter id, or a secret.
// App.Close cancels ctx, waits for this goroutine, then closes the store.
// A cancel produces no line.
func logMissingPlacementAudio(ctx context.Context, wg *sync.WaitGroup, store *db.Store) {
	logger := slog.Default()
	wg.Add(1)
	go func() {
		defer wg.Done()
		reportMissingPlacementAudio(ctx, store, logger)
	}()
}

// reportMissingPlacementAudio writes one line. missing is how many manifest
// clips Start would refuse for lack of a stored clip, in any voice.
func reportMissingPlacementAudio(ctx context.Context, store *db.Store, logger *slog.Logger) {
	n, err := missingManifestClips(ctx, store)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		logger.Warn("placement audio", "err", err)
		return
	}
	if n > 0 {
		logger.Warn("placement audio", "missing", n)
		return
	}
	logger.Info("placement audio", "missing", n)
}

// missingManifestClips is the number of embedded manifest clips whose text
// is not an audio_assets.text_nfc key. One query, then the same comparison
// the manifest tools use. The key is the stored text, which is what
// Engine.Start looks up after normalizing the letter syllable.
func missingManifestClips(ctx context.Context, store *db.Store) (int, error) {
	manifest, err := placement.EmbeddedManifest()
	if err != nil {
		return 0, err
	}
	conn, err := store.DB()
	if err != nil {
		return 0, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT DISTINCT text_nfc FROM audio_assets`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	have := make(map[string]struct{})
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return 0, err
		}
		have[text] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return len(placement.MissingAudio(manifest.Clips, have)), nil
}
