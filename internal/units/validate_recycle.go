package units

import "fmt"

// Recycling counts token occurrences in lessons after the lesson that
// introduces the word. Occurrences in the introducing lesson, and any
// earlier lesson, do not count. The pointer is the newWords entry, which
// is the word's introduction record.
//
// recycle.count warns when the later-lesson total is under 4.
// recycle.spread warns when those occurrences sit in fewer than 3 later
// lessons. A unit that does not have enough later lessons still warns:
// the shortfall is real.

func recycleIssues(doc *Unit, words []wordRef) []Issue {
	var out []Issue
	for _, w := range words {
		if w.dup || w.norm == "" || !w.found {
			continue
		}
		count := 0
		lessons := 0
		for li, les := range doc.Lessons {
			if li <= w.lesson {
				continue
			}
			n := countWord(les, w)
			if n == 0 {
				continue
			}
			count += n
			lessons++
		}
		ptr := at("newWords", w.index)
		if count < RecycleMinCount {
			out = append(out, issue(RuleRecycleCount, SeverityWarn, ptr,
				fmt.Sprintf("new word %s appears %d times in later lessons; at least %d are required", strconvQuote(w.raw), count, RecycleMinCount)))
		}
		if lessons < RecycleMinLessons {
			out = append(out, issue(RuleRecycleSpread, SeverityWarn, ptr,
				fmt.Sprintf("new word %s appears in %d later lessons; at least %d are required", strconvQuote(w.raw), lessons, RecycleMinLessons)))
		}
	}
	return out
}
