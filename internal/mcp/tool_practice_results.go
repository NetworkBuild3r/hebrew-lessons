package mcp

import (
	"context"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// notEnoughYet is the practiceResults note when Enough is false.
// INIT-029/SPEC-008; ADR-0025 D8.
const notEnoughYet = "not enough yet"

// laterDayResultsReader is the Course half that reports later-day practice
// outcomes. mcpstore calls evidence.LaterDayResults; Memory stores seed data
// for tests. INIT-029/SPEC-008.
type laterDayResultsReader interface {
	LaterDayResults(ctx context.Context, learnerID int64, now time.Time, zone studyday.Zone) ([]evidence.PracticeResult, error)
}

// PracticeResultView is one practice kind for get_context. When enough
// answers exist, Share is Right/Total. Otherwise Note is "not enough yet".
// Count is always the later-day answer count. INIT-029/SPEC-008; ADR-0025 D8.
type PracticeResultView struct {
	Kind  string   `json:"kind"`
	Share *float64 `json:"share,omitempty"`
	Count int      `json:"count"`
	Note  string   `json:"note,omitempty"`
}

// attachPracticeResults fills practiceResults for a learner, using that
// learner's zone the same way other get_context day tools do.
// Without learnerId the field stays nil and is omitted from JSON.
// INIT-029/SPEC-008.
func (s *service) attachPracticeResults(ctx context.Context, out *getContextOutput, learnerID *int64) error {
	if learnerID == nil {
		return nil
	}
	reader, ok := s.course.(laterDayResultsReader)
	if !ok {
		out.PracticeResults = []PracticeResultView{}
		return nil
	}
	zone, err := s.learnerZone(ctx, *learnerID)
	if err != nil {
		return err
	}
	rows, err := reader.LaterDayResults(ctx, *learnerID, s.now(), zone)
	if err != nil {
		return storeErr("The learner's practice results could not be read.", err)
	}
	out.PracticeResults = practiceResultViews(rows)
	return nil
}

func practiceResultViews(rows []evidence.PracticeResult) []PracticeResultView {
	out := make([]PracticeResultView, 0, len(rows))
	for _, r := range rows {
		view := PracticeResultView{Kind: r.Kind, Count: r.Total}
		if r.Enough && r.Total > 0 {
			share := float64(r.Right) / float64(r.Total)
			view.Share = &share
		} else {
			view.Note = notEnoughYet
		}
		out = append(out, view)
	}
	if out == nil {
		out = []PracticeResultView{}
	}
	return out
}
