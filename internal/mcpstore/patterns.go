package mcpstore

import (
	"context"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func (s *Store) UpsertPatterns(ctx context.Context, rows []db.PatternInput) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.UpsertPatterns(ctx, rows)
}

func (s *Store) Patterns(ctx context.Context) ([]db.PatternInfo, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.Patterns(ctx)
}

func (s *Store) Prereqs(ctx context.Context) (map[string][]string, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.Prereqs(ctx)
}

func (s *Store) PatternUnits(ctx context.Context) ([]db.PatternUnit, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.PatternUnits(ctx)
}

func (s *Store) UpsertExplainer(ctx context.Context, row db.Explainer) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.UpsertExplainer(ctx, row)
}

func (s *Store) HasPattern(ctx context.Context, slug string) (bool, error) {
	if err := s.ready(ctx); err != nil {
		return false, err
	}
	return s.db.HasPattern(ctx, slug)
}

func (s *Store) SetUnitMeta(ctx context.Context, id int64, pattern, worksheet *string) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.SetUnitMeta(ctx, id, pattern, worksheet)
}

func (s *Store) AuditContent(ctx context.Context, actor, action, scope string) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	if actor == "" {
		actor = defaultActor
	}
	return s.db.AuditContent(ctx, actor, action, scope)
}
