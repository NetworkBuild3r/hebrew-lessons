package web

import (
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// LearnerVM is the signed-in learner shown on Today.
type LearnerVM struct {
	ID   int64
	Name string
}

// UnitRefVM is one shared unit the learner has not finished.
type UnitRefVM struct {
	ID       int64
	Position int
	Title    string
}

// PlacementVM is the learner's newest placement run.
// A nil pointer means the learner has never taken one.
// Active means an open run. Finished means the newest run has been scored.
type PlacementVM struct {
	Active              bool
	Finished            bool
	CardsRated          int
	Level               int
	EstimatedKnownWords int
	TakenAt             time.Time
}

// HomeVM is the Today page.
type HomeVM struct {
	Learner    LearnerVM
	DueCount   int
	NextUnit   *UnitRefVM
	KnownWords int
	Placement  *PlacementVM
}

// CardBackVM is the revealed face of a review card.
type CardBackVM struct {
	Gloss    string
	Translit string
	Gender   string
}

// ReviewCardVM is one visit in the study session.
// Kind is read, recall, listen, spell, letter, or pattern.
// Pointed is the pointed Hebrew. Unpointed is the same word with niqqud removed.
// ShowUnpointed follows fsrs.ShowUnpointed: read only, stability at least 21
// days, and no Again in the last three ratings. Translit is on the prompt,
// before the flip. Back keeps the gloss, the same transliteration, and gender.
type ReviewCardVM struct {
	CardID        int64
	Kind          string
	Pointed       string
	Unpointed     string
	ShowUnpointed bool
	Translit      string
	AudioURL      string
	Back          CardBackVM
	Index         int
	Total         int
	Retry         bool
	Done          bool
	Revealed      bool
	Answered      int
	Correct       int
	// Item is the checked activity this visit is asked through, with Answer set once
	// it has been checked. Both are nil for a flashcard and a drill.
	Item   *activity.Item
	Answer *answerState
	// Sides and Example* are the letter or pattern answer. reviewView copies them
	// onto the page only after the reveal.
	Sides           []views.DrillSide
	ExampleText     string
	ExampleTranslit string
	ExampleGloss    string
	// Asked is the one letter a pair card shows. TellApart and DageshNote are
	// empty when the stored data has no cue and no sounds. DageshHard is the
	// word-initial line for a single ב, כ, or פ, and for a dagesh pair of them.
	Asked      string
	TellApart  string
	DageshNote string
	DageshHard string
}
