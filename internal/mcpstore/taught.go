package mcpstore

import (
	"context"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Taught calls evidence.Recorder.Taught. INIT-026/SPEC-008; ADR-0022 D2.
func (s *Store) Taught(ctx context.Context, learnerID int64, lexemeIDs []int64, on string, z studyday.Zone) (evidence.TaughtResult, error) {
	if err := s.ready(ctx); err != nil {
		return evidence.TaughtResult{}, err
	}
	return evidence.New(s.db, nil, s.db.Clock()).Taught(ctx, learnerID, lexemeIDs, on, z)
}

// RemoveTaught calls evidence.Recorder.RemoveTaught. INIT-026/SPEC-008; ADR-0022 D2.
func (s *Store) RemoveTaught(ctx context.Context, learnerID int64, on string, lexemeIDs []int64) (evidence.RemoveTaughtResult, error) {
	if err := s.ready(ctx); err != nil {
		return evidence.RemoveTaughtResult{}, err
	}
	return evidence.New(s.db, nil, s.db.Clock()).RemoveTaught(ctx, learnerID, on, lexemeIDs)
}

// ResolveTaughtWord maps one word input to a lexeme id. he is NFC-normalized
// and looked up; missing Hebrew returns ok false. INIT-026/SPEC-008.
func (s *Store) ResolveTaughtWord(ctx context.Context, lexemeID int64, he string) (id int64, ok bool, err error) {
	if err := s.ready(ctx); err != nil {
		return 0, false, err
	}
	if lexemeID > 0 {
		var n int
		conn, err := s.conn(ctx)
		if err != nil {
			return 0, false, err
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM lexemes WHERE id = ?`, lexemeID).Scan(&n); err != nil {
			return 0, false, err
		}
		return lexemeID, n == 1, nil
	}
	he = units.Normalize(he)
	if he == "" {
		return 0, false, nil
	}
	return s.LexemeByText(ctx, he)
}
