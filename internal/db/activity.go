package db

import (
	"context"
	"fmt"
	"time"
)

// This file is the query layer of internal/activity (INIT-015/SPEC-015). It reads the
// words an activity can use as options and a word's recent confusions. Nothing here
// decides what is fair: internal/activity does.

// PoolWord is one dictionary entry as the activity engine sees it.
// AudioID is lexemes.audio_id, empty when that column is null. A clip that
// exists only as an audio_assets row for the word's text is not copied here;
// ResolveLexemeAudio is the lookup that finds it. Gloss and POS are empty when
// the lexicon has not filled them in. Seen is true when the learner has an
// attempt or a card for the word, so a match item can prefer words the learner
// already met.
type PoolWord struct {
	ID      int64
	TextNFC string
	Gloss   string
	POS     string
	AudioID string
	Seen    bool
}

// ActivityPool returns every lexeme, by id, for the learner. One query reads the
// whole lexicon: the options of an item are drawn from it.
func (s *Store) ActivityPool(ctx context.Context, userID int64) ([]PoolWord, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.id, l.text_nfc, l.gloss_en, l.pos, COALESCE(l.audio_id, ''),
			EXISTS (SELECT 1 FROM attempts AS a WHERE a.user_id = ? AND a.lexeme_id = l.id)
			OR EXISTS (SELECT 1 FROM cards AS c WHERE c.user_id = ? AND c.lexeme_id = l.id)
		FROM lexemes AS l
		ORDER BY l.id`, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("db: activity pool: %w", err)
	}
	defer rows.Close()
	var out []PoolWord
	for rows.Next() {
		var w PoolWord
		var seen int
		if err := rows.Scan(&w.ID, &w.TextNFC, &w.Gloss, &w.POS, &w.AudioID, &seen); err != nil {
			return nil, fmt.Errorf("db: activity pool: %w", err)
		}
		w.Seen = seen != 0
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: activity pool: %w", err)
	}
	return out, nil
}

// WordConfusion is one confusion_misses row about a word the learner was asked.
type WordConfusion struct {
	Kind string
	Key  string
	At   time.Time
}

// WordConfusions returns the learner's confusion rows about one asked word since a
// time, newest first.
func (s *Store) WordConfusions(ctx context.Context, userID, lexemeID int64, since time.Time) ([]WordConfusion, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT kind, key, at
		FROM confusion_misses
		WHERE learner_id = ? AND lexeme_id = ? AND at >= ?
		ORDER BY at DESC, id DESC`, userID, lexemeID, FormatTime(since))
	if err != nil {
		return nil, fmt.Errorf("db: word confusions: %w", err)
	}
	defer rows.Close()
	var out []WordConfusion
	for rows.Next() {
		var c WordConfusion
		var at string
		if err := rows.Scan(&c.Kind, &c.Key, &at); err != nil {
			return nil, fmt.Errorf("db: word confusions: %w", err)
		}
		if c.At, err = ParseTime(at); err != nil {
			return nil, fmt.Errorf("db: word confusions: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: word confusions: %w", err)
	}
	return out, nil
}
