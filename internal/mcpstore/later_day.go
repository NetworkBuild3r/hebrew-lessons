package mcpstore

import (
	"context"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// LaterDayResults calls evidence.LaterDayResults for get_context practiceResults.
// INIT-029/SPEC-008; ADR-0025 D8.
func (s *Store) LaterDayResults(ctx context.Context, learnerID int64, now time.Time, zone studyday.Zone) ([]evidence.PracticeResult, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return evidence.LaterDayResults(ctx, s.db, learnerID, now, zone)
}
