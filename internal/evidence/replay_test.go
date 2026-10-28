package evidence

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// cardSnap is the card fields a live Attempt and a replay must agree on.
type cardSnap struct {
	State      string
	Due        time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	LastReview time.Time
}

func snapCard(c db.Card) cardSnap {
	return cardSnap{
		State: c.State, Due: c.Due, Stability: c.Stability, Difficulty: c.Difficulty,
		Reps: c.Reps, Lapses: c.Lapses, LastReview: c.LastReview,
	}
}

type logSnap struct {
	Rating        int
	ReviewedAt    time.Time
	ElapsedDays   int
	ScheduledDays int
}

func snapLogs(logs []db.ReviewLog) []logSnap {
	out := make([]logSnap, len(logs))
	for i, l := range logs {
		out[i] = logSnap{l.Rating, l.ReviewedAt, l.ElapsedDays, l.ScheduledDays}
	}
	return out
}

// insertListenAttempts writes listen_pick lesson attempts from a history string
// (R right, W wrong), oldest first, one minute apart ending a minute before now.
func (e *env) insertListenAttempts(lexeme int64, history string) {
	e.t.Helper()
	now := e.clk.t
	for i := range len(history) {
		at := now.Add(-time.Minute).Add(-time.Duration(len(history)-1-i) * time.Minute)
		correct := history[i] == 'R'
		_, err := e.store.WriteAttempt(context.Background(), db.AttemptWrite{
			Attempt: db.Attempt{
				UserID: e.user, LexemeID: lexeme, Skill: fsrs.SkillListen,
				Activity: "listen_pick", Source: SourceLesson,
				Prompt: "audio", Response: "x", Correct: correct, CreatedAt: at,
			},
		})
		if err != nil {
			e.t.Fatal(err)
		}
	}
}

// liveListenAttempts grades the same history through Recorder.Attempt at the
// same times the fixture rows use, advancing the clock between answers.
func (e *env) liveListenAttempts(lexeme int64, history string) {
	e.t.Helper()
	start := e.clk.t.Add(-time.Minute).Add(-time.Duration(len(history)-1) * time.Minute)
	e.clk.t = start
	for i := range len(history) {
		a := Attempt{
			UserID: e.user, LexemeID: lexeme, Skill: fsrs.SkillListen,
			Activity: "listen_pick", Source: SourceLesson,
			Prompt: "audio", Response: "x", Correct: history[i] == 'R',
		}
		if _, err := e.rec.Attempt(context.Background(), a); err != nil {
			e.t.Fatal(err)
		}
		if i+1 < len(history) {
			e.clk.Advance(time.Minute)
		}
	}
	e.clk.t = start.Add(time.Duration(len(history)) * time.Minute)
}

// INIT-023/SPEC-003 ac-1: R, R, W, R with no card ends like four live Attempts.
func TestReplayListenMatchesLiveAttempts(t *testing.T) {
	const history = "RRWR"
	live := newEnv(t)
	lexLive := live.lexeme(kelev)
	live.liveListenAttempts(lexLive, history)
	liveCard, ok := live.card(lexLive, fsrs.SkillListen)
	if !ok {
		t.Fatal("live path created no listen card")
	}
	liveLogs, err := live.store.ReviewLogs(context.Background(), liveCard.ID)
	if err != nil {
		t.Fatal(err)
	}

	replay := newEnv(t)
	lexReplay := replay.lexeme(kelev)
	replay.clk.t = live.clk.t
	replay.insertListenAttempts(lexReplay, history)
	if _, ok := replay.card(lexReplay, fsrs.SkillListen); ok {
		t.Fatal("fixture already had a listen card")
	}
	wrote, err := ReplayListenWord(context.Background(), replay.store, nil, replay.user, lexReplay)
	if err != nil || !wrote {
		t.Fatalf("replay wrote = %v, err = %v", wrote, err)
	}
	replayCard, ok := replay.card(lexReplay, fsrs.SkillListen)
	if !ok {
		t.Fatal("replay created no listen card")
	}
	replayLogs, err := replay.store.ReviewLogs(context.Background(), replayCard.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapCard(liveCard) != snapCard(replayCard) {
		t.Fatalf("card\nlive:   %+v\nreplay: %+v", snapCard(liveCard), snapCard(replayCard))
	}
	if !reflect.DeepEqual(snapLogs(liveLogs), snapLogs(replayLogs)) {
		t.Fatalf("review_log\nlive:   %+v\nreplay: %+v", snapLogs(liveLogs), snapLogs(replayLogs))
	}
}

// INIT-023/SPEC-003 ac-2: all wrong answers get no card.
func TestReplayListenAllWrongGetsNoCard(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	e.insertListenAttempts(lex, "WWW")
	wrote, err := ReplayListenWord(context.Background(), e.store, nil, e.user, lex)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("all-wrong replay wrote a card")
	}
	if _, ok := e.card(lex, fsrs.SkillListen); ok {
		t.Fatal("all-wrong replay left a listen card")
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log`); n != 0 {
		t.Fatalf("review_log = %d, want 0", n)
	}
}

// INIT-023/SPEC-003 ac-3: a second run writes no rows.
func TestReplayListenSecondRunWritesNothing(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	e.insertListenAttempts(lex, "RR")
	if _, err := ReplayListenWord(context.Background(), e.store, nil, e.user, lex); err != nil {
		t.Fatal(err)
	}
	cards := e.count(`SELECT COUNT(*) FROM cards`)
	logs := e.count(`SELECT COUNT(*) FROM review_log`)
	res, err := ReplayListenCards(context.Background(), e.store, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Targets != 0 || res.Written != 0 {
		t.Fatalf("second run = %+v, want no targets", res)
	}
	if e.count(`SELECT COUNT(*) FROM cards`) != cards || e.count(`SELECT COUNT(*) FROM review_log`) != logs {
		t.Fatal("second run changed cards or review_log")
	}
}

// INIT-023/SPEC-003 ac-4: a word that already had a listen card is unchanged.
func TestReplayListenLeavesExistingCardAlone(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	e.liveListenAttempts(lex, "R")
	before, ok := e.card(lex, fsrs.SkillListen)
	if !ok {
		t.Fatal("no listen card after live attempt")
	}
	beforeLogs, err := e.store.ReviewLogs(context.Background(), before.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Extra listen answers that a naive replay might re-apply.
	e.clk.Advance(time.Minute)
	e.insertListenAttempts(lex, "RR")
	wrote, err := ReplayListenWord(context.Background(), e.store, nil, e.user, lex)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("replay rewrote a word that already had a listen card")
	}
	after, ok := e.card(lex, fsrs.SkillListen)
	if !ok || after.ID != before.ID || snapCard(before) != snapCard(after) {
		t.Fatalf("card changed\nbefore: %+v\nafter:  %+v", before, after)
	}
	afterLogs, err := e.store.ReviewLogs(context.Background(), after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapLogs(beforeLogs), snapLogs(afterLogs)) {
		t.Fatalf("review_log changed\nbefore: %+v\nafter:  %+v", snapLogs(beforeLogs), snapLogs(afterLogs))
	}
}

func TestReplayListenDryRunWritesNothing(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	e.insertListenAttempts(lex, "R")
	res, err := ReplayListenCards(context.Background(), e.store, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Targets != 1 || res.Written != 1 {
		t.Fatalf("dry-run = %+v", res)
	}
	if _, ok := e.card(lex, fsrs.SkillListen); ok {
		t.Fatal("dry-run wrote a card")
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log`); n != 0 {
		t.Fatalf("dry-run wrote review_log rows: %d", n)
	}
}

func TestReplayListenWrongThenRightCreatesCard(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	e.insertListenAttempts(lex, "WR")
	wrote, err := ReplayListenWord(context.Background(), e.store, nil, e.user, lex)
	if err != nil || !wrote {
		t.Fatalf("wrote = %v, err = %v", wrote, err)
	}
	card, ok := e.card(lex, fsrs.SkillListen)
	if !ok || card.Reps != 1 {
		t.Fatalf("card = %+v, ok = %v; want one Good review from the right answer", card, ok)
	}
	logs, err := e.store.ReviewLogs(context.Background(), card.ID)
	if err != nil || len(logs) != 1 || logs[0].Rating != int(fsrs.Good) {
		t.Fatalf("review_log = %+v, %v", logs, err)
	}
}
