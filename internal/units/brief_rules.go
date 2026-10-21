package units

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// The skills a struggle word can fail. They are the strings of fsrs.Skill*,
// repeated here because this package must not import internal/brief or internal/fsrs.
const (
	SkillListen = "listen"
	SkillRead   = "read"
	SkillRecall = "recall"
	SkillSpell  = "spell"
)

// BriefFacts is what ValidateWithBrief needs to know about a lesson brief and
// the learner it is for. This package owns the type so that it never imports
// internal/brief (see the package comment). The caller builds it, and the
// caller is responsible for reading the brief as the learner the unit is for.
//
// The maps are sets of lexicon ids. A nil map is an empty set.
type BriefFacts struct {
	// Tier names the coverage tier the brief applied. It only appears in messages.
	Tier string `json:"tier,omitempty"`
	// MinKnownShare is the share of passage tokens that must be known or in the
	// brief, from 0 to 1. This is the brief's staged tier.min_known_share. Zero
	// or less means DefaultMinKnownShare, so a malformed brief fails closed.
	MinKnownShare float64 `json:"minKnownShare"`
	// MaxTextTokens is the most Hebrew tokens one passage may have. Zero is no limit.
	MaxTextTokens int `json:"maxTextTokens,omitempty"`
	// Used reports that a unit was already built from this brief.
	Used bool `json:"used,omitempty"`
	// UsedByUnitID is that unit, when Used is set. ValidateWithBrief does not read
	// it; the publish path does, to treat an exact repeat of a publish as unchanged.
	UsedByUnitID int64 `json:"usedByUnitId,omitempty"`

	NewWords      []BriefNewWord      `json:"newWords,omitempty"`
	StruggleWords []BriefStruggleWord `json:"struggleWords,omitempty"`
	// BriefLexemeIDs is every word the brief lists: new, struggle, and comfortable.
	BriefLexemeIDs map[int64]bool `json:"briefLexemeIds,omitempty"`
	// KnownLexemeIDs is every word the learner knows now: known or solid.
	KnownLexemeIDs map[int64]bool `json:"knownLexemeIds,omitempty"`
}

// BriefNewWord is a word the unit must teach.
type BriefNewWord struct {
	LexemeID int64  `json:"lexemeId"`
	Text     string `json:"text"`
	// MinUses is how many times the unit must use the word. Less than
	// DefaultNewWordMinUses means DefaultNewWordMinUses.
	MinUses int `json:"minUses"`
}

// BriefStruggleWord is a word the unit must bring back.
type BriefStruggleWord struct {
	LexemeID int64  `json:"lexemeId"`
	Text     string `json:"text"`
	// RequiredCount is how many times the unit must use the word. Less than
	// DefaultStruggleUses means DefaultStruggleUses.
	RequiredCount int `json:"requiredCount"`
	// Skill is the skill the word is failing: listen, read, recall, or spell.
	// The word must appear in at least one activity of that skill. Empty means
	// no skill is required.
	Skill string `json:"skill,omitempty"`
}

// SkillOfActivity is the skill an activity type trains, or "" when it has none.
// listen_pick and shadow train listen. story, dialogue, cloze, and pattern train
// read. recall trains recall. produce, which the learner types, trains spell.
// Older stored briefs named SPEC-015 review activities. Those names still map,
// so a stored brief keeps its skill, and a new brief does not emit them:
// audio_match is listen, read_pick and word_match are read, type_it is recall,
// and dictation is spell.
func SkillOfActivity(typ string) string {
	switch typ {
	case ActivityListenPick, ActivityShadow, "audio_match":
		return SkillListen
	case ActivityStory, ActivityDialogue, ActivityCloze, ActivityPattern, "read_pick", "word_match":
		return SkillRead
	case ActivityRecall, "type_it":
		return SkillRecall
	case ActivityProduce, "dictation":
		return SkillSpell
	default:
		return ""
	}
}

// activitySkillOrder is the unit types a brief may suggest, in the order a brief
// lists them. ActivitiesForSkill keeps a name only when knownActivity accepts it
// and SkillOfActivity assigns it to the skill, so the list cannot drift ahead of
// the validator. SPEC-015 review names are not in this order.
var activitySkillOrder = []string{
	ActivityListenPick,
	ActivityShadow,
	ActivityStory,
	ActivityDialogue,
	ActivityCloze,
	ActivityPattern,
	ActivityRecall,
	ActivityProduce,
}

// ActivitiesForSkill returns the unit activity types that train skill, in the
// order a lesson brief lists them. Every name is one the validator accepts.
// An unknown skill returns nil.
func ActivitiesForSkill(skill string) []string {
	var out []string
	for _, typ := range activitySkillOrder {
		if knownActivity(typ) && SkillOfActivity(typ) == skill {
			out = append(out, typ)
		}
	}
	return out
}

// exposureActivity reports an activity that shows a word without checking it.
// A story or a dialogue is read and a shadow is repeated aloud; none takes an
// answer, so any of them can introduce a new word. Every other type is checked.
func exposureActivity(typ string) bool {
	return typ == ActivityStory || typ == ActivityShadow || typ == ActivityDialogue
}

// passageActivity reports the types that hold running text, which coverage and
// the length limit judge. A dialogue is judged one line at a time, because
// briefIssues reads the flattened document.
func passageActivity(typ string) bool {
	return typ == ActivityStory || typ == ActivityListenPick || typ == ActivityDialogue
}

// briefPointer is where a brief issue about the brief itself points. The brief
// id is an argument of put_unit and validate_unit, not a field of the document.
const briefPointer = "/briefId"

// ValidateWithBrief runs Validate and then the brief rules, and returns one
// sorted list. facts is the brief the unit is checked against. A nil facts is
// brief.not_found. The brief rules need the lexicon, so a nil lookup, or a
// lookup that fails, leaves them out and reports schema.lookup once. The two
// v2 coverage rules are replaced by brief.coverage; see the package comment.
// unit.no_markup runs on every call, whatever the lookup does.
func ValidateWithBrief(ctx context.Context, doc *Unit, lk Lookup, facts *BriefFacts) []Issue {
	return ValidateWithBriefIn(ctx, doc, lk, facts, ModeWrite)
}

// ValidateWithBriefIn is ValidateWithBrief with an explicit mode.
// ModeWrite is new input and every revision other than the current
// published one. ModePublished rechecks that current published revision.
// A recall, produce, or cloze of two or more lexicon words may omit
// target, and a listen_pick of two or more lexicon words may omit
// translation. listen_pick may omit target in both modes.
func ValidateWithBriefIn(ctx context.Context, doc *Unit, lk Lookup, facts *BriefFacts, mode Mode) []Issue {
	if ctx == nil {
		ctx = context.Background()
	}
	issues := collect(ctx, doc, lk, mode)
	if doc == nil {
		return sortedIssues(issues)
	}
	issues = withoutRules(issues, RuleCoverageStory, RuleCoverageListening)
	issues = append(issues, markupIssues(doc)...)
	if facts == nil {
		issues = append(issues, issue(RuleBriefNotFound, SeverityBlock, briefPointer, "no brief was found for this unit"))
		return sortedIssues(issues)
	}
	if facts.Used {
		issues = append(issues, issue(RuleBriefUsed, SeverityBlock, briefPointer, "this brief already built a unit, and a brief can be used once"))
	}
	if lk != nil && !hasRule(issues, RuleSchemaLookup) {
		issues = append(issues, briefIssues(ctx, doc, lk, facts)...)
	}
	return sortedIssues(issues)
}

func withoutRules(issues []Issue, rules ...string) []Issue {
	out := issues[:0:0]
	for _, is := range issues {
		drop := false
		for _, r := range rules {
			if is.Rule == r {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, is)
		}
	}
	return out
}

func hasRule(issues []Issue, rule string) bool {
	for _, is := range issues {
		if is.Rule == rule {
			return true
		}
	}
	return false
}

// markupIssues flags every string of the document that carries markup.
// The field list is explicit, and TestNoMarkupCoversEveryStringField walks the
// Unit struct by reflection, so a string field added later cannot be forgotten.
func markupIssues(doc *Unit) []Issue {
	var out []Issue
	check := func(pointer, s string) {
		if hasMarkup(s) {
			out = append(out, issue(RuleUnitNoMarkup, SeverityBlock, pointer, "text contains <, >, or javascript:, and a unit is plain text"))
		}
	}
	check("/title", doc.Title)
	check("/situation", doc.Situation)
	check("/concept", doc.Concept)
	for i, w := range doc.NewWords {
		check(at("newWords", i), w)
	}
	for i, les := range doc.Lessons {
		check(at("lessons", i, "kind"), les.Kind)
		for j, act := range les.Activities {
			check(at("lessons", i, "activities", j, "type"), act.Type)
			check(at("lessons", i, "activities", j, "text"), act.Text)
			check(at("lessons", i, "activities", j, "target"), act.Target)
			check(at("lessons", i, "activities", j, "translation"), act.Translation)
			for k, line := range act.Lines {
				check(at("lessons", i, "activities", j, "lines", k, "speaker"), line.Speaker)
				check(at("lessons", i, "activities", j, "lines", k, "text"), line.Text)
				check(at("lessons", i, "activities", j, "lines", k, "translation"), line.Translation)
			}
		}
	}
	return out
}

// hasMarkup reports <, >, or javascript: in s. It looks at s as written and
// again after Unicode compatibility folding, so a full-width ＜ or a
// full-width javascript is caught. For javascript: it also drops spaces,
// control characters, and format characters, because a browser ignores
// them inside a URL scheme, and it ignores letter case.
func hasMarkup(s string) bool {
	for _, form := range []string{s, norm.NFKC.String(s)} {
		if strings.ContainsAny(form, "<>") {
			return true
		}
		folded := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				return -1
			}
			return unicode.ToLower(r)
		}, form)
		if strings.Contains(folded, "javascript:") {
			return true
		}
	}
	return false
}

// lexRef is a token matched to a lexicon entry. key is the lexicon key that matched.
type lexRef struct {
	id  int64
	key string
	ok  bool
}

type briefScan struct {
	*scan
	ids  map[string]*idProbe
	refs map[string]lexRef
}

type idProbe struct {
	id int64
	ok bool
}

type tokRef struct {
	tok string
	ref lexRef
}

type actTokens struct {
	lesson, act int
	typ, text   string
	toks        []tokRef
}

func (a actTokens) pointer() string { return at("lessons", a.lesson, "activities", a.act, "text") }
func (a actTokens) actPointer() string {
	return at("lessons", a.lesson, "activities", a.act)
}

func (a actTokens) count(id int64) int {
	n := 0
	for _, t := range a.toks {
		if t.ref.ok && t.ref.id == id {
			n++
		}
	}
	return n
}

// briefIssues reads the flattened document, so a dialogue line is one passage
// for coverage and length, and its words count toward uses exactly like a
// story's. The pointers are mapped back to the real document on the way out.
func briefIssues(ctx context.Context, doc *Unit, lk Lookup, f *BriefFacts) []Issue {
	flat, refs := flatten(doc)
	return remapIssues(briefIssuesFlat(ctx, flat, lk, f), refs)
}

func briefIssuesFlat(ctx context.Context, doc *Unit, lk Lookup, f *BriefFacts) []Issue {
	s := &scan{ctx: ctx, lk: lk, position: doc.Position, cache: map[string]*probe{}}
	b := &briefScan{scan: s, ids: map[string]*idProbe{}, refs: map[string]lexRef{}}

	var acts []actTokens
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			a := actTokens{lesson: i, act: j, typ: act.Type, text: act.Text}
			for _, tok := range tokenize(act.Text) {
				if !isHebrewWord(tok) {
					continue
				}
				a.toks = append(a.toks, tokRef{tok: tok, ref: b.resolve(tok)})
			}
			acts = append(acts, a)
		}
	}
	if s.err != nil {
		return []Issue{lookupIssue(s.err)}
	}

	var out []Issue
	out = append(out, b.audioAllIssues(doc)...)
	out = append(out, b.passageIssues(doc, acts, f)...)
	out = append(out, b.newWordIssues(doc, acts, f)...)
	out = append(out, struggleIssues(acts, f)...)
	out = append(out, b.glossIssues(acts, f)...)
	if s.err != nil {
		return []Issue{lookupIssue(s.err)}
	}
	return out
}

// audioAllIssues covers the Hebrew fields that audio.missing does not: each new
// word, and a title, situation, or concept that contains Hebrew. Activity text
// is audio.missing's, so it is not reported twice.
func (b *briefScan) audioAllIssues(doc *Unit) []Issue {
	var out []Issue
	check := func(pointer, text string) {
		if strings.TrimSpace(text) == "" || !containsHebrew(text) {
			return
		}
		if b.hasAudio(text) {
			return
		}
		if b.err != nil {
			return
		}
		out = append(out, issue(RuleBriefAudioAll, SeverityBlock, pointer, "Hebrew text has no audio clip"))
	}
	check("/title", doc.Title)
	check("/situation", doc.Situation)
	check("/concept", doc.Concept)
	for i, w := range doc.NewWords {
		check(at("newWords", i), w)
	}
	return out
}

// passageIssues applies brief.coverage and brief.text_too_long to each story
// and listen_pick passage, the same two activity types the v2 coverage rules use.
// A token also counts as covered when its lexeme is one of this unit's newWords
// resolved by exact pointed text. Intro and minimum uses run on the brief's
// new words and on every newWord that this credit covers.
func (b *briefScan) passageIssues(doc *Unit, acts []actTokens, f *BriefFacts) []Issue {
	covered := b.exactNewWordIDs(doc)
	if b.err != nil {
		return nil
	}
	share := f.MinKnownShare
	if share <= 0 {
		share = DefaultMinKnownShare
	}
	if share > 1 {
		share = 1
	}
	var out []Issue
	for _, a := range acts {
		if !passageActivity(a.typ) {
			continue
		}
		if f.MaxTextTokens > 0 && len(a.toks) > f.MaxTextTokens {
			out = append(out, issue(RuleBriefTextTooLong, SeverityBlock, a.actPointer(),
				fmt.Sprintf("passage has %d tokens, and the %s tier allows at most %d", len(a.toks), tierLabel(f), f.MaxTextTokens)))
		}
		known, total := 0, 0
		for _, t := range a.toks {
			if t.ref.ok && b.leftOut(t.ref) {
				continue
			}
			total++
			if t.ref.ok && (f.BriefLexemeIDs[t.ref.id] || f.KnownLexemeIDs[t.ref.id] || covered[t.ref.id]) {
				known++
			}
		}
		if b.err != nil {
			return out
		}
		if total == 0 {
			continue
		}
		need := int(math.Ceil(share*float64(total) - 1e-9))
		if known >= need {
			continue
		}
		out = append(out, issue(RuleBriefCoverage, SeverityBlock, a.actPointer(),
			fmt.Sprintf("coverage is %s (%d/%d tokens); the %s tier needs at least %s",
				formatPercent(known, total), known, total, tierLabel(f), formatShare(share))))
	}
	return out
}

// exactNewWordIDs is the lexicon ids of this unit's newWords, matched by
// LexemeByText on the normalized pointed spelling. The consonant skeleton
// that inLexicon accepts does not count: a row for גר does not cover גָּר.
func (b *briefScan) exactNewWordIDs(doc *Unit) map[int64]bool {
	ids := map[int64]bool{}
	if doc == nil {
		return ids
	}
	for _, w := range doc.NewWords {
		n := normalize(w)
		if n == "" || !b.probeLex(n) {
			if b.err != nil {
				return nil
			}
			continue
		}
		id, ok := b.lexID(n)
		if b.err != nil {
			return nil
		}
		if ok {
			ids[id] = true
		}
	}
	return ids
}

func tierLabel(f *BriefFacts) string {
	if f.Tier == "" {
		return "brief"
	}
	return f.Tier
}

func formatShare(share float64) string {
	return fmt.Sprintf("%.1f%%", share*100)
}

// leftOut reports a token that coverage does not count: a proper name or a
// function word, by the lexicon's part of speech. A numeral is not a Hebrew
// word token here, so it never reaches the count.
func (b *briefScan) leftOut(ref lexRef) bool {
	facts, ok := b.probeFacts(ref.key)
	if !ok {
		return false
	}
	return namePOS(facts.POS) || functionPOS(facts.POS)
}

// functionPOS reports a closed-class part of speech. The lexicon's part of
// speech is free text, so this lists the spellings in use.
func functionPOS(pos string) bool {
	switch strings.ToLower(strings.TrimSpace(pos)) {
	case "prep", "preposition", "conj", "conjunction", "pron", "pronoun", "particle",
		"art", "article", "det", "determiner", "aux", "auxiliary",
		"function", "func", "function word", "function-word":
		return true
	default:
		return false
	}
}

// newWordIssues applies brief.new_min_uses and brief.new_intro_before_test
// to the brief's new words and to every unit newWord that coverage credits.
// A word in both is reported once, from the brief. A credited word outside
// the brief uses DefaultNewWordMinUses.
func (b *briefScan) newWordIssues(doc *Unit, acts []actTokens, f *BriefFacts) []Issue {
	var out []Issue
	seen := map[int64]bool{}
	for _, w := range f.NewWords {
		seen[w.LexemeID] = true
		out = append(out, b.oneNewWord(doc, acts, w.LexemeID, w.Text, w.MinUses)...)
	}
	credited := b.exactNewWordIDs(doc)
	if b.err != nil {
		return out
	}
	for _, w := range doc.NewWords {
		n := normalize(w)
		if n == "" || !b.probeLex(n) {
			if b.err != nil {
				return out
			}
			continue
		}
		id, ok := b.lexID(n)
		if b.err != nil {
			return out
		}
		if !ok || !credited[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, b.oneNewWord(doc, acts, id, w, DefaultNewWordMinUses)...)
	}
	return out
}

func (b *briefScan) oneNewWord(doc *Unit, acts []actTokens, id int64, text string, minUses int) []Issue {
	want := minUses
	if want < DefaultNewWordMinUses {
		want = DefaultNewWordMinUses
	}
	uses := 0
	for _, a := range acts {
		uses += a.count(id)
	}
	var out []Issue
	if uses < want {
		out = append(out, issue(RuleBriefNewMinUses, SeverityBlock, b.newWordPointer(doc, id),
			fmt.Sprintf("new word %s appears %d times, and the brief asks for at least %d", text, uses, want)))
	}
	exposed := false
	for _, a := range acts {
		if a.count(id) == 0 {
			continue
		}
		if exposureActivity(a.typ) {
			exposed = true
			continue
		}
		if !exposed {
			out = append(out, issue(RuleBriefIntro, SeverityBlock, a.pointer(),
				fmt.Sprintf("new word %s is checked here before any story or shadow step shows it", text)))
		}
		break
	}
	return out
}

// newWordPointer is the newWords entry that is this lexeme, or /newWords when
// the unit does not list it.
func (b *briefScan) newWordPointer(doc *Unit, id int64) string {
	for i, w := range doc.NewWords {
		if r := b.resolve(w); r.ok && r.id == id {
			return at("newWords", i)
		}
	}
	return "/newWords"
}

// struggleIssues applies brief.struggle_reuse: the required count, and at least
// one use in an activity of the word's failing skill. A word that fails both
// gets one issue that says both.
func struggleIssues(acts []actTokens, f *BriefFacts) []Issue {
	var out []Issue
	for _, w := range f.StruggleWords {
		need := w.RequiredCount
		if need < DefaultStruggleUses {
			need = DefaultStruggleUses
		}
		uses, inSkill := 0, w.Skill == ""
		for _, a := range acts {
			n := a.count(w.LexemeID)
			uses += n
			if n > 0 && w.Skill != "" && SkillOfActivity(a.typ) == w.Skill {
				inSkill = true
			}
		}
		var problems []string
		if uses < need {
			problems = append(problems, fmt.Sprintf("appears %d times, and the brief requires %d", uses, need))
		}
		if !inSkill {
			problems = append(problems, fmt.Sprintf("never appears in an activity that trains %s, the skill it is failing", w.Skill))
		}
		if len(problems) > 0 {
			out = append(out, issue(RuleBriefStruggle, SeverityBlock, "/lessons",
				fmt.Sprintf("struggle word %s %s", w.Text, strings.Join(problems, " and "))))
		}
	}
	return out
}

// glossIssues applies brief.gloss_out_of_brief. A word that matches a lexeme,
// is not in the brief, and is not known or solid, must have an English gloss in
// the lexicon, because the player shows that gloss on tap. A token that matches
// no lexeme is another rule's finding (lexeme.unknown or story.unmapped), so it
// is not reported again here.
func (b *briefScan) glossIssues(acts []actTokens, f *BriefFacts) []Issue {
	var out []Issue
	for _, a := range acts {
		done := map[int64]bool{}
		for _, t := range a.toks {
			if !t.ref.ok || done[t.ref.id] || f.BriefLexemeIDs[t.ref.id] || f.KnownLexemeIDs[t.ref.id] {
				continue
			}
			done[t.ref.id] = true
			facts, ok := b.probeFacts(t.ref.key)
			if b.err != nil {
				return out
			}
			if ok && strings.TrimSpace(facts.Gloss) != "" {
				continue
			}
			out = append(out, issue(RuleBriefGloss, SeverityBlock, a.pointer(),
				fmt.Sprintf("word %s is outside the brief and the learner's known words, and it has no gloss", t.tok)))
		}
	}
	return out
}

// Token matching.
//
// A token is matched to a lexeme in this order, and the first hit wins:
//  1. the token as written, then with the dagesh of its first letter dropped,
//     then its consonant skeleton;
//  2. the same three forms after one prefix letter (ו ה ב ל מ ש כ) is cut off,
//     and its niqqud with it, when at least two letters remain;
//  3. after a second prefix letter is cut, only when the first was ו or ש, as
//     in וְהַ or שֶׁבַּ.
//
// Matching the token as written first keeps שלום and בית from being read as a
// prefix and a stem. The lookup keys are the lexicon's own text_nfc, so the
// consonant skeleton only hits a lexicon that stores unpointed text. A token
// that matches nothing is unknown.
const prefixLetters = "והבלמשכ"

func (b *briefScan) resolve(tok string) lexRef {
	n := normalize(tok)
	if r, ok := b.refs[n]; ok {
		return r
	}
	r := lexRef{}
	for _, cand := range tokenCandidates(n) {
		if id, ok := b.lexID(cand); ok {
			r = lexRef{id: id, key: cand, ok: true}
			break
		}
		if b.err != nil {
			break
		}
	}
	if b.err == nil {
		b.refs[n] = r
	}
	return r
}

func (b *briefScan) lexID(key string) (int64, bool) {
	if b.err != nil {
		return 0, false
	}
	if p, ok := b.ids[key]; ok {
		return p.id, p.ok
	}
	if err := b.ctx.Err(); b.fail(err) {
		return 0, false
	}
	id, ok, err := b.lk.LexemeByText(b.ctx, key)
	if b.fail(err) {
		return 0, false
	}
	b.ids[key] = &idProbe{id: id, ok: ok}
	return id, ok
}

func tokenCandidates(n string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(c string) {
		if c != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	forms := func(s string) {
		add(s)
		add(dropFirstDagesh(s))
		add(stripNiqqud(s))
	}
	forms(n)
	rest := n
	for i := 0; i < 2; i++ {
		r, letter, ok := cutPrefix(rest)
		if !ok || hebrewLetterCount(r) < 2 {
			break
		}
		forms(r)
		if i == 0 && letter != 'ו' && letter != 'ש' {
			break
		}
		rest = r
	}
	return out
}

// cutPrefix removes a leading prefix letter and the points on it.
func cutPrefix(s string) (rest string, letter rune, ok bool) {
	runes := []rune(s)
	if len(runes) == 0 || !strings.ContainsRune(prefixLetters, runes[0]) {
		return s, 0, false
	}
	i := 1
	for i < len(runes) && isMark(runes[i]) {
		i++
	}
	return string(runes[i:]), runes[0], true
}

func isMark(r rune) bool {
	return isStrippedMark(r) || r == 0x05C1 || r == 0x05C2
}

// dropFirstDagesh removes the dagesh from the first letter only. After the
// article or a prefix the next letter usually takes a dagesh, and the lexicon
// does not store it.
func dropFirstDagesh(s string) string {
	runes := []rune(s)
	start := -1
	for i, r := range runes {
		if isHebrewLetter(r) {
			start = i
			break
		}
	}
	if start < 0 {
		return s
	}
	out := make([]rune, 0, len(runes))
	out = append(out, runes[:start+1]...)
	i := start + 1
	for ; i < len(runes) && !isHebrewLetter(runes[i]); i++ {
		if runes[i] != 0x05BC {
			out = append(out, runes[i])
		}
	}
	out = append(out, runes[i:]...)
	return string(out)
}
func hebrewLetterCount(s string) int {
	n := 0
	for _, r := range s {
		if isHebrewLetter(r) {
			n++
		}
	}
	return n
}
