package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// INIT-027/SPEC-004 move-up and implied-credit tests.

func moveUpCand(id int64, due time.Time) Candidate {
	c := knownCand(id, due)
	c.Skills = []activity.SkillView{
		skillView(fsrs.SkillListen, learner.Learning, 0.5, 4),
		skillView(fsrs.SkillRead, learner.Known, 0.95, 8),
		skillView(fsrs.SkillRecall, learner.Learning, 0.7, 4),
		skillView(fsrs.SkillSpell, learner.Learning, 0.6, 4),
	}
	c.Facts = richFacts
	c.Exposed = true
	c.ImplyingCardID = id + 1000
	return c
}

func TestMoveUpWhenEveryConditionHolds(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	c := moveUpCand(1, now.Add(-time.Hour))
	got := mix([]Candidate{c}, TargetSuccess, now, 1)
	if len(got) != 1 {
		t.Fatalf("queue = %+v", got)
	}
	ref := got[0]
	if !ref.MovedUp || ref.AskSkill != fsrs.SkillRecall || ref.Activity != activity.TypeIt {
		t.Fatalf("ref = %+v, want moved-up recall type_it", ref)
	}
	if ref.OwnActivity != activity.ReadPick || ref.Skill != fsrs.SkillRead {
		t.Fatalf("ref = %+v, want own read_pick on the read card", ref)
	}
}

func TestMoveUpBlockedWhenAnyConditionBreaks(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	due := now.Add(-time.Hour)
	cases := []struct {
		name string
		edit func(*Candidate)
	}{
		{"new card", func(c *Candidate) { c.HasRating = false }},
		{"own skill learning", func(c *Candidate) {
			for i := range c.Skills {
				if c.Skills[i].Skill == fsrs.SkillRead {
					c.Skills[i].State = learner.Learning
				}
			}
		}},
		{"implying again/hard", func(c *Candidate) { c.ImplyingRecentAgainOrHard = true }},
		{"implying card queued", func(c *Candidate) {}}, // handled below with two cards
		{"gate closed for recall", func(c *Candidate) {}}, // handled with mixGate
	}
	for _, tc := range cases {
		if tc.name == "implying card queued" || tc.name == "gate closed for recall" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			c := moveUpCand(1, due)
			tc.edit(&c)
			got := mix([]Candidate{c}, TargetSuccess, now, 1)
			if len(got) != 1 {
				t.Fatalf("queue = %+v", got)
			}
			if got[0].MovedUp || got[0].AskSkill != fsrs.SkillRead {
				t.Fatalf("ref = %+v, want own read skill", got[0])
			}
		})
	}
	t.Run("implying card queued", func(t *testing.T) {
		read := moveUpCand(1, due)
		recall := knownCand(1001, due.Add(-time.Minute))
		recall.Skill = fsrs.SkillRecall
		recall.LexemeID = read.LexemeID
		recall.Skills, recall.Facts, recall.Exposed = read.Skills, richFacts, true
		got := mix([]Candidate{read, recall}, TargetSuccess, now, 1)
		for _, ref := range got {
			if ref.CardID == 1 && ref.MovedUp {
				t.Fatalf("read moved up while recall is queued: %+v", got)
			}
		}
	})
	t.Run("gate closed for recall", func(t *testing.T) {
		c := moveUpCand(1, due)
		got := mixGate([]Candidate{c}, TargetSuccess, now, 1, activity.Gate{})
		if got[0].MovedUp || got[0].AskSkill != fsrs.SkillRead {
			t.Fatalf("ref = %+v, want own skill when recall is closed", got[0])
		}
	})
	t.Run("no checked activity on imply", func(t *testing.T) {
		c := moveUpCand(1, due)
		c.Facts = activity.Facts{HasAudio: true, GlossedOthers: 40, VoicedOthers: 40} // no gloss → no type_it
		got := mix([]Candidate{c}, TargetSuccess, now, 1)
		if got[0].MovedUp {
			t.Fatalf("moved up without a recall activity: %+v", got[0])
		}
	})
}

func TestHardInsideWindowBlocksMoveUp(t *testing.T) {
	// ac-3: Hard on the recall card inside 7 days blocks move-up even when latest is Good.
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	c := moveUpCand(1, now.Add(-time.Hour))
	c.ImplyingRecentAgainOrHard = true
	got := mix([]Candidate{c}, TargetSuccess, now, 1)
	if got[0].MovedUp {
		t.Fatalf("Hard inside the window still moved up: %+v", got[0])
	}
}

func TestAnsweredImpliedCreditAndRequeue(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	word := addVoicedLexeme(t, store, "ספר", "book", true)
	due := clk.Now().Add(-time.Hour)
	reviewed := clk.Now().Add(-48 * time.Hour)
	// High stability + many right answers so the learner model marks read Known.
	readID, err := store.InsertCard(ctx, dbCard(userID, word, fsrs.SkillRead, due, reviewed, 30))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		insertReview(t, store, readID, int(fsrs.Good), reviewed.Add(time.Duration(i)*time.Hour))
	}
	recallID, err := store.InsertCard(ctx, dbCard(userID, word, fsrs.SkillRecall, clk.Now().Add(48*time.Hour), reviewed, 10))
	if err != nil {
		t.Fatal(err)
	}
	insertReview(t, store, recallID, int(fsrs.Good), reviewed)
	lessonAttempts(t, svc, userID, word, fsrs.SkillRead, 8, 0)
	lessonAttempts(t, svc, userID, word, fsrs.SkillRecall, 4, 0)

	sess := mustBuild(t, svc, userID)
	var visit CardRef
	found := false
	for {
		ref, ok := sess.Next()
		if !ok {
			break
		}
		if ref.CardID == readID {
			visit = ref
			found = true
			break
		}
	}
	if !found {
		t.Fatal("due read card was not served")
	}
	if !visit.MovedUp {
		t.Fatalf("visit = %+v, want MovedUp (read Known, recall clean)", visit)
	}

	// Right Good → implied on read, no attempts for read.
	beforeRead, _ := store.GetCard(ctx, readID)
	beforeRecall, _ := store.GetCard(ctx, recallID)
	out, err := sess.Answered(ctx, readID, activity.Result{Correct: true, Rating: fsrs.Good})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Implied || out.Slow {
		t.Fatalf("outcome = %+v", out)
	}
	afterRead, _ := store.GetCard(ctx, readID)
	if !afterRead.Due.After(clk.Now()) {
		t.Fatalf("read due = %v, want future", afterRead.Due)
	}
	if afterRead.Due.Equal(beforeRead.Due) {
		t.Fatal("read schedule did not move")
	}
	logs := mustReviewLogs(t, store, readID)
	last := logs[len(logs)-1]
	if !last.Implied || last.Rating != int(fsrs.Good) {
		t.Fatalf("last log = %+v, want implied Good", last)
	}
	attempts, err := store.ListAttempts(ctx, userID, word)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range attempts {
		if a.Skill == fsrs.SkillRead && a.Source == "review" {
			t.Fatalf("read got an attempts row: %+v", a)
		}
	}
	_ = beforeRecall

	// Wrong moved-up answer: no write on own card, requeue on own skill.
	sess = mustBuild(t, svc, userID)
	// Advance clock so the previously implied read is not due; seed a fresh due read.
	// Rebuild with a new word instead.
	word2 := addVoicedLexeme(t, store, "מים", "water", true)
	read2, err := store.InsertCard(ctx, dbCard(userID, word2, fsrs.SkillRead, due, reviewed, 30))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		insertReview(t, store, read2, int(fsrs.Good), reviewed.Add(time.Duration(i)*time.Hour))
	}
	recall2, err := store.InsertCard(ctx, dbCard(userID, word2, fsrs.SkillRecall, clk.Now().Add(48*time.Hour), reviewed, 10))
	if err != nil {
		t.Fatal(err)
	}
	insertReview(t, store, recall2, int(fsrs.Good), reviewed)
	lessonAttempts(t, svc, userID, word2, fsrs.SkillRead, 8, 0)
	lessonAttempts(t, svc, userID, word2, fsrs.SkillRecall, 4, 0)
	// Filler due cards so a miss is requeued (oneLexemeLocked would skip it).
	for i := 0; i < 4; i++ {
		w := addVoicedLexeme(t, store, fmt.Sprintf("מילוי-%d", i), fmt.Sprintf("fill%d", i), true)
		addCardFor(t, store, userID, w, fsrs.SkillRead, due.Add(-time.Duration(i+1)*time.Minute), int(fsrs.Good), reviewed)
	}
	sess = mustBuild(t, svc, userID)
	var moved CardRef
	for {
		ref, ok := sess.Next()
		if !ok {
			t.Fatal("queue empty")
		}
		if ref.CardID == read2 && ref.MovedUp {
			moved = ref
			break
		}
	}
	before, _ := store.GetCard(ctx, read2)
	logsBefore := len(mustReviewLogs(t, store, read2))
	if _, err := sess.Answered(ctx, read2, activity.Result{Correct: false, Rating: fsrs.Again}); err != nil {
		t.Fatal(err)
	}
	after, _ := store.GetCard(ctx, read2)
	if after.Due != before.Due || after.Reps != before.Reps {
		t.Fatalf("wrong moved-up answer changed the read card: before %+v after %+v", before, after)
	}
	if len(mustReviewLogs(t, store, read2)) != logsBefore {
		t.Fatal("wrong moved-up answer wrote a review_log row")
	}
	rest := drainAll(t, sess)
	foundRetry := false
	for _, ref := range rest {
		if ref.CardID == read2 {
			foundRetry = true
			if ref.MovedUp || ref.AskSkill != fsrs.SkillRead || ref.Activity != moved.OwnActivity {
				t.Fatalf("retry = %+v, want own skill", ref)
			}
		}
	}
	if !foundRetry {
		t.Fatal("wrong moved-up answer did not requeue")
	}

	// Hard on a moved-up answer: no implied credit.
	word3 := addVoicedLexeme(t, store, "אור", "light", true)
	read3, err := store.InsertCard(ctx, dbCard(userID, word3, fsrs.SkillRead, due, reviewed, 30))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		insertReview(t, store, read3, int(fsrs.Good), reviewed.Add(time.Duration(i)*time.Hour))
	}
	recall3, err := store.InsertCard(ctx, dbCard(userID, word3, fsrs.SkillRecall, clk.Now().Add(48*time.Hour), reviewed, 10))
	if err != nil {
		t.Fatal(err)
	}
	insertReview(t, store, recall3, int(fsrs.Good), reviewed)
	lessonAttempts(t, svc, userID, word3, fsrs.SkillRead, 8, 0)
	lessonAttempts(t, svc, userID, word3, fsrs.SkillRecall, 4, 0)
	sess = mustBuild(t, svc, userID)
	for {
		ref, ok := sess.Next()
		if !ok {
			t.Fatal("no moved-up visit")
		}
		if ref.CardID == read3 && ref.MovedUp {
			before, _ = store.GetCard(ctx, read3)
			logsBefore = len(mustReviewLogs(t, store, read3))
			out, err = sess.Answered(ctx, read3, activity.Result{Correct: true, Rating: fsrs.Hard})
			if err != nil {
				t.Fatal(err)
			}
			if out.Implied || !out.Slow {
				t.Fatalf("Hard outcome = %+v, want Slow and no Implied", out)
			}
			after, _ = store.GetCard(ctx, read3)
			if after.Due != before.Due {
				t.Fatal("Hard moved the read card")
			}
			if len(mustReviewLogs(t, store, read3)) != logsBefore {
				t.Fatal("Hard wrote an implied log")
			}
			break
		}
	}
}

func dbCard(userID, lexemeID int64, skill string, due, reviewed time.Time, stability float64) db.Card {
	return db.Card{
		UserID: userID, LexemeID: lexemeID, Skill: skill, State: fsrs.StateReview,
		Due: due, Stability: stability, Difficulty: 5, Reps: 5, LastReview: reviewed,
	}
}

func TestSessionTally(t *testing.T) {
	// NewMet counts when Next serves a CardRef.New visit (documented in Session.Tally).
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	due := clk.Now().Add(-time.Hour)
	reviewed := clk.Now().Add(-48 * time.Hour)
	w1 := addVoicedLexeme(t, store, "כלב", "dog", true)
	w2 := addVoicedLexeme(t, store, "חתול", "cat", true)
	addCardFor(t, store, userID, w1, fsrs.SkillRead, due, int(fsrs.Good), reviewed)
	addCardFor(t, store, userID, w2, fsrs.SkillRead, due, 0, time.Time{}) // new
	sess := mustBuild(t, svc, userID)
	var answered, right, slow, newMet int
	for {
		ref, ok := sess.Next()
		if !ok {
			break
		}
		if ref.New {
			newMet++
		}
		if ref.Activity == "" || ref.Activity == activity.Flashcard {
			revealAndRecord(t, sess, ctx, ref.CardID, fsrs.Good)
			answered++
			right++
			continue
		}
		res := activity.Result{Correct: true, Rating: fsrs.Good}
		if answered == 0 {
			res = activity.Result{Correct: true, Rating: fsrs.Hard}
			slow++
		}
		if _, err := sess.Answered(ctx, ref.CardID, res); err != nil {
			t.Fatal(err)
		}
		answered++
		right++
	}
	got := sess.Tally()
	if got.Answered != answered || got.Right != right {
		t.Fatalf("tally answered/right = %+v, want %d/%d", got, answered, right)
	}
	if got.NewMet != newMet || got.NewMet < 1 {
		t.Fatalf("NewMet = %d, served new = %d", got.NewMet, newMet)
	}
	if got.Slow != slow {
		t.Fatalf("Slow = %d, want %d", got.Slow, slow)
	}
}
