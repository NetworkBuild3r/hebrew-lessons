package session

import (
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// learner-1 shaped due mix from INIT-033 / ADR-0028 D6: listen 51 (no clips),
// read 35, recall 56, spell 13. Askable total 104.
func TestMixLearner1DueShareByDirection(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	due := now.Add(-time.Hour)
	var cands []Candidate
	id := int64(1)
	add := func(n int, skill string, audio bool) {
		for i := 0; i < n; i++ {
			c := Candidate{
				ID: id, LexemeID: id, Skill: skill, Due: due.Add(-time.Duration(id) * time.Second),
				HasRating: true, Latest: int(fsrs.Good), Exposed: true,
				Facts: activity.Facts{HasGloss: true, HasAudio: audio, GlossedOthers: 200, VoicedOthers: 200},
				Skills: []activity.SkillView{
					skillView(fsrs.SkillListen, learner.Learning, 0.5, 4),
					skillView(fsrs.SkillRead, learner.Learning, 0.5, 4),
					skillView(fsrs.SkillRecall, learner.Learning, 0.5, 4),
					skillView(fsrs.SkillSpell, learner.Learning, 0.5, 4),
				},
			}
			cands = append(cands, c)
			id++
		}
	}
	add(51, fsrs.SkillListen, false)
	add(35, fsrs.SkillRead, false)
	add(56, fsrs.SkillRecall, false)
	add(13, fsrs.SkillSpell, false)

	got, skipped := mixGate(cands, TargetSuccess, now, 42, activity.OpenGate())
	if skipped != 51 {
		t.Fatalf("SkippedNoAudio = %d, want 51", skipped)
	}
	bySkill := map[string]int{}
	byKind := map[activity.Kind]int{}
	flash := 0
	for _, ref := range got {
		if ref.Kind == KindLetter || ref.Kind == KindPattern {
			continue
		}
		if ref.Activity == activity.Flashcard {
			flash++
		}
		bySkill[ref.AskSkill]++
		byKind[ref.Activity]++
	}
	if flash != 0 {
		t.Fatalf("flashcards = %d, want 0", flash)
	}
	if bySkill[fsrs.SkillListen] != 0 {
		t.Fatalf("listen asks = %d, want 0", bySkill[fsrs.SkillListen])
	}
	study := bySkill[fsrs.SkillRead] + bySkill[fsrs.SkillRecall] + bySkill[fsrs.SkillSpell]
	if study != SessionCap {
		t.Fatalf("study cards = %d, want %d (skills=%v kinds=%v)", study, SessionCap, bySkill, byKind)
	}
	totalAskable := 35 + 56 + 13
	for skill, dueN := range map[string]int{
		fsrs.SkillRead: 35, fsrs.SkillRecall: 56, fsrs.SkillSpell: 13,
	} {
		ideal := float64(SessionCap) * float64(dueN) / float64(totalAskable)
		gotN := bySkill[skill]
		if gotN < 1 {
			t.Fatalf("%s share = %d, want at least 1", skill, gotN)
		}
		if float64(gotN) < ideal-1 || float64(gotN) > ideal+1 {
			t.Fatalf("%s share = %d, want within ±1 of %.2f", skill, gotN, ideal)
		}
	}
	if byKind[activity.ListenPick] != 0 || byKind[activity.Dictation] != 0 {
		t.Fatalf("kinds = %v: no audio activities without clips", byKind)
	}
	if byKind[activity.TypeIt] == 0 || byKind[activity.ReadPick]+byKind[activity.WordMatch] == 0 {
		t.Fatalf("kinds = %v, want type_it and reading formats", byKind)
	}
	t.Logf("learner-1 20-slot mix: skills=%v kinds=%v skipped_no_audio=%d", bySkill, byKind, skipped)
}

func TestMixAsksSpellWithoutClipAsTypeIt(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	c := modelCand(1, fsrs.SkillSpell, now.Add(-time.Hour))
	c.Facts = activity.Facts{HasGloss: true, GlossedOthers: 40}
	got := mix([]Candidate{c}, TargetSuccess, now, 1)
	if len(got) != 1 {
		t.Fatalf("queue = %+v", got)
	}
	if got[0].Activity != activity.TypeIt || got[0].AskSkill != fsrs.SkillSpell {
		t.Fatalf("ref = %+v, want type_it through spell", got[0])
	}
}

func TestMixPrefersDictationWhenClipExists(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	c := modelCand(1, fsrs.SkillSpell, now.Add(-time.Hour))
	got := mix([]Candidate{c}, TargetSuccess, now, 1)
	if len(got) != 1 || got[0].Activity != activity.Dictation {
		t.Fatalf("ref = %+v, want dictation when a clip exists", got)
	}
}

func TestMixAsksRecallAndSpellOnce(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	spell := modelCand(1, fsrs.SkillSpell, now.Add(-2*time.Hour))
	spell.Facts = activity.Facts{HasGloss: true, GlossedOthers: 40}
	recall := modelCand(2, fsrs.SkillRecall, now.Add(-time.Hour))
	recall.LexemeID = spell.LexemeID
	recall.Facts = spell.Facts
	got := mix([]Candidate{spell, recall}, TargetSuccess, now, 1)
	if len(got) != 1 {
		t.Fatalf("queue = %+v, want one type_it for both cards", got)
	}
	if got[0].Activity != activity.TypeIt || got[0].AskSkill != fsrs.SkillSpell || got[0].CardID != spell.ID {
		t.Fatalf("ref = %+v, want the spell card asked as type_it", got[0])
	}
}

func TestMixLimitsKindRuns(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	var cands []Candidate
	// Many recall cards (only type_it) and a few read cards so another kind exists.
	for i := 1; i <= 16; i++ {
		c := modelCand(int64(i), fsrs.SkillRecall, now.Add(-time.Duration(20-i)*time.Minute))
		c.Facts = activity.Facts{HasGloss: true, GlossedOthers: 40}
		cands = append(cands, c)
	}
	for i := 17; i <= 20; i++ {
		cands = append(cands, modelCand(int64(i), fsrs.SkillRead, now.Add(-time.Duration(40-i)*time.Minute)))
	}
	got := mix(cands, TargetSuccess, now, 7)
	assertKindRunsWithinMax(t, got)
}

// Kind weave runs after the first lexeme pass; a second separateLexemes must
// keep paired read/recall of one word from sitting side by side when another
// seat exists, without letting a kind run past MaxKindRun.
func TestMixSeparatesLexemesAfterKindWeave(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	var cands []Candidate
	id := int64(1)
	for i := 0; i < 8; i++ {
		lex := int64(100 + i)
		read := modelCand(id, fsrs.SkillRead, now.Add(-time.Duration(40-i)*time.Minute))
		read.LexemeID = lex
		cands = append(cands, read)
		id++
		recall := modelCand(id, fsrs.SkillRecall, now.Add(-time.Duration(39-i)*time.Minute))
		recall.LexemeID = lex
		recall.Facts = activity.Facts{HasGloss: true, GlossedOthers: 40}
		cands = append(cands, recall)
		id++
	}
	for i := 0; i < 8; i++ {
		c := modelCand(id, fsrs.SkillRecall, now.Add(-time.Duration(20-i)*time.Minute))
		c.Facts = activity.Facts{HasGloss: true, GlossedOthers: 40}
		cands = append(cands, c)
		id++
	}
	got := mix(cands, TargetSuccess, now, 11)
	if i, lex := adjacentLexemeAt(got); i >= 0 {
		t.Fatalf("lexeme %d sits adjacent at %d after kind weave: %v", lex, i, idsOf(got))
	}
	assertKindRunsWithinMax(t, got)
}

func assertKindRunsWithinMax(t *testing.T, got []CardRef) {
	t.Helper()
	kinds := activitiesOf(got)
	kindSet := map[activity.Kind]bool{}
	for _, k := range kinds {
		kindSet[k] = true
	}
	if len(kindSet) < 2 {
		t.Fatalf("kinds = %v, want more than one kind in the session", kinds)
	}
	run, prev := 0, activity.Kind("")
	for i, k := range kinds {
		if k == prev {
			run++
		} else {
			prev, run = k, 1
		}
		if run <= MaxKindRun {
			continue
		}
		laterOther := false
		for _, later := range kinds[i+1:] {
			if later != prev {
				laterOther = true
				break
			}
		}
		if laterOther {
			t.Fatalf("kind run of %q exceeds %d with another kind waiting: %v", prev, MaxKindRun, kinds)
		}
	}
}

func activitiesOf(refs []CardRef) []activity.Kind {
	var out []activity.Kind
	for _, r := range refs {
		if studyKind(r) {
			out = append(out, r.Activity)
		}
	}
	return out
}
