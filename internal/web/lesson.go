package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/choices"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// player serves one activity at a time. Progress lives on lesson_runs.
// The stored document only has an activity type and a Hebrew text, so the
// screen for each type is derived here:
//
//   - story, pattern, shadow, and dialogue show every Hebrew word as a
//     tap-to-flip card: pointed text and transliteration in front, meaning
//     behind the tap. Continue counts the step correct. Flipping is not an
//     answer and writes no attempt.
//   - recall shows the pointed Hebrew and hides the transliteration and
//     gloss until the answer. Choices are English glosses.
//   - listen_pick hides the Hebrew, the transliteration, and the gloss
//     until the answer. The clip is the prompt. Choices are English glosses.
//     The button value is the lexeme id, so the pointed word is not in the markup.
//   - produce hides the Hebrew, the transliteration, and the clip until the
//     answer. Choices show pointed Hebrew. The button value is the lexeme id.
//   - cloze gaps the first new word. Until the answer, the other words show
//     pointed Hebrew and transliteration, with no gloss, and the sentence
//     clip stays hidden. After the answer, every word shows its pointed
//     text, transliteration, and gloss. Choice values are lexeme ids.
//   - any other type shows the words and a continue button.
//
// A step with no lexicon target is exposure, not a graded choice.
//
//   - dialogue shows its lines in order, each with its speaker's name and the
//     words with their sound and meaning. Each line plays from the clip in its
//     speaker's voice, and the learner can replay one line or the whole
//     conversation. Continue counts the step correct. A line with no clip in
//     its voice has no play control, and is never played in the other voice.
type player struct {
	store    *db.Store
	audioDir string
	voices   audio.Voices
}

func (p player) lessonIndex(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page := views.NewPage(lang, csrf, "lesson")
	userID, ok := p.learnerID(r)
	if !ok {
		render(w, r, views.Lesson(page, views.Screen{Empty: true}))
		return
	}
	lesson, ok, err := p.store.NextCourseLesson(r.Context(), userID)
	if err != nil {
		serverError(w)
		return
	}
	if !ok {
		render(w, r, views.Lesson(page, views.Screen{Empty: true}))
		return
	}
	p.writePlaying(w, r, page, userID, lesson, false)
}

func (p player) lessonAt(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page := views.NewPage(lang, csrf, "lesson")
	unitID, lessonIx, ok := parseLessonPath(r)
	if !ok {
		notFound(w, r)
		return
	}
	userID, ok := p.learnerID(r)
	if !ok {
		notFound(w, r)
		return
	}
	lesson, err := p.store.CourseLesson(r.Context(), userID, unitID, lessonIx)
	if errors.Is(err, db.ErrNotFound) {
		notFound(w, r)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	score, finished, err := p.store.FinishedLessonScore(r.Context(), userID, unitID, lessonIx)
	if err != nil {
		serverError(w)
		return
	}
	if finished {
		p.writeScreen(w, r, page, doneScreen(page, lesson, score), false)
		return
	}
	next, ok, err := p.store.NextCourseLesson(r.Context(), userID)
	if err != nil {
		serverError(w)
		return
	}
	if !ok || next.UnitID != lesson.UnitID || next.LessonIx != lesson.LessonIx {
		notFound(w, r)
		return
	}
	p.writePlaying(w, r, page, userID, lesson, false)
}

func (p player) lessonSubmit(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page := views.NewPage(lang, csrf, "lesson")
	partial := r.Header.Get("HX-Request") == "true"
	unitID, lessonIx, ok := parseLessonPath(r)
	if !ok {
		notFound(w, r)
		return
	}
	userID, ok := p.learnerID(r)
	if !ok {
		notFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		notFound(w, r)
		return
	}
	runID, err := strconv.ParseInt(r.PostForm.Get("run"), 10, 64)
	if err != nil || runID <= 0 {
		notFound(w, r)
		return
	}
	run, err := p.store.PlayerRunFor(r.Context(), userID, runID)
	if errors.Is(err, db.ErrNotFound) {
		notFound(w, r)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	if run.UnitID != unitID || run.LessonIx != lessonIx {
		notFound(w, r)
		return
	}
	lesson, err := p.store.CourseLesson(r.Context(), userID, unitID, lessonIx)
	if errors.Is(err, db.ErrNotFound) {
		notFound(w, r)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	if run.Finished {
		p.writeScreen(w, r, page, doneScreen(page, lesson, run.Score), partial)
		return
	}
	score, finished, err := p.store.FinishedLessonScore(r.Context(), userID, unitID, lessonIx)
	if err != nil {
		serverError(w)
		return
	}
	if finished {
		p.writeScreen(w, r, page, doneScreen(page, lesson, score), partial)
		return
	}

	total := len(lesson.Activities)
	answer := r.PostForm.Get("answer")
	if total == 0 {
		if answer == "continue" {
			run, _, err = p.store.ContinuePlayerRun(r.Context(), userID, run.ID, run.ActivityIx, 0)
			if err != nil {
				serverError(w)
				return
			}
		}
		screen, err := p.screenFor(r.Context(), page, lesson, run, lessonQueryFrom(r, page.Lang))
		if err != nil {
			serverError(w)
			return
		}
		p.writeScreen(w, r, page, screen, partial)
		return
	}
	if run.ActivityIx < 0 || run.ActivityIx >= total {
		p.writeScreen(w, r, page, doneScreen(page, lesson, float64(run.CorrectN)/float64(total)), partial)
		return
	}

	act := lesson.Activities[run.ActivityIx]
	target, err := p.targetOf(r.Context(), lesson, act)
	if err != nil {
		serverError(w)
		return
	}
	graded := gradedActivity(act.Type) && target.ID != 0
	if graded && recallOrCloze(act.Type) {
		n, countErr := p.askChoiceCount(r.Context(), userID, lesson, run, act, target)
		if countErr != nil {
			serverError(w)
			return
		}
		// Fewer than three options is a reveal, not a question. Continue advances.
		if n < lessonMinChoices {
			graded = false
		}
	}
	switch {
	case run.Phase == "ask" && graded && answer != "" && answer != "continue":
		// Grade the lexeme id on the button. Position and label are not the answer.
		picked, parseErr := strconv.ParseInt(answer, 10, 64)
		correct := parseErr == nil && picked == target.ID
		var applied bool
		run, applied, err = p.store.AnswerPlayerRun(r.Context(), userID, run.ID, run.ActivityIx, correct)
		// A repeated post is not applied, and it is not a second answer.
		if err == nil && applied {
			err = p.recordAnswer(r.Context(), userID, lesson, act, target, answer, correct)
		}
	case answer == "continue" && (run.Phase == "reveal" || (run.Phase == "ask" && !graded)):
		run, _, err = p.store.ContinuePlayerRun(r.Context(), userID, run.ID, run.ActivityIx, total)
	}
	if err != nil {
		serverError(w)
		return
	}
	screen, err := p.screenFor(r.Context(), page, lesson, run, lessonQueryFrom(r, page.Lang))
	if err != nil {
		serverError(w)
		return
	}
	p.writeScreen(w, r, page, screen, partial)
}

// recordAnswer writes one checked lesson answer through evidence.Recorder.
// userID is the signed-in learner. answer is the lexeme id the learner picked.
// The response stored is that word's pointed text, or the raw form value when
// no such word exists. The recorder caps its length.
func (p player) recordAnswer(ctx context.Context, userID int64, lesson db.CourseLesson, act units.Activity, target db.LexemeView, answer string, correct bool) error {
	picked, _ := strconv.ParseInt(answer, 10, 64)
	response := answer
	if picked > 0 {
		face, ok, err := p.store.LexemeByID(ctx, picked)
		if err != nil {
			return err
		}
		if ok {
			response = face.TextNFC
		}
	}
	_, err := evidence.New(p.store, nil, nil).Attempt(ctx, evidence.Attempt{
		UserID:         userID,
		LexemeID:       target.ID,
		Skill:          lessonSkill(act.Type),
		Activity:       act.Type,
		Source:         evidence.SourceLesson,
		UnitID:         lesson.UnitID,
		Prompt:         act.Text,
		Response:       response,
		Correct:        correct,
		ChosenLexemeID: picked,
		// A lesson's new words get their cards when the lesson ends (v2).
		// Scheduling from lesson answers is not part of this change.
		SkipSchedule: true,
	})
	return err
}

// lessonSkill is the skill a graded lesson activity exercises.
// recall and cloze show Hebrew and ask for meaning or the missing word (read).
// listen_pick plays a clip (listen). produce shows the meaning and asks for the Hebrew (recall).
func lessonSkill(typ string) string {
	switch typ {
	case units.ActivityListenPick:
		return fsrs.SkillListen
	case units.ActivityProduce:
		return fsrs.SkillRecall
	default:
		return fsrs.SkillRead
	}
}

func (p player) writePlaying(w http.ResponseWriter, r *http.Request, page views.Page, userID int64, lesson db.CourseLesson, partial bool) {
	run, err := p.store.StartPlayerRun(r.Context(), userID, lesson.UnitID, lesson.LessonIx)
	if err != nil {
		serverError(w)
		return
	}
	screen, err := p.screenFor(r.Context(), page, lesson, run, lessonQueryFrom(r, page.Lang))
	if err != nil {
		serverError(w)
		return
	}
	p.writeScreen(w, r, page, screen, partial)
}

func (p player) writeScreen(w http.ResponseWriter, r *http.Request, page views.Page, screen views.Screen, partial bool) {
	if partial {
		// The whole frame comes back, so the progress bar and the outline move with the step.
		render(w, r, views.LessonFrameView(page, screen))
		return
	}
	render(w, r, views.Lesson(page, screen))
}

func (p player) learnerID(r *http.Request) (int64, bool) {
	if p.store == nil {
		return 0, false
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		return 0, false
	}
	return user.ID, true
}

// screenFor is the screen for one run: the step, and the frame around it.
func (p player) screenFor(ctx context.Context, page views.Page, lesson db.CourseLesson, run db.PlayerRun, q lessonQuery) (views.Screen, error) {
	screen, err := p.stepScreen(ctx, page, lesson, run, q)
	if err != nil {
		return views.Screen{}, err
	}
	screen.Frame = frameFor(page, lesson, run, screen.Activity.Done)
	return screen, nil
}

// frameFor is the lesson frame: the bar, the step's name, and the outline. It holds
// labels and counts only. Activity types become learner names, so no internal name shows.
func frameFor(page views.Page, lesson db.CourseLesson, run db.PlayerRun, done bool) views.LessonFrame {
	total := len(lesson.Activities)
	f := views.LessonFrame{
		UnitID:       lesson.UnitID,
		Total:        max(total, 1),
		BackHref:     page.Href("/"),
		ReportHref:   page.Href(views.ReportPath(lesson.UnitID)),
		ReportAction: views.ReportPath(lesson.UnitID),
		CSRF:         page.CSRF,
	}
	for _, act := range lesson.Activities {
		f.Outline = append(f.Outline, views.OutlineStep{Label: views.LessonLabel(act.Type), State: "todo"})
	}
	if done || run.Finished || (total > 0 && run.ActivityIx >= total) {
		f.Done, f.Step, f.Value, f.Kind = true, f.Total, f.Total, "All done"
		for i := range f.Outline {
			f.Outline[i].State = "done"
		}
		return f
	}
	f.Step = min(max(run.ActivityIx, 0)+1, f.Total)
	f.Value = f.Step - 1
	if run.Phase == "reveal" {
		f.Value = f.Step
	}
	f.Kind = "Practice"
	if total > 0 {
		f.Kind = views.LessonLabel(lesson.Activities[run.ActivityIx].Type)
		for i := range f.Outline {
			switch {
			case i < run.ActivityIx:
				f.Outline[i].State = "done"
			case i == run.ActivityIx:
				f.Outline[i].State = "current"
			}
		}
	}
	return f
}

func (p player) stepScreen(ctx context.Context, page views.Page, lesson db.CourseLesson, run db.PlayerRun, q lessonQuery) (views.Screen, error) {
	gloss, err := titleGloss(ctx, p.store, lesson.Title)
	if err != nil {
		return views.Screen{}, err
	}
	screen := views.Screen{
		Title:     lesson.Title,
		Gloss:     gloss,
		Situation: lesson.Situation,
	}
	if run.Finished {
		screen.Activity = doneActivity(page, run.Score)
		return screen, nil
	}
	total := len(lesson.Activities)
	if total == 0 {
		screen.Activity = p.step(page, lesson, run, units.Activity{}, db.LexemeView{}, nil, nil, nil, true, q)
		return screen, nil
	}
	if run.ActivityIx < 0 || run.ActivityIx >= total {
		score := 0.0
		if total > 0 {
			score = float64(run.CorrectN) / float64(total)
		}
		screen.Activity = doneActivity(page, score)
		return screen, nil
	}
	lex, err := p.store.LexemesByText(ctx, activityTexts(lesson))
	if err != nil {
		return views.Screen{}, err
	}
	pool, err := p.store.UnitLexemes(ctx, lesson.UnitID)
	if err != nil {
		return views.Screen{}, err
	}
	act := lesson.Activities[run.ActivityIx]
	target, err := targetFrom(act, lex, lesson.NewWords)
	if err != nil {
		return views.Screen{}, err
	}
	if act.Type == units.ActivityDialogue {
		screen.Activity = p.step(page, lesson, run, act, target, lex, pool, nil, false, q)
		screen.Activity.Dialogue, err = p.dialogueView(ctx, act, lex, lesson.NewWords, q.meaningsAll)
		if err != nil {
			return views.Screen{}, err
		}
		return screen, nil
	}
	var earlier []db.LexemeView
	if run.Phase == "ask" && gradedActivity(act.Type) && target.ID != 0 {
		earlier, err = p.store.EarlierUnitLexemes(ctx, run.UserID, lesson.Position)
		if err != nil {
			return views.Screen{}, err
		}
	}
	clips, err := p.clips(ctx, act)
	if err != nil {
		return views.Screen{}, err
	}
	screen.Activity = p.step(page, lesson, run, act, target, lex, pool, earlier, false, q)
	phase := run.Phase
	// A recall or cloze that cannot find three options is not a graded ask.
	// Show the word the way a reveal does, and let Continue move on.
	if recallOrCloze(act.Type) && run.Phase == "ask" && target.ID != 0 && !screen.Activity.Asking {
		phase = "reveal"
	}
	screen.Activity.Audio = phraseAudio(act, phase, clips.phrase)
	screen.Activity.Words = wordsFor(act, lex, lesson.NewWords, phase, clips, target, q.meaningsAll)
	if screen.Activity.Audio != "" {
		screen.Activity.AudioLabel, screen.Activity.AudioHebrew = phrasePlayLabel(act, phase)
	}
	return screen, nil
}

func (p player) step(page views.Page, lesson db.CourseLesson, run db.PlayerRun, act units.Activity, target db.LexemeView, lex map[string]db.LexemeView, pool, earlier []db.LexemeView, empty bool, q lessonQuery) views.Activity {
	total := len(lesson.Activities)
	post := lessonPath(lesson.UnitID, lesson.LessonIx)
	feedback, class := feedbackFor(run)
	prompt := activityPrompt(lesson, act, target, lex, empty)
	var promptWord string
	if !empty && act.Type == units.ActivityRecall && lexemeTokenCount(act, lex) > 1 && target.Text != "" {
		// The question is HTML (a bdi around the word), so the plain prompt
		// stays empty and the template writes the sentence.
		promptWord = target.Text
		prompt = ""
	}
	activity := views.Activity{
		Prompt:        prompt,
		PromptWord:    promptWord,
		Progress:      progressText(run.ActivityIx, total),
		Feedback:      feedback,
		FeedbackClass: class,
		CSRF:          page.CSRF,
		Run:           strconv.FormatInt(run.ID, 10),
		Post:          post,
		Continue:      continueLabel(run.ActivityIx, total),
		HomeHref:      page.Href("/"),
		MeaningsOpen:  q.meaningsAll,
		MeaningsHref:  q.meaningsHref,
	}
	if empty || run.Phase != "ask" || !gradedActivity(act.Type) || target.ID == 0 {
		return activity
	}
	activity.Choices = askChoices(act.Type, pool, earlier, target, choiceSeed(run, target), lineMateExclude(act, lex, target))
	activity.Asking = len(activity.Choices) > 0
	if !activity.Asking {
		activity.Continue = continueLabel(run.ActivityIx, total)
	}
	return activity
}

type clipSet struct {
	phrase string
	words  map[string]string
}

func (p player) clips(ctx context.Context, act units.Activity) (clipSet, error) {
	out := clipSet{words: map[string]string{}}
	if p.store == nil {
		return out, nil
	}
	phrase, err := p.audioSrc(ctx, units.Normalize(act.Text))
	if err != nil {
		return clipSet{}, err
	}
	out.phrase = phrase
	tokens := units.Tokenize(act.Text)
	hebrewN := 0
	for _, tok := range tokens {
		if hebrewToken(tok) {
			hebrewN++
		}
	}
	// One Hebrew word that already has a phrase clip does not get a second control.
	if hebrewN == 1 && phrase != "" {
		return out, nil
	}
	for _, tok := range tokens {
		if !hebrewToken(tok) {
			continue
		}
		src, err := p.audioSrc(ctx, units.Normalize(tok))
		if err != nil {
			return clipSet{}, err
		}
		if src != "" {
			out.words[tok] = src
		}
	}
	return out, nil
}

func (p player) audioSrc(ctx context.Context, text string) (string, error) {
	if text == "" || p.store == nil {
		return "", nil
	}
	asset, ok, err := p.store.AudioByText(ctx, text)
	if err != nil || !ok || asset.ID == "" {
		return "", err
	}
	return "/audio/" + asset.ID + ".mp3", nil
}

// actTokens is every word token of an activity: its text, or the lines of a dialogue.
func actTokens(act units.Activity) []string {
	var out []string
	for _, text := range act.Texts() {
		out = append(out, units.Tokenize(text)...)
	}
	return out
}

// dialogueView is the lines of a dialogue, each with its speaker's fixed name,
// its words, and the clip in its speaker's voice. A line whose speaker is not a
// or b, whose voice is not configured, or that has no clip in that voice has no
// audio. It is never played in the other voice: the lookup is by voice id.
func (p player) dialogueView(ctx context.Context, act units.Activity, lex map[string]db.LexemeView, newWords []db.LexemeView, open bool) (*views.Dialogue, error) {
	d := &views.Dialogue{}
	for i, line := range act.Lines {
		view := views.DialogueLine{
			Index:   i,
			Speaker: line.Speaker,
			Name:    views.SpeakerName(line.Speaker),
		}
		if p.store != nil {
			if voice, err := p.voices.ForSpeaker(line.Speaker); err == nil {
				asset, ok, err := p.store.AudioByTextVoice(ctx, units.Normalize(line.Text), voice)
				if err != nil {
					return nil, err
				}
				if ok && asset.ID != "" {
					view.Audio = "/audio/" + asset.ID + ".mp3"
				}
			}
		}
		view.Words = wordsForText(line.Text, units.ActivityDialogue, lex, newWords, "reveal", clipSet{}, false, "", open)
		d.Lines = append(d.Lines, view)
	}
	return d, nil
}

func (p player) targetOf(ctx context.Context, lesson db.CourseLesson, act units.Activity) (db.LexemeView, error) {
	lex, err := p.store.LexemesByText(ctx, actTokens(act))
	if err != nil {
		return db.LexemeView{}, err
	}
	return targetFrom(act, lex, lesson.NewWords)
}

func wordsFor(act units.Activity, lex map[string]db.LexemeView, newWords []db.LexemeView, phase string, clips clipSet, target db.LexemeView, open bool) []views.Word {
	words := wordsForText(act.Text, act.Type, lex, newWords, phase, clips, act.Type == units.ActivityCloze && phase == "ask", act.Target, open)
	if act.Type == units.ActivityRecall {
		markRecallTarget(words, lexemeTokenCount(act, lex), target.Text, phase)
	}
	if recallOrCloze(act.Type) {
		dropBorrowedGloss(words, target.Text, target.Gloss)
	}
	return words
}

// wordsForText is wordsFor for one string of Hebrew: an activity's text, or one
// line of a dialogue. blank gaps the target token when the activity sets one,
// and otherwise the first new word, for a cloze in its ask phase.
func wordsForText(text, typ string, lex map[string]db.LexemeView, newWords []db.LexemeView, phase string, clips clipSet, blank bool, target string, open bool) []views.Word {
	tokens := units.Tokenize(text)
	if len(tokens) == 0 {
		return nil
	}
	show := visibleFields(typ, phase)
	flip := !gradedActivity(typ)
	fresh := map[string]bool{}
	for _, word := range newWords {
		fresh[word.Text] = true
	}
	gap := ""
	if blank {
		gap = gapToken(tokens, lex, newWords, target)
	}
	hebrewN := 0
	for _, tok := range tokens {
		if hebrewToken(tok) {
			hebrewN++
		}
	}
	skipWordAudio := hebrewN == 1 && clips.phrase != ""
	var words []views.Word
	for _, tok := range tokens {
		if !hebrewToken(tok) {
			words = append(words, views.Word{Text: tok, Plain: true, Flip: flip})
			continue
		}
		if tok == gap {
			words = append(words, views.Word{Gap: true})
			continue
		}
		// A token missing from the lexicon keeps an empty gloss. The
		// activity and the target are not a fallback for that field.
		view := lex[tok]
		word := views.Word{
			Text:         tok,
			Translit:     view.Translit,
			Gloss:        view.Gloss,
			Gender:       view.Gender,
			POS:          view.POS,
			ShowText:     show.text,
			ShowTranslit: show.translit,
			ShowGloss:    show.gloss,
			Flip:         flip,
			New:          fresh[tok],
			Open:         open && flip,
		}
		if show.text && !skipWordAudio {
			word.Audio = clips.words[tok]
		}
		words = append(words, word)
	}
	return words
}

// phraseAudio hides the sentence clip until the answer for produce and cloze.
// A cloze clip speaks the gapped word. It is split out so screenFor can set
// Activity.Audio after wordsFor.
func phraseAudio(act units.Activity, phase, src string) string {
	if phase == "ask" && (act.Type == units.ActivityProduce || act.Type == units.ActivityCloze) {
		return ""
	}
	return src
}

type fieldVis struct {
	text, translit, gloss bool
}

func visibleFields(typ, phase string) fieldVis {
	revealed := phase != "ask"
	switch typ {
	case units.ActivityRecall:
		// A single word hides its sound and meaning until the answer.
		// A line of several words is adjusted afterwards: the tested word
		// stays hidden, and the other words are shown as help.
		return fieldVis{text: true, translit: revealed, gloss: revealed}
	case units.ActivityListenPick, units.ActivityProduce:
		return fieldVis{text: revealed, translit: revealed, gloss: revealed}
	case units.ActivityCloze:
		// Before the answer the other words show pointed Hebrew and transliteration.
		// After the answer every word, including the gap, shows transliteration and gloss.
		return fieldVis{text: true, translit: true, gloss: revealed}
	default:
		return fieldVis{text: true, translit: true, gloss: true}
	}
}

func gradedActivity(typ string) bool {
	switch typ {
	case units.ActivityRecall, units.ActivityListenPick, units.ActivityProduce, units.ActivityCloze:
		return true
	default:
		return false
	}
}

func targetFrom(act units.Activity, lex map[string]db.LexemeView, newWords []db.LexemeView) (db.LexemeView, error) {
	if tok, ok := tokenForTarget(actTokens(act), act.Target); ok {
		if view, found := lex[tok]; found {
			return view, nil
		}
	}
	fresh := map[string]bool{}
	for _, word := range newWords {
		fresh[word.Text] = true
	}
	var first db.LexemeView
	for _, tok := range actTokens(act) {
		view, ok := lex[tok]
		if !ok {
			continue
		}
		if first.ID == 0 {
			first = view
		}
		if fresh[tok] {
			return view, nil
		}
	}
	return first, nil
}

func gapToken(tokens []string, lex map[string]db.LexemeView, newWords []db.LexemeView, target string) string {
	if tok, ok := tokenForTarget(tokens, target); ok {
		if _, found := lex[tok]; found {
			return tok
		}
	}
	fresh := map[string]bool{}
	for _, word := range newWords {
		fresh[word.Text] = true
	}
	var first string
	for _, tok := range tokens {
		if _, ok := lex[tok]; !ok {
			continue
		}
		if first == "" {
			first = tok
		}
		if fresh[tok] {
			return tok
		}
	}
	return first
}

// lessonChoiceButtons is the target plus two distractors. listen_pick and
// produce still use it. lessonWideChoices is four buttons for recall and cloze.
const (
	lessonChoiceButtons = 3
	lessonWideChoices   = 4
	// lessonMinChoices is the smallest graded recall or cloze. Four is the
	// target. Fewer than three is a reveal with Continue.
	lessonMinChoices = 3
)

func recallOrCloze(typ string) bool {
	return typ == units.ActivityRecall || typ == units.ActivityCloze
}

// askChoiceCount is how many buttons this ask would render. The post path
// uses it so a recall or cloze that cannot reach three options is not graded.
func (p player) askChoiceCount(ctx context.Context, userID int64, lesson db.CourseLesson, run db.PlayerRun, act units.Activity, target db.LexemeView) (int, error) {
	lex, err := p.store.LexemesByText(ctx, actTokens(act))
	if err != nil {
		return 0, err
	}
	pool, err := p.store.UnitLexemes(ctx, lesson.UnitID)
	if err != nil {
		return 0, err
	}
	earlier, err := p.store.EarlierUnitLexemes(ctx, userID, lesson.Position)
	if err != nil {
		return 0, err
	}
	return len(askChoices(act.Type, pool, earlier, target, choiceSeed(run, target), lineMateExclude(act, lex, target))), nil
}

func choiceSeed(run db.PlayerRun, target db.LexemeView) choices.ChoiceSeed {
	return choices.ChoiceSeed{
		RunID:    run.ID,
		Step:     run.ActivityIx,
		Attempt:  0, // lesson_runs has no attempt counter
		TargetID: target.ID,
	}
}

// askChoices is the buttons for one graded activity. listen_pick and produce
// stay at three. recall and cloze aim for four, and never use a word that is
// already on the line:
//
//  1. same part of speech, outside the line, this unit first
//  2. same part of speech, outside the line, from earlier units
//  3. any other part of speech, outside the line (unit, then earlier)
//
// A distractor whose gloss equals the answer is never used, and two buttons
// never share a gloss. Fewer than three buttons is not an ask: the caller
// shows a reveal instead.
func askChoices(typ string, unit, earlier []db.LexemeView, target db.LexemeView, seed choices.ChoiceSeed, block choiceExclude) []views.Choice {
	hebrew := typ == units.ActivityProduce || typ == units.ActivityCloze
	if !recallOrCloze(typ) {
		return lessonChoices(unit, earlier, target, seed, hebrew, false, lessonChoiceButtons, block)
	}
	return wideChoices(unit, earlier, target, seed, hebrew, typ == units.ActivityCloze, block)
}

// wideChoices fills a recall or cloze ask from outside the line only. Same
// part of speech comes first (this unit, then earlier units). Other parts of
// speech from outside the line are next. Words on the line are help, or are
// the rest of a cloze sentence, so they are never buttons. The answer's gloss
// is never repeated, and neither is any other gloss. The result is nil when
// three buttons cannot be filled.
func wideChoices(unit, earlier []db.LexemeView, target db.LexemeView, seed choices.ChoiceSeed, hebrew, translit bool, block choiceExclude) []views.Choice {
	if target.ID == 0 {
		return nil
	}
	want := lessonWideChoices - 1
	seen := map[int64]bool{target.ID: true}
	usedGloss := map[string]bool{}
	if key := glossKey(target.Gloss); key != "" {
		usedGloss[key] = true
	}
	var distractors []db.LexemeView
	tiers := [][]db.LexemeView{
		outsideTier(unit, target, hebrew, block, true),
		outsideTier(earlier, target, hebrew, block, true),
		outsideTier(unit, target, hebrew, block, false),
		outsideTier(earlier, target, hebrew, block, false),
	}
	for _, tier := range tiers {
		if len(distractors) >= want {
			break
		}
		fresh := make([]db.LexemeView, 0, len(tier))
		for _, lex := range tier {
			if seen[lex.ID] {
				continue
			}
			if key := glossKey(lex.Gloss); key != "" && usedGloss[key] {
				continue
			}
			fresh = append(fresh, lex)
		}
		for _, lex := range takeDistractors(seed, target, fresh, want-len(distractors)) {
			if seen[lex.ID] {
				continue
			}
			key := glossKey(lex.Gloss)
			if key != "" && usedGloss[key] {
				continue
			}
			seen[lex.ID] = true
			if key != "" {
				usedGloss[key] = true
			}
			distractors = append(distractors, lex)
		}
	}
	if len(distractors) < lessonMinChoices-1 {
		return nil
	}
	return lessonViews(shuffleWithTarget(seed, target, distractors), hebrew, translit)
}

// outsideTier is one fallback tier: this list, either the target's part of
// speech or every other part. Line-mates and the answer's gloss are left out.
// A target with no part of speech treats every remaining word as the same.
func outsideTier(rows []db.LexemeView, target db.LexemeView, hebrew bool, block choiceExclude, same bool) []db.LexemeView {
	out := make([]db.LexemeView, 0, len(rows))
	for _, lex := range rows {
		if !choiceUsable(lex, target, hebrew) || choiceBlocked(lex, target, block) {
			continue
		}
		if samePOSAs(lex, target) != same {
			continue
		}
		out = append(out, lex)
	}
	return out
}

func samePOSAs(lex, target db.LexemeView) bool {
	return target.POS == "" || lex.POS == target.POS
}

// choiceUsable rejects a missing row, the target, a second button with the
// answer's gloss, and an English button with no gloss.
func choiceUsable(lex, target db.LexemeView, hebrew bool) bool {
	if lex.ID == 0 || lex.ID == target.ID {
		return false
	}
	if key := glossKey(lex.Gloss); key != "" && key == glossKey(target.Gloss) {
		return false
	}
	if !hebrew && strings.TrimSpace(lex.Gloss) == "" {
		return false
	}
	return true
}

// takeDistractors asks choices.Pick for n rows from tier. Pick counts the
// target as one of n, so the call asks for n+1 and then drops the target.
// The seed keeps the selection stable across reloads.
func takeDistractors(seed choices.ChoiceSeed, target db.LexemeView, tier []db.LexemeView, n int) []db.LexemeView {
	if n <= 0 || len(tier) == 0 {
		return nil
	}
	picked := viewsInPickOrder(seed, target, tier, n+1)
	out := make([]db.LexemeView, 0, n)
	for _, lex := range picked {
		if lex.ID == target.ID {
			continue
		}
		out = append(out, lex)
	}
	return out
}

// shuffleWithTarget is the button order: the target and the chosen
// distractors, shuffled by the same seed Pick uses.
func shuffleWithTarget(seed choices.ChoiceSeed, target db.LexemeView, distractors []db.LexemeView) []db.LexemeView {
	return viewsInPickOrder(seed, target, distractors, len(distractors)+1)
}

func viewsInPickOrder(seed choices.ChoiceSeed, target db.LexemeView, rows []db.LexemeView, n int) []db.LexemeView {
	pool := make([]choices.Lexeme, len(rows))
	byID := make(map[int64]db.LexemeView, len(rows)+1)
	byID[target.ID] = target
	for i, lex := range rows {
		pool[i] = choiceLexeme(lex, false)
		byID[lex.ID] = lex
	}
	picked := choices.Pick(seed, choiceLexeme(target, false), pool, n)
	out := make([]db.LexemeView, 0, len(picked))
	for _, lex := range picked {
		view, ok := byID[lex.ID]
		if !ok {
			view = db.LexemeView{ID: lex.ID, Text: lex.Text, POS: lex.POS}
		}
		out = append(out, view)
	}
	return out
}

func lessonViews(rows []db.LexemeView, hebrew, translit bool) []views.Choice {
	out := make([]views.Choice, 0, len(rows))
	for _, view := range rows {
		choice := views.Choice{Value: strconv.FormatInt(view.ID, 10)}
		if hebrew {
			choice.Hebrew = true
			choice.Text = view.Text
			if translit {
				choice.Translit = view.Translit
			}
		} else {
			choice.Label = view.Gloss
		}
		out = append(out, choice)
	}
	return out
}

// tokenForTarget matches target to one of tokens by units.Normalize, the same
// key a token uses when it is looked up as a lexeme. The result is the token
// from the activity, so a differently composed target still gaps that token.
func tokenForTarget(tokens []string, target string) (string, bool) {
	want := units.Normalize(target)
	if want == "" {
		return "", false
	}
	for _, tok := range tokens {
		if units.Normalize(tok) == want {
			return tok, true
		}
	}
	return "", false
}

// lessonChoices builds the buttons for a graded ask. recall, listen_pick,
// produce, and cloze all use it. A checkpoint lesson is a sequence of those
// activities, so it has no choice builder of its own. Order and membership
// come from choices.Pick. n is the button count, target included. The button
// value stays the lexeme id. block drops line-mates the learner can already see.
func lessonChoices(unit, earlier []db.LexemeView, target db.LexemeView, seed choices.ChoiceSeed, hebrew, translit bool, n int, block choiceExclude) []views.Choice {
	if target.ID == 0 {
		return nil
	}
	if n < 1 {
		n = lessonChoiceButtons
	}
	picked := choices.Pick(seed, choiceLexeme(target, false), choicePool(unit, earlier, target, hebrew, block), n)
	byID := make(map[int64]db.LexemeView, len(unit)+len(earlier)+1)
	byID[target.ID] = target
	for _, lex := range unit {
		byID[lex.ID] = lex
	}
	for _, lex := range earlier {
		if _, ok := byID[lex.ID]; !ok {
			byID[lex.ID] = lex
		}
	}
	out := make([]views.Choice, 0, len(picked))
	for _, lex := range picked {
		view, ok := byID[lex.ID]
		if !ok {
			view = db.LexemeView{ID: lex.ID, Text: lex.Text, POS: lex.POS}
		}
		choice := views.Choice{Value: strconv.FormatInt(view.ID, 10)}
		if hebrew {
			choice.Hebrew = true
			choice.Text = view.Text
			if translit {
				choice.Translit = view.Translit
			}
		} else {
			choice.Label = view.Gloss
		}
		out = append(out, choice)
	}
	return out
}

// choiceExclude is the set of distractors a learner could dismiss because the
// line already shows them. ids are the other lexemes in the line. glosses are
// those words' meanings, folded, so a different spelling with the same gloss
// is excluded too. Matching the token text alone does not catch that.
type choiceExclude struct {
	ids     map[int64]bool
	glosses map[string]bool
}

// choicePool is the unit, then earlier units. An English button needs a gloss,
// so a blank gloss is left out of that pool. The target is kept either way.
// A blocked line-mate is left out of both tiers. What remains is the fallback:
// other words from this unit, then earlier lexicon words. The line's own
// words are not put back.
func choicePool(unit, earlier []db.LexemeView, target db.LexemeView, hebrew bool, block choiceExclude) []choices.Lexeme {
	pool := make([]choices.Lexeme, 0, len(unit)+len(earlier))
	for _, lex := range unit {
		if choiceBlocked(lex, target, block) {
			continue
		}
		if !hebrew && lex.ID != target.ID && strings.TrimSpace(lex.Gloss) == "" {
			continue
		}
		pool = append(pool, choiceLexeme(lex, false))
	}
	for _, lex := range earlier {
		if choiceBlocked(lex, target, block) {
			continue
		}
		if !hebrew && strings.TrimSpace(lex.Gloss) == "" {
			continue
		}
		pool = append(pool, choiceLexeme(lex, true))
	}
	return pool
}

func choiceBlocked(lex, target db.LexemeView, block choiceExclude) bool {
	if lex.ID == 0 || lex.ID == target.ID {
		return false
	}
	if block.ids[lex.ID] {
		return true
	}
	key := glossKey(lex.Gloss)
	return key != "" && block.glosses[key]
}

// lineMateExclude builds the block for an ask that shows the line's other
// words as help. A multi-word recall shows their gloss. A cloze shows the
// other words, and its choices are words, so those lexemes and any word with
// their gloss would give the gap away. A single-word recall, listen-and-pick,
// and produce hide that help, so they do not block.
func lineMateExclude(act units.Activity, lex map[string]db.LexemeView, target db.LexemeView) choiceExclude {
	if !showsLineHelp(act, lex) {
		return choiceExclude{}
	}
	block := choiceExclude{ids: map[int64]bool{}, glosses: map[string]bool{}}
	for _, tok := range actTokens(act) {
		view, ok := lex[tok]
		if !ok || view.ID == 0 || view.ID == target.ID || block.ids[view.ID] {
			continue
		}
		block.ids[view.ID] = true
		if key := glossKey(view.Gloss); key != "" {
			block.glosses[key] = true
		}
	}
	return block
}

func showsLineHelp(act units.Activity, lex map[string]db.LexemeView) bool {
	switch act.Type {
	case units.ActivityRecall:
		return lexemeTokenCount(act, lex) > 1
	case units.ActivityCloze:
		return true
	default:
		return false
	}
}

func glossKey(gloss string) string {
	return strings.ToLower(strings.TrimSpace(gloss))
}

func choiceLexeme(view db.LexemeView, earlier bool) choices.Lexeme {
	return choices.Lexeme{ID: view.ID, Text: view.Text, POS: view.POS, Earlier: earlier}
}

// lexemeTokenCount is how many tokens of the activity are lexicon entries.
// Punctuation and unknown words are not counted.
func lexemeTokenCount(act units.Activity, lex map[string]db.LexemeView) int {
	n := 0
	for _, tok := range actTokens(act) {
		if _, ok := lex[tok]; ok {
			n++
		}
	}
	return n
}

// dropBorrowedGloss stops a helper token from showing the answer.
// The tested word keeps its own gloss (hidden until the reveal). Every
// other token keeps only the gloss stored on its own lexeme. The same
// gloss as the answer is cleared, and an empty gloss stays empty.
func dropBorrowedGloss(words []views.Word, targetText, targetGloss string) {
	key := glossKey(targetGloss)
	if key == "" {
		return
	}
	for i := range words {
		if words[i].Plain || words[i].Gap || words[i].Target || words[i].Text == targetText {
			continue
		}
		if glossKey(words[i].Gloss) == key {
			words[i].Gloss = ""
		}
	}
}

// markRecallTarget applies the multi-word recall rule. The tested word keeps
// its transliteration and gloss hidden until the answer, and is marked in the
// line. Every other word is help, so both show during the ask. After the
// answer, visibleFields has already turned both on for every word. A recall
// of one lexeme is left exactly as visibleFields set it.
func markRecallTarget(words []views.Word, n int, target, phase string) {
	if n < 2 || target == "" {
		return
	}
	ask := phase == "ask"
	for i := range words {
		if words[i].Plain || words[i].Gap {
			continue
		}
		if words[i].Text == target {
			words[i].Target = true
			if ask {
				words[i].ShowTranslit = false
				words[i].ShowGloss = false
			}
			continue
		}
		if ask {
			words[i].ShowTranslit = true
			words[i].ShowGloss = true
		}
	}
}

// phrasePlayLabel is the accessible name of the line clip, and the one Hebrew
// word in that name when the line is a single word. While the spelling is
// hidden, the name is "Play" and does not speak the Hebrew.
func phrasePlayLabel(act units.Activity, phase string) (label, hebrew string) {
	if !visibleFields(act.Type, phase).text {
		return "Play", ""
	}
	var he []string
	for _, tok := range units.Tokenize(act.Text) {
		if hebrewToken(tok) {
			he = append(he, tok)
		}
	}
	if len(he) == 1 {
		return "Play " + he[0], he[0]
	}
	return "Play the line", ""
}

func activityPrompt(lesson db.CourseLesson, act units.Activity, target db.LexemeView, lex map[string]db.LexemeView, empty bool) string {
	if empty {
		return "This lesson has nothing to practise."
	}
	switch act.Type {
	case units.ActivityStory:
		return "Read the story. Each word shows its sound and its meaning."
	case units.ActivityPattern:
		if strings.TrimSpace(lesson.Concept) != "" {
			return lesson.Concept
		}
		return "Read the example."
	case units.ActivityShadow:
		return "Listen, then say the line."
	case units.ActivityDialogue:
		return "Listen to the conversation. Tap a line to hear it again."
	case units.ActivityRecall:
		// A line of several words is still a question about one word. step
		// names that word in the template. A whole-line meaning question is
		// a different activity. A single word keeps this sentence.
		return "What does this Hebrew mean?"
	case units.ActivityListenPick:
		return "Listen, then pick the meaning."
	case units.ActivityProduce:
		if target.Gloss != "" {
			return "Pick the Hebrew for “" + target.Gloss + "”."
		}
		return "Pick the Hebrew."
	case units.ActivityCloze:
		return "Pick the missing word."
	default:
		return "Read this."
	}
}

func feedbackFor(run db.PlayerRun) (string, string) {
	if run.Finished || run.Phase != "reveal" {
		return "", ""
	}
	if run.Pending {
		return "That's right.", "feedback feedback--right"
	}
	return "Not quite. Here it is.", "feedback feedback--wrong"
}

func progressText(ix, total int) string {
	if total <= 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", ix+1, total)
}

func continueLabel(ix, total int) string {
	if total <= 0 || ix+1 >= total {
		return "Finish"
	}
	return "Continue"
}

func doneScreen(page views.Page, lesson db.CourseLesson, score float64) views.Screen {
	return views.Screen{
		Title:     lesson.Title,
		Situation: lesson.Situation,
		Activity:  doneActivity(page, score),
		Frame:     frameFor(page, lesson, db.PlayerRun{Finished: true}, true),
	}
}

func doneActivity(page views.Page, score float64) views.Activity {
	return views.Activity{
		Done:     true,
		Score:    scoreText(score),
		HomeHref: page.Href("/"),
	}
}

func scoreText(score float64) string {
	return fmt.Sprintf("Score %.0f%%", score*100)
}

func activityTexts(lesson db.CourseLesson) []string {
	var texts []string
	texts = append(texts, units.Tokenize(lesson.Title)...)
	for _, act := range lesson.Activities {
		texts = append(texts, actTokens(act)...)
	}
	return texts
}

func hebrewToken(tok string) bool {
	for _, r := range tok {
		if r >= 0x05D0 && r <= 0x05EA {
			return true
		}
	}
	return false
}

func parseLessonPath(r *http.Request) (int64, int, bool) {
	unitID, err := strconv.ParseInt(r.PathValue("unitID"), 10, 64)
	if err != nil || unitID <= 0 {
		return 0, 0, false
	}
	ix, err := strconv.Atoi(r.PathValue("lessonIx"))
	if err != nil || ix < 0 {
		return 0, 0, false
	}
	return unitID, ix, true
}

func lessonPath(unitID int64, lessonIx int) string {
	return fmt.Sprintf("/lesson/%d/%d", unitID, lessonIx)
}

// lessonQuery is the teach-step flip state read from the request.
// meaningsAll opens every card. meaningsHref is the other state of this
// step: closed pages link to meanings=all, and an open page drops that
// parameter so the no-JS control can close them. Other query parameters
// stay. A Hebrew shell whose request omitted lang gets lang=he, so the
// reload stays Hebrew.
type lessonQuery struct {
	meaningsAll  bool
	meaningsHref string
}

func lessonQueryFrom(r *http.Request, lang string) lessonQuery {
	q := r.URL.Query()
	all := q.Get("meanings") == "all"
	if q.Get("lang") == "" && lang == "he" {
		q.Set("lang", "he")
	}
	if all {
		q.Del("meanings")
	} else {
		q.Set("meanings", "all")
	}
	href := r.URL.Path
	if enc := q.Encode(); enc != "" {
		href += "?" + enc
	}
	return lessonQuery{meaningsAll: all, meaningsHref: href}
}

func chipsFor(words []db.LexemeView) []views.Chip {
	out := make([]views.Chip, 0, len(words))
	for _, word := range words {
		chip := views.Chip{Text: word.Text, Gloss: parenGloss(word.Translit, word.Gloss)}
		if views.GenderLabel(word.Gender) != "" {
			chip.Gender = word.Gender
		}
		out = append(out, chip)
	}
	return out
}

func titleGloss(ctx context.Context, store *db.Store, title string) (string, error) {
	tokens := units.Tokenize(title)
	if store == nil || len(tokens) == 0 {
		return "", nil
	}
	lex, err := store.LexemesByText(ctx, tokens)
	if err != nil {
		return "", err
	}
	var translits, glosses []string
	for _, tok := range tokens {
		view, ok := lex[tok]
		if !ok {
			return "", nil
		}
		if view.Translit != "" {
			translits = append(translits, view.Translit)
		}
		if view.Gloss != "" {
			glosses = append(glosses, view.Gloss)
		}
	}
	return parenGloss(strings.Join(translits, " "), strings.Join(glosses, " ")), nil
}

func parenGloss(translit, gloss string) string {
	translit = strings.TrimSpace(translit)
	gloss = strings.TrimSpace(gloss)
	switch {
	case translit != "" && gloss != "":
		return "(" + translit + ", " + gloss + ")"
	case gloss != "":
		return "(" + gloss + ")"
	case translit != "":
		return "(" + translit + ")"
	default:
		return ""
	}
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.NotFound(w, r)
}

func serverError(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

var audioFile = regexp.MustCompile(`^[0-9a-f]{64}\.mp3$`)

func serveAudio(dir string, w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if dir == "" || !audioFile.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, name, stat.ModTime(), f)
}
