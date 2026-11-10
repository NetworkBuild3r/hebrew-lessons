package learner

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestLadderBoundaries(t *testing.T) {
	base := history("RRR")
	base = withCard(base, 3, 10, 0.95)
	base.LatestRight = true
	base.Taught = true
	base.TaughtDay = "2026-11-01"
	base.SuccessDays = 3

	proved := base
	proved.FirstSuccessDay = "2026-11-01"
	proved.LastSuccessDay = "2026-11-08"
	proved.Retrievability = 0.1 // late card; recall chance does not move the rung
	if rung, ok := LadderOf(proved); !ok || rung != Proved {
		t.Fatalf("span 7 late card = %q ok=%v, want proved", rung, ok)
	}
	if KnownRule4(proved) {
		t.Fatal("rule 4 must not count a card whose recall chance has fallen")
	}

	short := proved
	short.LastSuccessDay = "2026-11-07" // 6 days
	short.Retrievability = 0.95
	if rung, ok := LadderOf(short); !ok || rung != RecalledLater {
		t.Fatalf("span 6 = %q ok=%v, want recalled later", rung, ok)
	}

	once := base
	once.SuccessDays = 1
	once.FirstSuccessDay = "2026-11-02"
	once.LastSuccessDay = "2026-11-02"
	if rung, ok := LadderOf(once); !ok || rung != RecalledLater {
		t.Fatalf("one later right = %q ok=%v, want recalled later", rung, ok)
	}
	if rung, _ := LadderOf(once); rung == Proved {
		t.Fatal("one later right was proved")
	}

	slip := proved
	slip.LatestRight = false
	if rung, ok := LadderOf(slip); !ok || rung != Slipping {
		t.Fatalf("proved then wrong = %q ok=%v, want slipping", rung, ok)
	}

	sameDay := base
	sameDay.SuccessDays = 1
	sameDay.FirstSuccessDay = "2026-11-01"
	sameDay.LastSuccessDay = "2026-11-01"
	if rung, ok := LadderOf(sameDay); !ok || rung != NotYetAsked {
		t.Fatalf("right only on the taught day = %q ok=%v, want not yet asked", rung, ok)
	}

	untouched := base
	untouched.Outcomes = 0
	untouched.SuccessDays = 0
	untouched.LatestRight = false
	if rung, ok := LadderOf(untouched); !ok || rung != NotYetAsked {
		t.Fatalf("no answer = %q ok=%v, want not yet asked", rung, ok)
	}

	if _, ok := LadderOf(Evidence{SuccessDays: 3, LatestRight: true}); ok {
		t.Fatal("an untaught word was counted")
	}
}

func TestKnownRule4StillCountsTwoStudyDays(t *testing.T) {
	e := withDays(withCard(history("RRR"), 3, 10, 0.9), 2)
	e.Taught = true
	e.TaughtDay = "2026-11-01"
	e.FirstSuccessDay = "2026-11-01"
	e.LastSuccessDay = "2026-11-02"
	e.LatestRight = true
	if !KnownRule4(e) {
		t.Fatal("rule 4 did not count two study days")
	}
	if rung, ok := LadderOf(e); !ok || rung != RecalledLater {
		t.Fatalf("ladder = %q ok=%v, want recalled later", rung, ok)
	}
}

func TestHeadlineNames(t *testing.T) {
	if HeadlineListen != "Understand by ear" || HeadlineRecall != "Hebrew from English" {
		t.Fatalf("headlines %q %q", HeadlineListen, HeadlineRecall)
	}
}

func TestLadderCountsFromTheLog(t *testing.T) {
	e := newEnv(t)
	e.clk.t = time.Date(2027, 1, 15, 18, 0, 0, 0, time.UTC)
	ctx := context.Background()

	proved := e.lexeme()
	e.checkedCard(proved, fsrs.SkillRecall, day(2026, 11, 1), 0.2, []review{
		{day(2026, 11, 1), 3},
		{day(2026, 11, 4), 3},
		{day(2026, 11, 8), 3},
	})
	short := e.lexeme()
	e.checkedCard(short, fsrs.SkillRecall, day(2026, 11, 1), 10, []review{
		{day(2026, 11, 1), 3},
		{day(2026, 11, 4), 3},
		{day(2026, 11, 7), 3},
	})
	once := e.lexeme()
	e.checkedCard(once, fsrs.SkillRecall, day(2026, 11, 1), 10, []review{
		{day(2026, 11, 2), 3},
	})
	slipped := e.lexeme()
	e.checkedCard(slipped, fsrs.SkillRecall, day(2026, 11, 1), 10, []review{
		{day(2026, 11, 1), 3},
		{day(2026, 11, 4), 3},
		{day(2026, 11, 8), 3},
		{day(2026, 11, 9), 1},
	})
	fresh := e.lexeme()
	e.checkedCard(fresh, fsrs.SkillRecall, day(2026, 11, 1), 0, nil)
	placed := e.lexeme()
	e.placementOnly(placed, fsrs.SkillRecall, day(2026, 11, 1))

	sum, err := e.model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	var recall LadderCounts
	for _, sk := range sum.Skills {
		if sk.Skill == fsrs.SkillRecall {
			recall = sk.Ladder
		}
		if got := sk.Ladder.Proved + sk.Ladder.RecalledLater + sk.Ladder.Slipping + sk.Ladder.NotYetAsked; got != sk.Ladder.Taught {
			t.Fatalf("%s rungs %d, taught %d", sk.Skill, got, sk.Ladder.Taught)
		}
	}
	if recall.Proved != 1 || recall.RecalledLater != 2 || recall.Slipping != 1 || recall.NotYetAsked != 1 || recall.Taught != 5 {
		t.Fatalf("recall ladder = %+v", recall)
	}
	if skillOf(e.word(proved), fsrs.SkillRecall).Ladder != Proved {
		t.Fatalf("proved word ladder = %s", skillOf(e.word(proved), fsrs.SkillRecall).Ladder)
	}
	if skillOf(e.word(placed), fsrs.SkillRecall).Ladder != "" {
		t.Fatal("placement-only card was counted as taught")
	}

	// The old count still sees the two-day card; the ladder does not call it proved.
	if !KnownRule4(withDays(withCard(history("RRR"), 3, 10, 0.95), 2)) {
		t.Fatal("rule 4 function lost the old count")
	}
}

func TestVoidedAnswerCountsForNothing(t *testing.T) {
	e := newEnv(t)
	e.clk.t = time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)
	lex := e.lexeme()
	cardID := e.checkedCard(lex, fsrs.SkillRecall, day(2026, 11, 1), 10, nil)
	reviewID := e.insertReview(cardID, 3, day(2026, 11, 3))
	attemptID := e.insertAttempt(lex, fsrs.SkillRecall, true, day(2026, 11, 3), "review")

	sum := e.summary(t)
	if got := ladderOf(sum, fsrs.SkillRecall).RecalledLater; got != 1 {
		t.Fatalf("before void recalledLater = %d", got)
	}

	e.exec(`INSERT INTO answer_voids (attempt_id, review_log_id, learner_id, reason, actor, batch_id, created_at)
		VALUES (?, ?, ?, 'also_right', 'test', 'batch-1', ?)`,
		attemptID, reviewID, e.user, db.FormatTime(e.clk.t))

	sum = e.summary(t)
	got := ladderOf(sum, fsrs.SkillRecall)
	if got.RecalledLater != 0 || got.Proved != 0 || got.NotYetAsked != 1 {
		t.Fatalf("after void ladder = %+v", got)
	}
}

type review struct {
	at     time.Time
	rating int
}

func day(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 18, 0, 0, 0, time.UTC)
}

func (e *env) checkedCard(lex int64, skill string, created time.Time, stability float64, reviews []review) int64 {
	e.t.Helper()
	id, err := e.store.InsertCard(context.Background(), db.Card{
		UserID: e.user, LexemeID: lex, Skill: skill, State: fsrs.StateReview,
		Due: e.clk.t, Stability: stability, Difficulty: 5, Reps: len(reviews),
		LastReview: created, CreatedAt: created,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	var last time.Time
	for _, r := range reviews {
		e.insertReview(id, r.rating, r.at)
		last = r.at
	}
	if !last.IsZero() {
		e.exec(`UPDATE cards SET last_review = ?, reps = ? WHERE id = ?`, db.FormatTime(last), len(reviews), id)
	}
	return id
}

func (e *env) insertReview(cardID int64, rating int, at time.Time) int64 {
	e.t.Helper()
	res, err := e.store.ExecContext(context.Background(),
		`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, ?, ?, 1, 1)`,
		cardID, rating, db.FormatTime(at))
	if err != nil {
		e.t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) insertAttempt(lex int64, skill string, right bool, at time.Time, source string) int64 {
	e.t.Helper()
	correct := 0
	if right {
		correct = 1
	}
	res, err := e.store.ExecContext(context.Background(),
		`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		 VALUES (?, ?, ?, 'test', ?, '', '', ?, 0, 0, 0, ?)`,
		e.user, lex, skill, source, correct, db.FormatTime(at))
	if err != nil {
		e.t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) placementOnly(lex int64, skill string, at time.Time) {
	e.t.Helper()
	cardID := e.checkedCard(lex, skill, at, 10, []review{{at, 3}})
	e.insertAttempt(lex, skill, true, at, "placement")
	_ = cardID
}

func (e *env) summary(t *testing.T) Summary {
	t.Helper()
	sum, err := e.model.Summary(context.Background(), e.user)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func ladderOf(sum Summary, skill string) LadderCounts {
	for _, sk := range sum.Skills {
		if sk.Skill == skill {
			return sk.Ladder
		}
	}
	return LadderCounts{}
}
