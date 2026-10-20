package activity

import (
	"context"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Check grades a response to a built item and records it through evidence.Recorder.
//
// The token must be one Build returned to this learner, unexpired, and not yet answered.
// A token from another learner is refused with ErrTokenUser and the item stays pending, so
// nobody can burn another learner's item. A response that does not fit the item is refused
// with ErrBadResponse and the item stays pending too. Otherwise the item is used up, also
// when the learner answered wrong.
//
// What is written, per word asked: one attempts row (skill and activity of the item, the
// item's source, what was shown, what was answered, correct, "I don't know", hint, and the
// latency from Build to Check on the engine clock), and, unless the source is lesson, the
// card for that word and skill, which is created at its first attempt. A wrong pick of
// another word writes a confusion, and a typed answer within two letters of the right one
// writes a letter pair for each wrong letter. A match item writes one attempt per word.
//
// Practice and review give feedback. A placement item does not: Result then holds only the
// outcome.
func (e *Engine) Check(ctx context.Context, userID int64, token string, resp Response) (Result, error) {
	if err := e.ready(); err != nil {
		return Result{}, err
	}
	c, err := verifyClaims(e.key, token)
	if err != nil {
		return Result{}, err
	}
	if c.User != userID || userID <= 0 {
		return Result{}, ErrTokenUser
	}
	now := e.clock.Now()
	if now.Unix() > c.Exp {
		e.pending.remove(c.ID)
		return Result{}, ErrTokenExpired
	}
	p, ok := e.pending.claim(c.ID)
	if !ok {
		return Result{}, ErrTokenUnknown
	}
	if p.user != c.User || p.lexeme != c.Lexeme || p.kind != c.Kind {
		e.pending.release(p)
		return Result{}, ErrTokenInvalid
	}
	if !now.Before(p.expires) {
		e.pending.remove(c.ID)
		return Result{}, ErrTokenExpired
	}

	lookup := func(text string) (Word, bool) { return e.wordByText(ctx, userID, p.lexeme, text) }
	g, err := grade(p, resp, now.Sub(p.builtAt), lookup)
	if err != nil {
		e.pending.release(p)
		return Result{}, err
	}

	for i := p.wrote; i < len(g.attempts); i++ {
		a := g.attempts[i]
		a.UserID, a.Source, a.UnitID = p.user, p.source, p.unit
		a.SkipSchedule = p.source == evidence.SourceLesson
		rating, err := e.rec.Attempt(ctx, a)
		if err != nil {
			e.pending.release(p)
			return Result{}, err
		}
		p.wrote = i + 1
		p.ratings = append(p.ratings, rating)
	}
	e.pending.remove(c.ID)

	res := g.result
	res.Rating = p.ratings[g.anchor]
	res.Feedback = p.source != evidence.SourcePlacement
	if !res.Feedback {
		res = Result{Correct: res.Correct, NearMiss: res.NearMiss, Rating: res.Rating}
	}
	return res, nil
}

// wordByText finds another word whose spelling is exactly the typed text, ignoring niqqud.
func (e *Engine) wordByText(ctx context.Context, userID, except int64, text string) (Word, bool) {
	want := Normalize(text)
	words, err := e.loadWords(ctx, userID)
	if err != nil {
		return Word{}, false
	}
	for _, w := range words {
		if w.ID != except && Normalize(w.Text) == want {
			return w, true
		}
	}
	return Word{}, false
}

// graded is what grade works out before anything is written.
type graded struct {
	attempts []evidence.Attempt
	// anchor is the index of the attempt about the word the item was built for.
	anchor int
	result Result
}

// grade turns a response into the attempts to write and the result. It reads no store: lookup
// finds the word a typed answer spells when it is another word. It validates the response
// against the item and returns ErrBadResponse when it does not fit.
func grade(p *pending, resp Response, elapsed time.Duration, lookup func(string) (Word, bool)) (graded, error) {
	if elapsed < 0 {
		elapsed = 0
	}
	spec, _ := SpecOf(p.kind)
	switch {
	case spec.Multi:
		return gradeMatch(p, resp, elapsed)
	case spec.Typed:
		return gradeTyped(p, resp, elapsed, lookup)
	default:
		return gradePick(p, resp, elapsed)
	}
}

func baseAttempt(p *pending, w Word, resp Response, latency time.Duration) evidence.Attempt {
	return evidence.Attempt{
		LexemeID: w.ID,
		Skill:    p.skill,
		Activity: string(p.kind),
		Prompt:   promptText(p.kind, w),
		DontKnow: resp.DontKnow,
		HintUsed: resp.Hinted,
		Latency:  latency,
	}
}

// promptText is what the learner was shown, as text, for the attempts row. A clip is stored
// as the Hebrew it speaks, so the row says what was asked without storing audio.
func promptText(k Kind, w Word) string {
	if k == TypeIt {
		return w.Gloss
	}
	return w.Text
}

func gradePick(p *pending, resp Response, elapsed time.Duration) (graded, error) {
	if !resp.DontKnow && (resp.Choice < 0 || resp.Choice >= len(p.words)) {
		return graded{}, ErrBadResponse
	}
	anchor := p.words[p.correct]
	a := baseAttempt(p, anchor, resp, elapsed)
	res := Result{Answer: anchor.Gloss, Text: anchor.Text, Gloss: anchor.Gloss, AudioID: anchor.AudioID}
	if !resp.DontKnow {
		picked := p.words[resp.Choice]
		a.Response = picked.Gloss
		a.Correct = resp.Choice == p.correct
		if !a.Correct {
			a.ChosenLexemeID = picked.ID
			res.Confused = append(res.Confused, confusedOf(anchor, picked))
		}
	}
	res.Correct = a.Correct
	return graded{attempts: []evidence.Attempt{a}, result: res}, nil
}

func gradeTyped(p *pending, resp Response, elapsed time.Duration, lookup func(string) (Word, bool)) (graded, error) {
	text := strings.TrimSpace(resp.Text)
	if !resp.DontKnow && (text == "" || len(resp.Text) > MaxResponseBytes) {
		return graded{}, ErrBadResponse
	}
	anchor := p.words[0]
	a := baseAttempt(p, anchor, resp, elapsed)
	res := Result{Answer: anchor.Text, Text: anchor.Text, Gloss: anchor.Gloss, AudioID: anchor.AudioID}
	if !resp.DontKnow {
		a.Response = text
		rating := fsrs.GradeTyped(p.expected, text)
		a.Correct = rating != fsrs.Again
		a.NearMiss = rating == fsrs.Hard
		res.NearMiss = a.NearMiss
		var chosen Word
		if !a.Correct {
			if w, ok := lookup(text); ok {
				chosen = w
				a.ChosenLexemeID = w.ID
				res.Confused = append(res.Confused, confusedOf(anchor, w))
			}
		}
		// A typed answer that is exactly another word is logged by the recorder as a mix-up
		// of two words. Otherwise each wrong letter is a letter pair.
		if rating != fsrs.Good && chosen.ID == 0 {
			for _, key := range LetterPairs(p.expected, text) {
				a.Confusions = append(a.Confusions, evidence.Confusion{Kind: fsrs.MissLetterPair, Key: key})
				res.Confused = append(res.Confused, Confused{Kind: fsrs.MissLetterPair, Key: key, Word: anchor.Text})
			}
		}
	}
	res.Correct = a.Correct
	return graded{attempts: []evidence.Attempt{a}, result: res}, nil
}

func gradeMatch(p *pending, resp Response, elapsed time.Duration) (graded, error) {
	n := len(p.words)
	if !resp.DontKnow {
		if len(resp.Pairs) != n {
			return graded{}, ErrBadResponse
		}
		used := make([]bool, n)
		for _, j := range resp.Pairs {
			if j < 0 || j >= n || used[j] {
				return graded{}, ErrBadResponse
			}
			used[j] = true
		}
	}
	// The time of the item is shared by its words.
	per := elapsed / time.Duration(n)
	g := graded{}
	allRight := true
	for i, w := range p.words {
		a := baseAttempt(p, w, resp, per)
		pr := PairResult{Left: i, Chosen: -1, Text: w.Text, Gloss: w.Gloss, AudioID: w.AudioID}
		if !resp.DontKnow {
			j := resp.Pairs[i]
			picked := p.rightWords[j]
			pr.Chosen = j
			a.Response = picked.Gloss
			a.Correct = j == p.rightOf[i]
			if !a.Correct {
				a.ChosenLexemeID = picked.ID
				g.result.Confused = append(g.result.Confused, confusedOf(w, picked))
			}
		}
		pr.Correct = a.Correct
		allRight = allRight && a.Correct
		if w.ID == p.lexeme {
			g.anchor = i
		}
		g.attempts = append(g.attempts, a)
		g.result.Pairs = append(g.result.Pairs, pr)
	}
	g.result.Correct = allRight
	anchor := p.words[g.anchor]
	g.result.Answer, g.result.Text, g.result.Gloss, g.result.AudioID = anchor.Gloss, anchor.Text, anchor.Gloss, anchor.AudioID
	return g, nil
}

// confusedOf describes asked taken for picked the way internal/evidence stores it: a letter
// pair when the two spellings differ by one letter (final letters counted as the ordinary
// letter, niqqud ignored), otherwise the two words in code point order.
func confusedOf(asked, picked Word) Confused {
	c := Confused{Word: asked.Text, With: picked.Text}
	if key, ok := foldedPair(asked.Text, picked.Text); ok {
		c.Kind, c.Key = fsrs.MissLetterPair, key
		return c
	}
	a, b := asked.Text, picked.Text
	if a > b {
		a, b = b, a
	}
	c.Kind, c.Key = fsrs.MissOther, a+"/"+b
	return c
}

func foldedPair(a, b string) (string, bool) {
	return oneLetterPair(foldFinals(Normalize(a)), foldFinals(Normalize(b)))
}

func foldFinals(s string) string {
	return strings.Map(ordinary, s)
}
