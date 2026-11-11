package lessons

import (
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// ReviewCapMinutes is the soft cap on review before an easy lesson is the one that opens.
// A word lesson is reserved only when review is inside this cap. ADR-0034.
const ReviewCapMinutes = 15

// Offer is what Today shows after review.
type Offer struct {
	Kind  string // word, easy, or review
	Label string // class date first
	Topic string
	Note  string
	Heavy bool
	Plan  Plan
}

// TodayOffer picks the lesson for one day. Review over the cap gets the easy
// lesson. No passing passage means review only.
func TodayOffer(reviewMinutes int, classOn, topic string, word, easy Plan) Offer {
	label := classLabel(classOn)
	if reviewMinutes > ReviewCapMinutes {
		if easy.Kind == "easy" && len(easy.Unit.Lessons) > 0 && gradedCount(easy.Unit.Lessons[0].Activities) > 0 {
			return Offer{Kind: "easy", Label: label, Topic: topic, Heavy: true, Plan: easy}
		}
		return Offer{Kind: "review", Label: label, Topic: topic, Heavy: true, Note: "no lesson today: nothing passed its checks"}
	}
	if word.Kind == "word" {
		return Offer{Kind: "word", Label: label, Topic: topic, Plan: word}
	}
	if easy.Kind == "easy" {
		return Offer{Kind: "easy", Label: label, Topic: topic, Plan: easy}
	}
	return Offer{Kind: "review", Label: label, Topic: topic, Note: "no lesson today: nothing passed its checks"}
}

func classLabel(classOn string) string {
	t, err := time.Parse(time.DateOnly, classOn)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("from class of %d %s", t.Day(), t.Format("Jan"))
}

// Preflight drops a graded step whose key is not in allowed, which is the T0
// set plus words already taught. A lesson with no graded key left does not open.
func Preflight(p Plan, allowed map[string]bool) (Plan, bool) {
	if p.Kind == "none" || len(p.Unit.Lessons) == 0 {
		return Plan{Kind: "none", Note: "no graded key"}, false
	}
	var kept []units.Activity
	for _, a := range p.Unit.Lessons[0].Activities {
		if gradedActivity(a.Type) && !keyAllowed(a.Text, allowed) {
			p.Dropped = append(p.Dropped, "key.t0")
			continue
		}
		kept = append(kept, a)
	}
	p.Unit.Lessons[0].Activities = kept
	if gradedCount(kept) == 0 {
		return Plan{Kind: "none", Note: "no graded key", Dropped: p.Dropped}, false
	}
	return p, true
}

func gradedActivity(typ string) bool {
	return typ == units.ActivityRecall || typ == units.ActivityProduce || typ == units.ActivityOrder
}

func keyAllowed(text string, allowed map[string]bool) bool {
	if allowed[text] {
		return true
	}
	toks := tokens(text)
	if len(toks) == 0 {
		return false
	}
	for _, t := range toks {
		if !allowed[t] {
			return false
		}
	}
	return true
}

// Answer counts graded steps. right is how many answers equal the stored key.
func Answer(p Plan, reply func(units.Activity) string) (right, graded int) {
	if len(p.Unit.Lessons) == 0 {
		return 0, 0
	}
	for _, a := range p.Unit.Lessons[0].Activities {
		if !gradedActivity(a.Type) {
			continue
		}
		graded++
		if reply != nil && reply(a) == a.Text {
			right++
		}
	}
	return right, graded
}
