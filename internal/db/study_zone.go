package db

import (
	"context"
	"os"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// StudyZone is the learner's stored zone, or the app zone from TZ when none
// is stored. Scheduling and the due count both use it (ADR-0033 D10).
func (s *Store) StudyZone(ctx context.Context, userID int64) (studyday.Zone, error) {
	name, ok, err := s.UserTimeZone(ctx, userID)
	if err != nil {
		return studyday.Zone{}, err
	}
	if ok {
		loc, err := studyday.ParseZone(name)
		if err != nil {
			return studyday.Zone{}, err
		}
		return studyday.Zone{Name: name, Loc: loc, Source: studyday.SourceLearner}, nil
	}
	loc, err := studyday.AppZone(os.Getenv("TZ"))
	if err != nil {
		return studyday.Zone{}, err
	}
	return studyday.Zone{Name: loc.String(), Loc: loc, Source: studyday.SourceApp}, nil
}
