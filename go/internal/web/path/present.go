package path

import (
	"encoding/json"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/config"
	lpath "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/path"
)

// Present turns a learner Path into the snapshot the templates render.
// Node states on the Path stay done, current, next, not-yet, or review-stop.
// The visual state (checkpoint, tutor, review) is display chrome.
func Present(cfg config.Config, course lpath.Path, due int, dueKnown bool) Snapshot {
	snap := Snapshot{
		PlanID: course.PlanID, UnitCount: fixtureCourseUnits, Due: due, DueKnown: dueKnown,
		WarmupDue: cfg.LessonWarmupDue, ReviewStopDue: cfg.ReviewStopDueCount,
	}
	snap.DueEn, snap.DueHe = DueExplainer(cfg.LessonWarmupDue, cfg.ReviewStopDueCount)
	if dueKnown && due > 0 {
		snap.DueWords = demoDueWords()
	}
	lessons := map[string]lpath.Lesson{}
	for _, u := range course.Units {
		for _, lesson := range u.Lessons {
			lessons[u.ID+"\x00"+lesson.ID] = lesson
		}
	}
	var current *Node
	var currentUnit *Unit
	for i, u := range course.Units {
		view := viewUnit(i+1, u)
		for _, n := range course.Nodes {
			if n.UnitID != u.ID {
				continue
			}
			view.Nodes = append(view.Nodes, viewNode(cfg, u, n, lessons[u.ID+"\x00"+n.LessonID], due))
		}
		if view.Current {
			currentUnit = &view
		}
		for ni := range view.Nodes {
			if view.Nodes[ni].State == "current" {
				current = &view.Nodes[ni]
			}
		}
		snap.Units = append(snap.Units, view)
		if u.Source == "tutor" {
			face := unitChrome(u.ID)
			snap.Tutor = &TutorCard{
				UnitID: u.ID, DateLabel: face.DateLabel, DateHe: face.DateHe,
				DateISO: u.TutorClassDate, SentenceEn: face.SentenceEn, SentenceHe: face.SentenceHe,
				Phrase: face.Phrase, PhraseEn: face.PhraseEn, Href: face.TutorHref,
			}
		}
	}
	if currentUnit != nil {
		wordsEn, wordsHe := UnitWords(currentUnit.NewLearned, currentUnit.NewTotal)
		snap.Rail = Rail{
			UnitID: currentUnit.ID, UnitN: currentUnit.Order,
			Learned: currentUnit.NewLearned, Total: currentUnit.NewTotal,
			WordsEn: wordsEn, WordsHe: wordsHe,
		}
		snap.Strip = append([]Node(nil), currentUnit.Nodes...)
		stops := stopsUntilCheckpoint(currentUnit.Nodes)
		snap.StripEn = fmt.Sprintf("%d of %d new words · checkpoint in %d stops", currentUnit.NewLearned, currentUnit.NewTotal, stops)
		snap.StripHe = fmt.Sprintf("%d מתוך %d מילים חדשות · מִבְחָן בְּעוֹד %d תַּחֲנוֹת", currentUnit.NewLearned, currentUnit.NewTotal, stops)
	}
	if current != nil && currentUnit != nil {
		snap.HeroTitleHe = current.TitleHe
		snap.HeroSpeak = current.TitleHe
		snap.ContinueHref = current.Href
		snap.HeroKickerEn = fmt.Sprintf("Next on your path · Unit %d · Lesson %d", currentUnit.Order, current.LessonNumber)
		snap.HeroKickerHe = fmt.Sprintf("הַבָּא בַּמַּסְלוּל · יחידה %d · שיעור %d", currentUnit.Order, current.LessonNumber)
		snap.HeroMetaEn = fmt.Sprintf("%d new, %d you know", current.NewCount, current.KnownCount)
		snap.HeroMetaHe = fmt.Sprintf("%d חדשות, %d מוכרות", current.NewCount, current.KnownCount)
	}
	return snap
}

func viewUnit(order int, u lpath.Unit) Unit {
	face := unitChrome(u.ID)
	view := Unit{
		ID: u.ID, Order: order, Title: u.Title, TitleHe: face.TitleHe,
		Concept: face.Concept, ConceptHe: face.ConceptHe,
		NewLearned: u.NewWordsDone, NewTotal: u.NewWordsIntroduced,
		NotYet: u.NotYet, Current: u.State == lpath.StateCurrent,
		LessonCount: len(u.Lessons), GuideHref: face.GuideHref,
	}
	view.BannerEn, view.BannerHe = UnitWordsShort(view.NewLearned, view.NewTotal)
	for i, lesson := range u.Lessons {
		if lesson.State == lpath.StateCurrent {
			view.LessonIndex = i + 1
			view.Current = true
		}
	}
	if view.Current {
		count := fixtureCourseUnits
		view.KickerEn = fmt.Sprintf("Unit %d of %d · Lesson %d of %d", order, count, view.LessonIndex, view.LessonCount)
		view.KickerHe = fmt.Sprintf("יחידה %d מתוך %d · שיעור %d מתוך %d", order, count, view.LessonIndex, view.LessonCount)
	}
	return view
}

func viewNode(cfg config.Config, u lpath.Unit, n lpath.Node, lesson lpath.Lesson, due int) Node {
	face := lessonFace(n.LessonID)
	number := 0
	for i, item := range u.Lessons {
		if item.ID == n.LessonID {
			number = i + 1
			break
		}
	}
	href := face.Href
	if href == "" && n.LessonID != "" {
		href = "/worksheets/" + n.LessonID + "/"
	}
	titleHe := face.He
	if titleHe == "" {
		titleHe = n.Title
	}
	view := Node{
		Slug: n.LessonID, Title: n.Title, TitleHe: titleHe, UnitID: u.ID,
		NewCount: len(lesson.NewWords), KnownCount: face.Known, Strength: face.Strength,
		Fading: face.Fading, Href: href, LessonNumber: number,
	}
	switch {
	case n.Kind == lpath.KindReviewStop || n.State == lpath.StateReviewStop:
		view.Kind = "review"
		view.State = "review"
		view.Title = "Review"
		view.TitleHe = "חזרה"
		view.Due = due
		view.Href = "/review/"
	case u.Source == "tutor":
		face := unitChrome(u.ID)
		view.Kind = "tutor"
		view.State = "tutor"
		view.Gate = "open"
		view.ClassDate = face.DateLabel
		view.ClassDateHe = face.DateHe
		view.ClassDateISO = u.TutorClassDate
	case face.Checkpoint:
		view.Kind = "checkpoint"
		view.State = "checkpoint"
		view.Total = face.Total
		view.Href = face.Href
		view.CheckpointHref = face.Href
		view.Gate = gateFor(n.State)
		if need, err := cfg.PassNeeded(face.Total); err == nil {
			view.PassNeeded = need
		}
	default:
		view.Kind = "lesson"
		view.State = visualState(n.State)
		view.Gate = gateFor(n.State)
	}
	if view.Kind == "lesson" || view.Kind == "checkpoint" {
		if u.ID == "unit-2" {
			view.CheckpointHref = "/worksheets/out-and-about-checkpoint/"
		}
		if u.ID == "unit-3" {
			view.CheckpointHref = "/worksheets/cafe-checkpoint/"
		}
	}
	view.Label = nodeLabel(view)
	view.SubEn, view.SubHe = nodeSub(view)
	if view.Gate == "warn" {
		view.HXVals = hxVals(view.Href, view.CheckpointHref)
	}
	return view
}

func visualState(state lpath.State) string {
	switch state {
	case lpath.StateDone:
		return "done"
	case lpath.StateCurrent:
		return "current"
	default:
		return "not-yet"
	}
}

func gateFor(state lpath.State) string {
	switch state {
	case lpath.StateNotYet:
		return "warn"
	case lpath.StateNext:
		return "open"
	default:
		return ""
	}
}

func nodeLabel(n Node) string {
	if n.State == "not-yet" {
		if n.LessonNumber > 0 && n.Title != "" {
			return fmt.Sprintf("Lesson %d, %s, not yet started", n.LessonNumber, n.Title)
		}
		if n.Title != "" {
			return n.Title + ", not yet started"
		}
		return "not yet started"
	}
	if n.State == "current" {
		return fmt.Sprintf("Lesson %d, %s, current, %d new words", n.LessonNumber, n.Title, n.NewCount)
	}
	switch n.Kind {
	case "review":
		return fmt.Sprintf("Review, %d words due", n.Due)
	case "checkpoint":
		if n.Score != nil {
			return fmt.Sprintf("Checkpoint, %d of %d", *n.Score, n.Total)
		}
		return "Checkpoint, Test out"
	case "tutor":
		if n.ClassDate != "" {
			return n.Title + ", " + n.ClassDate
		}
		return n.Title
	default:
		if n.LessonNumber > 0 {
			return fmt.Sprintf("Lesson %d, %s", n.LessonNumber, n.Title)
		}
		return n.Title
	}
}

func nodeSub(n Node) (string, string) {
	switch n.Kind {
	case "review":
		return fmt.Sprintf("%d words due", n.Due), fmt.Sprintf("%d מילים ממתינות", n.Due)
	case "checkpoint":
		if n.Score != nil {
			return fmt.Sprintf("%d of %d", *n.Score, n.Total), fmt.Sprintf("%d מתוך %d", *n.Score, n.Total)
		}
		return "Test out", "מִבְחַן יְצִיאָה"
	case "tutor":
		return n.ClassDate, n.ClassDateHe
	default:
		return fmt.Sprintf("%d new · %d known", n.NewCount, n.KnownCount), fmt.Sprintf("%d חדשות · %d מוכרות", n.NewCount, n.KnownCount)
	}
}

func hxVals(next, checkpoint string) string {
	raw, err := json.Marshal(map[string]string{
		"next":       next,
		"checkpoint": checkpoint,
	})
	if err != nil {
		return ""
	}
	return string(raw)
}

func stopsUntilCheckpoint(nodes []Node) int {
	seen := false
	n := 0
	for _, node := range nodes {
		if node.State == "current" {
			seen = true
			continue
		}
		if !seen {
			continue
		}
		if node.Kind == "checkpoint" {
			return n
		}
		n++
	}
	return n
}

// UnfinishedUnit is the earlier unit named in the soft-gate warning.
// The unit being opened is unitID. The copy names the unit before it.
func UnfinishedUnit(course lpath.Path, unitID string) (int, bool) {
	for i, u := range course.Units {
		if u.ID != unitID {
			continue
		}
		if i > 0 {
			return i, true
		}
		return i + 1, true
	}
	return 0, false
}

// StrengthName is the pip bucket. 1 fragile, 2 shaky, 3 steady, 4 solid.
func StrengthName(bucket int) (en, he string) {
	switch bucket {
	case 1:
		return "fragile", "שביר"
	case 2:
		return "shaky", "רופס"
	case 3:
		return "steady", "יציב"
	case 4:
		return "solid", "איתן"
	default:
		return "empty", "ריק"
	}
}

// UnitWords is the rail line. UnitWordsShort is the banner line.
// Both use the same learned and total counts as the ring.
func UnitWords(learned, total int) (en, he string) {
	return fmt.Sprintf("%d of %d new words this unit", learned, total),
		fmt.Sprintf("%d מתוך %d מילים חדשות ביחידה", learned, total)
}

// UnitWordsShort is the banner line under the progress bar.
func UnitWordsShort(learned, total int) (en, he string) {
	return fmt.Sprintf("%d of %d new words", learned, total),
		fmt.Sprintf("%d מתוך %d מילים חדשות", learned, total)
}

// DueExplainer uses the config counts. The template does not embed 3 or 8.
func DueExplainer(warmup, stop int) (en, he string) {
	return fmt.Sprintf("%d go into the lesson warm-up; a review stop appears at %d+ due", warmup, stop),
		fmt.Sprintf("%d נכנסות לחימום השיעור; עצירת חזרה מופיעה מ-%d ממתינות", warmup, stop)
}
