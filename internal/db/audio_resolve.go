package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// audioResolveChunk bounds one IN list. SQLite's variable limit is far
// above this. audio_assets.text_nfc has no index, so each chunk is a scan
// of the rows whose text is in the list, not one query per word.
const audioResolveChunk = 80

// LexemeAudio is the pointed text and the lexemes.audio_id of one word.
// Text may still contain format characters or be NFD. AudioID may be empty.
type LexemeAudio struct {
	Text    string
	AudioID string
}

// ResolveLexemeAudio returns the clip id to play for one lexeme.
// units.Normalize(Text) is matched the way AudioByText matches it: the exact
// pointed NFC string, then eleven_v4, eleven_v3, legacy, any other model,
// and within a model the newest created_at, then the lowest id. A different
// pointing is a different clip. When no row matches, AudioID is returned if
// it is set. ok is false when neither source has a clip.
func (s *Store) ResolveLexemeAudio(ctx context.Context, lexeme LexemeAudio) (assetID string, ok bool, err error) {
	ids, err := s.ResolveLexemeAudioBatch(ctx, []LexemeAudio{lexeme})
	if err != nil {
		return "", false, err
	}
	if ids[0] == "" {
		return "", false, nil
	}
	return ids[0], true, nil
}

// ResolveLexemeAudioBatch is ResolveLexemeAudio for many lexemes.
// ids[i] is the clip for lexemes[i], or empty when that word has none.
// One review build resolves the whole lexicon in a few reads.
func (s *Store) ResolveLexemeAudioBatch(ctx context.Context, lexemes []LexemeAudio) ([]string, error) {
	out := make([]string, len(lexemes))
	if len(lexemes) == 0 {
		return out, nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	norms := make([]string, len(lexemes))
	seen := map[string]struct{}{}
	var keys []string
	for i, lx := range lexemes {
		n := units.Normalize(lx.Text)
		norms[i] = n
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		keys = append(keys, n)
	}
	found, err := audioIDsByTexts(ctx, conn, keys)
	if err != nil {
		return nil, err
	}
	for i, lx := range lexemes {
		if id, ok := found[norms[i]]; ok && norms[i] != "" {
			out[i] = id
			continue
		}
		out[i] = lx.AudioID
	}
	return out, nil
}

type audioPick struct {
	id, model, created string
}

func audioIDsByTexts(ctx context.Context, conn *sql.DB, texts []string) (map[string]string, error) {
	best := map[string]audioPick{}
	for start := 0; start < len(texts); start += audioResolveChunk {
		end := min(start+audioResolveChunk, len(texts))
		if err := scanAudioChunk(ctx, conn, texts[start:end], best); err != nil {
			return nil, err
		}
	}
	out := make(map[string]string, len(best))
	for text, pick := range best {
		out[text] = pick.id
	}
	return out, nil
}

func scanAudioChunk(ctx context.Context, conn *sql.DB, texts []string, best map[string]audioPick) error {
	if len(texts) == 0 {
		return nil
	}
	args := make([]any, len(texts))
	holders := make([]string, len(texts))
	for i, text := range texts {
		args[i] = text
		holders[i] = "?"
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, text_nfc, model, created_at
		FROM audio_assets
		WHERE text_nfc IN (`+strings.Join(holders, ",")+`)`, args...)
	if err != nil {
		return fmt.Errorf("db: resolve lexeme audio: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pick audioPick
		var text string
		if err := rows.Scan(&pick.id, &text, &pick.model, &pick.created); err != nil {
			return fmt.Errorf("db: resolve lexeme audio: %w", err)
		}
		if prev, ok := best[text]; !ok || preferAudio(pick, prev) {
			best[text] = pick
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("db: resolve lexeme audio: %w", err)
	}
	return nil
}

// preferAudio reports whether a replaces b. The order is audioByTextQuery:
// model rank, then created_at descending, then id ascending. created_at is
// text, and AudioByText sorts that text, so the comparison is text too.
func preferAudio(a, b audioPick) bool {
	if ra, rb := audioModelRank(a.model), audioModelRank(b.model); ra != rb {
		return ra < rb
	}
	if a.created != b.created {
		return a.created > b.created
	}
	return a.id < b.id
}

func audioModelRank(model string) int {
	switch model {
	case "eleven_v4":
		return 0
	case "eleven_v3":
		return 1
	case "legacy":
		return 2
	default:
		return 3
	}
}
