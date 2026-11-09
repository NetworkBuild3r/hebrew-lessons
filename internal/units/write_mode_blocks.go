package units

import (
	"context"
	"sort"
)

// WriteModeBlocks runs ModeWrite validation with no brief and returns the
// block-severity count and rule ids. earlierFirstStories and prevActivityTypes
// feed unit.story_repeat and unit.same_shape without a brief.
// same_shape is a warn; it is included in rule ids when it fires but does
// not raise the block count. INIT-037/SPEC-009.
func WriteModeBlocks(ctx context.Context, doc *Unit, lk Lookup, earlierFirstStories, prevActivityTypes []string) (blocks int, rules []string) {
	if doc == nil {
		return 1, []string{RuleSchemaRequired}
	}
	cp := *doc
	if len(earlierFirstStories) > 0 {
		cp.EarlierFirstStories = append([]string(nil), earlierFirstStories...)
	}
	issues := ValidateIn(ctx, &cp, lk, ModeWrite)
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, is := range issues {
		if is.Severity != SeverityBlock {
			continue
		}
		blocks++
		add(is.Rule)
	}
	// Shape comparison without a brief: reuse the shipped same_shape rule via
	// a synthetic BriefFacts that only carries the previous activity types.
	// Filter to that rule so other brief/shape checks do not invent blocks.
	if len(prevActivityTypes) > 0 {
		facts := &BriefFacts{
			PreviousUnits: []BriefPreviousUnit{{ActivityTypes: append([]string(nil), prevActivityTypes...)}},
		}
		for _, is := range ValidateWithBriefIn(ctx, &cp, nil, facts, ModeWrite) {
			if is.Rule == RuleUnitSameShape {
				add(is.Rule)
			}
		}
	}
	sort.Strings(out)
	return blocks, out
}

// DocumentLocalBlocks is ModeWrite with no brief and no path shape context.
// Cached on get_context by revision and rule-catalog version.
func DocumentLocalBlocks(ctx context.Context, doc *Unit, lk Lookup) (int, []string) {
	return WriteModeBlocks(ctx, doc, lk, nil, nil)
}

// ActivityTypes is the activity-type sequence unit.same_shape compares.
func ActivityTypes(doc *Unit) []string {
	if doc == nil {
		return nil
	}
	var out []string
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			out = append(out, act.Type)
		}
	}
	return out
}
