package units

import (
	"fmt"
	"strings"
)

// Progressive lesson shape rules. INIT-030/SPEC-004; ADR-0026 D2–D5.

const (
	// MaxDrillShare is the highest share of non-checkpoint activities that may
	// be single-word drills. ADR-0026 D4.
	MaxDrillShare = 0.40
	// SameShapeMin is the lowest activity-type similarity to the first previous
	// unit that does not warn. Compared by edit distance over the type list.
	SameShapeMin = 0.80
	// RecyclePreviousMin is how many distinct previous-unit newWords must appear
	// in meaning steps, unless the previous units have fewer.
	RecyclePreviousMin = 4
)

// shapeIssues applies the INIT-030 progression checks that read the real
// document. drill_share here is always a block; Validate emits the warn form.
func shapeIssues(doc *Unit, f *BriefFacts) []Issue {
	if doc == nil || f == nil {
		return nil
	}
	var out []Issue
	out = append(out, storyLengthIssues(doc, f)...)
	out = append(out, drillShareIssues(doc, SeverityBlock)...)
	out = append(out, patternBriefIssues(doc, f)...)
	out = append(out, recyclePreviousIssues(doc, f)...)
	out = append(out, consolidationIssues(doc, f)...)
	out = append(out, sameShapeIssues(doc, f)...)
	return out
}

// storyLengthIssues is brief.story_length. Each lesson needs a story, or a
// dialogue whose lines together reach MinStoryTokens Hebrew words. A
// checkpoint is exempt. Zero MinStoryTokens skips the rule.
func storyLengthIssues(doc *Unit, f *BriefFacts) []Issue {
	if f.MinStoryTokens <= 0 {
		return nil
	}
	var out []Issue
	for i, les := range doc.Lessons {
		if strings.TrimSpace(les.Kind) == KindCheckpoint {
			continue
		}
		if lessonMeetsStoryFloor(les, f.MinStoryTokens) {
			continue
		}
		out = append(out, issue(RuleBriefStoryLength, SeverityBlock, at("lessons", i),
			fmt.Sprintf("lesson needs a story or dialogue of at least %d Hebrew words", f.MinStoryTokens)))
	}
	return out
}

func lessonMeetsStoryFloor(les Lesson, min int) bool {
	for _, act := range les.Activities {
		switch act.Type {
		case ActivityStory:
			if hebrewTokenCount(act.Text) >= min {
				return true
			}
		case ActivityDialogue:
			n := 0
			for _, line := range act.Lines {
				n += hebrewTokenCount(line.Text)
			}
			if n >= min {
				return true
			}
		}
	}
	return false
}

func hebrewTokenCount(text string) int {
	n := 0
	for _, tok := range tokenize(text) {
		if isHebrewWord(tok) {
			n++
		}
	}
	return n
}

// drillShareIssues is unit.drill_share. Checkpoint activities are left out of
// the count: they are review by design. A single-word drill is a shadow,
// listen_pick, recall, or produce whose text is one Hebrew token.
func drillShareIssues(doc *Unit, sev string) []Issue {
	drills, total := 0, 0
	for _, les := range doc.Lessons {
		if strings.TrimSpace(les.Kind) == KindCheckpoint {
			continue
		}
		for _, act := range les.Activities {
			total++
			if singleWordDrill(act) {
				drills++
			}
		}
	}
	if total == 0 || float64(drills)/float64(total) <= MaxDrillShare {
		return nil
	}
	return []Issue{issue(RuleUnitDrillShare, sev, "/lessons",
		fmt.Sprintf("%d of %d activities are single-word drills; at most %.0f%% is allowed",
			drills, total, MaxDrillShare*100))}
}

func singleWordDrill(act Activity) bool {
	switch act.Type {
	case ActivityShadow, ActivityListenPick, ActivityRecall, ActivityProduce:
		return hebrewTokenCount(act.Text) == 1
	default:
		return false
	}
}

// patternBriefIssues is brief.pattern.
func patternBriefIssues(doc *Unit, f *BriefFacts) []Issue {
	pat := strings.TrimSpace(doc.Pattern)
	if f.Consolidate {
		if pat == "" {
			return nil
		}
		for _, slug := range f.ReusePatterns {
			if pat == strings.TrimSpace(slug) {
				return nil
			}
		}
		return []Issue{issue(RuleBriefPattern, SeverityBlock, "/pattern",
			"consolidation unit must name no new pattern, or one of the reusePatterns slugs")}
	}
	// When the brief does not name a next pattern and is not consolidating,
	// there is nothing to check (legacy brief).
	if strings.TrimSpace(f.NextPattern) == "" && len(f.ReusePatterns) == 0 {
		return nil
	}
	want := strings.TrimSpace(f.NextPattern)
	if pat == want {
		return nil
	}
	if want == "" {
		return []Issue{issue(RuleBriefPattern, SeverityBlock, "/pattern",
			"unit pattern must be empty when the brief names no nextPattern")}
	}
	return []Issue{issue(RuleBriefPattern, SeverityBlock, "/pattern",
		fmt.Sprintf("unit pattern %q does not match the brief's nextPattern %q", pat, want))}
}

// recyclePreviousIssues is brief.recycle_previous.
func recyclePreviousIssues(doc *Unit, f *BriefFacts) []Issue {
	if len(f.PreviousUnits) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, prev := range f.PreviousUnits {
		for _, w := range prev.NewWords {
			n := normalize(w)
			if n != "" {
				want[n] = true
			}
		}
	}
	if len(want) == 0 {
		return nil
	}
	need := RecyclePreviousMin
	if len(want) < need {
		need = len(want)
	}
	found := map[string]bool{}
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			for _, text := range meaningTexts(act) {
				for _, tok := range tokenize(text) {
					if !isHebrewWord(tok) {
						continue
					}
					n := normalize(tok)
					if want[n] {
						found[n] = true
					}
				}
			}
		}
	}
	if len(found) >= need {
		return nil
	}
	return []Issue{issue(RuleBriefRecyclePrevious, SeverityBlock, "/lessons",
		fmt.Sprintf("only %d of the previous units' new words appear in meaning steps; need at least %d",
			len(found), need))}
}

// meaningTexts are the Hebrew strings that count for brief.recycle_previous:
// a story, each dialogue line, a multi-word listen_pick, cloze, say, or order.
func meaningTexts(act Activity) []string {
	switch act.Type {
	case ActivityStory, ActivityCloze, ActivitySay, ActivityOrder:
		return []string{act.Text}
	case ActivityDialogue:
		out := make([]string, 0, len(act.Lines))
		for _, line := range act.Lines {
			out = append(out, line.Text)
		}
		return out
	case ActivityListenPick:
		if hebrewTokenCount(act.Text) >= 2 {
			return []string{act.Text}
		}
		return nil
	default:
		return nil
	}
}

// consolidationIssues is brief.consolidation.
func consolidationIssues(doc *Unit, f *BriefFacts) []Issue {
	if !f.Consolidate {
		return nil
	}
	for _, w := range doc.NewWords {
		if strings.TrimSpace(w) != "" {
			return []Issue{issue(RuleBriefConsolidation, SeverityBlock, "/newWords",
				"consolidation unit must list no new words")}
		}
	}
	return nil
}

// sameShapeIssues is unit.same_shape. Warns when the activity-type sequence is
// under SameShapeMin similar to the first previous unit by edit distance.
func sameShapeIssues(doc *Unit, f *BriefFacts) []Issue {
	if len(f.PreviousUnits) == 0 || len(f.PreviousUnits[0].ActivityTypes) == 0 {
		return nil
	}
	prev := f.PreviousUnits[0].ActivityTypes
	cur := activityTypeSeq(doc)
	if shapeSimilarity(cur, prev) >= SameShapeMin {
		return nil
	}
	return []Issue{issue(RuleUnitSameShape, SeverityWarn, "/lessons",
		fmt.Sprintf("activity types are under %.0f%% the same as the previous unit", SameShapeMin*100))}
}

func activityTypeSeq(doc *Unit) []string {
	var out []string
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			out = append(out, act.Type)
		}
	}
	return out
}

// shapeSimilarity is 1 - editDistance/max(len), or 1 when both are empty.
func shapeSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	max := len(a)
	if len(b) > max {
		max = len(b)
	}
	if max == 0 {
		return 1
	}
	return 1 - float64(editDistance(a, b))/float64(max)
}

func editDistance(a, b []string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := cur[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			cur[j] = ins
			if del < cur[j] {
				cur[j] = del
			}
			if sub < cur[j] {
				cur[j] = sub
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
