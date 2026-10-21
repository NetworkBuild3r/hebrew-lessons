package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// sentenceListenChoicesAreTranslations is the sentence listen_pick choice
// rule. A line of two or more lexicon words is asked as the author's English
// of the whole line, including when target credits one word of that line.
const sentenceListenChoicesAreTranslations = true

// sentenceChoiceCount is the buttons on a graded sentence listen_pick:
// the translation plus three distractors. Hebrew Dev decision 22.
const sentenceChoiceCount = 4

// sentenceMinChoices is the smallest set that is still a question.
// Fewer than this plays ungraded: listen, then reveal, one lesson_listen
// exposure, and no credit.
const sentenceMinChoices = 3

// sentenceListenPick reports a listen_pick whose choices are whole-line
// English. A single lexicon word stays on the gloss buttons.
func sentenceListenPick(act units.Activity, lex map[string]db.LexemeView) bool {
	if act.Type != units.ActivityListenPick || lexemeTokenCount(act, lex) < 2 {
		return false
	}
	if strings.TrimSpace(act.Target) != "" && !sentenceListenChoicesAreTranslations {
		return false
	}
	return true
}

// listenPickTarget is the word a listen_pick credits.
// A set target credits only that word, and does not fall back when the token
// is missing. With no target, a line of one lexicon word credits that word.
// Two or more lexicon words credit nobody: the caller logs an exposure.
func listenPickTarget(act units.Activity, lex map[string]db.LexemeView) db.LexemeView {
	if tok, ok := tokenForTarget(actTokens(act), act.Target); ok {
		if view, found := lex[tok]; found {
			return view
		}
	}
	if strings.TrimSpace(act.Target) != "" {
		return db.LexemeView{}
	}
	var only db.LexemeView
	n := 0
	for _, tok := range actTokens(act) {
		view, ok := lex[tok]
		if !ok {
			continue
		}
		n++
		only = view
	}
	if n == 1 {
		return only
	}
	return db.LexemeView{}
}

// listenLexemeIDs is every lexicon token on the line, in order, once each.
func listenLexemeIDs(act units.Activity, lex map[string]db.LexemeView) []int64 {
	var ids []int64
	seen := map[int64]bool{}
	for _, tok := range actTokens(act) {
		view, ok := lex[tok]
		if !ok || view.ID == 0 || seen[view.ID] {
			continue
		}
		seen[view.ID] = true
		ids = append(ids, view.ID)
	}
	return ids
}

// foldTranslation is the key the unit contract uses to tell English lines
// apart: lowercase, punctuation removed, and runs of space collapsed to one.
// units.foldTranslation is the same function. It is not exported.
func foldTranslation(s string) string {
	var b strings.Builder
	gap := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsPunct(r) {
			continue
		}
		if unicode.IsSpace(r) {
			gap = true
			continue
		}
		if gap && b.Len() > 0 {
			b.WriteByte(' ')
		}
		gap = false
		b.WriteRune(r)
	}
	return b.String()
}

// spokenLine is one author-written English line and the Hebrew it translates.
// Hebrew is units.Normalize of that line, empty when the line has none.
type spokenLine struct {
	Hebrew  string
	English string
}

// functionWords are English words that do not count as content for the
// preference and the near-duplicate rules.
var functionWords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "to": true, "and": true,
	"or": true, "in": true, "on": true, "at": true, "for": true, "from": true,
	"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "it": true, "this": true, "that": true,
}

// sentenceAsk is the graded buttons for one sentence listen_pick.
// Nil means fewer than sentenceMinChoices distinct lines, so the step is ungraded.
// Values are HMAC choice ids, not the English.
func sentenceAsk(key []byte, run db.PlayerRun, act units.Activity, same, earlier []spokenLine) []views.Choice {
	if foldTranslation(act.Translation) == "" || len(key) == 0 {
		return nil
	}
	rng := sentenceChoiceRand(run, act)
	distractors := sentenceDistractors(act.Translation, units.Normalize(act.Text), same, earlier, rng)
	if 1+len(distractors) < sentenceMinChoices {
		return nil
	}
	options := make([]string, 0, 1+len(distractors))
	options = append(options, act.Translation)
	options = append(options, distractors...)
	rng.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})
	out := make([]views.Choice, len(options))
	for i, text := range options {
		out[i] = views.Choice{Value: sentenceChoiceID(key, run.ID, run.ActivityIx, i), Label: text}
	}
	return out
}

// sentenceDistractors is up to sentenceChoiceCount-1 other lines.
// Same-unit lines come first, then earlier units. Within each of those,
// lines within two words of the answer that share a content word come first.
// Each of those four groups is shuffled before the cap, so two items in one
// unit do not share one document-order set. A line is left out when its
// Hebrew is the heard line, when it folds to a line already kept, or when it
// is a near duplicate of the answer or of a kept line.
func sentenceDistractors(answer, heard string, same, earlier []spokenLine, rng *rand.Rand) []string {
	if rng == nil || foldTranslation(answer) == "" {
		return nil
	}
	samePref, sameRest := splitSentenceLines(same, answer, heard)
	earPref, earRest := splitSentenceLines(earlier, answer, heard)
	cap := sentenceChoiceCount - 1
	var picked []spokenLine
	seen := map[string]bool{foldTranslation(answer): true}
	take := func(group []spokenLine) {
		cp := append([]spokenLine(nil), group...)
		rng.Shuffle(len(cp), func(i, j int) {
			cp[i], cp[j] = cp[j], cp[i]
		})
		for _, line := range cp {
			if len(picked) >= cap {
				return
			}
			key := foldTranslation(line.English)
			if key == "" || seen[key] || sentenceNear(answer, line.English) {
				continue
			}
			if nearKept(line.English, picked) {
				continue
			}
			seen[key] = true
			picked = append(picked, line)
		}
	}
	take(samePref)
	take(sameRest)
	take(earPref)
	take(earRest)
	out := make([]string, len(picked))
	for i, line := range picked {
		out[i] = line.English
	}
	return out
}

// splitSentenceLines drops lines that cannot be a distractor, then splits
// the rest into the preferred group and the remainder. Document order is kept
// until the caller shuffles.
func splitSentenceLines(lines []spokenLine, answer, heard string) (pref, rest []spokenLine) {
	seen := map[string]bool{}
	for _, line := range lines {
		key := foldTranslation(line.English)
		if key == "" || key == foldTranslation(answer) || seen[key] {
			continue
		}
		if heard != "" && line.Hebrew != "" && line.Hebrew == heard {
			continue
		}
		if sentenceNear(answer, line.English) {
			continue
		}
		seen[key] = true
		if sentencePreferred(answer, line.English) {
			pref = append(pref, line)
		} else {
			rest = append(rest, line)
		}
	}
	return pref, rest
}

// sentencePreferred is a line within two words of the answer that shares a
// content word with it.
func sentencePreferred(answer, line string) bool {
	d := wordCount(answer) - wordCount(line)
	if d < 0 {
		d = -d
	}
	if d > 2 {
		return false
	}
	return sharesContent(answer, line)
}

// sentenceNear is the near-duplicate rule: content-word Jaccard at least
// 0.75, or one folded line containing the other.
func sentenceNear(a, b string) bool {
	fa, fb := foldTranslation(a), foldTranslation(b)
	if fa == "" || fb == "" || fa == fb {
		return false
	}
	if strings.Contains(fa, fb) || strings.Contains(fb, fa) {
		return true
	}
	return jaccard(contentWords(a), contentWords(b)) >= 0.75
}

func nearKept(line string, picked []spokenLine) bool {
	for _, have := range picked {
		if sentenceNear(line, have.English) {
			return true
		}
	}
	return false
}

func wordCount(s string) int {
	f := foldTranslation(s)
	if f == "" {
		return 0
	}
	return len(strings.Fields(f))
}

func contentWords(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(foldTranslation(s)) {
		if functionWords[w] {
			continue
		}
		out[w] = true
	}
	return out
}

func sharesContent(a, b string) bool {
	bb := contentWords(b)
	for w := range contentWords(a) {
		if bb[w] {
			return true
		}
	}
	return false
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	inter := 0
	union := len(a)
	for w := range b {
		if a[w] {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// sentenceChoiceID is the opaque button value. It binds the run, the activity,
// and the index in the shuffled list. Grading recomputes it.
func sentenceChoiceID(key []byte, runID int64, activityIx, index int) string {
	mac := hmac.New(sha256.New, key)
	var buf [24]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(runID))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(uint(activityIx)))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(index))
	mac.Write(buf[:])
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// sentenceChoiceIndex is the choice whose id matches posted.
// The comparison is over the id bytes. A raw English string does not match.
func sentenceChoiceIndex(key []byte, run db.PlayerRun, choices []views.Choice, posted string) (int, bool) {
	posted = strings.TrimSpace(posted)
	if posted == "" || len(key) == 0 {
		return 0, false
	}
	for i := range choices {
		want := sentenceChoiceID(key, run.ID, run.ActivityIx, i)
		if len(posted) == len(want) && hmac.Equal([]byte(posted), []byte(want)) {
			return i, true
		}
	}
	return 0, false
}

// markSentenceChoices keeps every button after the answer, disabled.
// The translation is correct ("Right"). The learner's other pick is wrong
// ("Your pick"). The rest are dim.
func markSentenceChoices(choices []views.Choice, translation, picked string) []views.Choice {
	out := append([]views.Choice(nil), choices...)
	for i := range out {
		out[i].Disabled = true
		switch {
		case out[i].Label == translation:
			out[i].State = "correct"
			out[i].Tag = "Right"
		case picked != "" && out[i].Value == picked:
			out[i].State = "wrong"
			out[i].Tag = "Your pick"
		default:
			out[i].State = "dim"
		}
	}
	return out
}

// sentenceGradeCopy is the status-sheet text.
// A miss names one Hebrew word the heard line has and the picked line lacks,
// and leaves that sentence out when no such word can be found.
func sentenceGradeCopy(act units.Activity, lex map[string]db.LexemeView, pool []spokenLine, pickedEnglish string, correct bool) (lead, class string, contrast *views.ChoiceContrast) {
	quoted := sentenceQuote(act.Translation)
	if correct {
		return "Right. " + quoted, "feedback feedback--right", nil
	}
	lead = "Not quite. It says " + quoted
	class = "feedback feedback--wrong"
	if pickedEnglish == "" {
		return lead, class, nil
	}
	word, ok := sentenceMissingWord(act, lex, pool, pickedEnglish)
	if !ok {
		return lead, class, nil
	}
	return lead, class, &views.ChoiceContrast{
		Picked:   pickedEnglish,
		Hebrew:   word.Text,
		Translit: word.Translit,
		Gloss:    word.Gloss,
	}
}

// sentenceQuote wraps an English line as a sentence inside curly quotes.
// A line that already ends with sentence punctuation is left as it is.
func sentenceQuote(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "“”"
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	if r != '.' && r != '!' && r != '?' && r != '…' {
		s += "."
	}
	return "“" + s + "”"
}

// sentenceMissingWord is the first lexicon word of the heard line that the
// picked line's Hebrew does not contain.
func sentenceMissingWord(act units.Activity, lex map[string]db.LexemeView, pool []spokenLine, pickedEnglish string) (db.LexemeView, bool) {
	key := foldTranslation(pickedEnglish)
	var pickedHebrew string
	found := false
	for _, line := range pool {
		if foldTranslation(line.English) == key {
			pickedHebrew = line.Hebrew
			found = true
			break
		}
	}
	if !found {
		return db.LexemeView{}, false
	}
	have := map[string]bool{}
	for _, tok := range units.Tokenize(pickedHebrew) {
		have[units.Normalize(tok)] = true
	}
	for _, tok := range units.Tokenize(act.Text) {
		view, ok := lex[tok]
		if !ok || view.ID == 0 {
			continue
		}
		if have[units.Normalize(tok)] {
			continue
		}
		return view, true
	}
	return db.LexemeView{}, false
}

// sentenceChoiceRand shuffles one ask. The seed is the run id, the activity
// index, and the Hebrew line, so a reload keeps the order and another run
// does not.
func sentenceChoiceRand(run db.PlayerRun, act units.Activity) *rand.Rand {
	h := sha256.New()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(run.ID))
	h.Write(buf[:])
	binary.LittleEndian.PutUint64(buf[:], uint64(run.ActivityIx))
	h.Write(buf[:])
	h.Write([]byte(act.Text))
	sum := h.Sum(nil)
	return rand.New(rand.NewPCG(
		binary.LittleEndian.Uint64(sum[0:8]),
		binary.LittleEndian.Uint64(sum[8:16]),
	))
}

// spokenLines is the same-unit pool, then earlier published units this learner
// can see. EarlierUnitTranslations is that earlier-unit read.
func (p player) spokenLines(ctx context.Context, userID int64, lesson db.CourseLesson) (same, earlier []spokenLine, err error) {
	if p.store == nil || lesson.UnitID <= 0 {
		return nil, nil, nil
	}
	snap, err := p.store.LoadUnit(ctx, lesson.UnitID, nil)
	if err != nil {
		return nil, nil, err
	}
	same = linesOfDoc(snap.Doc)
	rows, err := p.store.EarlierUnitTranslations(ctx, userID, lesson.Position)
	if err != nil {
		return nil, nil, err
	}
	earlier = make([]spokenLine, len(rows))
	for i, row := range rows {
		earlier[i] = spokenLine{Hebrew: row.TextNFC, English: row.Translation}
	}
	return same, earlier, nil
}

func linesOfDoc(doc units.Unit) []spokenLine {
	var lines []spokenLine
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			if line, ok := spokenOf(act.Text, act.Translation); ok {
				lines = append(lines, line)
			}
			for _, ln := range act.Lines {
				if line, ok := spokenOf(ln.Text, ln.Translation); ok {
					lines = append(lines, line)
				}
			}
		}
	}
	return lines
}

func spokenOf(hebrew, english string) (spokenLine, bool) {
	if strings.TrimSpace(english) == "" {
		return spokenLine{}, false
	}
	return spokenLine{Hebrew: units.Normalize(hebrew), English: english}, true
}

// applySentence fills a sentence listen_pick step: no word placeholders
// before the answer, the reveal under the orb after it, and the marked choices.
func (p player) applySentence(ctx context.Context, lesson db.CourseLesson, run db.PlayerRun, act units.Activity, lex map[string]db.LexemeView, activity *views.Activity, picked string) error {
	same, earlier, err := p.spokenLines(ctx, run.UserID, lesson)
	if err != nil {
		return err
	}
	choices := sentenceAsk(p.choiceKey, run, act, same, earlier)
	graded := len(choices) >= sentenceMinChoices
	activity.Sentence = true
	activity.Feedback = ""
	activity.FeedbackClass = ""
	activity.Words = nil
	if run.Phase == "ask" {
		activity.Choices = choices
		activity.Asking = graded
		activity.Sheet = false
		activity.Reveal = false
		return nil
	}
	activity.Reveal = true
	activity.Translation = act.Translation
	activity.Words, activity.GlossWords = sentenceRevealWords(act, lex)
	activity.Sheet = true
	activity.Asking = false
	if !graded {
		activity.Choices = nil
		return nil
	}
	activity.Choices = markSentenceChoices(choices, act.Translation, picked)
	pickedEnglish := ""
	for _, c := range choices {
		if c.Value == picked {
			pickedEnglish = c.Label
		}
	}
	correct := pickedEnglish != "" && pickedEnglish == act.Translation
	pool := append(append([]spokenLine{}, same...), earlier...)
	activity.Feedback, activity.FeedbackClass, activity.Contrast = sentenceGradeCopy(act, lex, pool, pickedEnglish, correct)
	return nil
}

// sentenceRevealWords is the Hebrew line and, separately, the muted glosses.
// The author's translation is the main English, so the glosses are not the
// meaning line.
func sentenceRevealWords(act units.Activity, lex map[string]db.LexemeView) (line, glosses []views.Word) {
	for _, tok := range units.Tokenize(act.Text) {
		if !hebrewToken(tok) {
			continue
		}
		view := lex[tok]
		line = append(line, views.Word{Text: tok, Translit: view.Translit, ShowText: true, ShowTranslit: view.Translit != ""})
		if view.Gloss != "" {
			glosses = append(glosses, views.Word{Gloss: view.Gloss, ShowGloss: true})
		}
	}
	return line, glosses
}
