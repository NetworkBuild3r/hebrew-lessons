package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Placement skills that may carry band tallies. Spell is never included.
var placementSkillKeys = []string{"listen", "read", "recall"}

// PlacementSkillBands returns per-skill band tallies for a learner.
//
// For each of listen, read, and recall it takes the newest finished run that
// answered at least one band item of that skill (letters and typing left out).
// An unfinished newest run is ignored. Band numbers 1–5 map to 0–4.
// GoodPlusRate uses the same guessing-corrected share as v3BandScores (v3Share).
//
// A v2 run (no placement_v3_runs row) is self-rated read only: its result.bands
// map to read unchanged. A skill no finished run has tested has no map entry.
// Spell never has an entry.
//
// PlacementBands is unchanged: callers that need the stored run result keep using it.
//
// Authored-by-spec: INIT-032/SPEC-002
func (s *Store) PlacementSkillBands(ctx context.Context, userID int64) (map[string][]BandScore, error) {
	out := make(map[string][]BandScore)
	for _, skill := range placementSkillKeys {
		bands, ok, err := s.placementSkillBandsFor(ctx, userID, skill)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		out[skill] = bands
	}
	return out, nil
}

func (s *Store) placementSkillBandsFor(ctx context.Context, userID int64, skill string) ([]BandScore, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, false, err
	}
	var runID int64
	var isV2 int
	var result sql.NullString
	err = conn.QueryRowContext(ctx, `
		SELECT r.id,
		       CASE WHEN v.run_id IS NULL THEN 1 ELSE 0 END,
		       r.result
		FROM placement_runs AS r
		LEFT JOIN placement_v3_runs AS v ON v.run_id = r.id
		WHERE r.user_id = ?
		  AND r.finished_at IS NOT NULL
		  AND (
		    (v.run_id IS NULL AND ? = 'read' AND r.result IS NOT NULL AND r.result != '')
		    OR EXISTS (
		      SELECT 1 FROM placement_items AS i
		      WHERE i.run_id = r.id
		        AND i.outcome IS NOT NULL
		        AND i.skill = ?
		        AND i.part IN ('listening', 'reading')
		        AND i.band BETWEEN 1 AND 5
		    )
		  )
		ORDER BY r.id DESC
		LIMIT 1`, userID, skill, skill).Scan(&runID, &isV2, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("db: placement skill bands for user %d skill %s: %w", userID, skill, err)
	}
	if isV2 == 1 {
		var body placementResultJSON
		if !result.Valid || result.String == "" {
			return nil, false, nil
		}
		if err := json.Unmarshal([]byte(result.String), &body); err != nil {
			return nil, false, fmt.Errorf("db: placement skill bands for user %d: %w", userID, err)
		}
		return body.Bands, true, nil
	}
	bands, err := s.tallyPlacementSkillBands(ctx, runID, skill)
	if err != nil {
		return nil, false, err
	}
	if len(bands) == 0 {
		return nil, false, nil
	}
	return bands, true, nil
}

// tallyPlacementSkillBands pools answered listening/reading items of one skill
// into deck bands 0–4. Letters and typing are excluded. GoodPlusRate is v3Share,
// matching production result.bands and v3BandScores.
func (s *Store) tallyPlacementSkillBands(ctx context.Context, runID int64, skill string) ([]BandScore, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT band, outcome FROM placement_items
		WHERE run_id = ?
		  AND outcome IS NOT NULL
		  AND skill = ?
		  AND part IN ('listening', 'reading')
		  AND band BETWEEN 1 AND 5`, runID, skill)
	if err != nil {
		return nil, fmt.Errorf("db: placement skill bands: %w", err)
	}
	defer rows.Close()
	type tally struct {
		rated, again, good, dont int
	}
	var tallies [5]tally
	for rows.Next() {
		var band int
		var outcome string
		if err := rows.Scan(&band, &outcome); err != nil {
			return nil, fmt.Errorf("db: placement skill bands: %w", err)
		}
		if band < 1 || band > len(tallies) {
			continue
		}
		t := &tallies[band-1]
		switch outcome {
		case "right":
			t.good++
		case "wrong":
			t.again++
		case "dont_know":
			t.dont++
		default:
			continue
		}
		t.rated++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement skill bands: %w", err)
	}
	var out []BandScore
	for i, t := range tallies {
		if t.rated == 0 {
			continue
		}
		out = append(out, BandScore{
			Band:         i,
			Rated:        t.rated,
			Again:        t.again,
			Good:         t.good,
			DontKnow:     t.dont,
			GoodPlusRate: v3Share(t.good, t.again, t.rated),
		})
	}
	return out, nil
}
