package brief

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Load is how hard a lesson pushes, read from the learner's recent accuracy.
type Load string

const (
	LoadEase    Load = "ease"    // Missing more than the aim: fewer new words, lean on what is known.
	LoadSteady  Load = "steady"  // Near the aim: the usual mix.
	LoadStretch Load = "stretch" // Cruising: more new words, less review of what is known.
)

const (
	// LoadWindow is how many of the newest checked attempts recent accuracy reads.
	LoadWindow = 40
	// LoadMinAttempts is the fewest attempts before the load leaves steady.
	LoadMinAttempts = 15
	// LoadLow and LoadHigh are the edges. Below LoadLow is ease, above LoadHigh is stretch.
	LoadLow  = 0.70
	LoadHigh = 0.90
	// LoadMargin is the hysteresis. Leaving ease or stretch takes a rate this far past the edge,
	// so a result that bounces on the line does not flip the lesson back and forth.
	LoadMargin = 0.03
	// EaseStruggleCap is the most struggle words an ease lesson lists.
	EaseStruggleCap = 3
	// StretchComfortableWords is the most comfortable words a stretch lesson lists, the low end.
	StretchComfortableWords = 30
)

// LoadFor is the load for a recent accuracy. total is how many attempts the rate covers.
// prev is the load of the learner's previous brief, and it holds across the margin.
// Fewer than LoadMinAttempts attempts is always steady.
func LoadFor(rate float64, total int, prev Load) Load {
	if total < LoadMinAttempts {
		return LoadSteady
	}
	switch prev {
	case LoadEase:
		if rate < LoadLow+LoadMargin {
			return LoadEase
		}
	case LoadStretch:
		if rate > LoadHigh-LoadMargin {
			return LoadStretch
		}
	}
	switch {
	case rate < LoadLow:
		return LoadEase
	case rate > LoadHigh:
		return LoadStretch
	default:
		return LoadSteady
	}
}

// TunedNewWords is how many new words a lesson takes for a load. Ease takes the low end of the
// tier's range and stretch the high end. Steady keeps the struggle-adjusted count.
func TunedNewWords(t Tier, struggle int, load Load) int {
	lo, hi := MinNewWords, MaxNewWords
	if t.Name == TierBeginner {
		lo, hi = BeginnerMinNewWords, BeginnerMaxNewWords
	}
	switch load {
	case LoadEase:
		return lo
	case LoadStretch:
		return hi
	default:
		return NewWordCount(t, struggle)
	}
}

// ComfortableCap is how many comfortable words a load lists. Ease and steady take the high end.
func ComfortableCap(load Load) int {
	if load == LoadStretch {
		return StretchComfortableWords
	}
	return MaxComfortableWords
}

func (b *Builder) recentAccuracy(ctx context.Context, userID int64) (float64, int, error) {
	correct, total, err := b.store.RecentChecked(ctx, userID, LoadWindow)
	if err != nil {
		return 0, 0, fmt.Errorf("brief: recent accuracy: %w", err)
	}
	if total == 0 {
		return 0, 0, nil
	}
	return float64(correct) / float64(total), total, nil
}

// previousLoad is the load of the learner's newest brief, or steady when there is none.
func (b *Builder) previousLoad(ctx context.Context, userID int64) (Load, error) {
	conn, err := b.store.DB()
	if err != nil {
		return LoadSteady, fmt.Errorf("brief: previous load: %w", err)
	}
	var payload string
	err = conn.QueryRowContext(ctx,
		`SELECT payload_json FROM lesson_briefs WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`,
		userID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return LoadSteady, nil
	}
	if err != nil {
		return LoadSteady, fmt.Errorf("brief: previous load: %w", err)
	}
	var parsed struct {
		Load Load `json:"load"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return LoadSteady, nil
	}
	switch parsed.Load {
	case LoadEase, LoadSteady, LoadStretch:
		return parsed.Load, nil
	default:
		return LoadSteady, nil
	}
}
