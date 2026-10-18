package path

import (
	"sort"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/learner/config"
)

// assemble computes path states from a parsed published plan, the set of
// finished lesson slugs, and the due count. It does not touch the database.
//
// Lesson state, in order: done if the lesson is finished; otherwise not-yet
// if the unit is soft-gated; otherwise current if it is the single frontier
// lesson; otherwise next.
//
// The frontier is the first unfinished lesson, in path order, whose unit is
// neither passed nor not-yet. Unfinished lessons in a passed unit are next.
// They are not the frontier: the unit was already passed by its checkpoint
// (test out) or by finishing every lesson.
//
// There is one current lesson. Every later reachable unfinished lesson is
// next. ADR-0017 and RSCH-010 describe the screen naming one next lesson,
// and the state list has no sixth value for "later". The first next node
// is that named lesson.
//
// A review stop is inserted after the current lesson when the due count is
// at least review_stop_due_count. After a stop, the next
// review_stop_gap_lessons lessons do not get another stop. The following
// lesson can. Stops are not stored. No current lesson means no stop. The
// walk includes lessons in not-yet units, because §4 says the stop sits on
// the path and does not say to skip a soft-gated unit.
func assemble(doc planDoc, finished map[string]bool, due int, cfg config.Config) Path {
	ordered := append([]unitDoc(nil), doc.Units...)
	sortUnits(ordered)

	passed := make([]bool, len(ordered))
	notYet := make([]bool, len(ordered))
	for i := range ordered {
		passed[i] = unitPassed(ordered[i], finished)
	}
	for i := range ordered {
		notYet[i] = prereqOpen(ordered, passed, ordered[i].Prerequisites)
	}

	currentU, currentL := -1, -1
	for i := range ordered {
		if passed[i] || notYet[i] {
			continue
		}
		for j := range ordered[i].Lessons {
			if !lessonFinished(ordered[i].Lessons[j], finished) {
				currentU, currentL = i, j
				break
			}
		}
		if currentU >= 0 {
			break
		}
	}

	out := Path{
		PlanID:        doc.ID,
		Title:         doc.Title,
		DueCount:      due,
		ReviewStopDue: cfg.ReviewStopDueCount,
		ReviewStopGap: cfg.ReviewStopGapLessons,
		WarmupLimit:   cfg.LessonWarmupDue,
		PassMark:      cfg.CheckpointPassMark,
		Units:         make([]Unit, 0, len(ordered)),
		Nodes:         []Node{},
	}
	if out.PassMark == "" {
		out.PassMark = config.DefaultCheckpointPassMark
	}

	type slot struct {
		u, l int
	}
	var flats []slot
	for i := range ordered {
		for j := range ordered[i].Lessons {
			flats = append(flats, slot{i, j})
		}
	}
	currentFlat := -1
	for n, slot := range flats {
		if slot.u == currentU && slot.l == currentL {
			currentFlat = n
			break
		}
	}

	sinceStop := -1
	flatN := 0
	for i := range ordered {
		u := ordered[i]
		keys := unitWordKeys(u)
		unit := Unit{
			ID:                 u.ID,
			Title:              u.Title,
			Theme:              u.Theme,
			Prerequisites:      append([]string(nil), u.Prerequisites...),
			Source:             u.Source,
			TutorClassDate:     u.TutorClassDate,
			Passed:             passed[i],
			NotYet:             notYet[i],
			State:              unitState(i, passed[i], notYet[i], currentU),
			NewWordsIntroduced: len(keys),
			NewWordsDone:       doneWordCount(keys, u, finished),
			Lessons:            make([]Lesson, 0, len(u.Lessons)),
		}
		if unit.Prerequisites == nil {
			unit.Prerequisites = []string{}
		}
		if unit.Source == "" {
			unit.Source = "core"
		}
		for j := range u.Lessons {
			lesson := u.Lessons[j]
			finishedLesson := lessonFinished(lesson, finished)
			state := lessonState(finishedLesson, notYet[i], i == currentU && j == currentL)
			view := Lesson{
				ID:       lesson.ID,
				Title:    lesson.Title,
				NewWords: lessonWordKeys(lesson),
				Recycles: lesson.Recycles,
				Done:     finishedLesson,
				State:    state,
			}
			if view.NewWords == nil {
				view.NewWords = []string{}
			}
			if view.Recycles == nil {
				view.Recycles = []Recycle{}
			}
			unit.Lessons = append(unit.Lessons, view)
			out.Nodes = append(out.Nodes, Node{
				Kind:     KindLesson,
				State:    state,
				PlanID:   doc.ID,
				UnitID:   u.ID,
				LessonID: lesson.ID,
				Title:    lesson.Title,
			})
			if currentFlat >= 0 && flatN >= currentFlat && due >= cfg.ReviewStopDueCount {
				if sinceStop < 0 {
					if flatN == currentFlat {
						out.Nodes = append(out.Nodes, stopNode(doc.ID, u.ID, lesson.ID))
						sinceStop = 0
					}
				} else {
					sinceStop++
					if sinceStop > cfg.ReviewStopGapLessons {
						out.Nodes = append(out.Nodes, stopNode(doc.ID, u.ID, lesson.ID))
						sinceStop = 0
					}
				}
			}
			flatN++
		}
		out.Units = append(out.Units, unit)
	}
	return out
}

func stopNode(planID, unitID, afterLesson string) Node {
	return Node{
		Kind:          KindReviewStop,
		State:         StateReviewStop,
		PlanID:        planID,
		UnitID:        unitID,
		AfterLessonID: afterLesson,
	}
}

func unitState(index int, passed, notYet bool, current int) State {
	if passed {
		return StateDone
	}
	if notYet {
		return StateNotYet
	}
	if index == current {
		return StateCurrent
	}
	return StateNext
}

func lessonState(finished, notYet, current bool) State {
	if finished {
		return StateDone
	}
	if notYet {
		return StateNotYet
	}
	if current {
		return StateCurrent
	}
	return StateNext
}

func sortUnits(units []unitDoc) {
	anyOrder := false
	for i := range units {
		if units[i].HasOrder {
			anyOrder = true
			break
		}
	}
	if !anyOrder {
		return
	}
	sort.SliceStable(units, func(i, j int) bool {
		oi, oj := 0, 0
		if units[i].HasOrder {
			oi = units[i].Order
		}
		if units[j].HasOrder {
			oj = units[j].Order
		}
		if oi != oj {
			return oi < oj
		}
		return units[i].Index < units[j].Index
	})
}

func unitPassed(u unitDoc, finished map[string]bool) bool {
	if len(u.Lessons) == 0 {
		return false
	}
	checkpoints := 0
	checkpointsDone := 0
	allDone := true
	for _, lesson := range u.Lessons {
		done := lessonFinished(lesson, finished)
		if !done {
			allDone = false
		}
		if lesson.Checkpoint {
			checkpoints++
			if done {
				checkpointsDone++
			}
		}
	}
	if checkpoints > 0 {
		return checkpointsDone == checkpoints
	}
	return allDone
}

func prereqOpen(units []unitDoc, passed []bool, prereqs []string) bool {
	for _, id := range prereqs {
		if !idPassed(units, passed, id) {
			return true
		}
	}
	return false
}

func idPassed(units []unitDoc, passed []bool, id string) bool {
	for i := range units {
		if units[i].ID == id && passed[i] {
			return true
		}
	}
	return false
}

func lessonFinished(lesson lessonDoc, finished map[string]bool) bool {
	if finished[lesson.ID] {
		return true
	}
	return lesson.LegacyKey != "" && lesson.LegacyKey != lesson.ID && finished[lesson.LegacyKey]
}

func unitWordKeys(u unitDoc) []string {
	if u.HasIntroduces {
		return uniqueKeys(u.Introduces)
	}
	var flat []string
	for _, lesson := range u.Lessons {
		flat = append(flat, lesson.NewWords...)
	}
	return uniqueKeys(flat)
}

func lessonWordKeys(lesson lessonDoc) []string {
	if lesson.HasIntroduces {
		return uniqueKeys(lesson.Introduces)
	}
	return uniqueKeys(lesson.NewWords)
}

func doneWordCount(keys []string, u unitDoc, finished map[string]bool) int {
	have := map[string]struct{}{}
	for _, lesson := range u.Lessons {
		if !lessonFinished(lesson, finished) {
			continue
		}
		for _, key := range lessonWordKeys(lesson) {
			have[key] = struct{}{}
		}
	}
	n := 0
	for _, key := range keys {
		if _, ok := have[key]; ok {
			n++
		}
	}
	return n
}

func uniqueKeys(words []string) []string {
	if len(words) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(words))
	for _, word := range words {
		key := ExactKey(word)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}
