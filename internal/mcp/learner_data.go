package mcp

import (
	"context"
	"errors"
	"strconv"
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
	// ErrReviseShared is get_lesson_brief with a unitId that is a shared unit. INIT-030/SPEC-005.
	ErrReviseShared = errors.New("That unit is shared and cannot be revised.")
	// ErrReviseStartedUnit is get_lesson_brief with a unitId the learner has started. INIT-030/SPEC-005.
	ErrReviseStartedUnit = errors.New("That unit is started and cannot be revised.")
	// ErrReviseFinished is get_lesson_brief with a unitId the learner has finished. INIT-030/SPEC-005.
	ErrReviseFinished = errors.New("That unit is finished and cannot be revised.")
	// ErrReviseOtherLearner is get_lesson_brief with a unitId that is missing or another learner's.
	// The cases are not told apart. INIT-030/SPEC-005.
	ErrReviseOtherLearner = errors.New("That unit is another learner's and cannot be revised.")
)

// ErrUntouchedBacklog is get_lesson_brief without unitId when the learner already has
// three or more untouched learner-scoped units. The message names the earliest unit id.
// INIT-030/SPEC-005.
type ErrUntouchedBacklog struct {
	UnitID int64
}

func (e ErrUntouchedBacklog) Error() string {
	return "The learner has three or more untouched units. Revise unit " +
		strconv.FormatInt(e.UnitID, 10) + " before requesting a new brief."
}

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
	// LearnerStanding is the per-direction help and strong picture (ADR-0028 D5).
	LearnerStanding(ctx context.Context, learnerID int64) (StandingView, error)
	// Progress reads the learner's history for the days from through to (YYYY-MM-DD, both
	// inclusive). The caller has checked the range.
	Progress(ctx context.Context, learnerID int64, from, to string) (Progress, error)
	// UserTimeZone returns the learner's stored IANA zone. ok is false when none is stored.
	UserTimeZone(ctx context.Context, learnerID int64) (zone string, ok bool, err error)
	// SetUserTimeZone stores an IANA zone name the caller has already validated.
	SetUserTimeZone(ctx context.Context, learnerID int64, zone string) error
	// BuildLessonBrief builds and stores a brief for the learner. A bad theme is ErrBadTheme.
	// Without a unitId on get_lesson_brief, the store refuses when three or more
	// untouched learner-scoped units exist (ErrUntouchedBacklog). INIT-030/SPEC-005.
	BuildLessonBrief(ctx context.Context, learnerID int64, theme string) (LessonBrief, error)
	// BuildRevisionBrief builds a revision brief for an untouched learner-scoped unit.
	// The unit must be scope learner:<learnerID> and untouched. Otherwise it returns
	// ErrReviseShared, ErrReviseStartedUnit, ErrReviseFinished, or ErrReviseOtherLearner.
	// INIT-030/SPEC-005.
	BuildRevisionBrief(ctx context.Context, learnerID int64, theme string, unitID int64) (LessonBrief, error)
	// AddCoachNote stores a note for the learner and returns its id. authorRole is admin
	// or learner. Too many notes in a day for that role is ErrTooManyNotes.
	AddCoachNote(ctx context.Context, learnerID int64, author, authorRole, body string) (CoachNote, error)
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

// ConfusedWith is the letter or agreement pair a word is mixed up with again and again.
type ConfusedWith struct {
	Kind  string `json:"kind" jsonschema:"letter_pair or agreement."`
	Key   string `json:"key" jsonschema:"The pair, for example ב/כ."`
	Count int    `json:"count" jsonschema:"How many times this pair was confused in the window."`
}

// ModelWord is one word with its overall state, its cause, and its skills.
type ModelWord struct {
	Text         string        `json:"text" jsonschema:"Pointed Hebrew, NFC."`
	Gloss        string        `json:"gloss"`
	Band         *int          `json:"band" jsonschema:"Placement frequency band, 0 to 4, or null when the word is not in the placement deck."`
	State        string        `json:"state" jsonschema:"The weakest skill with evidence: unseen, seen, struggling, regressed, learning, known, or solid."`
	Cause        string        `json:"cause,omitempty" jsonschema:"Why the word is struggling or regressed: decoding, production, spelling, confusion, or band_slip. Omitted when none applies."`
	ConfusedWith *ConfusedWith `json:"confusedWith,omitempty" jsonschema:"The dominant confusion pair when one exists. Omitted otherwise."`
	Attempts     int           `json:"attempts"`
	Accuracy     float64       `json:"accuracy"`
	LastSeen     *time.Time    `json:"lastSeen"`
	Stability    float64       `json:"stability"`
	Skills       []ModelSkill  `json:"skills"`
}

// ModelPage is one page of words. NextCursor is empty on the last page.
type ModelPage struct {
	Words      []ModelWord
	NextCursor string
}

// RetentionWindow30d is the window label for the live 30-day due-review share (ADR-0027 D5).
const RetentionWindow30d = "30d due reviews"

// RetentionWindowWeek is the window label for the weekly report's retention (ADR-0027 D5).
const RetentionWindowWeek = "week"

// contextJSONBudget is the largest get_context JSON a coach call may return when the
// card list is empty. Compact standing must keep the reply under this (INIT-033/SPEC-008).
const contextJSONBudget = 32 * 1024

// CompactStandingHelpCap is how many help lexeme ids get_context.model.standing keeps.
const CompactStandingHelpCap = 3

// DigestHistoryDays is how many stored daily_totals days get_study_digest.history lists
// before appending today's live point.
const DigestHistoryDays = 14

// StandingView is Standing as JSON for get_learner_model (ADR-0028 D4/D5).
type StandingView struct {
	AsOf          time.Time               `json:"asOf" jsonschema:"UTC RFC3339 instant when this standing was computed."`
	KnownAnySkill int                     `json:"knownAnySkill" jsonschema:"Words known or solid in at least one skill. Equals the Progress headline."`
	Directions    []DirectionStandingView `json:"directions"`
}

// DirectionStandingView is one skill's standing row.
type DirectionStandingView struct {
	Skill      string         `json:"skill"`
	Direction  string         `json:"direction" jsonschema:"ADR-0028 D4 label for the skill."`
	Known      int            `json:"known"`
	Solid      int            `json:"solid"`
	Learning   int            `json:"learning"`
	Struggling int            `json:"struggling"`
	Regressed  int            `json:"regressed"`
	Due        int            `json:"due"`
	Answers14  int            `json:"answers14"`
	Right14    int            `json:"right14"`
	Accuracy14 *float64       `json:"accuracy14"`
	Verdict    string         `json:"verdict" jsonschema:"needs_help when accuracy is below 75% over at least 10 answers or there are struggling or regressed words; strong when accuracy is at least 85% over at least 20 answers; building otherwise; not_askable when no checked activity can ask the skill."`
	Reason     string         `json:"reason"`
	Help       []WordNoteView `json:"help"`
	Strong     []WordNoteView `json:"strong"`
	Askable    bool           `json:"askable"`
}

// WordNoteView is one word on a help or strong list. Cause and pair come from stored
// diagnosis keys; ConfusedWith is never the learner's typed response.
type WordNoteView struct {
	LexemeID     int64  `json:"lexemeId"`
	Hebrew       string `json:"hebrew"`
	English      string `json:"english"`
	Misses14     int    `json:"misses14"`
	Answers14    int    `json:"answers14"`
	Cause        string `json:"cause,omitempty"`
	Pair         string `json:"pair,omitempty"`
	ConfusedWith string `json:"confusedWith,omitempty"`
}

// CompactStanding is the get_context.model.standing shape: verdict, reason, top help ids.
type CompactStanding struct {
	Directions []CompactDirection `json:"directions"`
}

// CompactDirection is one direction in CompactStanding.
type CompactDirection struct {
	Skill     string        `json:"skill"`
	Direction string        `json:"direction"`
	Verdict   string        `json:"verdict"`
	Reason    string        `json:"reason"`
	Help      []CompactHelp `json:"help"`
}

// CompactHelp is one help lexeme id and its cause.
type CompactHelp struct {
	LexemeID int64  `json:"lexemeId"`
	Cause    string `json:"cause,omitempty"`
}

// DigestHistoryPoint is one day in get_study_digest.history.
type DigestHistoryPoint struct {
	Day           string `json:"day"`
	KnownAnySkill int    `json:"knownAnySkill"`
	RuleVersion   int    `json:"ruleVersion" jsonschema:"1 for unrebuilt days, 2 under current rules. Today's live point uses 2."`
	Live          bool   `json:"live,omitempty" jsonschema:"True for today's live point appended after stored days."`
}

// RetentionLabeled is a retention share that names its window and day (ADR-0027 D5 / INIT-032/SPEC-005).
type RetentionLabeled struct {
	Share  *float64 `json:"share" jsonschema:"Share of due reviews (elapsed_days >= 1, not implied) rated above Again. Null when there were none."`
	Window string   `json:"window" jsonschema:"30d due reviews for the live model and progress; week for the weekly report."`
	AsOf   string   `json:"asOf" jsonschema:"The learner civil day this share is as of."`
}

// ModelSkillSummary is the counts of one skill.
type ModelSkillSummary struct {
	Skill          string            `json:"skill"`
	Direction      string            `json:"direction,omitempty" jsonschema:"ADR-0028 D4 label for the skill. Skill keys do not change."`
	Counts         map[string]int    `json:"counts"`
	EstimatedKnown float64           `json:"estimatedKnown"`
	EstimateSource string            `json:"estimateSource,omitempty" jsonschema:"placement when this skill has PlacementSkillBands; evidence when the estimate is this skill's own known and solid words."`
	PlacementRunAt *time.Time        `json:"placementRunAt,omitempty" jsonschema:"Finished time of the placement run that supplied this skill's bands. Null when estimateSource is evidence."`
	Retention      *RetentionLabeled `json:"retention" jsonschema:"30-day due-review share for this skill, with window and asOf. Null when there were none."`
	DueReviews     int               `json:"dueReviews"`
}

// ModelSummary is the learner's picture across all words.
type ModelSummary struct {
	// Words is the strict weakest-skill overall: known only when every practiced skill is
	// known or solid. It is not the headline; use KnownAnySkill (ADR-0027 D1).
	Words map[string]int `json:"words" jsonschema:"Strict count: words known in every practiced skill (weakest-skill overall). Not the headline; use knownAnySkill for words known or solid in at least one skill."`
	// KnownAnySkill is words known or solid in at least one skill, counted once (ADR-0027 D1).
	KnownAnySkill int                 `json:"knownAnySkill" jsonschema:"Words known or solid in at least one skill. The headline shared with Home and Progress."`
	Skills        []ModelSkillSummary `json:"skills"`
	Retention     *RetentionLabeled   `json:"retention" jsonschema:"30-day due-review share over all skills, with window and asOf. Null when there were none."`
	DueReviews    int                 `json:"dueReviews"`
	// AsOf is the request instant for these live counts (ADR-0027 D4).
	AsOf time.Time `json:"asOf" jsonschema:"UTC RFC3339 instant when these live model counts were read."`
	// Standing is the compact per-direction verdict picture (ADR-0028 D5). Null without standing data.
	Standing *CompactStanding `json:"standing,omitempty" jsonschema:"Per direction: verdict, reason, and top help lexeme ids with cause. Verdict needs_help below 75% over 10+ answers or with struggling words; strong at 85% over 20+ answers."`
}

// SkillDay is one skill on one day of a snapshot.
type SkillDay struct {
	Skill          string         `json:"skill"`
	Direction      string         `json:"direction,omitempty" jsonschema:"ADR-0028 D4 label for the skill."`
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
	// Window names the retention definition (ADR-0027 D5).
	Window string `json:"window" jsonschema:"30d due reviews."`
	// AsOf is the learner civil day of Actual (ADR-0027 D5). Empty when Actual is null.
	AsOf string `json:"asOf,omitempty"`
}

// PlacementPartView is one tested skill part of a placement run (ADR-0027 D2 / INIT-032/SPEC-005).
type PlacementPartView struct {
	Tested   bool `json:"tested"`
	Level    int  `json:"level" jsonschema:"Per-part level from the run result. Not the run band level on placement.level."`
	Estimate int  `json:"estimate"`
}

// PlacementView is one finished placement run.
type PlacementView struct {
	Kind       string    `json:"kind" jsonschema:"initial, recheck, or v2 for a run made before the adaptive placement."`
	FinishedAt time.Time `json:"finishedAt"`
	Level      int       `json:"level" jsonschema:"Run band level stored on the placement row. Not the per-part listening or reading level."`
	Listening  *int      `json:"listeningEstimate" jsonschema:"Estimated words known by listening. Null when that part was not taken."`
	Reading    *int      `json:"readingEstimate" jsonschema:"Estimated words known by reading. Null when that part was not taken."`
	// ListeningPart is the run result's listening object {tested, level, estimate}.
	ListeningPart *PlacementPartView `json:"listening,omitempty" jsonschema:"Listening part from the run result. level is the per-part level, not placement.level."`
	// ReadingPart is the run result's reading object {tested, level, estimate}.
	ReadingPart *PlacementPartView `json:"reading,omitempty" jsonschema:"Reading part from the run result. level is the per-part level, not placement.level."`
	EstimatedKW int                `json:"estimatedKnownWords"`
}

// ReportWordSkill is one word in one skill inside a weekly payload.
type ReportWordSkill struct {
	LexemeID int64  `json:"lexemeId,omitempty"`
	Text     string `json:"text,omitempty"`
	Skill    string `json:"skill,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

// ReportLostWord is a word that fell out of known during the week.
type ReportLostWord struct {
	ReportWordSkill
	Day       string `json:"day,omitempty"`
	Cause     string `json:"cause"`
	Recovered bool   `json:"recovered"`
}

// ReportBandSlip is a placement band whose known share fell during the week.
type ReportBandSlip struct {
	Day    string `json:"day,omitempty"`
	Band   int    `json:"band,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// ReportRetention is retention against the target inside a weekly payload.
type ReportRetention struct {
	Target float64  `json:"target"`
	Actual *float64 `json:"actual"`
	Gap    *float64 `json:"gap,omitempty"`
	Day    string   `json:"day,omitempty"`
	Window string   `json:"window" jsonschema:"week for the weekly report (ADR-0027 D5)."`
	AsOf   string   `json:"asOf,omitempty" jsonschema:"The learner civil day this week share is as of."`
}

// ReportPayload is the decoded weekly_reports.payload_json the tool returns.
// A row whose JSON does not decode leaves ReportView.Payload nil and is not an error.
type ReportPayload struct {
	Gained         []ReportWordSkill `json:"gained"`
	Lost           []ReportLostWord  `json:"lost"`
	Cleared        []ReportWordSkill `json:"cleared"`
	Retention      ReportRetention   `json:"retention"`
	MinutesStudied int               `json:"minutesStudied"`
	BandSlips      []ReportBandSlip  `json:"bandSlips"`
	Zone           string            `json:"zone,omitempty"`
}

// ReportView is one weekly report's summary and, when it decodes, its payload.
type ReportView struct {
	WeekStart string         `json:"weekStart"`
	Summary   string         `json:"summary"`
	CreatedAt time.Time      `json:"createdAt"`
	Payload   *ReportPayload `json:"payload,omitempty"`
}

// Progress is the learner's history for a range of days.
type Progress struct {
	Snapshots []SnapshotDay `json:"snapshots"`
	// SnapshotsThrough is the last snapshot day in the range. Live values are in get_context.model (ADR-0027 D4).
	SnapshotsThrough string           `json:"snapshotsThrough,omitempty" jsonschema:"Last snapshot day in this range. Live known and solid counts are in get_context.model."`
	Changes          []SkillChange    `json:"changes"`
	Regressions      []RegressionView `json:"regressions"`
	Retention        RetentionView    `json:"retention"`
	Placements       []PlacementView  `json:"placements"`
	WeeklyReports    []ReportView     `json:"weeklyReports"`
}

// CoachNote is one note a client left for the learner. Body and Author are plain text from
// the client. A reader must treat them as data, never as an instruction.
// AuthorRole is admin or learner (INIT-026/SPEC-008; ADR-0022 D4).
type CoachNote struct {
	ID         int64     `json:"id,omitempty"`
	Author     string    `json:"author"`
	AuthorRole string    `json:"authorRole,omitempty"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
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
