package path

import (
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/config"
)

// DemoCourse is the fixture Path the dev server renders.
// due is the queue count copied onto Path.DueCount. A review-stop node
// is inserted after the current lesson when due meets the config threshold.
// The learner service applies the gap itself; this fixture inserts one stop.
func DemoCourse(cfg config.Config, due int) Path {
	units := []Unit{
		{
			ID: "unit-1", Title: "Getting around", Theme: "town", Source: "core",
			Passed: true, State: StateDone,
			NewWordsIntroduced: 6, NewWordsDone: 6,
			Lessons: []Lesson{
				demoLesson("hello", "Hello", StateDone, 2),
				demoLesson("what-i-have", "What I have", StateDone, 2),
				demoLesson("where-is-it", "Where is it", StateDone, 2),
			},
		},
		{
			ID: "unit-2", Title: "Out and about", Theme: "town", Source: "core",
			Prerequisites: []string{"unit-1"}, State: StateCurrent,
			NewWordsIntroduced: 8, NewWordsDone: 5,
			Lessons: []Lesson{
				demoLesson("morning", "Morning", StateDone, 2),
				demoLesson("the-street", "The street", StateDone, 3),
				demoLesson("out-the-door", "Out the door", StateCurrent, 3),
				demoLesson("at-the-shop", "At the shop", StateNext, 2),
				demoLesson("asking", "Asking", StateNext, 1),
				demoLesson("out-and-about-checkpoint", "Checkpoint", StateNext, 0),
			},
		},
		{
			ID: "unit-3", Title: "At the café", Theme: "café", Source: "core",
			Prerequisites: []string{"unit-2"}, NotYet: true, State: StateNotYet,
			NewWordsIntroduced: 8, NewWordsDone: 0,
			Lessons: []Lesson{
				demoLesson("coffee", "Coffee", StateNotYet, 2),
				demoLesson("the-bill", "The bill", StateNotYet, 2),
				demoLesson("cafe-checkpoint", "Checkpoint", StateNotYet, 0),
			},
		},
		{
			ID: "unit-tutor", Title: "Class with Aaron", Source: "tutor",
			TutorClassDate: "2026-10-20", State: StateNext,
			NewWordsIntroduced: 4, NewWordsDone: 0,
			Lessons: []Lesson{
				demoLesson("class-with-aaron", "Class with Aaron", StateNext, 0),
			},
		},
	}
	var nodes []Node
	for _, u := range units {
		for _, lesson := range u.Lessons {
			nodes = append(nodes, Node{
				Kind: KindLesson, State: lesson.State, PlanID: DemoPlanID,
				UnitID: u.ID, LessonID: lesson.ID, Title: lesson.Title,
			})
		}
	}
	if due >= cfg.ReviewStopDueCount {
		nodes = insertReviewStop(nodes, "unit-2", "out-the-door")
	}
	return Path{
		PlanID: DemoPlanID, Title: "Path demo", DueCount: due,
		ReviewStopDue: cfg.ReviewStopDueCount,
		ReviewStopGap: cfg.ReviewStopGapLessons,
		WarmupLimit:   cfg.LessonWarmupDue,
		PassMark:      cfg.CheckpointPassMark,
		Units:         units,
		Nodes:         nodes,
	}
}

func demoLesson(id, title string, state State, words int) Lesson {
	list := make([]string, words)
	for i := range list {
		list[i] = id + "-" + strconv.Itoa(i+1)
	}
	return Lesson{
		ID: id, Title: title, NewWords: list, Recycles: []Recycle{},
		Done: state == StateDone, State: state,
	}
}

func insertReviewStop(nodes []Node, unitID, afterLesson string) []Node {
	out := make([]Node, 0, len(nodes)+1)
	for _, n := range nodes {
		out = append(out, n)
		if n.LessonID == afterLesson {
			out = append(out, Node{
				Kind: KindReviewStop, State: StateReviewStop, PlanID: DemoPlanID,
				UnitID: unitID, Title: "Review", AfterLessonID: afterLesson,
			})
		}
	}
	return out
}
