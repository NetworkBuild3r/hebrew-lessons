package library

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const msPerDay int64 = 24 * 60 * 60 * 1000

// Ladder and weak-flag thresholds from server/learner/thresholds.js.
// They are copied so a library read does not import the player.
var (
	ladderStabilityDays      = [...]int{1, 3, 7, 14, 30}
	ladderLongTermStep       = 3
	longTermStabilityDays    = 14.0
	ladderRetrievabilityBase = 0.9
	strengthFragileBelow     = 0.5
	strengthShakyBelow       = 0.8
	strengthSteadyBelow      = 0.95
	weakLongTermLapses       = 4
	weakVeryWeakLapses       = 3
	weakLapses               = 2
	weakAgainWithLapses      = 1
	weakAgainGrade           = 1
	weakFSRSDifficulty       = 8.0
	weakFSRSReps             = 3
	scorePass                = 1
	scoreMinPassGrade        = 2
	fsrsMinStability         = 0.001
	fsrsW20                  = 0.1542
	fsrsDecay                float64
	fsrsFactor               float64
)

func init() {
	fsrsDecay = -fsrsW20
	fsrsFactor = math.Pow(0.9, 1/fsrsDecay) - 1
}

type reviewRow struct {
	Word         string
	Direction    string
	Due          string
	Step         int64
	Lapses       int64
	LastResult   string
	LastResultOK bool
	LastAt       string
	LastAtOK     bool
	Sources      string
	Stability    float64
	StabilityOK  bool
	Difficulty   float64
	DifficultyOK bool
	Reps         int64
	LastGrade    int64
	LastGradeOK  bool
	Suspended    int64
	PassedEvent  int64
}

func (row reviewRow) everPassed() bool {
	if row.LastResultOK && row.LastResult == "pass" {
		return true
	}
	if row.Step >= int64(scorePass) {
		return true
	}
	if row.LastGradeOK && row.LastGrade >= int64(scoreMinPassGrade) {
		return true
	}
	return row.PassedEvent == int64(scorePass)
}

func (row reviewRow) graded() bool {
	if row.LastGradeOK {
		return true
	}
	return row.LastResultOK && (row.LastResult == "pass" || row.LastResult == "miss")
}

func stageOf(row reviewRow, scheduler string) string {
	if !row.everPassed() {
		return "new"
	}
	if scheduler == "fsrs" {
		stability := float64(ladderStability(row.Step))
		if row.StabilityOK && !math.IsNaN(row.Stability) && !math.IsInf(row.Stability, 0) {
			stability = row.Stability
		}
		if stability >= longTermStabilityDays {
			return "longTerm"
		}
		return "learning"
	}
	if row.Step >= int64(ladderLongTermStep) {
		return "longTerm"
	}
	return "learning"
}

func ladderStability(step int64) int {
	index := 0
	if step >= 0 {
		index = int(step)
	}
	if index >= len(ladderStabilityDays) {
		index = len(ladderStabilityDays) - 1
	}
	return ladderStabilityDays[index]
}

func strengthBucket(retrievability *float64) *string {
	if retrievability == nil || math.IsNaN(*retrievability) || math.IsInf(*retrievability, 0) {
		return nil
	}
	var name string
	switch {
	case *retrievability < strengthFragileBelow:
		name = "fragile"
	case *retrievability < strengthShakyBelow:
		name = "shaky"
	case *retrievability < strengthSteadyBelow:
		name = "steady"
	default:
		name = "solid"
	}
	return &name
}

func flagForDirection(row reviewRow, stage, scheduler string) string {
	if !row.graded() {
		return ""
	}
	lapses := row.Lapses
	if stage == "longTerm" && lapses >= int64(weakLongTermLapses) {
		return "weak"
	}
	if stage == "longTerm" {
		return ""
	}
	if lapses >= int64(weakVeryWeakLapses) {
		return "very_weak"
	}
	again := row.LastGradeOK && row.LastGrade == int64(weakAgainGrade)
	if lapses == int64(weakLapses) || (lapses == int64(weakAgainWithLapses) && again) {
		return "weak"
	}
	if scheduler == "fsrs" && row.DifficultyOK && !math.IsNaN(row.Difficulty) && !math.IsInf(row.Difficulty, 0) &&
		row.Difficulty >= weakFSRSDifficulty && row.Reps >= int64(weakFSRSReps) {
		return "weak"
	}
	return ""
}

func worseFlag(flags []string) string {
	worst := ""
	for _, flag := range flags {
		if flag == "very_weak" {
			return "very_weak"
		}
		if flag == "weak" {
			worst = "weak"
		}
	}
	return worst
}

var bucketRank = map[string]int{"fragile": 0, "shaky": 1, "steady": 2, "solid": 3}

func worseBucket(a, b *string) *string {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if bucketRank[*a] <= bucketRank[*b] {
		return a
	}
	return b
}

func retrievabilityOf(row reviewRow, scheduler string, nowMs int64) *float64 {
	if scheduler == "fsrs" {
		if !row.StabilityOK || math.IsNaN(row.Stability) || math.IsInf(row.Stability, 0) || row.Stability <= 0 {
			return nil
		}
		lastMs, ok := parseTimeMs(row.LastAt, row.LastAtOK)
		if !ok {
			return nil
		}
		elapsed := calendarDays(lastMs, nowMs)
		stability := row.Stability
		if stability < fsrsMinStability {
			stability = fsrsMinStability
		}
		value := math.Pow(1+fsrsFactor*float64(elapsed)/stability, fsrsDecay)
		return &value
	}
	return ladderRetrievability(row, nowMs)
}

func ladderRetrievability(row reviewRow, nowMs int64) *float64 {
	stability := float64(ladderStability(row.Step))
	elapsed, ok := elapsedDays(row, nowMs)
	if !ok || !(stability > 0) {
		return nil
	}
	value := math.Pow(ladderRetrievabilityBase, float64(elapsed)/stability)
	return &value
}

func elapsedDays(row reviewRow, nowMs int64) (int, bool) {
	lastMs, ok := parseTimeMs(row.LastAt, row.LastAtOK)
	if !ok {
		return 0, false
	}
	return calendarDays(lastMs, nowMs), true
}

func calendarDays(lastMs, nowMs int64) int {
	days := localDayNumber(nowMs) - localDayNumber(lastMs)
	if days < 0 {
		return 0
	}
	return days
}

func dueInDays(due string, nowMs int64) *int {
	if due == "" {
		return nil
	}
	dueMs, ok := parseTimeMs(due, true)
	if !ok {
		return nil
	}
	days := localDayNumber(dueMs) - localDayNumber(nowMs)
	return &days
}

// localDayNumber matches server/learner/model.js: the UTC day number of the
// local calendar date. Tests pin time.Local to UTC; production uses the
// process zone, the same zone Node uses for Date#getFullYear.
func localDayNumber(ms int64) int {
	t := time.UnixMilli(ms).In(time.Local)
	y, m, d := t.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return int(math.Round(float64(midnight.UnixMilli()) / float64(msPerDay)))
}

func parseTimeMs(value string, ok bool) (int64, bool) {
	if !ok || value == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return 0, false
	}
	return t.UnixMilli(), true
}

func parseSources(raw string) []string {
	if raw == "" {
		return nil
	}
	var parsed []any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	out := make([]string, 0, len(parsed))
	for _, value := range parsed {
		out = append(out, fmt.Sprint(value))
	}
	return out
}

type wordState struct {
	Word      string
	HeEN      *Direction
	EnHE      *Direction
	Weak      string
	Suspended bool
	Sources   []string
}

func wordFromRows(rows []reviewRow, scheduler string, nowMs int64) wordState {
	ordered := append([]reviewRow(nil), rows...)
	sortDirections(ordered)
	state := wordState{Word: rows[0].Word, Sources: []string{}}
	flags := []string{}
	seen := map[string]bool{}
	for _, row := range ordered {
		if row.Direction != "he_en" && row.Direction != "en_he" {
			continue
		}
		stage := stageOf(row, scheduler)
		retrievability := retrievabilityOf(row, scheduler, nowMs)
		dir := &Direction{
			Stage:          stage,
			StrengthBucket: strengthBucket(retrievability),
			Retrievability: retrievability,
			Lapses:         int(row.Lapses),
			DueInDays:      dueInDays(row.Due, nowMs),
		}
		if row.Direction == "he_en" {
			state.HeEN = dir
		} else {
			state.EnHE = dir
		}
		flags = append(flags, flagForDirection(row, stage, scheduler))
		if row.Suspended == 1 {
			state.Suspended = true
		}
		for _, source := range parseSources(row.Sources) {
			if source == "" || seen[source] {
				continue
			}
			seen[source] = true
			state.Sources = append(state.Sources, source)
		}
	}
	state.Weak = worseFlag(flags)
	return state
}

func sortDirections(rows []reviewRow) {
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		j := i
		for j > 0 && directionIndex(rows[j-1].Direction) > directionIndex(row.Direction) {
			rows[j] = rows[j-1]
			j--
		}
		rows[j] = row
	}
}

func directionIndex(direction string) int {
	switch direction {
	case "he_en":
		return 0
	case "en_he":
		return 1
	default:
		return 2
	}
}
