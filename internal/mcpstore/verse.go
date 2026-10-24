package mcpstore

import (
	"context"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func (s *Store) UpsertVerseNotes(ctx context.Context, book string, chapter, verse int, notes []db.VerseNoteInput) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.UpsertVerseNotes(ctx, book, chapter, verse, notes)
}

func (s *Store) UpsertVerseGlosses(ctx context.Context, book string, chapter, verse int, glosses []db.VerseGlossInput) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.UpsertVerseGlosses(ctx, book, chapter, verse, glosses)
}

func (s *Store) VerseNotes(ctx context.Context, book string, chapter, verse int, checkedOnly bool) ([]db.VerseNote, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.VerseNotes(ctx, book, chapter, verse, checkedOnly)
}

func (s *Store) VerseGlosses(ctx context.Context, book string, chapter, verse int, checkedOnly bool) ([]db.VerseGloss, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.VerseGlosses(ctx, book, chapter, verse, checkedOnly)
}

func (s *Store) TanakhVerse(ctx context.Context, book string, chapter, verse int) (db.TanakhVerse, bool, error) {
	if err := s.ready(ctx); err != nil {
		return db.TanakhVerse{}, false, err
	}
	return s.db.TanakhVerse(ctx, book, chapter, verse)
}
