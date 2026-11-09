package units

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// AuthoringMetaPhrases is the authoring vocabulary unit.meta_text rejects in
// learner-facing English. The list lives only here. INIT-037/SPEC-006; ADR-0032 D3.
var AuthoringMetaPhrases = []string{
	"correct form",
	"incorrect form",
	"wrong form",
	"first;",
	"second.",
	"target",
	"distractor",
	"checkpoint",
}

// SkippedWithoutBrief is the rules that need a brief and are omitted when
// Validate runs without one. validate_unit lists them as skipped.
// INIT-037/SPEC-006.
func SkippedWithoutBrief() []string {
	return []string{RuleUnitTargetUntaught, RuleBriefCheckpointCoverage}
}

// ContainsMetaText reports that s holds authoring vocabulary from
// AuthoringMetaPhrases. Matching is case-insensitive substring search.
func ContainsMetaText(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	lower := strings.ToLower(s)
	for _, p := range AuthoringMetaPhrases {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// metaTextIssues is unit.meta_text. Blocks in ModeWrite and warns in
// ModePublished. It runs with and without a brief. INIT-037/SPEC-006.
func metaTextIssues(doc *Unit, mode Mode) []Issue {
	if doc == nil {
		return nil
	}
	sev := SeverityBlock
	if mode == ModePublished {
		sev = SeverityWarn
	}
	var out []Issue
	check := func(pointer, s string) {
		if !ContainsMetaText(s) {
			return
		}
		out = append(out, issue(RuleUnitMetaText, sev, pointer,
			"learner-facing English contains authoring vocabulary"))
	}
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			check(at("lessons", i, "activities", j, "translation"), act.Translation)
			check(at("lessons", i, "activities", j, "prompt_en"), act.PromptEN)
			for k, line := range act.Lines {
				check(at("lessons", i, "activities", j, "lines", k, "translation"), line.Translation)
				check(at("lessons", i, "activities", j, "lines", k, "prompt_en"), line.PromptEN)
			}
		}
	}
	return out
}

// targetUntaughtTypes are the activity types whose graded word unit.target_untaught checks.
func targetUntaughtTypes(typ string) bool {
	switch typ {
	case ActivityRecall, ActivityProduce, ActivityCloze, ActivitySay, ActivityListenPick:
		return true
	default:
		return false
	}
}

// checkpointCoverageTypes are pattern and graded lines for brief.checkpoint_coverage.
func checkpointCoverageTypes(typ string) bool {
	return typ == ActivityPattern || targetUntaughtTypes(typ)
}

// targetUntaughtIssues is unit.target_untaught. The graded word must be taught.
// Skipped when facts is nil. INIT-037/SPEC-006; ADR-0032 D1.
func targetUntaughtIssues(ctx context.Context, doc *Unit, lk Lookup, f *BriefFacts, mode Mode) []Issue {
	if doc == nil || f == nil || lk == nil {
		return nil
	}
	sev := SeverityBlock
	if mode == ModePublished {
		sev = SeverityWarn
	}
	s := &scan{ctx: ctx, lk: lk, position: doc.Position, cache: map[string]*probe{}}
	b := &briefScan{scan: s, ids: map[string]*idProbe{}, refs: map[string]lexRef{}}

	prevTexts := map[string]bool{}
	for _, prev := range f.PreviousUnits {
		for _, w := range prev.NewWords {
			if n := normalize(w); n != "" {
				prevTexts[n] = true
			}
		}
	}
	newNorms := make([]string, len(doc.NewWords))
	for i, w := range doc.NewWords {
		newNorms[i] = normalize(w)
	}

	newWordSet := map[string]bool{}
	for _, nw := range newNorms {
		if nw != "" {
			newWordSet[nw] = true
			newWordSet[stripNiqqud(nw)] = true
		}
	}
	introduced := map[string]bool{}
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if targetUntaughtTypes(act.Type) {
				tok := resolveGradedToken(b, act, newWordSet)
				if s.err != nil {
					return []Issue{lookupIssue(s.err)}
				}
				if tok != "" && !taughtToken(b, f, tok, introduced, prevTexts) {
					if s.err != nil {
						return []Issue{lookupIssue(s.err)}
					}
					out = append(out, issue(RuleUnitTargetUntaught, sev, at("lessons", i, "activities", j),
						fmt.Sprintf("graded word %s is not taught for this learner", tok)))
				}
			}
			// Mark newWords that appear on this activity as introduced for later asks.
			for _, text := range act.Texts() {
				for _, tok := range tokenize(text) {
					n := normalize(tok)
					if n == "" {
						continue
					}
					for _, nw := range newNorms {
						if nw != "" && (n == nw || stripNiqqud(n) == stripNiqqud(nw)) {
							introduced[nw] = true
							introduced[n] = true
						}
					}
				}
			}
		}
	}
	if s.err != nil {
		return []Issue{lookupIssue(s.err)}
	}
	return out
}

// resolveGradedToken is the pointed token this activity grades, matching the
// player's targetFrom: an explicit target when set, else the first newWord on
// the line, else the first lexicon word.
func resolveGradedToken(b *briefScan, act Activity, newWordSet map[string]bool) string {
	if tok, ok := targetToken(act); ok {
		return tok
	}
	var first string
	for _, text := range act.Texts() {
		for _, tok := range tokenize(text) {
			if !isHebrewWord(tok) {
				continue
			}
			ref := b.resolve(tok)
			if b.err != nil {
				return ""
			}
			if !ref.ok {
				continue
			}
			if first == "" {
				first = tok
			}
			n := normalize(tok)
			if newWordSet[n] || newWordSet[stripNiqqud(n)] {
				return tok
			}
		}
	}
	return first
}

func taughtToken(b *briefScan, f *BriefFacts, tok string, introduced, prevTexts map[string]bool) bool {
	n := normalize(tok)
	if n == "" {
		return true
	}
	if introduced[n] || introduced[stripNiqqud(n)] {
		return true
	}
	if prevTexts[n] {
		return true
	}
	ref := b.resolve(tok)
	if b.err != nil || !ref.ok {
		return false
	}
	// Known, learning, or any word the brief scoped for this lesson (new,
	// struggle, comfortable). An unscoped word with no card fails.
	if f.KnownLexemeIDs[ref.id] || f.LearningLexemeIDs[ref.id] || f.BriefLexemeIDs[ref.id] {
		return true
	}
	return false
}

// checkpointCoverageIssues is brief.checkpoint_coverage. In a checkpoint, the
// share of Hebrew tokens on pattern and graded lines must meet the tier figure.
// Skipped when facts is nil. INIT-037/SPEC-006.
func checkpointCoverageIssues(ctx context.Context, doc *Unit, lk Lookup, f *BriefFacts, mode Mode) []Issue {
	if doc == nil || f == nil || lk == nil {
		return nil
	}
	sev := SeverityBlock
	if mode == ModePublished {
		sev = SeverityWarn
	}
	share := f.MinKnownShare
	if share <= 0 {
		share = DefaultMinKnownShare
	}
	if share > 1 {
		share = 1
	}

	s := &scan{ctx: ctx, lk: lk, position: doc.Position, cache: map[string]*probe{}}
	b := &briefScan{scan: s, ids: map[string]*idProbe{}, refs: map[string]lexRef{}}
	covered := b.exactNewWordIDs(doc)
	if b.err != nil {
		return []Issue{lookupIssue(b.err)}
	}

	var out []Issue
	for i, les := range doc.Lessons {
		if les.Kind != KindCheckpoint {
			continue
		}
		known, total := 0, 0
		for j, act := range les.Activities {
			if !checkpointCoverageTypes(act.Type) {
				continue
			}
			_ = j
			for _, text := range act.Texts() {
				for _, tok := range tokenize(text) {
					if !isHebrewWord(tok) {
						continue
					}
					ref := b.resolve(tok)
					if b.err != nil {
						return []Issue{lookupIssue(b.err)}
					}
					if ref.ok && b.leftOut(ref) {
						continue
					}
					total++
					if ref.ok && (f.BriefLexemeIDs[ref.id] || f.KnownLexemeIDs[ref.id] || f.LearningLexemeIDs[ref.id] || covered[ref.id]) {
						known++
					}
				}
			}
		}
		if b.err != nil {
			return []Issue{lookupIssue(b.err)}
		}
		if total == 0 {
			continue
		}
		need := int(math.Ceil(share*float64(total) - 1e-9))
		if known >= need {
			continue
		}
		out = append(out, issue(RuleBriefCheckpointCoverage, sev, at("lessons", i),
			fmt.Sprintf("checkpoint coverage is %s (%d/%d tokens); the %s tier needs at least %s",
				formatPercent(known, total), known, total, tierLabel(f), formatShare(share))))
	}
	return out
}
