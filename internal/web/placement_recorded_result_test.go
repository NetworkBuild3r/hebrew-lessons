package web

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// TestPlacementFinishRecordsRatedCardsLevelStopReasonAndEstimates is the fix
// for a finished placement that reports cardsRated 0, level -1, an empty
// stopReason, and estimatedKnownWords 0.
//
// A learner who finishes the check has already had each answer stored.
// Engine.Answer claims the open placement_items row before it returns
// (internal/placement/service.go ClaimPlacementItem) and, for a word, writes
// an evidence attempt in the same call. Letters are the warm-up and name no
// word. The run is scored only when it finishes.
//
// What is wrong on v3, and the fix:
//
//   - cardsRated stays 0. db.LatestPlacement (internal/db/placement.go) sets
//     CardsRated from CountPlacementRatings, which counts placement_answers.
//     The v3 engine never inserts those rows. Count answered placement_items
//     whose part is listening, reading, or typing. Leave the letters warm-up
//     out: those items are not word cards.
//   - stopReason stays empty. Engine.finish passes "" into FinishPlacement,
//     and FinishPlacement stores an empty reason as NULL. When plan returns
//     actFinish because nothing is left to deal, store exhausted. When a
//     skill stopped on the adaptive rule, store band_stop. The column allows
//     only band_stop, cap_cards, and exhausted.
//   - level -1 means no band was passed (legacyLevel is the skill level minus
//     one, and LatestPlacement uses -1 when the column is NULL). A run that
//     passed a band must store that level. Do not skip it on the normal finish.
//   - estimatedKnownWords is the reading estimate when reading was tested, and
//     the listening estimate otherwise (toCompat). It has to match the answers.
//   - get_progress reads placement_runs plus placement_v3_runs (kind) and the
//     result JSON (mcpstore placementView). A v3 finish must keep writing
//     listening.tested and reading.tested there, or both estimates stay null
//     and the kind falls back to v2. get_learner_model reads attempts and
//     cards through learner.Model, which the evidence recorder already writes
//     per answer. It must list those words. It does not read placement_answers.
//
// Owner of the store and the engine write: Hebrew Backend Dev.
func TestPlacementFinishRecordsRatedCardsLevelStopReasonAndEstimates(t *testing.T) {
	for _, plain := range []bool{false, true} {
		name := "htmx"
		if plain {
			name = "nojs"
		}
		t.Run(name, func(t *testing.T) {
			h, store, learner, _ := newPlacementRouter(t)
			d := newDriver(t, h, store, learner)
			d.plain = plain
			body := d.run(plScript{
				letters:     placement.LettersAsked,
				typing:      "take",
				typingRight: true,
				pick:        func(db.PlacementItem, int) string { return "right" },
			})
			if screenOf(body) != "results" {
				t.Fatalf("finished on screen %q", screenOf(body))
			}
			assertPlacementRecorded(t, store, learner.id, name)
		})
	}
}

// assertPlacementRecorded checks the row the placement web flow writes and the
// two MCP reads. On failure it names the fix.
func assertPlacementRecorded(t *testing.T, store *db.Store, userID int64, path string) {
	t.Helper()
	ctx := context.Background()
	runID := countRows(t, store, `SELECT id FROM placement_runs WHERE user_id = ? ORDER BY id DESC LIMIT 1`, userID)
	wordCards := countRows(t, store, `
		SELECT COUNT(*) FROM placement_items
		WHERE run_id = ? AND outcome IS NOT NULL AND part IN ('listening', 'reading', 'typing')`, runID)
	answerRows := countRows(t, store, `
		SELECT COUNT(*) FROM placement_answers WHERE run_id = ? AND rating IS NOT NULL`, runID)
	if wordCards == 0 {
		t.Fatalf("%s: the handler finished without storing a word answer on placement_items", path)
	}

	wantEstimate, wantLevel := placementScore(t, store, runID)
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok || !place.Finished {
		t.Fatalf("%s: latest placement ok=%v finished=%v err=%v", path, ok, place.Finished, err)
	}

	var problems []string
	if place.CardsRated != wordCards {
		problems = append(problems, fmt.Sprintf("cardsRated=%d, want %d answered word cards (placement_answers ratings=%d; v3 never writes that table)", place.CardsRated, wordCards, answerRows))
	}
	if place.Level != wantLevel || place.Level < 0 {
		problems = append(problems, fmt.Sprintf("level=%d, want %d (level -1 means no band was passed, or the column was left NULL)", place.Level, wantLevel))
	}
	if place.StopReason == "" {
		problems = append(problems, `stopReason is empty (Engine.finish passes "" and FinishPlacement stores that as NULL; a deck that has nothing left to deal is exhausted)`)
	}
	if place.EstimatedKnownWords != wantEstimate {
		problems = append(problems, fmt.Sprintf("estimatedKnownWords=%d, want %d from the listening and reading answers", place.EstimatedKnownWords, wantEstimate))
	}

	progress, err := mcpstore.New(store).Progress(ctx, userID, "2026-10-01", "2026-10-31")
	if err != nil {
		t.Fatalf("%s: get_progress store: %v", path, err)
	}
	if len(progress.Placements) != 1 {
		problems = append(problems, fmt.Sprintf("get_progress placements=%d, want 1", len(progress.Placements)))
	} else {
		row := progress.Placements[0]
		if row.Kind != "initial" {
			problems = append(problems, fmt.Sprintf("get_progress kind=%q, want initial (empty kind is reported as v2)", row.Kind))
		}
		if row.Level != wantLevel {
			problems = append(problems, fmt.Sprintf("get_progress level=%d, want %d", row.Level, wantLevel))
		}
		if row.EstimatedKW != wantEstimate {
			problems = append(problems, fmt.Sprintf("get_progress estimatedKnownWords=%d, want %d", row.EstimatedKW, wantEstimate))
		}
		if row.Listening == nil || row.Reading == nil {
			listen, read := "null", "null"
			if row.Listening != nil {
				listen = fmt.Sprintf("%d", *row.Listening)
			}
			if row.Reading != nil {
				read = fmt.Sprintf("%d", *row.Reading)
			}
			problems = append(problems, fmt.Sprintf("get_progress listening=%s reading=%s, want both set (null means result JSON has no tested skill)", listen, read))
		}
	}

	listenWords, readWords := modelSkillCounts(t, store, userID)
	if listenWords == 0 || readWords == 0 {
		problems = append(problems, fmt.Sprintf("get_learner_model words with attempts: listen=%d read=%d, want both above 0", listenWords, readWords))
	}

	if len(problems) == 0 {
		return
	}
	t.Fatalf(`%s placement was not recorded.

%s

Fix: count answered listening, reading, and typing placement_items as cardsRated (internal/db/placement.go LatestPlacement and CountPlacementRatings read only placement_answers, which Engine.Answer does not write). In Engine.finish (internal/placement/service.go) pass exhausted when nothing is left to deal, or band_stop when a skill stopped, instead of "". Keep writing the toCompat result JSON so get_progress can read listening and reading. get_learner_model already reads the evidence attempts written per card; do not move that onto placement_answers. Owner: Hebrew Backend Dev.`,
		path, strings.Join(problems, "\n"))
}

// placementScore is the estimate and the legacy level the answers imply.
// The estimate is the reading one when reading was tested, else listening.
// The legacy level is that skill's level minus one.
func placementScore(t *testing.T, store *db.Store, runID int) (estimate, level int) {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(context.Background(), `
		SELECT part, band, outcome FROM placement_items
		WHERE run_id = ? AND outcome IS NOT NULL`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var listen, read [placement.BandCount]placement.Tally
	lettersRight := 0
	for rows.Next() {
		var part, outcome string
		var band *int
		if err := rows.Scan(&part, &band, &outcome); err != nil {
			t.Fatal(err)
		}
		switch part {
		case "letters":
			if outcome == "right" {
				lettersRight++
			}
		case "listening":
			if band == nil || *band < 1 || *band > placement.BandCount {
				t.Fatalf("listening item band %v", band)
			}
			listen[*band-1].Add(outcome)
		case "reading":
			if band == nil || *band < 1 || *band > placement.BandCount {
				t.Fatalf("reading item band %v", band)
			}
			read[*band-1].Add(outcome)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	readingTested := lettersRight >= placement.LettersPass
	skill := listen
	if readingTested {
		skill = read
	}
	return int(math.Round(placement.EstimateWords(skill))), placement.LevelOf(skill) - 1
}

// modelSkillCounts is how many words get_learner_model returns with a listen
// attempt and with a read attempt.
func modelSkillCounts(t *testing.T, store *db.Store, userID int64) (listen, read int) {
	t.Helper()
	data := mcpstore.New(store)
	var cursor string
	for pageN := 0; pageN < 20; pageN++ {
		page, err := data.LearnerModel(context.Background(), userID, mcp.ModelQuery{Limit: 200, Cursor: cursor})
		if err != nil {
			t.Fatalf("get_learner_model: %v", err)
		}
		for _, w := range page.Words {
			var heard, saw bool
			for _, sk := range w.Skills {
				if sk.Skill == "listen" && sk.Attempts > 0 {
					heard = true
				}
				if sk.Skill == "read" && sk.Attempts > 0 {
					saw = true
				}
			}
			if heard {
				listen++
			}
			if saw {
				read++
			}
		}
		if page.NextCursor == "" {
			return listen, read
		}
		cursor = page.NextCursor
	}
	t.Fatal("get_learner_model did not end")
	return 0, 0
}
