package learner

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestDirectionLabel(t *testing.T) {
	cases := []struct {
		skill, want string
	}{
		{fsrs.SkillListen, "Listening"},
		{fsrs.SkillRead, "Reading (Hebrew → English)"},
		{fsrs.SkillRecall, "English → Hebrew"},
		{fsrs.SkillSpell, "Spelling"},
	}
	for _, c := range cases {
		if got := DirectionLabel(c.skill); got != c.want {
			t.Errorf("DirectionLabel(%q) = %q, want %q", c.skill, got, c.want)
		}
	}
}

func TestStandingLearner1ShapedVerdicts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// Enough lexemes so Snapshot has something to count; no audio_id → listen not_askable.
	for i := 0; i < 8; i++ {
		e.lexeme()
	}
	seedAttemptWindow(t, e, fsrs.SkillRecall, 50, 18) // 50 of 68 → needs_help
	seedAttemptWindow(t, e, fsrs.SkillRead, 189, 33)  // 189 of 222 → strong
	seedAttemptWindow(t, e, fsrs.SkillSpell, 2, 1)    // 3 answers → building
	// Placement answers must not change Answers14.
	place := e.lexeme()
	for i := 0; i < 40; i++ {
		at := testNow.Add(-time.Hour).Add(-time.Duration(i) * time.Minute)
		e.exec(`INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, 'recall', 'test', 'placement', '', '', 0, 0, 0, 0, ?)`,
			e.user, place, db.FormatTime(at))
	}

	got, err := e.model.Standing(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]DirectionStanding{}
	for _, d := range got.Directions {
		by[d.Skill] = d
	}
	assertVerdict(t, by, fsrs.SkillRecall, "needs_help", 68, 50)
	assertVerdict(t, by, fsrs.SkillRead, "strong", 222, 189)
	assertVerdict(t, by, fsrs.SkillSpell, "building", 3, 2)
	listen := by[fsrs.SkillListen]
	if listen.Verdict != "not_askable" || listen.Askable {
		t.Fatalf("listen = %+v, want not_askable", listen)
	}
	if listen.Reason != "No audio yet — practiced in lessons only" {
		t.Fatalf("listen reason = %q", listen.Reason)
	}
	if by[fsrs.SkillRecall].Answers14 != 68 {
		t.Fatalf("placement leaked into recall Answers14 = %d", by[fsrs.SkillRecall].Answers14)
	}
}

func TestStandingKnownAnySkillMatchesSummary(t *testing.T) {
	e := newEnv(t)
	a, b := e.lexeme(), e.lexeme()
	e.known(e.user, a, fsrs.SkillRead, 30)
	e.known(e.user, b, fsrs.SkillRecall, 10)

	snap, err := e.model.Snapshot(context.Background(), e.user)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.model.Standing(context.Background(), e.user)
	if err != nil {
		t.Fatal(err)
	}
	if got.KnownAnySkill != snap.Summary.KnownAnySkill {
		t.Fatalf("KnownAnySkill = %d, Summary = %d", got.KnownAnySkill, snap.Summary.KnownAnySkill)
	}
	for _, d := range got.Directions {
		var ss SkillSummary
		for _, s := range snap.Summary.Skills {
			if s.Skill == d.Skill {
				ss = s
				break
			}
		}
		if d.Known != ss.Counts[Known] || d.Solid != ss.Counts[Solid] ||
			d.Learning != ss.Counts[Learning] || d.Struggling != ss.Counts[Struggling] ||
			d.Regressed != ss.Counts[Regressed] || d.Due != ss.DueReviews {
			t.Fatalf("%s counts standing=%+v summary=%+v due=%d", d.Skill, d, ss.Counts, ss.DueReviews)
		}
	}
}

func TestStandingHelpRanksMissesAndCausePair(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// Three recall words with different miss counts; the top miss also has a letter pair.
	w1, w2, w3 := e.lexeme(), e.lexeme(), e.lexeme()
	// Make w1 struggling in recall via again rule (3 of last 8 wrong) and many recent misses.
	e.attempts(e.user, w1, fsrs.SkillRecall, "WWWWRRRR")
	e.attempts(e.user, w2, fsrs.SkillRecall, "WWRRRRRR")
	e.attempts(e.user, w3, fsrs.SkillRecall, "WRRRRRRR")
	// Extra 14-day misses so ranking is by Misses14 (attempts helper already writes review source).
	seedMisses(t, e, w1, fsrs.SkillRecall, 5)
	seedMisses(t, e, w2, fsrs.SkillRecall, 3)
	seedMisses(t, e, w3, fsrs.SkillRecall, 2)
	// Letter-pair confusions on w1 so DominantPair and Diagnose yield spelling/confusion.
	for i := 0; i < ConfusionPairMin; i++ {
		at := testNow.Add(-time.Duration(i+1) * time.Hour)
		e.exec(`INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at)
			VALUES (?, 'letter_pair', 'ב/כ', ?, ?)`, e.user, w1, db.FormatTime(at))
	}

	got, err := e.model.Standing(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	var recall DirectionStanding
	for _, d := range got.Directions {
		if d.Skill == fsrs.SkillRecall {
			recall = d
		}
	}
	if len(recall.Help) < 1 {
		t.Fatalf("help empty: %+v", recall)
	}
	if recall.Help[0].LexemeID != w1 {
		t.Fatalf("help[0] = %d, want word with most misses %d; help=%v", recall.Help[0].LexemeID, w1, noteIDs(recall.Help))
	}
	if recall.Help[0].Pair != "ב/כ" {
		t.Fatalf("help[0].Pair = %q, want ב/כ", recall.Help[0].Pair)
	}
	if recall.Help[0].Cause == "" {
		t.Fatalf("help[0].Cause empty; want Diagnose cause")
	}
	if recall.Help[0].Hebrew == "" || recall.Help[0].English == "" {
		t.Fatalf("help[0] missing gloss: %+v", recall.Help[0])
	}
	// Strong must not overlap Help.
	helpSet := map[int64]bool{}
	for _, n := range recall.Help {
		helpSet[n.LexemeID] = true
	}
	for _, n := range recall.Strong {
		if helpSet[n.LexemeID] {
			t.Fatalf("lexeme %d in both Help and Strong", n.LexemeID)
		}
	}
}

func TestStandingHelpConfusedWithOtherWord(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	asked := e.lexeme()
	other := e.lexeme()
	// Pointed texts in code-point order for an "other" confusion key.
	e.exec(`UPDATE lexemes SET text_nfc = 'אָלֶף', gloss_en = 'aleph' WHERE id = ?`, asked)
	e.exec(`UPDATE lexemes SET text_nfc = 'בֵּית', gloss_en = 'bet' WHERE id = ?`, other)
	e.attempts(e.user, asked, fsrs.SkillRecall, "WWWWRRRR")
	seedMisses(t, e, asked, fsrs.SkillRecall, 3)
	key := "אָלֶף/בֵּית"
	for i := 0; i < ConfusionPairMin; i++ {
		at := testNow.Add(-time.Duration(i+1) * time.Hour)
		e.exec(`INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at)
			VALUES (?, 'other', ?, ?, ?)`, e.user, key, asked, db.FormatTime(at))
	}

	got, err := e.model.Standing(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	var recall DirectionStanding
	for _, d := range got.Directions {
		if d.Skill == fsrs.SkillRecall {
			recall = d
		}
	}
	if len(recall.Help) < 1 || recall.Help[0].LexemeID != asked {
		t.Fatalf("help = %v, want asked %d first", noteIDs(recall.Help), asked)
	}
	if recall.Help[0].ConfusedWith != "בֵּית" {
		t.Fatalf("ConfusedWith = %q, want בֵּית (other lemma, not a typed response)", recall.Help[0].ConfusedWith)
	}
	if recall.Help[0].Pair != key {
		t.Fatalf("Pair = %q, want %q", recall.Help[0].Pair, key)
	}
}

func TestStandingThresholdChangeChangesVerdict(t *testing.T) {
	// 50/68 ≈ 0.735: needs_help under HelpAccuracy 0.75 / HelpMinAnswers 10.
	if got := directionVerdict(true, 68, 50, 0, HelpAccuracy, HelpMinAnswers, StrongAccuracy, StrongMinAnswers); got != "needs_help" {
		t.Fatalf("default bars = %s, want needs_help", got)
	}
	// Raising the help accuracy floor so 73.5% clears it → building.
	if got := directionVerdict(true, 68, 50, 0, 0.70, HelpMinAnswers, StrongAccuracy, StrongMinAnswers); got != "building" {
		t.Fatalf("HelpAccuracy=0.70 = %s, want building", got)
	}
	// 189/222 ≈ 0.851 is strong; raising StrongMinAnswers past 222 → building.
	if got := directionVerdict(true, 222, 189, 0, HelpAccuracy, HelpMinAnswers, StrongAccuracy, StrongMinAnswers); got != "strong" {
		t.Fatalf("default strong = %s", got)
	}
	if got := directionVerdict(true, 222, 189, 0, HelpAccuracy, HelpMinAnswers, StrongAccuracy, 300); got != "building" {
		t.Fatalf("StrongMinAnswers=300 = %s, want building", got)
	}
}

func TestStandingThresholdsArePinned(t *testing.T) {
	if HelpAccuracy != 0.75 || HelpMinAnswers != 10 || StrongAccuracy != 0.85 ||
		StrongMinAnswers != 20 || StandingWindowDays != 14 || StandingTop != 5 {
		t.Fatalf("standing thresholds drifted: %v %d %v %d %d %d",
			HelpAccuracy, HelpMinAnswers, StrongAccuracy, StrongMinAnswers, StandingWindowDays, StandingTop)
	}
}

func assertVerdict(t *testing.T, by map[string]DirectionStanding, skill, want string, answers, right int) {
	t.Helper()
	d := by[skill]
	if d.Verdict != want {
		t.Fatalf("%s verdict = %s, want %s (reason %q)", skill, d.Verdict, want, d.Reason)
	}
	if d.Answers14 != answers || d.Right14 != right {
		t.Fatalf("%s answers = %d/%d, want %d/%d", skill, d.Right14, d.Answers14, right, answers)
	}
	if !d.Askable {
		t.Fatalf("%s Askable=false", skill)
	}
}

// seedAttemptWindow writes right+wrong review attempts inside the 14-day window,
// spread across a few lexemes so no single word trips struggling rules by accident.
func seedAttemptWindow(t *testing.T, e *env, skill string, right, wrong int) {
	t.Helper()
	ids := []int64{e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme()}
	n := 0
	write := func(correct int) {
		id := ids[n%len(ids)]
		n++
		at := testNow.Add(-time.Hour).Add(-time.Duration(n) * time.Minute)
		e.exec(`INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, ?, 'test', 'review', '', '', ?, 0, 0, 0, ?)`,
			e.user, id, skill, correct, db.FormatTime(at))
	}
	for i := 0; i < right; i++ {
		write(1)
	}
	for i := 0; i < wrong; i++ {
		write(0)
	}
}

func seedMisses(t *testing.T, e *env, lexeme int64, skill string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		at := testNow.Add(-30 * time.Minute).Add(-time.Duration(i) * time.Minute)
		e.exec(`INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, ?, 'test', 'review', '', '', 0, 0, 0, 0, ?)`,
			e.user, lexeme, skill, db.FormatTime(at))
	}
}

func noteIDs(notes []WordNote) []int64 {
	out := make([]int64, len(notes))
	for i, n := range notes {
		out[i] = n.LexemeID
	}
	return out
}
