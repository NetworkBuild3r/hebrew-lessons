package evidence

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// INIT-015/SPEC-015: a typed answer one letter off is rated Hard, and an attempt can carry
// the letter pairs found in what was typed.

func TestGradeNearMissIsHard(t *testing.T) {
	for _, tc := range []struct {
		name   string
		a      Attempt
		median time.Duration
		want   fsrs.Rating
	}{
		{"near miss", Attempt{Correct: true, NearMiss: true, Latency: time.Second}, 5 * time.Second, fsrs.Hard},
		{"near miss, no timing", Attempt{Correct: true, NearMiss: true}, 0, fsrs.Hard},
		{"near miss never Easy", Attempt{Correct: true, NearMiss: true, Latency: time.Millisecond}, time.Hour, fsrs.Hard},
		{"near miss and a hint", Attempt{Correct: true, NearMiss: true, HintUsed: true}, 0, fsrs.Hard},
		{"right is still Good", Attempt{Correct: true, Latency: 9 * time.Second}, 5 * time.Second, fsrs.Good},
		{"dont know beats near miss", Attempt{Correct: true, NearMiss: true, DontKnow: true}, 0, fsrs.Again},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Grade(tc.a, tc.median); got != tc.want {
				t.Fatalf("Grade = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestNearMissIsRatedHardAndStoredCorrect(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	a := e.attempt(lex)
	a.NearMiss = true
	rating, err := e.rec.Attempt(context.Background(), a)
	if err != nil || rating != fsrs.Hard {
		t.Fatalf("rating = %d, %v, want Hard", rating, err)
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts WHERE correct = 1`); n != 1 {
		t.Fatalf("%d correct attempts", n)
	}
	card, ok := e.card(lex, fsrs.SkillRead)
	if !ok {
		t.Fatal("no card")
	}
	logs, err := e.store.ReviewLogs(context.Background(), card.ID)
	if err != nil || len(logs) != 1 || logs[0].Rating != int(fsrs.Hard) {
		t.Fatalf("review_log = %+v, %v", logs, err)
	}
}

func TestNearMissMustBeRight(t *testing.T) {
	e := newEnv(t)
	a := e.attempt(e.lexeme(kelev))
	a.Correct, a.NearMiss = false, true
	if _, err := e.rec.Attempt(context.Background(), a); err == nil {
		t.Fatal("a wrong answer cannot be a near miss")
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`); n != 0 {
		t.Fatalf("%d attempts written", n)
	}
}

func TestAttemptWritesItsConfusions(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	a := e.attempt(lex)
	a.Correct, a.NearMiss = true, true
	a.Confusions = []Confusion{
		{Kind: fsrs.MissLetterPair, Key: "ם/ן"},
		{Kind: fsrs.MissLetterPair, Key: "ח/כ"},
	}
	if _, err := e.rec.Attempt(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	rows, err := e.conn().Query(`SELECT kind, key, lexeme_id, at FROM confusion_misses WHERE learner_id = ? ORDER BY id`, e.user)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var kind, key, at string
		var lexeme int64
		if err := rows.Scan(&kind, &key, &lexeme, &at); err != nil {
			t.Fatal(err)
		}
		if lexeme != lex {
			t.Fatalf("confusion about lexeme %d, want %d", lexeme, lex)
		}
		got = append(got, kind+":"+key)
	}
	if strings.Join(got, ",") != "letter_pair:ם/ן,letter_pair:ח/כ" {
		t.Fatalf("confusions = %v", got)
	}
}

// The picked-word confusion and the extra ones are all written, in one transaction with the
// attempt: when one is refused nothing is stored.
func TestExtraConfusionsAreValidatedAndAtomic(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	for name, cs := range map[string][]Confusion{
		"unknown kind": {{Kind: "typo", Key: "א/ב"}},
		"empty key":    {{Kind: fsrs.MissLetterPair, Key: "  "}},
		"long key":     {{Kind: fsrs.MissLetterPair, Key: strings.Repeat("א", 201)}},
		"too many": func() []Confusion {
			var out []Confusion
			for i := 0; i <= MaxConfusions; i++ {
				out = append(out, Confusion{Kind: fsrs.MissOther, Key: "x"})
			}
			return out
		}(),
	} {
		a := e.attempt(lex)
		a.Confusions = cs
		if _, err := e.rec.Attempt(context.Background(), a); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`) + e.count(`SELECT COUNT(*) FROM confusion_misses`) + e.count(`SELECT COUNT(*) FROM cards`); n != 0 {
		t.Fatalf("%d rows written by refused attempts", n)
	}

	// A wrong pick of another word and an extra pair are both written.
	other := e.lexeme(kala)
	a := e.attempt(lex)
	a.Correct, a.ChosenLexemeID = false, other
	a.Confusions = []Confusion{{Kind: fsrs.MissAgreement, Key: "הוא/היא"}}
	if _, err := e.rec.Attempt(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM confusion_misses WHERE learner_id = ?`, e.user); n != 2 {
		t.Fatalf("%d confusion rows, want the picked word and the extra", n)
	}
}
