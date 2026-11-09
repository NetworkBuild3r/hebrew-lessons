package mcp

import (
	"context"
	"time"
)

// unitProgressReader is the Course half that lists a learner's progress per
// unit in scope. mcpstore maps db.UnitProgress; Memory seeds rows for tests.
// INIT-030/SPEC-005.
type unitProgressReader interface {
	UnitProgress(ctx context.Context, userID int64) ([]UnitProgressRow, error)
}

// UnitProgressRow is one unit's progress for attaching to an outline row.
// INIT-030/SPEC-005.
type UnitProgressRow struct {
	UnitID          int64
	State           string
	LessonsFinished int
	LessonsTotal    int
	LastFinishedAt  time.Time
}

// attachUnitProgress fills outline[].progress when learnerId is set.
// Without learnerId the field stays nil and is omitted from JSON.
// INIT-030/SPEC-005.
func (s *service) attachUnitProgress(ctx context.Context, out *getContextOutput, learnerID *int64) error {
	if learnerID == nil {
		return nil
	}
	reader, ok := s.course.(unitProgressReader)
	if !ok {
		return nil
	}
	rows, err := reader.UnitProgress(ctx, *learnerID)
	if err != nil {
		return storeErr("The learner's unit progress could not be read.", err)
	}
	byID := make(map[int64]UnitProgressRow, len(rows))
	for _, row := range rows {
		byID[row.UnitID] = row
	}
	for i := range out.Outline {
		row, ok := byID[out.Outline[i].ID]
		if !ok {
			continue
		}
		p := &UnitOutlineProgress{
			State:           row.State,
			LessonsFinished: row.LessonsFinished,
			LessonsTotal:    row.LessonsTotal,
		}
		if !row.LastFinishedAt.IsZero() {
			at := row.LastFinishedAt.UTC()
			p.LastFinishedAt = &at
		}
		out.Outline[i].Progress = p
	}
	return nil
}
