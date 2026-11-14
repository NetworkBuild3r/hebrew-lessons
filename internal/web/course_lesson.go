package web

import (
	"context"
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// serveCourseLesson is GET /lessons/{number}: one lesson of five common words.
// The number is over the lessons whose words are in the database, the same list
// the Lessons page shows.
func (s *server) serveCourseLesson(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		notFound(w, r)
		return
	}
	lesson, ok, err := s.studyableLesson(r, number)
	if err != nil {
		serverError(w)
		return
	}
	if !ok {
		notFound(w, r)
		return
	}
	page, user, err := s.pageFor(r, lang, csrf, "lessons")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Lesson " + strconv.Itoa(number) + " · " + views.AppName
	v := views.CourseLessonPage{Page: page, Number: number}
	if count, err := s.studyableLessonCount(r); err == nil && number < count {
		v.NextNumber = number + 1
		v.NextHref = page.Href("/lessons/" + strconv.Itoa(number+1))
	}
	allHaveClips := true
	for _, word := range lesson.Words {
		row := views.CourseWordRow{Pointed: word.Pointed, English: word.Gloss, Translit: word.Translit}
		if e, found, err := speech.Lookup(word.Lemma); err != nil {
			serverError(w)
			return
		} else if found {
			row.Times = e.TimesLabel() + " times in every 1,000 words"
		}
		v.Words = append(v.Words, row)
	}
	if user != nil && s.store != nil {
		ids, err := s.lessonLexemeIDs(r.Context(), user.ID, lesson.Words)
		if err != nil {
			serverError(w)
			return
		}
		for _, id := range ids {
			face, found, err := s.store.LexemeByID(r.Context(), id)
			if err != nil {
				serverError(w)
				return
			}
			if !found || face.AudioID == "" {
				allHaveClips = false
			}
		}
		have, err := s.store.CourseCardLexemes(r.Context(), user.ID)
		if err != nil {
			serverError(w)
			return
		}
		done := len(ids) > 0
		for _, id := range ids {
			if !have[id] {
				done = false
			}
		}
		v.Done = done
	} else {
		allHaveClips = false
	}
	if allHaveClips {
		v.Heard = "Every word here has a recording. Play it before you answer."
	}
	render(w, r, views.CourseLesson(v))
}

// postCourseLesson is POST /_hx/lessons/{number}/study. The words come from the
// lesson list on the server, never from the form.
func (s *server) postCourseLesson(w http.ResponseWriter, r *http.Request, lang, _ string) {
	back := "/review"
	if lang == "he" {
		back = "/review?lang=he"
	}
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil || s == nil || s.store == nil || s.study == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	lesson, found, err := s.studyableLesson(r, number)
	if err != nil {
		serverError(w)
		return
	}
	if !found {
		notFound(w, r)
		return
	}
	ids, err := s.lessonLexemeIDs(r.Context(), user.ID, lesson.Words)
	if err != nil {
		serverError(w)
		return
	}
	if len(ids) > 0 {
		built, err := s.study.BuildLesson(r.Context(), user.ID, ids)
		if err != nil {
			serverError(w)
			return
		}
		if err := s.replaceQueue(r.Context(), key, user.ID, built); err != nil {
			serverError(w)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// lessonLexemeIDs maps a lesson's words to stored lexemes by course order.
func (s *server) lessonLexemeIDs(ctx context.Context, userID int64, words []speech.CourseWord) ([]int64, error) {
	course, err := s.store.CourseLexemes(ctx)
	if err != nil {
		return nil, err
	}
	byOrder := map[int]db.CourseLexeme{}
	for _, row := range course {
		byOrder[row.Order] = row
	}
	out := make([]int64, 0, len(words))
	for _, w := range words {
		if row, ok := byOrder[w.Order]; ok {
			out = append(out, row.LexemeID)
		}
	}
	return out, nil
}

// studyableLesson is the lesson at that number among the lessons whose words are
// all in the database, which is the list the Lessons page shows.
func (s *server) studyableLesson(r *http.Request, number int) (speech.CourseLesson, bool, error) {
	kept, err := s.studyableLessons(r)
	if err != nil {
		return speech.CourseLesson{}, false, err
	}
	if number < 1 || number > len(kept) {
		return speech.CourseLesson{}, false, nil
	}
	return kept[number-1], true, nil
}

// studyableLessonCount is how many lessons the learner can open, so a lesson page
// knows whether there is a next one to link to.
func (s *server) studyableLessonCount(r *http.Request) (int, error) {
	kept, err := s.studyableLessons(r)
	if err != nil {
		return 0, err
	}
	return len(kept), nil
}

// studyableLessons is the course lessons whose words are all stored, which is the
// list the Lessons page shows.
func (s *server) studyableLessons(r *http.Request) ([]speech.CourseLesson, error) {
	lessons, err := speech.CourseLessons()
	if err != nil {
		return nil, err
	}
	course, err := s.store.CourseLexemes(r.Context())
	if err != nil {
		return nil, err
	}
	byOrder := map[int]db.CourseLexeme{}
	for _, row := range course {
		byOrder[row.Order] = row
	}
	kept := make([]speech.CourseLesson, 0, len(lessons))
	for _, lesson := range lessons {
		complete := true
		for _, w := range lesson.Words {
			if _, ok := byOrder[w.Order]; !ok {
				complete = false
				break
			}
		}
		if complete {
			kept = append(kept, lesson)
		}
	}
	return kept, nil
}
