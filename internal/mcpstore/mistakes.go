package mcpstore

import (
	"context"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func (s *Store) RecordMistakes(ctx context.Context, learnerID int64, rows []db.MistakeInput) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.RecordMistakes(ctx, learnerID, rows)
}

func (s *Store) MistakesFor(ctx context.Context, learnerID int64, limit int) ([]db.LearnerMistake, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.MistakesFor(ctx, learnerID, limit)
}

func (s *Store) UpsertCommonMistakes(ctx context.Context, rows []db.CommonMistakeInput) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.UpsertCommonMistakes(ctx, rows)
}

func (s *Store) UpsertVerbForms(ctx context.Context, rows []db.VerbFormInput) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.UpsertVerbForms(ctx, rows)
}
