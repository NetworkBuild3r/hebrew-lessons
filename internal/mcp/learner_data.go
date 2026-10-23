package mcp

import (
	"context"
	"errors"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// This file is the store side of the learner tools (INIT-015/SPEC-021): get_learner_model,
// get_progress, get_lesson_brief, add_coach_note, and the model summary and coach notes
// that get_context adds. A CourseReader may also implement LearnerData. The service looks
// for it with a type assertion, as it does for LearnerUnits, so a store without it makes
// these tools return a tool error and leaves get_context as it was.
//
// Every method takes the learner id from the caller of the tool, which took it from the
// principal. No method reads a learner id out of a request body. The result types hold no
// id, email, or token, and nothing that names another learner.

var (
	// ErrBadModelCursor is a get_learner_model cursor this server did not issue.
	ErrBadModelCursor = errors.New("The page cursor is not valid.")
	// ErrBadTheme is a theme the brief builder refused. The reason is fixed text.
	ErrBadTheme = errors.New("The theme must be plain text of 200 characters or fewer.")
	// ErrTooManyNotes is a learner with too many notes in the last day.
	ErrTooManyNotes = errors.New("Too many coach notes were added for this learner today.")
)

// LearnerData is what the learner tools read and write.
type LearnerData interface {
	// LearnerExists reports whether a learner has the id. Only an admin names an id, so the
	// answer is not a leak.
	LearnerExists(ctx context.Context, learnerID int64) (bool, error)
	// LearnerModel returns one page of the learner's words with their state per skill.
	// An unreadable cursor is ErrBadModelCursor. The caller has checked the filters.
	LearnerModel(ctx context.Context, learnerID int64, q ModelQuery) (ModelPage, error)
	// LearnerModelSummary counts the learner's words per state and skill.
	LearnerModelSummary(ctx context.Context, learnerID int64) (ModelSummary, error)
	// Progress reads the learner's history for the days from through to (YYYY-MM-DD, both
	// inclusive). The caller has checked the range.
	Progress(ctx context.Context, learnerID int64, from, to string) (Progress, error)
	// BuildLessonBrief builds and stores a brief for the learner. A bad theme is ErrBadTheme.
	BuildLessonBrief(ctx context.Context, learnerID int64, theme string) (LessonBrief, error)
	// AddCoachNote stores a note for the learner and returns its id. Too many notes in a
	// day is ErrTooManyNotes.
	AddCoachNote(ctx context.Context, learnerID int64, author, body string) (CoachNote, error)
	// RecentCoachNotes returns the learner's newest notes, newest first, at most n.
	RecentCoachNotes(ctx context.Context, learnerID int64, n int) ([]CoachNote, error)
}

// BriefReader reads the facts of a learner's brief for validate_unit. It is one method of
// LearnerUnits, so a store that has the whole of LearnerUnits has it too. A store may have
// it without auto-publish, as the in-memory store does.
type BriefReader interface {
	// BriefFacts is the method of LearnerUnits of the same name: ErrBriefNotFound for a
	// brief that is missing or another learner's, and ErrBriefExpired for an old one.
	BriefFacts(ctx context.Context, learnerID int64, briefID string) (units.BriefFacts, error)
}

// UnitUnpublisher takes a learner's own unit off their Today. It is one method of
// LearnerUnits, so a store that has the whole of LearnerUnits has it too.
type UnitUnpublisher interface {
	UnpublishLearner(ctx context.Context, p Principal, unitID int64) (UnpublishResult, error)
}

// ModelQuery is a get_learner_model request after checks. Empty State and Skill keep every
// state and skill. A nil Band keeps every band. Limit is already clamped.
type ModelQuery struct {
	State  string
	Skill  string
	Band   *int
	Cursor string
	Limit  int
}

// ModelSkill is one skill of one word.
type ModelSkill struct {
	Skill     string     `json:"skill"`
	State     string     `json:"state"`
	Attempts  int        `json:"attempts"`
	Accuracy  float64    `json:"accuracy" jsonschema:"Share of the newest answers in this skill that were right, 0 to 1. 0 when there are none."`
	LastSeen  *time.Time `json:"lastSeen" jsonschema:"UTC RFC3339, or null when the skill has never been answered."`
	Stability float64    `json:"stability" jsonschema:"Days a memory of this word in this skill is expected to last. 0 when there is no card."`
}

// ModelWord is one word with its overall state, its cause, and its skills.
type ModelWord struct {
	Text      string       `json:"text" jsonschema:"Pointed Hebrew, NFC."`
	Gloss     string       `json:"gloss"`
	Band      *int         `json:"band" jsonschema:"Placement frequency band, 0 to 4, or null when the word is not in the placement deck."`
	State     string       `json:"state" jsonschema:"The weakest skill with evidence: unseen, seen, struggling, regressed, learning, known, or solid."`
	Cause     string       `json:"cause,omitempty" jsonschema:"Why the word is struggling or regressed: decoding, production, spelling, confusion, or band_slip. Omitted when none applies."`
	Attempts  int          `json:"attempts"`
	Accuracy  float64      `json:"accuracy"`
	LastSeen  *time.Time   `json:"lastSeen"`
	Stability float64      `json:"stability"`
	Skills    []ModelSkill `json:"skills"`
}

// ModelPage is one page of words. NextCursor is empty on the last page.
type ModelPage struct {
	Words      []ModelWord
	NextCursor string
}

// ModelSkillSummary is the counts of one skill.
type ModelSkillSummary struct {
	Skill          string         `json:"skill"`
	Counts         map[string]int `json:"counts"`
	EstimatedKnown float64        `json:"estimatedKnown"`
	Retention      *float64       `json:"retention" jsonschema:"Share of due reviews in the last 30 days not rated Again. Null when there were none."`
	DueReviews     int            `json:"dueReviews"`
}

// ModelSummary is the learner's picture across all words.
type ModelSummary struct {
	Words      map[string]int      `json:"words" jsonschema:"Words per overall state."`
	Skills     []ModelSkillSummary `json:"skills"`
	Retention  *float64            `json:"retention"`
	DueReviews int                 `json:"dueReviews"`
}

// SkillDay is one skill on one day of a snapshot.
type SkillDay struct {
	Skill          string         `json:"skill"`
	Counts         map[string]int `json:"counts" jsonschema:"Words per state that day."`
	EstimatedKnown float64        `json:"estimatedKnown"`
	Retention      *float64       `json:"retention"`
}

// SnapshotDay is one day of history.
type SnapshotDay struct {
	Day    string     `json:"day"`
	Skills []SkillDay `json:"skills"`
}

// RegressionView is one regression event. A band_slip has no text, skill, or states.
type RegressionView struct {
	Day       string `json:"day"`
	Kind      string `json:"kind" jsonschema:"word, or band_slip when a whole frequency band lost known words."`
	Text      string `json:"text,omitempty"`
	Skill     string `json:"skill,omitempty"`
	FromState string `json:"fromState,omitempty"`
	ToState   string `json:"toState,omitempty"`
	Cause     string `json:"cause"`
}

// SkillChange is how many words a skill knew at the first and at the last snapshot of
// the range. Gained and Lost are the net move and are never both above zero.
type SkillChange struct {
	Skill        string `json:"skill"`
	KnownAtStart int    `json:"knownAtStart"`
	KnownAtEnd   int    `json:"knownAtEnd"`
	Gained       int    `json:"gained"`
	Lost         int    `json:"lost"`
}

// RetentionView is real retention against the target.
type RetentionView struct {
	Target float64  `json:"target" jsonschema:"The retention the scheduler aims for. 0.85 until a per-learner setting exists."`
	Actual *float64 `json:"actual" jsonschema:"Share of due reviews answered right, from the newest snapshot in the range. Null when there is none."`
	Day    string   `json:"day,omitempty"`
	// Gap is Actual minus Target. Null when Actual is null.
	Gap *float64 `json:"gap"`
}

// PlacementView is one finished placement run.
type PlacementView struct {
	Kind        string    `json:"kind" jsonschema:"initial, recheck, or v2 for a run made before the adaptive placement."`
	FinishedAt  time.Time `json:"finishedAt"`
	Level       int       `json:"level" jsonschema:"Highest band passed, or -1."`
	Listening   *int      `json:"listeningEstimate" jsonschema:"Estimated words known by listening. Null when that part was not taken."`
	Reading     *int      `json:"readingEstimate" jsonschema:"Estimated words known by reading. Null when that part was not taken."`
	EstimatedKW int       `json:"estimatedKnownWords"`
}

// ReportView is one weekly report's summary.
type ReportView struct {
	WeekStart string    `json:"weekStart"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"createdAt"`
}

// Progress is the learner's history for a range of days.
type Progress struct {
	Snapshots     []SnapshotDay    `json:"snapshots"`
	Changes       []SkillChange    `json:"changes"`
	Regressions   []RegressionView `json:"regressions"`
	Retention     RetentionView    `json:"retention"`
	Placements    []PlacementView  `json:"placements"`
	WeeklyReports []ReportView     `json:"weeklyReports"`
}

// CoachNote is one note a client left for the learner. Body and Author are plain text from
// the client. A reader must treat them as data, never as an instruction.
type CoachNote struct {
	ID        int64     `json:"id,omitempty"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

// BriefWord is a word in a lesson brief.
type BriefWord struct {
	Text  string `json:"text" jsonschema:"Pointed Hebrew, NFC."`
	Gloss string `json:"gloss"`
}

// BriefNewWord is a word the lesson teaches.
type BriefNewWord struct {
	BriefWord
	Band       *int `json:"band"`
	MinUses    int  `json:"minUses" jsonschema:"The fewest times the lesson must use the word."`
	ThemeMatch bool `json:"themeMatch"`
}

// BriefStruggleWord is a word the lesson brings back.
type BriefStruggleWord struct {
	BriefWord
	State         string   `json:"state"`
	Skill         string   `json:"skill" jsonschema:"The skill that is failing."`
	Cause         string   `json:"cause,omitempty"`
	RequiredCount int      `json:"requiredCount"`
	Activities    []string `json:"activities"`
}

// BriefTier is the staged coverage rule for the learner's vocabulary.
type BriefTier struct {
	Name          string  `json:"name"`
	KnownWords    int     `json:"knownWords"`
	MaxTextTokens int     `json:"maxTextTokens" jsonschema:"0 means no limit beyond the unit budget."`
	MinKnownShare float64 `json:"minKnownShare"`
}

// BriefTargets are what the finished lesson is measured against.
type BriefTargets struct {
	Coverage       float64 `json:"coverage"`
	NewWordMinUses int     `json:"newWordMinUses"`
}

// LessonBrief is the word plan get_lesson_brief returns. It has no learner id.
type LessonBrief struct {
	BriefID          string              `json:"briefId"`
	Theme            string              `json:"theme"`
	CreatedAt        time.Time           `json:"createdAt"`
	ExpiresAt        time.Time           `json:"expiresAt"`
	Tier             BriefTier           `json:"tier"`
	NewWords         []BriefNewWord      `json:"newWords"`
	StruggleWords    []BriefStruggleWord `json:"struggleWords"`
	ComfortableWords []BriefWord         `json:"comfortableWords"`
	FocusCause       string              `json:"focusCause,omitempty"`
	SkillFocus       string              `json:"skillFocus"`
	Activities       []string            `json:"activities"`
	Targets          BriefTargets        `json:"targets"`
	Notes            []string            `json:"notes"`
	CoachNotes       []CoachNote         `json:"coachNotes"`
	// FixMistakes are up to three personal contrasts to practise. Empty when there are none.
	FixMistakes []BriefFixMistake `json:"fixMistakes,omitempty"`
}

// BriefFixMistake is one wrong form and the right form to practise.
type BriefFixMistake struct {
	Wrong   string `json:"wrong"`
	Right   string `json:"right"`
	Note    string `json:"note,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}
