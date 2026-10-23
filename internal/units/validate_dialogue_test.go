package units

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// INIT-015/SPEC-026: the dialogue activity. These tests build on the brief fixture
// (brief_golden_test.go), whose vocabulary is already checked against Pealim.

// spkLookup is a lookup that can tell the two voices apart. The clips are listed as
// speaker + NUL + text, the way a store keeps them per voice.
type spkLookup struct {
	fakeLookup
	spk map[string]bool
	err error
}

func (l spkLookup) HasAudioForSpeaker(_ context.Context, text, speaker string) (bool, error) {
	if l.err != nil {
		return false, l.err
	}
	return l.spk[speaker+"\x00"+normalize(text)], nil
}

func spkKey(speaker, text string) string { return speaker + "\x00" + normalize(text) }

// dialogueFixture is the valid brief fixture plus one two-speaker dialogue in the last
// lesson. Every line has a clip in its own speaker's voice and in no other.
func dialogueFixture(lines ...DialogueLine) (Unit, spkLookup, BriefFacts, int, int) {
	u, lk, f := briefFixture()
	if len(lines) == 0 {
		lines = []DialogueLine{
			{Speaker: SpeakerA, Text: vocab8[0].text},
			{Speaker: SpeakerB, Text: vocab8[1].text},
		}
	}
	li := len(u.Lessons) - 1
	u.Lessons[li].Activities = append(u.Lessons[li].Activities, Activity{Type: ActivityDialogue, Lines: lines})
	ai := len(u.Lessons[li].Activities) - 1
	sl := spkLookup{fakeLookup: lk, spk: map[string]bool{}}
	for _, l := range lines {
		sl.spk[spkKey(l.Speaker, l.Text)] = true
	}
	return u, sl, f, li, ai
}

func dialogueRules(issues []Issue) []string {
	var out []string
	for _, is := range issues {
		if is.Rule == RuleUnitDialogueLines || is.Rule == RuleUnitDialogueSpeakers {
			out = append(out, is.Rule+" "+is.Pointer)
		}
	}
	return out
}

// ac-1, valid half: a dialogue of two speakers validates, with and without a brief.
func TestDialogueValidates(t *testing.T) {
	u, lk, f, _, _ := dialogueFixture()
	if got := validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("dialogue with a brief: %v", ruleSet(got))
	}
	for _, is := range Validate(context.Background(), &u, lk) {
		if is.Rule != RuleCoverageListening {
			t.Fatalf("v2 rules on a dialogue: %v", is)
		}
	}
}

// ac-1, failing half: one speaker, one line, nine lines, an unknown speaker.
func TestDialogueShapeRules(t *testing.T) {
	a := DialogueLine{Speaker: SpeakerA, Text: vocab8[0].text}
	b := DialogueLine{Speaker: SpeakerB, Text: vocab8[1].text}
	nine := make([]DialogueLine, 9)
	for i := range nine {
		nine[i] = []DialogueLine{a, b}[i%2]
	}
	cases := []struct {
		name  string
		lines []DialogueLine
		want  []string // dialogue rule + pointer, sorted
	}{
		{"one speaker", []DialogueLine{a, a, a}, []string{"unit.dialogue_speakers LINES"}},
		{"one line", []DialogueLine{a}, []string{"unit.dialogue_lines LINES"}},
		{"nine lines", nine, []string{"unit.dialogue_lines LINES"}},
		{"eight lines is fine", nine[:8], nil},
		{"two lines is fine", []DialogueLine{b, a}, nil},
		{"unknown speaker", []DialogueLine{a, {Speaker: "c", Text: vocab8[1].text}, b}, []string{"unit.dialogue_speakers LINES/1/speaker"}},
		{"upper case speaker", []DialogueLine{a, {Speaker: "B", Text: vocab8[1].text}}, []string{"unit.dialogue_speakers LINES", "unit.dialogue_speakers LINES/1/speaker"}},
		{"empty speaker", []DialogueLine{a, {Speaker: "", Text: vocab8[1].text}, b}, []string{"unit.dialogue_speakers LINES/1/speaker"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, lk, f, li, ai := dialogueFixture(c.lines...)
			base := at("lessons", li, "activities", ai)
			var want []string
			for _, w := range c.want {
				want = append(want, strings.ReplaceAll(w, "LINES", base+"/lines"))
			}
			slices.Sort(want)
			got := dialogueRules(validateBrief(&u, lk, &f))
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Fatalf("dialogue rules\n got %v\nwant %v", got, want)
			}
			// The same shape rules fire without a brief.
			plain := dialogueRules(Validate(context.Background(), &u, lk))
			slices.Sort(plain)
			if !slices.Equal(plain, want) {
				t.Fatalf("Validate dialogue rules\n got %v\nwant %v", plain, want)
			}
		})
	}
}

func TestDialogueTextAndLinesPlacement(t *testing.T) {
	u, lk, f, li, ai := dialogueFixture()
	u.Lessons[li].Activities[ai].Text = vocab8[0].text
	got := validateBrief(&u, lk, &f)
	want := []string{"unit.dialogue_lines " + at("lessons", li, "activities", ai, "text")}
	if r := dialogueRules(got); !slices.Equal(r, want) {
		t.Fatalf("a dialogue with text: %v", r)
	}

	// Lines on any other activity are refused, so they cannot carry text the
	// validator never reads.
	u2, lk2, f2, li2, _ := dialogueFixture()
	u2.Lessons[li2].Activities[0].Lines = []DialogueLine{{Speaker: SpeakerA, Text: vocab8[0].text}}
	r := dialogueRules(validateBrief(&u2, lk2, &f2))
	want = []string{"unit.dialogue_lines " + at("lessons", li2, "activities", 0, "lines")}
	if !slices.Equal(r, want) {
		t.Fatalf("lines on a recall: %v", r)
	}

	// An empty line is a missing field, pointed at the line.
	u3, lk3, f3, li3, ai3 := dialogueFixture(
		DialogueLine{Speaker: SpeakerA, Text: vocab8[0].text},
		DialogueLine{Speaker: SpeakerB, Text: "  "},
	)
	got = validateBrief(&u3, lk3, &f3)
	if !hasIssue(got, RuleSchemaRequired, at("lessons", li3, "activities", ai3, "lines", 1, "text")) {
		t.Fatalf("empty line: %v", ruleSet(got))
	}
}

func hasIssue(issues []Issue, rule, pointer string) bool {
	for _, is := range issues {
		if is.Rule == rule && is.Pointer == pointer {
			return true
		}
	}
	return false
}

// ac-5: a line whose clip exists only in the other speaker's voice is rejected.
func TestDialogueClipInOtherVoiceIsRejected(t *testing.T) {
	u, lk, f, li, ai := dialogueFixture()
	lines := u.Lessons[li].Activities[ai].Lines
	// Line 0 is spoken by a. Give it a clip in b's voice only.
	delete(lk.spk, spkKey(SpeakerA, lines[0].Text))
	lk.spk[spkKey(SpeakerB, lines[0].Text)] = true
	got := validateBrief(&u, lk, &f)
	want := []Issue{iss(RuleAudioMissing, SeverityBlock, at("lessons", li, "activities", ai, "lines", 0, "text"))}
	if !sameIssues(got, want) {
		t.Fatalf("clip in the other voice: %v", ruleSet(got))
	}
	if !strings.Contains(got[0].Msg, "speaker a") {
		t.Fatalf("message does not name the voice: %q", got[0].Msg)
	}
	// And the other way round, with no brief.
	lk.spk[spkKey(SpeakerA, lines[0].Text)] = true
	delete(lk.spk, spkKey(SpeakerB, lines[1].Text))
	lk.spk[spkKey(SpeakerA, lines[1].Text)] = true
	plain := Validate(context.Background(), &u, lk)
	if !hasIssue(plain, RuleAudioMissing, at("lessons", li, "activities", ai, "lines", 1, "text")) {
		t.Fatalf("b line with an a clip: %v", ruleSet(plain))
	}
}

// A lookup that cannot tell the voices apart must not pass a dialogue: it would
// accept a clip in the wrong voice.
func TestDialogueNeedsASpeakerLookup(t *testing.T) {
	u, lk, f, _, _ := dialogueFixture()
	got := validateBrief(&u, lk.fakeLookup, &f)
	if !hasRule(got, RuleSchemaLookup) {
		t.Fatalf("a lookup without voices passed a dialogue: %v", ruleSet(got))
	}
	for _, is := range got {
		if is.Rule == RuleSchemaLookup && !strings.Contains(is.Msg, "voice") {
			t.Fatalf("lookup issue does not say why: %q", is.Msg)
		}
	}
	bad := lk
	bad.err = errors.New("boom")
	if got := validateBrief(&u, bad, &f); !hasRule(got, RuleSchemaLookup) {
		t.Fatalf("a failing speaker lookup: %v", ruleSet(got))
	}
}

// Each line is a passage: the length limit and coverage are per line.
func TestDialogueBriefRulesArePerLine(t *testing.T) {
	w, other := vocab8[0].text, vocab8[1].text
	t.Run("text too long is per line", func(t *testing.T) {
		// Two lines of 2 tokens are 4 tokens in all, and the limit is 3.
		u, lk, f, li, ai := dialogueFixture(
			DialogueLine{Speaker: SpeakerA, Text: w + " " + other},
			DialogueLine{Speaker: SpeakerB, Text: other + " " + w},
		)
		f.Tier, f.MaxTextTokens, f.MinKnownShare = "beginner", 3, 0.6
		if got := validateBrief(&u, lk, &f); len(got) != 0 {
			t.Fatalf("two short lines: %v", ruleSet(got))
		}
		u2, lk2, f2, li2, ai2 := dialogueFixture(
			DialogueLine{Speaker: SpeakerA, Text: w},
			DialogueLine{Speaker: SpeakerB, Text: repeatJoin(w, 4)},
		)
		f2.Tier, f2.MaxTextTokens, f2.MinKnownShare = "beginner", 3, 0.6
		got := validateBrief(&u2, lk2, &f2)
		want := []Issue{iss(RuleBriefTextTooLong, SeverityBlock, at("lessons", li2, "activities", ai2, "lines", 1))}
		if !sameIssues(got, want) {
			t.Fatalf("a long line: %v", ruleSet(got))
		}
		_, _ = li, ai
	})
	t.Run("coverage is per line", func(t *testing.T) {
		// 30 known words in line 0 and one known and one unknown word in line 1.
		// Together 31 of 32 is 96.9%, over the 0.95 figure. Line 1 alone is 50%.
		u, lk, f, li, ai := dialogueFixture(
			DialogueLine{Speaker: SpeakerA, Text: repeatJoin(w, 30)},
			DialogueLine{Speaker: SpeakerB, Text: w + " " + naal},
		)
		got := validateBrief(&u, lk, &f)
		want := []Issue{iss(RuleBriefCoverage, SeverityBlock, at("lessons", li, "activities", ai, "lines", 1))}
		if !sameIssues(got, want) {
			t.Fatalf("a line the learner cannot follow: %v", ruleSet(got))
		}
	})
	t.Run("an unknown or unpointed word is a story-style finding at the line", func(t *testing.T) {
		u, lk, f, li, ai := dialogueFixture(
			DialogueLine{Speaker: SpeakerA, Text: w},
			DialogueLine{Speaker: SpeakerB, Text: stripNiqqud(other)},
		)
		// The audio check would also fire for the stripped text; give it a clip.
		lk.spk[spkKey(SpeakerB, stripNiqqud(other))] = true
		got := validateBrief(&u, lk, &f)
		if !hasIssue(got, RuleStoryUnpointed, at("lessons", li, "activities", ai, "lines", 1, "text")) {
			t.Fatalf("unpointed line: %v", ruleSet(got))
		}
		u2, lk2, _, li2, ai2 := dialogueFixture(
			DialogueLine{Speaker: SpeakerA, Text: w},
			DialogueLine{Speaker: SpeakerB, Text: gelida},
		)
		got = Validate(context.Background(), &u2, lk2)
		if !hasIssue(got, RuleStoryUnmapped, at("lessons", li2, "activities", ai2, "lines", 1, "text")) {
			t.Fatalf("unmapped line: %v", ruleSet(got))
		}
		_ = f
	})
}

// A dialogue is not a way round the brief: its words count toward the uses of a
// new word exactly as a story's do.
func TestDialogueWordsCountTowardTheBrief(t *testing.T) {
	// The base unit uses the first new word 8 times. A brief asking for 9 fails,
	// and one more use inside a dialogue line satisfies it.
	u, lk, f := briefFixture()
	f.NewWords[0].MinUses = 9
	if got := validateBrief(&u, lk, &f); !hasRule(got, RuleBriefNewMinUses) {
		t.Fatalf("base: %v", ruleSet(got))
	}
	ud, lkd, fd, _, _ := dialogueFixture(
		DialogueLine{Speaker: SpeakerA, Text: vocab8[0].text},
		DialogueLine{Speaker: SpeakerB, Text: vocab8[1].text},
	)
	fd.NewWords[0].MinUses = 9
	if got := validateBrief(&ud, lkd, &fd); hasRule(got, RuleBriefNewMinUses) {
		t.Fatalf("a use in a dialogue line was not counted: %v", ruleSet(got))
	}
	// A word in a dialogue line that is outside the brief and has no gloss is still blocked.
	ug, lkg, fg, li, ai := dialogueFixture(
		DialogueLine{Speaker: SpeakerA, Text: vocab8[0].text},
		DialogueLine{Speaker: SpeakerB, Text: naal},
	)
	fg.MinKnownShare = 0.4
	lex := lkg.lex[normalize(naal)]
	lex.Gloss = ""
	lkg.lex[normalize(naal)] = lex
	got := validateBrief(&ug, lkg, &fg)
	if !hasIssue(got, RuleBriefGloss, at("lessons", li, "activities", ai, "lines", 1, "text")) {
		t.Fatalf("gloss rule on a dialogue line: %v", ruleSet(got))
	}
}

// A dialogue line is exposure, so it can introduce a word before a check.
func TestDialogueIntroducesBeforeCheck(t *testing.T) {
	u, lk, f := briefFixture()
	// Lesson 0 holds shadow, shadow, recall, recall, listen_pick, story. Swap its first two
	// shadows for one dialogue that says both new words, one line each.
	l0 := u.Lessons[0].Activities
	first := []string{l0[0].Text, l0[1].Text}
	u.Lessons[0].Activities = append([]Activity{{
		Type: ActivityDialogue,
		Lines: []DialogueLine{
			{Speaker: SpeakerA, Text: first[0]},
			{Speaker: SpeakerB, Text: first[1]},
		},
	}}, l0[2:]...)
	spk := spkLookup{fakeLookup: lk, spk: map[string]bool{
		spkKey(SpeakerA, first[0]): true,
		spkKey(SpeakerB, first[1]): true,
	}}
	got := validateBrief(&u, spk, &f)
	for _, is := range got {
		if is.Rule == RuleBriefIntro {
			t.Fatalf("a dialogue did not introduce the words: %v", ruleSet(got))
		}
	}
	// Without the dialogue those words are checked first, and the rule fires.
	u.Lessons[0].Activities = l0[2:]
	if got := validateBrief(&u, spk, &f); !hasRule(got, RuleBriefIntro) {
		t.Fatalf("control: %v", ruleSet(got))
	}
}

// A pointer into the flat copy must come out as a pointer into the real document,
// including after a dialogue has shifted the activity indexes.
func TestDialogueIssuePointersAreRemapped(t *testing.T) {
	u, lk, _, li, ai := dialogueFixture()
	// An activity after the dialogue, with a word that is not in the lexicon.
	u.Lessons[li].Activities = append(u.Lessons[li].Activities, Activity{Type: ActivityRecall, Text: gelida})
	after := len(u.Lessons[li].Activities) - 1
	if after != ai+1 {
		t.Fatalf("fixture: after=%d ai=%d", after, ai)
	}
	got := Validate(context.Background(), &u, lk)
	if !hasIssue(got, RuleLexemeUnknown, at("lessons", li, "activities", after, "text")) {
		t.Fatalf("pointer after a dialogue: %v", ruleSet(got))
	}
	for _, is := range got {
		if strings.Contains(is.Pointer, "/activities/") {
			// No pointer may name an activity past the real list.
			var idx int
			rest := is.Pointer[strings.Index(is.Pointer, "/activities/")+len("/activities/"):]
			rest, _, _ = strings.Cut(rest, "/")
			for _, r := range rest {
				idx = idx*10 + int(r-'0')
			}
			if idx >= len(u.Lessons[li].Activities) && strings.HasPrefix(is.Pointer, at("lessons", li)) {
				t.Fatalf("pointer past the real list: %s", is.Pointer)
			}
		}
	}

	flat, refs := flatten(&u)
	if flat == &u || len(refs) != len(u.Lessons) {
		t.Fatalf("flatten returned the document itself or the wrong refs")
	}
	if n := len(flat.Lessons[li].Activities); n != len(u.Lessons[li].Activities)-1+2 {
		t.Fatalf("flat lesson has %d activities", n)
	}
	for _, a := range flat.Lessons[li].Activities {
		if a.Type == ActivityDialogue && (a.speaker == "" || a.Text == "" || len(a.Lines) != 0) {
			t.Fatalf("flat line: %+v", a)
		}
	}
	// A document with no dialogue is not copied, and remapping it is the identity.
	plain, _, _ := briefFixture()
	if p, r := flatten(&plain); p != &plain || r != nil {
		t.Fatal("a document with no dialogue was copied")
	}
	if got := remapPointer("/title", refs); got != "/title" {
		t.Fatalf("remap /title = %q", got)
	}
	if got := remapPointer("/lessons/99/activities/0", refs); got != "/lessons/99/activities/0" {
		t.Fatalf("remap out of range = %q", got)
	}
}

func TestDialogueJSONShape(t *testing.T) {
	u, _, _, li, ai := dialogueFixture()
	raw, err := json.Marshal(u.Lessons[li].Activities[ai])
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"dialogue","lines":[{"speaker":"a","text":"` + vocab8[0].text + `"},{"speaker":"b","text":"` + vocab8[1].text + `"}]}`
	if string(raw) != want {
		t.Fatalf("shape\n got %s\nwant %s", raw, want)
	}
	var back Activity
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.speaker != "" || !reflect.DeepEqual(back.Lines, u.Lessons[li].Activities[ai].Lines) {
		t.Fatalf("round trip: %+v", back)
	}
	// The validator's one-line view never leaks into a document.
	flat, _ := flatten(&u)
	for _, a := range flat.Lessons[li].Activities {
		if a.Type != ActivityDialogue {
			continue
		}
		b, _ := json.Marshal(a)
		if strings.Contains(string(b), "speaker") {
			t.Fatalf("the private speaker field was serialised: %s", b)
		}
	}
	if got := u.Lessons[li].Activities[ai].Texts(); !slices.Equal(got, []string{vocab8[0].text, vocab8[1].text}) {
		t.Fatalf("Texts() = %v", got)
	}
	if got := (Activity{Type: ActivityStory, Text: "x"}).Texts(); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("Texts() of a story = %v", got)
	}
}

func TestDialogueMarkupInEveryStringField(t *testing.T) {
	populated := func() Unit {
		return Unit{
			Title: "t", Situation: "s", Concept: "c",
			NewWords: []string{"w"},
			Lessons: []Lesson{{Kind: "lesson", Activities: []Activity{
				{Type: "dialogue", Lines: []DialogueLine{{Speaker: "a", Text: "x"}}},
			}}},
		}
	}
	var collect func(v reflect.Value, out *[]reflect.Value)
	collect = func(v reflect.Value, out *[]reflect.Value) {
		switch v.Kind() {
		case reflect.String:
			*out = append(*out, v)
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					collect(v.Field(i), out)
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				collect(v.Index(i), out)
			}
		}
	}
	base := populated()
	var fields []reflect.Value
	collect(reflect.ValueOf(&base).Elem(), &fields)
	if len(fields) != 13 {
		t.Fatalf("the unit has %d string fields with a dialogue, want 13. A new field needs a markup check.", len(fields))
	}
	for i := range fields {
		for _, bad := range []string{"<", ">", "javascript:alert(1)"} {
			d := populated()
			var fs []reflect.Value
			collect(reflect.ValueOf(&d).Elem(), &fs)
			fs[i].SetString(bad)
			if got := markupIssues(&d); len(got) != 1 || got[0].Rule != RuleUnitNoMarkup {
				t.Fatalf("string field %d with %q: %v", i, bad, ruleSet(got))
			}
		}
	}
	d := populated()
	d.Lessons[0].Activities[0].Lines[0].Text = "<b>"
	d.Lessons[0].Activities[0].Lines[0].Speaker = "javascript:x"
	got := markupIssues(&d)
	var ptrs []string
	for _, is := range got {
		ptrs = append(ptrs, is.Pointer)
	}
	slices.Sort(ptrs)
	want := []string{"/lessons/0/activities/0/lines/0/speaker", "/lessons/0/activities/0/lines/0/text"}
	if !slices.Equal(ptrs, want) {
		t.Fatalf("markup pointers %v", ptrs)
	}
}

func TestDialogueRulesAreCatalogedAndDocumented(t *testing.T) {
	cat := map[string]Rule{}
	var order []string
	for _, r := range Rules {
		cat[r.ID] = r
		order = append(order, r.ID)
	}
	if !slices.IsSorted(order) {
		t.Fatalf("catalog is not sorted: %v", order)
	}
	for _, id := range []string{RuleUnitDialogueLines, RuleUnitDialogueSpeakers} {
		r, ok := cat[id]
		if !ok || r.Severity != SeverityBlock || r.Description == "" {
			t.Fatalf("catalog row for %s: %+v", id, r)
		}
	}
	if RuleUnitDialogueLines != "unit.dialogue_lines" || RuleUnitDialogueSpeakers != "unit.dialogue_speakers" {
		t.Fatal("the dialogue rule ids are part of the contract")
	}
	if !knownActivity(ActivityDialogue) || SkillOfActivity(ActivityDialogue) != SkillRead || !exposureActivity(ActivityDialogue) || !passageActivity(ActivityDialogue) {
		t.Fatal("dialogue is not wired as a read-skill passage that exposes words")
	}
	if !ValidSpeaker("a") || !ValidSpeaker("b") || ValidSpeaker("A") || ValidSpeaker("") || ValidSpeaker("ab") || ValidSpeaker(" a") {
		t.Fatal("ValidSpeaker is not exact")
	}
}
