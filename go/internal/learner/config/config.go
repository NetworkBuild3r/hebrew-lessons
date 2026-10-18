// Package config is the Go learner package config from ADR-0017 §3.
//
// The checkpoint checker and the path scheduler read these values. They do
// not embed 0.80, 8, 2, or 3. Defaults live only here.
//
// Environment variables use the ADR key names, uppercased. An empty or
// unset variable selects the default. Leading and trailing spaces are
// ignored. Any other value that is not the documented grammar is an error.
//
//	CHECKPOINT_PASS_MARK      checkpoint_pass_mark       default 0.80
//	REVIEW_STOP_DUE_COUNT     review_stop_due_count      default 8
//	REVIEW_STOP_GAP_LESSONS   review_stop_gap_lessons    default 2
//	LESSON_WARMUP_DUE         lesson_warmup_due          default 3
//
// checkpoint_pass_mark is a decimal in the closed range 0 to 1, written
// without an exponent and without a leading plus. 0.80 and 1 and 0 are
// valid. 1.5, 80%, and 1e-1 are not.
//
// The three counts are integers greater than or equal to 0, written
// without a sign and without a fraction. 0 is valid: a pass mark of 0
// needs no correct answers, a due-count of 0 inserts a review stop
// whenever a current lesson exists, and a gap of 0 does not skip any
// lesson after a stop.
package config

import (
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ADR-0017 §3 key names. These are the names in the decision record, not
// the environment variable names.
const (
	KeyCheckpointPassMark   = "checkpoint_pass_mark"
	KeyReviewStopDueCount   = "review_stop_due_count"
	KeyReviewStopGapLessons = "review_stop_gap_lessons"
	KeyLessonWarmupDue      = "lesson_warmup_due"
)

// Environment variable names. Each one sets the key of the same words.
const (
	EnvCheckpointPassMark   = "CHECKPOINT_PASS_MARK"
	EnvReviewStopDueCount   = "REVIEW_STOP_DUE_COUNT"
	EnvReviewStopGapLessons = "REVIEW_STOP_GAP_LESSONS"
	EnvLessonWarmupDue      = "LESSON_WARMUP_DUE"
)

// Defaults from ADR-0017 §3. The path package does not repeat them.
const (
	DefaultCheckpointPassMark   = "0.80"
	DefaultReviewStopDueCount   = 8
	DefaultReviewStopGapLessons = 2
	DefaultLessonWarmupDue      = 3
)

var (
	decimalPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	countPattern   = regexp.MustCompile(`^(?:0|[1-9][0-9]*)$`)
)

// Config is the learner thresholds for one process.
type Config struct {
	// CheckpointPassMark is the decimal text of checkpoint_pass_mark,
	// exactly as configured or the default "0.80".
	CheckpointPassMark string

	// ReviewStopDueCount is review_stop_due_count. A review stop is
	// inserted when the due count is at least this value.
	ReviewStopDueCount int

	// ReviewStopGapLessons is review_stop_gap_lessons. After a stop, this
	// many following lessons do not get another stop.
	ReviewStopGapLessons int

	// LessonWarmupDue is lesson_warmup_due. A lesson opens with at most
	// this many due words. This package does not pick those words.
	LessonWarmupDue int

	mark *big.Rat
}

// Load reads the process environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	markText, err := parseMarkText(getenv(EnvCheckpointPassMark))
	if err != nil {
		return Config{}, err
	}
	due, err := parseCount(EnvReviewStopDueCount, getenv(EnvReviewStopDueCount), DefaultReviewStopDueCount)
	if err != nil {
		return Config{}, err
	}
	gap, err := parseCount(EnvReviewStopGapLessons, getenv(EnvReviewStopGapLessons), DefaultReviewStopGapLessons)
	if err != nil {
		return Config{}, err
	}
	warmup, err := parseCount(EnvLessonWarmupDue, getenv(EnvLessonWarmupDue), DefaultLessonWarmupDue)
	if err != nil {
		return Config{}, err
	}
	mark, err := markRat(markText)
	if err != nil {
		return Config{}, err
	}
	return Config{
		CheckpointPassMark:   markText,
		ReviewStopDueCount:   due,
		ReviewStopGapLessons: gap,
		LessonWarmupDue:      warmup,
		mark:                 mark,
	}, nil
}

func parseMarkText(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultCheckpointPassMark, nil
	}
	if !decimalPattern.MatchString(raw) {
		return "", fmt.Errorf("config: %s %q is not a decimal from 0 to 1", EnvCheckpointPassMark, raw)
	}
	if _, err := markRat(raw); err != nil {
		return "", err
	}
	return raw, nil
}

func markRat(text string) (*big.Rat, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(text); !ok {
		return nil, fmt.Errorf("config: %s %q is not a decimal from 0 to 1", EnvCheckpointPassMark, text)
	}
	if r.Sign() < 0 || r.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, fmt.Errorf("config: %s %q is not a decimal from 0 to 1", EnvCheckpointPassMark, text)
	}
	return r, nil
}

func parseCount(name, raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	if !countPattern.MatchString(raw) {
		return 0, fmt.Errorf("config: %s %q is not an integer >= 0", name, raw)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s %q is not an integer >= 0", name, raw)
	}
	return n, nil
}

func (c Config) ratio() (*big.Rat, error) {
	if c.mark != nil {
		return new(big.Rat).Set(c.mark), nil
	}
	if c.CheckpointPassMark == "" {
		return markRat(DefaultCheckpointPassMark)
	}
	return markRat(c.CheckpointPassMark)
}

// PassNeeded returns ceil(checkpoint_pass_mark * total).
//
// The product is exact rational arithmetic on the decimal text. It does not
// use float64. float64(0.07) * 100 is slightly above 7, and math.Ceil of
// that product is 8; the decimal result is 7. At the default mark, 20
// questions need 16 correct answers.
//
// total 0 returns 0. A negative total is an error.
func (c Config) PassNeeded(total int) (int, error) {
	if total < 0 {
		return 0, fmt.Errorf("config: total %d is negative", total)
	}
	if total == 0 {
		return 0, nil
	}
	mark, err := c.ratio()
	if err != nil {
		return 0, err
	}
	if mark.Sign() == 0 {
		return 0, nil
	}
	prod := new(big.Rat).Mul(mark, big.NewRat(int64(total), 1))
	num := prod.Num()
	den := prod.Denom()
	quot := new(big.Int).Quo(num, den)
	rem := new(big.Int).Rem(num, den)
	if rem.Sign() > 0 {
		quot.Add(quot, big.NewInt(1))
	}
	if !quot.IsInt64() {
		return 0, fmt.Errorf("config: pass total does not fit in an int")
	}
	n := quot.Int64()
	if int64(int(n)) != n {
		return 0, fmt.Errorf("config: pass total does not fit in an int")
	}
	return int(n), nil
}

// MeetsPassMark reports whether correct is at least PassNeeded(total).
// 16 of 20 meets the default mark. 15 of 20 does not. correct may be
// larger than total; the comparison does not clamp it. A negative correct
// is an error.
func (c Config) MeetsPassMark(correct, total int) (bool, error) {
	if correct < 0 {
		return false, fmt.Errorf("config: correct %d is negative", correct)
	}
	need, err := c.PassNeeded(total)
	if err != nil {
		return false, err
	}
	return correct >= need, nil
}
