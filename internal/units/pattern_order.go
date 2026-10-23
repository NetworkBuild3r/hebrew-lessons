package units

import (
	"fmt"
	"sort"
	"strings"
)

// Worksheet types. Empty means the unit has no worksheet shape.
const (
	SheetTopic       = "topic-conversation"
	SheetReview      = "review-sheet"
	SheetVerb        = "verb-workshop"
	SheetStory       = "short-story"
	SheetFoundations = "foundations-reading"
)

// CourseUnit is one unit on the path. Pattern is empty when the unit
// teaches none. Scope is shared or learner:<id>.
type CourseUnit struct {
	ID       int64
	Position int
	Scope    string
	Title    string
	Pattern  string
}

// PatternOrderIssues is unit.pattern_order. A shared unit that needs a
// pattern first taught at a later position blocks, and the message names
// that unit. A learner unit warns.
func PatternOrderIssues(scope string, position int, pattern string, course []CourseUnit, prereqs map[string][]string) []Issue {
	if pattern == "" {
		return nil
	}
	taught := teachers(course)
	sev := SeverityBlock
	if strings.HasPrefix(scope, "learner:") {
		sev = SeverityWarn
	}
	var out []Issue
	for _, req := range prereqs[pattern] {
		teacher, ok := taught[req]
		// Taught at this position or earlier is in order. A later position is not.
		if ok && teacher.Position <= position {
			continue
		}
		msg := fmt.Sprintf("%s is not taught before this unit.", req)
		if ok {
			msg = fmt.Sprintf("%s is taught later, in %s.", req, teacher.Title)
		}
		out = append(out, issue(RulePatternOrder, sev, "/pattern", msg))
	}
	return out
}

func teachers(course []CourseUnit) map[string]CourseUnit {
	out := map[string]CourseUnit{}
	for _, u := range course {
		if u.Scope != "shared" || u.Pattern == "" {
			continue
		}
		prev, ok := out[u.Pattern]
		if !ok || u.Position < prev.Position || (u.Position == prev.Position && u.ID < prev.ID) {
			out[u.Pattern] = u
		}
	}
	return out
}

// StablePath is a topological order of course that keeps class order
// wherever the prerequisites allow it. A cycle leaves the remaining units
// in class order.
func StablePath(course []CourseUnit, prereqs map[string][]string) []int64 {
	order := append([]CourseUnit(nil), course...)
	sort.Slice(order, func(i, j int) bool {
		if order[i].Position != order[j].Position {
			return order[i].Position < order[j].Position
		}
		return order[i].ID < order[j].ID
	})
	index := map[int64]int{}
	for i, u := range order {
		index[u.ID] = i
	}
	taught := teachers(order)
	edges := map[int64][]int64{}
	indeg := map[int64]int{}
	seenEdge := map[[2]int64]bool{}
	for _, u := range order {
		indeg[u.ID] = 0
	}
	for _, u := range order {
		if u.Pattern == "" {
			continue
		}
		for _, req := range prereqs[u.Pattern] {
			t, ok := taught[req]
			if !ok || t.ID == u.ID {
				continue
			}
			key := [2]int64{t.ID, u.ID}
			if seenEdge[key] {
				continue
			}
			seenEdge[key] = true
			edges[t.ID] = append(edges[t.ID], u.ID)
			indeg[u.ID]++
		}
	}
	var out []int64
	used := map[int64]bool{}
	for len(out) < len(order) {
		best := -1
		var bestID int64
		for _, u := range order {
			if used[u.ID] || indeg[u.ID] > 0 {
				continue
			}
			if best < 0 || index[u.ID] < best {
				best = index[u.ID]
				bestID = u.ID
			}
		}
		if best < 0 {
			for _, u := range order {
				if !used[u.ID] {
					out = append(out, u.ID)
				}
			}
			break
		}
		used[bestID] = true
		out = append(out, bestID)
		for _, to := range edges[bestID] {
			indeg[to]--
		}
	}
	return out
}

// PatternSequence orders slugs the way StablePath meets them, then any
// slug no unit teaches, by name.
func PatternSequence(slugs []string, course []CourseUnit, prereqs map[string][]string) []string {
	want := map[string]bool{}
	for _, s := range slugs {
		want[s] = true
	}
	var out []string
	seen := map[string]bool{}
	byID := map[int64]CourseUnit{}
	for _, u := range course {
		byID[u.ID] = u
	}
	for _, id := range StablePath(course, prereqs) {
		p := byID[id].Pattern
		if p == "" || !want[p] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	var rest []string
	for _, s := range slugs {
		if !seen[s] {
			rest = append(rest, s)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// WorksheetIssues is unit.worksheet_shape. An empty kind checks nothing.
// A true/false item is a listen_pick, which is the choice step this course has.
func WorksheetIssues(kind string, doc *Unit) []Issue {
	if kind == "" {
		return nil
	}
	if !knownSheet(kind) {
		return []Issue{issue(RuleWorksheetShape, SeverityBlock, "/worksheetType", "worksheet type is not one of the five")}
	}
	if doc == nil {
		return []Issue{issue(RuleWorksheetShape, SeverityBlock, "/worksheetType", "unit document is missing")}
	}
	stories, picks, dialogues, patterns := 0, 0, 0, 0
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			switch act.Type {
			case ActivityStory:
				stories++
			case ActivityListenPick:
				picks++
			case ActivityDialogue:
				dialogues++
			case ActivityPattern:
				patterns++
			}
		}
	}
	var msg string
	switch kind {
	case SheetStory:
		if stories < 1 || picks < 3 {
			msg = "a short story needs a story step and at least 3 true/false questions"
		}
	case SheetTopic:
		if dialogues < 1 {
			msg = "a topic conversation needs a dialogue"
		}
	case SheetVerb:
		if patterns < 1 {
			msg = "a verb workshop needs a pattern step"
		}
	case SheetReview:
		if len(doc.NewWords) > 0 {
			msg = "a review sheet introduces no new words"
		}
	case SheetFoundations:
		if stories < 1 {
			msg = "a foundations reading needs a story step"
		}
	}
	if msg == "" {
		return nil
	}
	return []Issue{issue(RuleWorksheetShape, SeverityBlock, "/worksheetType", msg)}
}

func knownSheet(kind string) bool {
	switch kind {
	case SheetTopic, SheetReview, SheetVerb, SheetStory, SheetFoundations:
		return true
	default:
		return false
	}
}
