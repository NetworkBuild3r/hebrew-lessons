package path

import (
	"encoding/json"
	"fmt"
)

// Present fills labels, the pass mark, and the Today lines from cfg.
// Node states and review stops stay as Path returned them.
func Present(cfg Config, snap Snapshot) Snapshot {
	snap.WarmupDue = cfg.LessonWarmupDue
	snap.ReviewStopDue = cfg.ReviewStopDueCount
	snap.DueEn, snap.DueHe = DueExplainer(cfg.LessonWarmupDue, cfg.ReviewStopDueCount)
	var current *Node
	var currentUnit *Unit
	for ui := range snap.Units {
		u := &snap.Units[ui]
		u.BannerEn, u.BannerHe = UnitWordsShort(u.NewLearned, u.NewTotal)
		if u.Current {
			u.KickerEn = fmt.Sprintf("Unit %d of %d · Lesson %d of %d", u.Order, snap.UnitCount, u.LessonIndex, u.LessonCount)
			u.KickerHe = fmt.Sprintf("יחידה %d מתוך %d · שיעור %d מתוך %d", u.Order, snap.UnitCount, u.LessonIndex, u.LessonCount)
			currentUnit = u
		}
		for ni := range u.Nodes {
			n := &u.Nodes[ni]
			if n.Kind == "checkpoint" && n.Total > 0 {
				n.PassNeeded = PassNeeded(cfg.CheckpointPassMark, n.Total)
			}
			n.Label = nodeLabel(*n)
			n.SubEn, n.SubHe = nodeSub(*n)
			if n.Gate == "warn" {
				n.HXVals = hxVals(n.Href, n.CheckpointHref)
			}
			if n.State == "current" {
				current = n
			}
		}
	}
	if currentUnit != nil {
		wordsEn, wordsHe := UnitWords(currentUnit.NewLearned, currentUnit.NewTotal)
		snap.Rail = Rail{
			UnitID:  currentUnit.ID,
			UnitN:   currentUnit.Order,
			Learned: currentUnit.NewLearned,
			Total:   currentUnit.NewTotal,
			WordsEn: wordsEn,
			WordsHe: wordsHe,
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
func UnfinishedUnit(snap Snapshot, unitID string) (int, bool) {
	for i, u := range snap.Units {
		if u.ID != unitID {
			continue
		}
		if i > 0 {
			return snap.Units[i-1].Order, true
		}
		return u.Order, true
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
