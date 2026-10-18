package path

// State is one node on the path map. The five values are the vocabulary in
// the path request: done, current, next, not-yet, and review-stop.
type State string

const (
	StateDone       State = "done"
	StateCurrent    State = "current"
	StateNext       State = "next"
	StateNotYet     State = "not-yet"
	StateReviewStop State = "review-stop"
)

// Kind separates a plan lesson from a computed review stop. A review stop
// is not a plan lesson and it has no slug (ADR-0017 §4).
type Kind string

const (
	KindLesson     Kind = "lesson"
	KindReviewStop Kind = "review-stop"
)

// Recycle is one schema 1.2.0 recycles entry. Word is an exact key.
type Recycle struct {
	Word         string `json:"word"`
	FromLessonID string `json:"fromLessonId"`
}

// Lesson is one plan lesson and the state computed for this learner.
type Lesson struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	NewWords []string  `json:"newWords"`
	Recycles []Recycle `json:"recycles"`
	Done     bool      `json:"done"`
	State    State     `json:"state"`
}

// Unit is one plan unit. Passed is derived. See the field comment.
type Unit struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Theme string `json:"theme"`

	// Prerequisites are unit ids. The field is optional. Schema 1.2.0 does
	// not declare it; ADR-0017 §2 adds it at 1.3.0. Absence means no
	// prerequisites. This reader is not a validator and does not reject a
	// snapshot for carrying the field.
	Prerequisites []string `json:"prerequisites"`

	// Source is "core" when the snapshot omits it (ADR-0017 §2). "tutor"
	// is passed through. TutorClassDate is the snapshot's tutor_class_date
	// when set. Ordering uses the order field, not the class date.
	Source         string `json:"source"`
	TutorClassDate string `json:"tutorClassDate,omitempty"`

	// Passed is the pre-cutover stand-in for unit_progress.passed_at.
	//
	// unit_progress is not created in this PR. ADR-0017 §9 says a finished
	// worksheet maps to a done node through the lesson progress the server
	// already stores, and that this mapping does not copy rows into
	// unit_progress. ADR-0017 §11 says the table lands after the Go cutover.
	//
	// A lesson is finished when lesson_progress.state_json has a JSON
	// number completedRounds greater than 0, including 0.5. That follows
	// server/learner/model.js journey (lessons_completed) for the values
	// the player stores. A JSON string is not finished, even though
	// Node's Number("2") would coerce. null is not finished. The slug match
	// is the lesson id, and also legacyStorageKey when that field is set,
	// because worksheet URLs stay /worksheets/<slug>/ (ADR-0017 §10).
	//
	// A unit is passed when every checkpoint lesson is finished. A
	// checkpoint lesson has kind "checkpoint" or a block of type
	// "spaced-review-checkpoint". kind is 1.3.0. On a 1.2.0 plan the block
	// type is what §9 calls "a lesson of the checkpoint activities the
	// player already has." Finishing the checkpoint marks the unit passed
	// even when earlier lessons are unfinished (ADR-0017 §4 test out).
	// When the unit has no checkpoint lesson, it is passed only when every
	// lesson is finished.
	//
	// NotYet is the soft gate: at least one prerequisite has no derived
	// pass. A missing prerequisite id counts as no pass. Passed and NotYet
	// are independent flags. State prefers Passed over NotYet.
	Passed bool  `json:"passed"`
	NotYet bool  `json:"notYet"`
	State  State `json:"state"`

	// NewWordsIntroduced is y in "x of y new words this unit".
	// NewWordsDone is x (ADR-0017 §8).
	//
	// y is the unit's introduces_words when that array is present. It is a
	// 1.3.0 field. On a 1.2.0 unit, y is the distinct newWords of the
	// unit's lessons. x is how many of those keys occur in the word list
	// of a finished lesson on this unit. A lesson that is not finished
	// does not add its words. The lesson word list is introduces_words
	// when that lesson sets the field, otherwise newWords.
	//
	// Keys are ExactKey values, first-seen order, duplicates dropped.
	// ExactKey is textkey.ExactKey: NFC, then the ECMAScript \s collapse,
	// then trim. See ExactKey.
	NewWordsIntroduced int `json:"newWordsIntroduced"`
	NewWordsDone       int `json:"newWordsDone"`

	Lessons []Lesson `json:"lessons"`
}

// Node is one item on the visible line, in path order. Review stops are
// inserted here. They are not copied onto Unit.Lessons.
//
// On a review stop, LessonID is empty and AfterLessonID is the lesson the
// stop follows. On a lesson, AfterLessonID is empty.
type Node struct {
	Kind          Kind   `json:"kind"`
	State         State  `json:"state"`
	PlanID        string `json:"planId,omitempty"`
	UnitID        string `json:"unitId,omitempty"`
	LessonID      string `json:"lessonId,omitempty"`
	Title         string `json:"title,omitempty"`
	AfterLessonID string `json:"afterLessonId,omitempty"`
}

// Path is the course line for one learner and one published plan.
//
// ReviewStopDue, ReviewStopGap, WarmupLimit, and PassMark are the config
// values used for this result. WarmupLimit is lesson_warmup_due. This
// service does not choose the warm-up cards.
type Path struct {
	PlanID        string `json:"planId"`
	Title         string `json:"title"`
	DueCount      int    `json:"dueCount"`
	ReviewStopDue int    `json:"reviewStopDue"`
	ReviewStopGap int    `json:"reviewStopGap"`
	WarmupLimit   int    `json:"warmupLimit"`
	PassMark      string `json:"passMark"`
	Units         []Unit `json:"units"`
	Nodes         []Node `json:"nodes"`
}
