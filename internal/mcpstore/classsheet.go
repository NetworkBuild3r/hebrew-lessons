package mcpstore

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
)

// SaveClassSheet stores one checked import for one learner.
func (s *Store) SaveClassSheet(ctx context.Context, learnerID int64, in lessons.Import) (int64, bool, error) {
	if s == nil || s.db == nil {
		return 0, false, errors.New("mcpstore: no database")
	}
	return s.db.SaveClassSheet(ctx, learnerID, in)
}

// ClassSheets lists one learner's sheets, newest class first.
func (s *Store) ClassSheets(ctx context.Context, learnerID int64) ([]db.ClassSheet, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("mcpstore: no database")
	}
	return s.db.ClassSheets(ctx, learnerID)
}

// RequirePassedSheet is the sheet a taught word may name.
func (s *Store) RequirePassedSheet(ctx context.Context, learnerID int64, ref string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("mcpstore: no database")
	}
	return s.db.RequirePassedSheet(ctx, learnerID, ref)
}

// AttachTaughtSheet records which class a taught word came from.
func (s *Store) AttachTaughtSheet(ctx context.Context, learnerID int64, lexemeIDs []int64, on string, sheetID int64) error {
	if s == nil || s.db == nil {
		return errors.New("mcpstore: no database")
	}
	return s.db.AttachTaughtSheet(ctx, learnerID, lexemeIDs, on, sheetID)
}

// SaveStory stores the one story for a lesson type.
func (s *Store) SaveStory(ctx context.Context, learnerID int64, lessonType, body, status, failures string) error {
	if s == nil || s.db == nil {
		return errors.New("mcpstore: no database")
	}
	return s.db.SaveStory(ctx, learnerID, lessonType, body, status, failures)
}

// LoadStory reads that story.
func (s *Store) LoadStory(ctx context.Context, learnerID int64, lessonType string) (db.Story, bool, error) {
	if s == nil || s.db == nil {
		return db.Story{}, false, errors.New("mcpstore: no database")
	}
	return s.db.LoadStory(ctx, learnerID, lessonType)
}
