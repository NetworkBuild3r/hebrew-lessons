package units

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Validate checks doc against the authoring rules.
// The result is nil when the document is clean. Otherwise it is sorted by
// JSON pointer, then by rule id.
func Validate(ctx context.Context, doc *Unit, lk Lookup) []Issue {
	if ctx == nil {
		ctx = context.Background()
	}
	return sortedIssues(collect(ctx, doc, lk))
}

// sortedIssues orders issues by JSON pointer, then rule id, then message.
// It returns nil for an empty list.
func sortedIssues(issues []Issue) []Issue {
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Pointer != issues[j].Pointer {
			return issues[i].Pointer < issues[j].Pointer
		}
		if issues[i].Rule != issues[j].Rule {
			return issues[i].Rule < issues[j].Rule
		}
		return issues[i].Msg < issues[j].Msg
	})
	if len(issues) == 0 {
		return nil
	}
	return issues
}

func collect(ctx context.Context, doc *Unit, lk Lookup) []Issue {
	if doc == nil {
		return []Issue{issue(RuleSchemaRequired, SeverityBlock, "", "unit document is missing")}
	}
	// The shape rules read the real document. The text rules read a copy in
	// which every dialogue line is its own activity (validate_dialogue.go).
	flat, refs := flatten(doc)
	issues := schemaIssues(doc)
	issues = append(issues, dialogueIssues(doc)...)
	issues = append(issues, remapIssues(textIssues(ctx, flat, lk), refs)...)
	// Target is checked on the real activities. A flattened dialogue would
	// drop the field, and remapping those pointers would point at a line.
	return append(issues, targetIssues(ctx, doc, lk)...)
}

// textIssues runs the rules that read activity text, on a flattened document.
func textIssues(ctx context.Context, doc *Unit, lk Lookup) []Issue {
	var issues []Issue
	words := indexWords(doc)
	issues = append(issues, budgetIssues(doc, words)...)
	issues = append(issues, refIssues(words)...)
	if lk == nil {
		issues = append(issues, issue(RuleSchemaLookup, SeverityBlock, "", "lexicon lookup is missing"))
		return issues
	}
	s := &scan{ctx: ctx, lk: lk, position: doc.Position}
	if err := s.preload(); err != nil {
		return append(issues, lookupIssue(err))
	}
	issues = append(issues, lexemeIssues(s, doc, words)...)
	if s.err != nil {
		return append(issues, lookupIssue(s.err))
	}
	issues = append(issues, audioIssues(s, doc)...)
	if s.err != nil {
		return append(issues, lookupIssue(s.err))
	}
	issues = append(issues, coverageIssues(s, doc, words)...)
	if s.err != nil {
		return append(issues, lookupIssue(s.err))
	}
	issues = append(issues, recycleIssues(doc, words)...)
	issues = append(issues, nounIssues(s, doc, words)...)
	if s.err != nil {
		return append(issues, lookupIssue(s.err))
	}
	issues = append(issues, storyUnmappedIssues(s, doc)...)
	if s.err != nil {
		return append(issues, lookupIssue(s.err))
	}
	issues = append(issues, storyPointIssues(s, doc)...)
	if s.err != nil {
		return append(issues, lookupIssue(s.err))
	}
	return issues
}

func issue(rule, sev, pointer, msg string) Issue {
	return Issue{Rule: rule, Severity: sev, Pointer: pointer, Msg: msg}
}

func lookupIssue(err error) Issue {
	return issue(RuleSchemaLookup, SeverityBlock, "", "lexicon lookup failed: "+err.Error())
}

func at(parts ...any) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteByte('/')
		switch v := p.(type) {
		case int:
			b.WriteString(strconv.Itoa(v))
		case string:
			b.WriteString(escapePointer(v))
		default:
			b.WriteString(escapePointer(fmt.Sprint(v)))
		}
	}
	return b.String()
}

func escapePointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

type wordRef struct {
	index    int
	raw      string
	norm     string
	bare     string
	lesson   int
	activity int
	found    bool
	dup      bool
}

func indexWords(doc *Unit) []wordRef {
	words := make([]wordRef, len(doc.NewWords))
	seen := map[string]int{}
	for i, raw := range doc.NewWords {
		n := normalize(raw)
		b := stripNiqqud(n)
		w := wordRef{index: i, raw: raw, norm: n, bare: b}
		if n != "" {
			if _, ok := seen[b]; ok {
				w.dup = true
			} else {
				seen[b] = i
				w.lesson, w.activity, w.found = findIntro(doc, n, b)
			}
		}
		words[i] = w
	}
	return words
}

func findIntro(doc *Unit, norm, bare string) (int, int, bool) {
	for li, les := range doc.Lessons {
		for ai, act := range les.Activities {
			for _, tok := range tokenize(act.Text) {
				n := normalize(tok)
				if n == norm || stripNiqqud(n) == bare {
					return li, ai, true
				}
			}
		}
	}
	return 0, 0, false
}

// introducedBy reports that w's introducing activity is this one or an
// earlier one. The introducing activity itself counts: a passage that
// introduces a new word does not score that word as unknown.
func introducedBy(w wordRef, lesson, activity int) bool {
	if !w.found {
		return false
	}
	if w.lesson < lesson {
		return true
	}
	return w.lesson == lesson && w.activity <= activity
}

func countWord(les Lesson, w wordRef) int {
	n := 0
	for _, act := range les.Activities {
		for _, tok := range tokenize(act.Text) {
			tn := normalize(tok)
			if tn == w.norm || stripNiqqud(tn) == w.bare {
				n++
			}
		}
	}
	return n
}

type probe struct {
	lexDone   bool
	lexOK     bool
	factsDone bool
	factsOK   bool
	facts     LexemeFacts
	audioDone bool
	audioOK   bool
}

type scan struct {
	ctx       context.Context
	lk        Lookup
	position  int
	err       error
	cache     map[string]*probe
	spk       map[string]bool
	knownNorm map[string]bool
	knownBare map[string]bool
}

func (s *scan) preload() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	s.cache = map[string]*probe{}
	s.knownNorm = map[string]bool{}
	s.knownBare = map[string]bool{}
	known, err := s.lk.KnownLexemes(s.ctx, s.position)
	if err != nil {
		return err
	}
	for text, ok := range known {
		if !ok {
			continue
		}
		n := normalize(text)
		if n == "" {
			continue
		}
		s.knownNorm[n] = true
		s.knownBare[stripNiqqud(n)] = true
	}
	return nil
}

func (s *scan) slot(key string) *probe {
	p := s.cache[key]
	if p == nil {
		p = &probe{}
		s.cache[key] = p
	}
	return p
}

func (s *scan) fail(err error) bool {
	if err == nil {
		return false
	}
	if s.err == nil {
		s.err = err
	}
	return true
}

func (s *scan) inLexicon(text string) bool {
	if s.err != nil {
		return false
	}
	n := normalize(text)
	if n == "" {
		return false
	}
	if s.probeLex(n) {
		return true
	}
	b := stripNiqqud(n)
	if b != n && s.probeLex(b) {
		return true
	}
	return false
}

func (s *scan) probeLex(key string) bool {
	p := s.slot(key)
	if p.lexDone {
		return p.lexOK
	}
	if err := s.ctx.Err(); s.fail(err) {
		return false
	}
	_, ok, err := s.lk.LexemeByText(s.ctx, key)
	if s.fail(err) {
		return false
	}
	p.lexDone = true
	p.lexOK = ok
	return ok
}

func (s *scan) factsFor(text string) (LexemeFacts, bool) {
	if s.err != nil {
		return LexemeFacts{}, false
	}
	n := normalize(text)
	if n == "" {
		return LexemeFacts{}, false
	}
	if f, ok := s.probeFacts(n); ok {
		return f, true
	}
	b := stripNiqqud(n)
	if b != n {
		if f, ok := s.probeFacts(b); ok {
			return f, true
		}
	}
	return LexemeFacts{}, false
}

func (s *scan) probeFacts(key string) (LexemeFacts, bool) {
	p := s.slot(key)
	if p.factsDone {
		return p.facts, p.factsOK
	}
	if err := s.ctx.Err(); s.fail(err) {
		return LexemeFacts{}, false
	}
	f, ok, err := s.lk.Facts(s.ctx, key)
	if s.fail(err) {
		return LexemeFacts{}, false
	}
	p.factsDone = true
	p.factsOK = ok
	p.facts = f
	return f, ok
}

// hasAudioFor reports a clip for text in the voice of speaker a or b. A lookup
// that is not a SpeakerLookup cannot tell the voices apart, so it fails the scan
// with a lookup issue rather than accept a clip in the wrong voice.
func (s *scan) hasAudioFor(text, speaker string) bool {
	if s.err != nil {
		return false
	}
	sl, ok := s.lk.(SpeakerLookup)
	if !ok {
		s.fail(errors.New("the lookup cannot check a clip in a speaker's voice"))
		return false
	}
	n := normalize(text)
	key := speaker + "\x00" + n
	if v, done := s.spk[key]; done {
		return v
	}
	if err := s.ctx.Err(); s.fail(err) {
		return false
	}
	has, err := sl.HasAudioForSpeaker(s.ctx, n, speaker)
	if s.fail(err) {
		return false
	}
	if s.spk == nil {
		s.spk = map[string]bool{}
	}
	s.spk[key] = has
	return has
}

func (s *scan) hasAudio(text string) bool {
	if s.err != nil {
		return false
	}
	n := normalize(text)
	p := s.slot(n)
	if p.audioDone {
		return p.audioOK
	}
	if err := s.ctx.Err(); s.fail(err) {
		return false
	}
	ok, err := s.lk.HasAudio(s.ctx, n)
	if s.fail(err) {
		return false
	}
	p.audioDone = true
	p.audioOK = ok
	return ok
}

func (s *scan) learnerKnows(tok string) bool {
	n := normalize(tok)
	if s.knownNorm[n] || s.knownBare[stripNiqqud(n)] {
		return true
	}
	return false
}

func (s *scan) isName(tok string) bool {
	f, ok := s.factsFor(tok)
	return ok && namePOS(f.POS)
}

func namePOS(pos string) bool {
	switch strings.ToLower(strings.TrimSpace(pos)) {
	case "name", "propn", "proper noun", "proper-noun":
		return true
	default:
		return false
	}
}

func nounPOS(pos string) bool {
	switch strings.ToLower(strings.TrimSpace(pos)) {
	case "noun", "n":
		return true
	default:
		return false
	}
}

func genderOK(g string) bool {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "m", "f", "both":
		return true
	default:
		return false
	}
}

func knownActivity(typ string) bool {
	switch typ {
	case ActivityListenPick, ActivityRecall, ActivityProduce, ActivityCloze, ActivityPattern, ActivityStory, ActivityShadow, ActivityDialogue:
		return true
	default:
		return false
	}
}

func quoteList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = strconv.Quote(s)
	}
	return strings.Join(parts, ", ")
}
