package speech

import (
	"bufio"
	"embed"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

//go:embed course.tsv
var courseFile embed.FS

// CourseWord is one row of the teachable course list.
type CourseWord struct {
	Order      int
	Rank       int
	Lemma      string
	Pointed    string
	Translit   string
	Gloss      string
	POS        string
	Gender     string
	Root       string
	Binyan     string
	Present    bool
	Kind       string
	Forms      []string
	Status     string
	Check      string
	Source     string
	ReviewedBy string
}

var (
	courseOnce sync.Once
	courseRows []CourseWord
	courseErr  error
)

// Course loads the embedded course list once.
func Course() ([]CourseWord, error) {
	courseOnce.Do(func() {
		f, err := courseFile.Open("course.tsv")
		if err != nil {
			courseErr = err
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		if !sc.Scan() {
			courseErr = sc.Err()
			return
		}
		for sc.Scan() {
			parts := strings.Split(sc.Text(), "\t")
			if len(parts) != 17 {
				courseErr = errCourseRow(parts)
				return
			}
			order, err := strconv.Atoi(parts[0])
			if err != nil {
				courseErr = err
				return
			}
			rank, err := strconv.Atoi(parts[1])
			if err != nil {
				courseErr = err
				return
			}
			var forms []string
			if parts[12] != "" {
				forms = strings.Split(parts[12], "|")
			}
			courseRows = append(courseRows, CourseWord{
				Order: order, Rank: rank, Lemma: parts[2], Pointed: parts[3],
				Translit: parts[4], Gloss: parts[5], POS: parts[6], Gender: parts[7],
				Root: parts[8], Binyan: parts[9], Present: parts[10] == "1",
				Kind: parts[11], Forms: forms, Status: parts[13], Check: parts[14],
				Source: parts[15], ReviewedBy: parts[16],
			})
		}
		courseErr = sc.Err()
	})
	return courseRows, courseErr
}

func errCourseRow(parts []string) error {
	return &rowError{n: len(parts)}
}

type rowError struct{ n int }

func (e *rowError) Error() string {
	return "speech: course row has " + strconv.Itoa(e.n) + " fields, want 17"
}

// CourseReady returns the rows a learner may be graded on, in course order.
func CourseReady() ([]CourseWord, error) {
	rows, err := Course()
	if err != nil {
		return nil, err
	}
	var out []CourseWord
	for _, w := range rows {
		if w.Status == "ready" {
			out = append(out, w)
		}
	}
	return out, nil
}

// LemmaKey is the consonant spelling used to join the frequency list to a
// dictionary headword. Shin and sin dots are removed, because the list does
// not write them and Plain keeps them.
func LemmaKey(s string) string {
	return strings.Map(func(r rune) rune {
		if r == 0x05C1 || r == 0x05C2 {
			return -1
		}
		return r
	}, units.Plain(s))
}

// CourseLessonSize is how many words one numbered lesson holds.
const CourseLessonSize = 5

// CourseLesson is one numbered lesson of the course. Words are in course order.
type CourseLesson struct {
	Number int
	Words  []CourseWord
}

// CourseLessons groups the ready course words into numbered lessons of
// CourseLessonSize, in course order. A prefix is not a lesson word: it is
// taught inside a line. The last lesson may be shorter.
func CourseLessons() ([]CourseLesson, error) {
	rows, err := CourseReady()
	if err != nil {
		return nil, err
	}
	var words []CourseWord
	for _, w := range rows {
		if w.Kind == "prefix" {
			continue
		}
		words = append(words, w)
	}
	out := make([]CourseLesson, 0, (len(words)+CourseLessonSize-1)/CourseLessonSize)
	for i := 0; i < len(words); i += CourseLessonSize {
		end := i + CourseLessonSize
		if end > len(words) {
			end = len(words)
		}
		out = append(out, CourseLesson{Number: len(out) + 1, Words: words[i:end]})
	}
	return out, nil
}

// CourseLessonOf is the lesson with that number. ok is false for a number
// below one or past the last lesson.
func CourseLessonOf(number int) (CourseLesson, bool, error) {
	lessons, err := CourseLessons()
	if err != nil {
		return CourseLesson{}, false, err
	}
	if number < 1 || number > len(lessons) {
		return CourseLesson{}, false, nil
	}
	return lessons[number-1], true, nil
}

// CourseCheck returns the first check a course row fails, or "" when it passes.
// A passing row may still be a draft until a reviewer signs it.
func CourseCheck(w CourseWord) string {
	gloss := strings.TrimSpace(w.Gloss)
	fields := strings.Fields(gloss)
	if len(fields) < 1 || len(fields) > 4 {
		return "entry.gloss"
	}
	for _, r := range gloss {
		if unicode.Is(unicode.Hebrew, r) || r == '<' || r == '>' {
			return "entry.gloss"
		}
	}
	switch w.POS {
	case "noun", "verb", "adjective", "adverb", "preposition", "pronoun",
		"conjunction", "determiner", "particle", "interjection", "numeral", "prefix":
	default:
		return "entry.pos"
	}
	if w.POS == "noun" && w.Gender != "m" && w.Gender != "f" {
		return "entry.gender"
	}
	if w.POS == "verb" && (w.Root == "" || w.Binyan == "" || !w.Present) {
		return "entry.verb"
	}
	if !hasCoursePoint(w.Pointed) {
		return "entry.points"
	}
	if strings.TrimSpace(w.Translit) == "" {
		return "entry.translit"
	}
	return ""
}

func hasCoursePoint(s string) bool {
	for _, r := range s {
		if (r >= 0x05B0 && r <= 0x05BC) || r == 0x05C7 {
			return true
		}
	}
	return false
}
