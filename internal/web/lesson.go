package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// player serves one activity at a time. Progress lives on lesson_runs.
// The stored document only has an activity type and a Hebrew text, so the
// screen for each type is derived here:
//
//   - story, pattern, and shadow show every Hebrew word with its pointed
//     text, transliteration, and gloss. Continue counts the step correct.
//   - recall shows the pointed Hebrew and hides the transliteration and
//     gloss until the answer. Choices are English glosses.
//   - listen_pick hides the Hebrew, the transliteration, and the gloss
//     until the answer. The clip is the prompt. Choices are glosses.
//   - produce hides the Hebrew and the transliteration until the answer,
//     and hides the clip until then too. Choices show pointed Hebrew and
//     transliteration.
//   - cloze gaps the first new word. The other words stay visible.
//     Choices show pointed Hebrew and transliteration.
//   - any other type shows the words and a continue button.
//
// A step with no lexicon target is exposure, not a graded choice.
type player struct {
	store    *db.Store
	audioDir string
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
		screen, err := p.screenFor(r.Context(), page, lesson, run)
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
	switch {
	case run.Phase == "ask" && graded && answer != "" && answer != "continue":
		correct := units.Normalize(answer) == target.Text
		run, _, err = p.store.AnswerPlayerRun(r.Context(), userID, run.ID, run.ActivityIx, correct)
	case answer == "continue" && (run.Phase == "reveal" || (run.Phase == "ask" && !graded)):
		run, _, err = p.store.ContinuePlayerRun(r.Context(), userID, run.ID, run.ActivityIx, total)
	}
	if err != nil {
		serverError(w)
		return
	}
	screen, err := p.screenFor(r.Context(), page, lesson, run)
	if err != nil {
		serverError(w)
		return
	}
	p.writeScreen(w, r, page, screen, partial)
}

func (p player) writePlaying(w http.ResponseWriter, r *http.Request, page views.Page, userID int64, lesson db.CourseLesson, partial bool) {
	run, err := p.store.StartPlayerRun(r.Context(), userID, lesson.UnitID, lesson.LessonIx)
	if err != nil {
		serverError(w)
		return
	}
	screen, err := p.screenFor(r.Context(), page, lesson, run)
	if err != nil {
		serverError(w)
		return
	}
	p.writeScreen(w, r, page, screen, partial)
}

func (p player) writeScreen(w http.ResponseWriter, r *http.Request, page views.Page, screen views.Screen, partial bool) {
	if partial {
		render(w, r, views.ActivityBlock(screen.Activity))
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

func (p player) screenFor(ctx context.Context, page views.Page, lesson db.CourseLesson, run db.PlayerRun) (views.Screen, error) {
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
		screen.Activity = p.step(page, lesson, run, units.Activity{}, db.LexemeView{}, nil, nil, true)
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
	clips, err := p.clips(ctx, act)
	if err != nil {
		return views.Screen{}, err
	}
	screen.Activity = p.step(page, lesson, run, act, target, lex, pool, false)
	screen.Activity.Audio = phraseAudio(act, run.Phase, clips.phrase)
	screen.Activity.Words = wordsFor(act, lex, lesson.NewWords, run.Phase, clips)
	return screen, nil
}

func (p player) step(page views.Page, lesson db.CourseLesson, run db.PlayerRun, act units.Activity, target db.LexemeView, lex map[string]db.LexemeView, pool []db.LexemeView, empty bool) views.Activity {
	total := len(lesson.Activities)
	post := lessonPath(lesson.UnitID, lesson.LessonIx)
	feedback, class := feedbackFor(run)
	activity := views.Activity{
		Prompt:        activityPrompt(lesson, act, target, empty),
		Progress:      progressText(run.ActivityIx, total),
		Feedback:      feedback,
		FeedbackClass: class,
		CSRF:          page.CSRF,
		Run:           strconv.FormatInt(run.ID, 10),
		Post:          post,
		Continue:      continueLabel(run.ActivityIx, total),
		HomeHref:      page.Href("/"),
	}
	if empty || run.Phase != "ask" || !gradedActivity(act.Type) || target.ID == 0 {
		return activity
	}
	hebrew := act.Type == units.ActivityProduce || act.Type == units.ActivityCloze
	activity.Choices = choicesFor(pool, target, hebrew)
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

func (p player) targetOf(ctx context.Context, lesson db.CourseLesson, act units.Activity) (db.LexemeView, error) {
	lex, err := p.store.LexemesByText(ctx, units.Tokenize(act.Text))
	if err != nil {
		return db.LexemeView{}, err
	}
	return targetFrom(act, lex, lesson.NewWords)
}

func wordsFor(act units.Activity, lex map[string]db.LexemeView, newWords []db.LexemeView, phase string, clips clipSet) []views.Word {
	tokens := units.Tokenize(act.Text)
	if len(tokens) == 0 {
		return nil
	}
	show := visibleFields(act.Type, phase)
	gap := ""
	if act.Type == units.ActivityCloze && phase == "ask" {
		gap = gapToken(tokens, lex, newWords)
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
			words = append(words, views.Word{Text: tok, Plain: true})
			continue
		}
		if tok == gap {
			words = append(words, views.Word{Gap: true})
			continue
		}
		view := lex[tok]
		word := views.Word{
			Text:         tok,
			Translit:     view.Translit,
			Gloss:        view.Gloss,
			Gender:       view.Gender,
			ShowText:     show.text,
			ShowTranslit: show.translit,
			ShowGloss:    show.gloss,
		}
		if show.text && !skipWordAudio {
			word.Audio = clips.words[tok]
		}
		words = append(words, word)
	}
	return words
}

// applyPhrase hides a produce clip until the answer. It is split out so
// screenFor can set Activity.Audio after wordsFor.
func phraseAudio(act units.Activity, phase, src string) string {
	if act.Type == units.ActivityProduce && phase == "ask" {
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
		return fieldVis{text: true, translit: revealed, gloss: revealed}
	case units.ActivityListenPick, units.ActivityProduce:
		return fieldVis{text: revealed, translit: revealed, gloss: revealed}
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
	fresh := map[string]bool{}
	for _, word := range newWords {
		fresh[word.Text] = true
	}
	var first db.LexemeView
	for _, tok := range units.Tokenize(act.Text) {
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

func gapToken(tokens []string, lex map[string]db.LexemeView, newWords []db.LexemeView) string {
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

func choicesFor(pool []db.LexemeView, target db.LexemeView, hebrew bool) []views.Choice {
	if target.ID == 0 {
		return nil
	}
	picked := []db.LexemeView{target}
	for _, lex := range pool {
		if len(picked) == 3 {
			break
		}
		if lex.ID == target.ID || lex.Text == target.Text {
			continue
		}
		if !hebrew && strings.TrimSpace(lex.Gloss) == "" {
			continue
		}
		picked = append(picked, lex)
	}
	sort.Slice(picked, func(i, j int) bool {
		if picked[i].Text == picked[j].Text {
			return picked[i].ID < picked[j].ID
		}
		return picked[i].Text < picked[j].Text
	})
	out := make([]views.Choice, 0, len(picked))
	for _, lex := range picked {
		out = append(out, views.Choice{
			Value:    lex.Text,
			Label:    lex.Gloss,
			Text:     lex.Text,
			Translit: lex.Translit,
			Hebrew:   hebrew,
		})
	}
	return out
}

func activityPrompt(lesson db.CourseLesson, act units.Activity, target db.LexemeView, empty bool) string {
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
	case units.ActivityRecall:
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
		texts = append(texts, units.Tokenize(act.Text)...)
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
