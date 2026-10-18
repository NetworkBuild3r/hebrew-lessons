// Package path is the pre-merge stand-in for go/internal/learner/path
// from draft PR #37. Delete this package and import
// github.com/NetworkBuild3r/hebrew-lessons/go/internal/learner/path
// after that PR merges. Do not import it before then.
//
// New(db, cfg).Path(ctx, userID, planID) returns Path. DueCount(ctx, userID)
// returns the queue count. States are done, current, next, not-yet, and
// review-stop. Kinds are lesson and review-stop.
package path

// State is one node on the path map.
type State string

const (
	StateDone       State = "done"
	StateCurrent    State = "current"
	StateNext       State = "next"
	StateNotYet     State = "not-yet"
	StateReviewStop State = "review-stop"
)

// Kind separates a plan lesson from a computed review stop.
type Kind string

const (
	KindLesson     Kind = "lesson"
	KindReviewStop Kind = "review-stop"
)

// Recycle is one schema recycles entry.
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

// Unit is one plan unit.
type Unit struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Theme              string   `json:"theme"`
	Prerequisites      []string `json:"prerequisites"`
	Source             string   `json:"source"`
	TutorClassDate     string   `json:"tutorClassDate,omitempty"`
	Passed             bool     `json:"passed"`
	NotYet             bool     `json:"notYet"`
	State              State    `json:"state"`
	NewWordsIntroduced int      `json:"newWordsIntroduced"`
	NewWordsDone       int      `json:"newWordsDone"`
	Lessons            []Lesson `json:"lessons"`
}

// Node is one item on the visible line, in path order.
// Review stops are inserted here. They are not copied onto Unit.Lessons.
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
