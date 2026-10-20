package units

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func briefFixture() (Unit, fakeLookup, BriefFacts) {
	u, lex, f := briefBase()
	lk := goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	return u, lk, f
}

func validateBrief(u *Unit, lk Lookup, f *BriefFacts) []Issue {
	return ValidateWithBrief(context.Background(), u, lk, f)
}

func ruleSet(issues []Issue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Rule+" "+is.Pointer)
	}
	return out
}

func TestBriefValidUnitIsClean(t *testing.T) {
	u, lk, f := briefFixture()
	if got := validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("valid unit: %v", ruleSet(got))
	}
	// The v2 rules agree except for coverage: they judge the struggle word's
	// passages against earlier shared units, and brief.coverage replaces that.
	for _, is := range Validate(context.Background(), &u, lk) {
		if is.Rule != RuleCoverageListening {
			t.Fatalf("v2 rules on the same unit: %v", is)
		}
	}
}

func TestBriefRulesAreCatalogedAndDocumented(t *testing.T) {
	doc, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{RuleBriefAudioAll, RuleBriefCoverage, RuleBriefExpired, RuleBriefGloss, RuleBriefIntro,
		RuleBriefNewMinUses, RuleBriefNotFound, RuleBriefStruggle, RuleBriefTextTooLong, RuleBriefUsed, RuleUnitNoMarkup}
	want := []string{"brief.audio_all", "brief.coverage", "brief.expired", "brief.gloss_out_of_brief",
		"brief.new_intro_before_test", "brief.new_min_uses", "brief.not_found", "brief.struggle_reuse",
		"brief.text_too_long", "brief.used", "unit.no_markup"}
	if !slices.Equal(ids, want) {
		t.Fatalf("rule ids changed: %v", ids)
	}
	catalog := map[string]Rule{}
	var order []string
	for _, r := range Rules {
		catalog[r.ID] = r
		order = append(order, r.ID)
	}
	if !slices.IsSorted(order) {
		t.Fatalf("catalog is not sorted by id: %v", order)
	}
	for _, id := range ids {
		r, ok := catalog[id]
		if !ok || r.Severity != SeverityBlock || r.Description == "" {
			t.Fatalf("catalog row for %s: %+v", id, r)
		}
		if !strings.Contains(string(doc), id) {
			t.Fatalf("%s is not documented in doc.go", id)
		}
	}
}

func TestBriefNilAndUsedAndLookup(t *testing.T) {
	u, lk, f := briefFixture()
	got := validateBrief(&u, lk, nil)
	if len(got) != 1 || got[0].Rule != RuleBriefNotFound || got[0].Pointer != "/briefId" {
		t.Fatalf("nil brief: %v", ruleSet(got))
	}
	got = ValidateWithBrief(context.Background(), nil, lk, &f)
	if len(got) != 1 || got[0].Rule != RuleSchemaRequired {
		t.Fatalf("nil doc: %v", ruleSet(got))
	}
	// A failing lookup is reported once, and unit.no_markup still runs.
	u.Title = "<b>"
	bad := lk
	bad.err = errors.New("boom")
	got = validateBrief(&u, bad, &f)
	if len(got) != 2 || got[0].Rule != RuleUnitNoMarkup && got[1].Rule != RuleUnitNoMarkup {
		t.Fatalf("lookup failure: %v", ruleSet(got))
	}
	n := 0
	for _, is := range got {
		if is.Rule == RuleSchemaLookup {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("schema.lookup reported %d times: %v", n, ruleSet(got))
	}
	// A nil lookup leaves out the brief rules but not the markup rule.
	got = validateBrief(&u, nil, &f)
	if !hasRule(got, RuleUnitNoMarkup) || !hasRule(got, RuleSchemaLookup) {
		t.Fatalf("nil lookup: %v", ruleSet(got))
	}
}

func TestBriefDeterministic(t *testing.T) {
	u, lk, f := briefFixture()
	u.Title = "<x>"
	f.Used = true
	f.NewWords[0].MinUses = 99
	a := validateBrief(&u, lk, &f)
	b := validateBrief(&u, lk, &f)
	if len(a) < 3 || !reflect.DeepEqual(a, b) {
		t.Fatalf("not deterministic\n%v\n%v", ruleSet(a), ruleSet(b))
	}
	if !sortedPair(a) {
		t.Fatal("issues are not sorted")
	}
}

// TestNoMarkupCoversEveryStringField walks the Unit struct by reflection. If a
// string field is added to the document and markupIssues does not look at it,
// this test fails.
func TestNoMarkupCoversEveryStringField(t *testing.T) {
	populated := func() Unit {
		return Unit{
			Title: "t", Situation: "s", Concept: "c",
			NewWords: []string{"w"},
			Lessons:  []Lesson{{Kind: "lesson", Activities: []Activity{{Type: "recall", Text: "x"}}}},
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
	if len(fields) != 9 {
		t.Fatalf("the unit has %d string fields, want 9. A new field needs a markup check.", len(fields))
	}
	for i := range fields {
		for _, bad := range []string{"<", ">", "javascript:alert(1)", "JAVASCRIPT:x"} {
			d := populated()
			var fs []reflect.Value
			collect(reflect.ValueOf(&d).Elem(), &fs)
			fs[i].SetString(bad)
			if got := markupIssues(&d); len(got) != 1 || got[0].Rule != RuleUnitNoMarkup {
				t.Fatalf("string field %d with %q: %v", i, bad, ruleSet(got))
			}
		}
	}
}

func TestNoMarkupPointersAndVariants(t *testing.T) {
	u, lk, f := briefFixture()
	u.Situation = "a <b>"
	u.Concept = "see javascript:void(0)"
	u.NewWords = append([]string(nil), u.NewWords...)
	u.NewWords[2] = u.NewWords[2] + ">"
	u.Lessons[1].Kind = "lesson<"
	u.Lessons[1].Activities[0].Type = "recall>"
	u.Lessons[2].Activities[0].Text = u.Lessons[2].Activities[0].Text + "<img>"
	var got []string
	for _, is := range validateBrief(&u, lk, &f) {
		if is.Rule == RuleUnitNoMarkup {
			got = append(got, is.Pointer)
		}
	}
	want := []string{"/concept", "/lessons/1/activities/0/type", "/lessons/1/kind", "/lessons/2/activities/0/text", "/newWords/2", "/situation"}
	if !slices.Equal(got, want) {
		t.Fatalf("pointers\n got %v\nwant %v", got, want)
	}
	for _, s := range []string{"javascript:", "JavaScript:alert(1)", "JAVASCRIPT:", "java\tscript:", "java\u200bscript :", "\uFF4A\uFF41\uFF56\uFF41\uFF53\uFF43\uFF52\uFF49\uFF50\uFF54:", "\uFF1Cb\uFF1E", "a>b", "a<b"} {
		if !hasMarkup(s) {
			t.Errorf("%q should be markup", s)
		}
	}
	for _, s := range []string{"", "Cafe morning", "javascript", "java script", "Ordering coffee & tea", "\u05E9\u05B8\u05C1\u05DC\u05D5\u05B9\u05DD"} {
		if hasMarkup(s) {
			t.Errorf("%q should be plain", s)
		}
	}
}

func TestBriefCoverageIsStaged(t *testing.T) {
	u, lk, f := briefFixture()
	// 20 tokens: 12 words of the brief and 8 the learner does not know. 60.0%.
	text := repeatJoin(vocab8[0].text, 12) + " " + repeatJoin(naal, 8)
	li, ai := addAct(&u, ActivityListenPick, text)
	lk = goldenLookup{Lexemes: lk.lexMap(), Audio: audiosOf(u)}.fake()
	f.Tier, f.MinKnownShare = "beginner", 0.60
	if got := validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("60%% at the beginner tier: %v", ruleSet(got))
	}
	f.Tier, f.MinKnownShare = "developing", 0.90
	got := validateBrief(&u, lk, &f)
	if len(got) != 1 || got[0].Rule != RuleBriefCoverage || got[0].Pointer != at("lessons", li, "activities", ai) {
		t.Fatalf("60%% at the developing tier: %v", ruleSet(got))
	}
	for _, sub := range []string{"60.0%", "12/20", "developing", "90.0%"} {
		if !strings.Contains(got[0].Msg, sub) {
			t.Fatalf("message %q lacks %q", got[0].Msg, sub)
		}
	}
	// Zero and negative figures fail closed at 0.95.
	for _, share := range []float64{0, -1} {
		f.MinKnownShare = share
		if got := validateBrief(&u, lk, &f); len(got) != 1 || got[0].Rule != RuleBriefCoverage || !strings.Contains(got[0].Msg, "95.0%") {
			t.Fatalf("share %v: %v", share, ruleSet(got))
		}
	}
	// Exactly at the line passes: 19 of 20 at 0.95, 18 of 20 at 0.90.
	f.Tier, f.MinKnownShare = "established", 0.95
	li2, ai2 := addAct(&u, ActivityListenPick, repeatJoin(vocab8[0].text, 19)+" "+naal)
	lk = goldenLookup{Lexemes: lk.lexMap(), Audio: audiosOf(u)}.fake()
	for _, is := range validateBrief(&u, lk, &f) {
		if is.Pointer == at("lessons", li2, "activities", ai2) {
			t.Fatalf("19 of 20 at 0.95 failed: %v", is)
		}
	}
}

func (f fakeLookup) lexMap() map[string]glex { return f.lex }

func TestBriefCoverageCountsKnownUnknownAndLeftOut(t *testing.T) {
	u, lk, f := briefFixture()
	lex := lk.lexMap()
	const (
		bayit = "\u05D1\u05B7\u05BC\u05D9\u05B4\u05EA"
		dani  = "\u05D3\u05B8\u05BC\u05E0\u05B4\u05D9"
		al    = "\u05E2\u05B7\u05DC"
	)
	lex[normalize(bayit)] = glex{ID: 60, POS: "noun", Gloss: "house"}
	lex[normalize(dani)] = glex{ID: 61, POS: "name", Gloss: "Dani"}
	lex[normalize(al)] = glex{ID: 62, POS: "preposition", Gloss: "on"}
	// 10 tokens that count: 5 brief words, 3 known words (bayit), 2 unmatched.
	// Dani (a name), al (a function word), and 7 (a numeral) are left out.
	f.KnownLexemeIDs[60] = true
	text := strings.Join([]string{
		vocab8[0].text, vocab8[1].text, vocab8[2].text, vocab8[3].text, vocab8[4].text,
		bayit, bayit, bayit,
		"\u05D0\u05B2\u05DC\u05B8\u05E4\u05B4\u05D9\u05DD", "\u05D1\u05B0\u05BC\u05DC\u05B4\u05D9",
		dani, al, "7",
	}, " ")
	li, ai := addAct(&u, ActivityStory, text)
	lk = goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	f.MinKnownShare = 0.80
	got := validateBrief(&u, lk, &f)
	var cov []Issue
	for _, is := range got {
		if is.Rule == RuleBriefCoverage {
			cov = append(cov, is)
		}
	}
	if len(cov) != 0 {
		t.Fatalf("8 of 10 at 0.80 should pass: %v", ruleSet(got))
	}
	f.MinKnownShare = 0.81
	got = validateBrief(&u, lk, &f)
	var found bool
	for _, is := range got {
		if is.Rule == RuleBriefCoverage {
			found = true
			if is.Pointer != at("lessons", li, "activities", ai) || !strings.Contains(is.Msg, "8/10") {
				t.Fatalf("coverage issue %+v", is)
			}
		}
	}
	if !found {
		t.Fatalf("8 of 10 at 0.81 should fail: %v", ruleSet(got))
	}
}

func TestBriefV2CoverageIsReplaced(t *testing.T) {
	u, lk, f := briefFixture()
	// A story of words the learner knows but no shared unit introduced.
	lex := lk.lexMap()
	const other = "\u05D1\u05B7\u05BC\u05D9\u05B4\u05EA"
	lex[normalize(other)] = glex{ID: 70, POS: "noun", Gender: "m", Gloss: "house"}
	f.KnownLexemeIDs[70] = true
	addAct(&u, ActivityListenPick, repeatJoin(other, 4))
	lk = goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	v2 := Validate(context.Background(), &u, lk)
	if !hasRule(v2, RuleCoverageListening) {
		t.Fatalf("test setup: v2 should flag the passage: %v", ruleSet(v2))
	}
	if got := validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("brief rules should replace v2 coverage: %v", ruleSet(got))
	}
}

func TestBriefTextTooLongBoundary(t *testing.T) {
	u, lk, f := briefFixture()
	f.MaxTextTokens = 20
	addAct(&u, ActivityStory, repeatJoin(vocab8[0].text, 20))
	lk = goldenLookup{Lexemes: lk.lexMap(), Audio: audiosOf(u)}.fake()
	if got := validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("20 tokens at a limit of 20: %v", ruleSet(got))
	}
	f.MaxTextTokens = 0
	addAct(&u, ActivityStory, repeatJoin(vocab8[0].text, 300))
	lk = goldenLookup{Lexemes: lk.lexMap(), Audio: audiosOf(u)}.fake()
	if got := validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("no limit: %v", ruleSet(got))
	}
}

func TestBriefNewWordRules(t *testing.T) {
	u, lk, f := briefFixture()
	// A brief word the unit never uses: min uses at /newWords, and no intro issue.
	const ghost = "\u05E1\u05B5\u05E4\u05B6\u05E8"
	lex := lk.lexMap()
	lex[normalize(ghost)] = glex{ID: 80, POS: "noun", Gender: "m", Gloss: "book"}
	lk = goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	f.NewWords = append(f.NewWords, BriefNewWord{LexemeID: 80, Text: ghost})
	f.BriefLexemeIDs[80] = true
	got := validateBrief(&u, lk, &f)
	if len(got) != 1 || got[0].Rule != RuleBriefNewMinUses || got[0].Pointer != "/newWords" || !strings.Contains(got[0].Msg, "0 times") {
		t.Fatalf("unused brief word: %v", ruleSet(got))
	}
	// MinUses below 3 is read as 3.
	f.NewWords[len(f.NewWords)-1].MinUses = 1
	addAct(&u, ActivityStory, ghost)
	addAct(&u, ActivityStory, ghost)
	lk = goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	got = validateBrief(&u, lk, &f)
	if len(got) != 1 || got[0].Rule != RuleBriefNewMinUses || !strings.Contains(got[0].Msg, "at least 3") {
		t.Fatalf("default min uses: %v", ruleSet(got))
	}
	addAct(&u, ActivityStory, ghost)
	lk = goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	if got = validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("3 exposures: %v", ruleSet(got))
	}
	// A word used only in checked steps has no exposure at all.
	g, lk2, f2 := briefFixture()
	for li := range g.Lessons {
		var keep []Activity
		for _, a := range g.Lessons[li].Activities {
			if a.Type != ActivityShadow {
				keep = append(keep, a)
			}
		}
		g.Lessons[li].Activities = keep
	}
	lk2 = goldenLookup{Lexemes: lk2.lexMap(), Audio: audiosOf(g)}.fake()
	n := 0
	for _, is := range validateBrief(&g, lk2, &f2) {
		if is.Rule == RuleBriefIntro {
			n++
		}
	}
	if n != 8 {
		t.Fatalf("intro issues = %d, want 8 (the story is later than the recall in each lesson)", n)
	}
}

func TestBriefStruggleSkill(t *testing.T) {
	u, lk, f := briefFixture()
	f.StruggleWords[0].Skill = SkillRecall
	got := validateBrief(&u, lk, &f)
	if len(got) != 1 || got[0].Rule != RuleBriefStruggle || !strings.Contains(got[0].Msg, "recall") {
		t.Fatalf("wrong skill: %v", ruleSet(got))
	}
	addAct(&u, ActivityRecall, kos)
	lk = goldenLookup{Lexemes: lk.lexMap(), Audio: audiosOf(u)}.fake()
	if got = validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("one recall use: %v", ruleSet(got))
	}
	// Both failures are one issue that says both.
	f.StruggleWords[0].RequiredCount = 9
	f.StruggleWords[0].Skill = SkillSpell
	got = validateBrief(&u, lk, &f)
	if len(got) != 1 || !strings.Contains(got[0].Msg, "requires 9") || !strings.Contains(got[0].Msg, "spell") {
		t.Fatalf("both: %v %v", ruleSet(got), got)
	}
	// An empty skill asks for the count only. A count below 3 is read as 3.
	f.StruggleWords[0].Skill = ""
	f.StruggleWords[0].RequiredCount = 0
	if got = validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("empty skill: %v", ruleSet(got))
	}
	for typ, want := range map[string]string{
		ActivityListenPick: SkillListen, ActivityShadow: SkillListen, "audio_match": SkillListen,
		ActivityStory: SkillRead, ActivityDialogue: SkillRead, ActivityCloze: SkillRead, ActivityPattern: SkillRead, "read_pick": SkillRead, "word_match": SkillRead,
		ActivityRecall: SkillRecall, "type_it": SkillRecall, ActivityProduce: SkillSpell, "dictation": SkillSpell, "other": "",
	} {
		if SkillOfActivity(typ) != want {
			t.Errorf("SkillOfActivity(%s) = %q, want %q", typ, SkillOfActivity(typ), want)
		}
	}
}

func TestActivitiesForSkillMatchTheValidator(t *testing.T) {
	known := []string{
		ActivityListenPick, ActivityRecall, ActivityProduce, ActivityCloze,
		ActivityPattern, ActivityStory, ActivityShadow, ActivityDialogue,
	}
	seen := map[string]bool{}
	for _, skill := range []string{SkillListen, SkillRead, SkillRecall, SkillSpell} {
		got := ActivitiesForSkill(skill)
		if len(got) == 0 {
			t.Errorf("%s has no activities", skill)
		}
		for _, typ := range got {
			if !knownActivity(typ) {
				t.Errorf("ActivitiesForSkill(%s) emits %q, which validate rejects", skill, typ)
			}
			if SkillOfActivity(typ) != skill {
				t.Errorf("ActivitiesForSkill(%s) emits %q, skill %q", skill, typ, SkillOfActivity(typ))
			}
			seen[typ] = true
		}
	}
	for _, typ := range known {
		if !knownActivity(typ) {
			t.Errorf("known list drifted: %s", typ)
		}
		if SkillOfActivity(typ) != "" && !seen[typ] {
			t.Errorf("known type %s is not suggested for its skill", typ)
		}
	}
	for _, alias := range []string{"read_pick", "word_match", "audio_match", "type_it", "dictation"} {
		if knownActivity(alias) {
			t.Errorf("%s was added to the validator", alias)
		}
		if seen[alias] {
			t.Errorf("a brief would suggest %s", alias)
		}
	}
	if !slices.Equal(ActivitiesForSkill(SkillListen), []string{ActivityListenPick, ActivityShadow}) {
		t.Errorf("listen = %v", ActivitiesForSkill(SkillListen))
	}
	if !slices.Equal(ActivitiesForSkill(SkillRead), []string{ActivityStory, ActivityDialogue, ActivityCloze, ActivityPattern}) {
		t.Errorf("read = %v", ActivitiesForSkill(SkillRead))
	}
	if !slices.Equal(ActivitiesForSkill(SkillRecall), []string{ActivityRecall}) || !slices.Equal(ActivitiesForSkill(SkillSpell), []string{ActivityProduce}) {
		t.Errorf("recall %v spell %v", ActivitiesForSkill(SkillRecall), ActivitiesForSkill(SkillSpell))
	}
	if ActivitiesForSkill("") != nil || ActivitiesForSkill("flashcard") != nil {
		t.Error("an unknown skill suggested activities")
	}
}

func TestBriefGlossRule(t *testing.T) {
	u, lk, f := briefFixture()
	lex := lk.lexMap()
	e := lex[normalize(naal)]
	e.Gloss = "  "
	lex[normalize(naal)] = e
	li, ai := addAct(&u, ActivityRecall, naal+" "+naal)
	lk = goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	got := validateBrief(&u, lk, &f)
	if len(got) != 1 || got[0].Rule != RuleBriefGloss || got[0].Pointer != at("lessons", li, "activities", ai, "text") {
		t.Fatalf("blank gloss: %v", ruleSet(got))
	}
	// Known words need no gloss, and neither do words of the brief.
	f.KnownLexemeIDs[50] = true
	if got = validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("known word without a gloss: %v", ruleSet(got))
	}
	delete(f.KnownLexemeIDs, 50)
	f.BriefLexemeIDs[50] = true
	if got = validateBrief(&u, lk, &f); len(got) != 0 {
		t.Fatalf("brief word without a gloss: %v", ruleSet(got))
	}
}

func TestBriefAudioAllCoversNewWordsOnce(t *testing.T) {
	u, lk, f := briefFixture()
	// Drop the clip of a new word that is only ever spoken inside phrases and
	// recall steps: remove its recall steps' clip by removing the audio entry.
	audio := audiosOf(u)
	word := normalize(vocab8[0].text)
	var rest []string
	for _, a := range audio {
		if a != word {
			rest = append(rest, a)
		}
	}
	lk = goldenLookup{Lexemes: lk.lexMap(), Audio: rest}.fake()
	got := validateBrief(&u, lk, &f)
	var all, v2 int
	for _, is := range got {
		switch is.Rule {
		case RuleBriefAudioAll:
			all++
			if is.Pointer != "/newWords/0" {
				t.Fatalf("pointer %s", is.Pointer)
			}
		case RuleAudioMissing:
			v2++
		}
	}
	if all != 1 || v2 == 0 {
		t.Fatalf("brief.audio_all %d, audio.missing %d: %v", all, v2, ruleSet(got))
	}
	// Activity text is audio.missing's alone: no brief.audio_all at an activity pointer.
	for _, is := range got {
		if is.Rule == RuleBriefAudioAll && strings.HasPrefix(is.Pointer, "/lessons") {
			t.Fatalf("activity text reported twice: %v", is)
		}
	}
}

func TestBriefTokenMatching(t *testing.T) {
	u, lk, f := briefFixture()
	lex := lk.lexMap()
	const (
		vav     = "\u05D5\u05B0"
		heA     = "\u05D4\u05B7"
		lechem  = "\u05DC\u05B6\u05D7\u05B6\u05DD"
		lechemD = "\u05DC\u05BC\u05B6\u05D7\u05B6\u05DD" // lamed with dagesh
	)
	// vav + shalom, and the article + lechem with the dagesh the lexicon omits.
	text := vav + vocab8[0].text + " " + heA + lechemD
	li, ai := addAct(&u, ActivityStory, text)
	lk = goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	b := &briefScan{scan: &scan{ctx: context.Background(), lk: lk, cache: map[string]*probe{}}, ids: map[string]*idProbe{}, refs: map[string]lexRef{}}
	if r := b.resolve(vav + vocab8[0].text); !r.ok || r.id != 1 {
		t.Fatalf("vav prefix: %+v", r)
	}
	if r := b.resolve(heA + lechemD); !r.ok || r.id != 5 {
		t.Fatalf("article and dagesh: %+v", r)
	}
	if r := b.resolve(vocab8[0].text); !r.ok || r.id != 1 {
		t.Fatalf("exact: %+v", r)
	}
	// Two prefixes after vav or shin.
	if r := b.resolve("\u05D5\u05B0" + heA + lechemD); !r.ok || r.id != 5 {
		t.Fatalf("vav and article: %+v", r)
	}
	// A lone letter left over is not a stem, and a miss is unknown.
	if r := b.resolve("\u05D4\u05DD"); r.ok {
		t.Fatalf("one letter after a prefix matched: %+v", r)
	}
	if r := b.resolve("\u05D0\u05D1\u05D2\u05D3"); r.ok {
		t.Fatalf("unknown word matched: %+v", r)
	}
	// In the unit: both tokens are words of the brief, so the passage is fully known.
	for _, is := range validateBrief(&u, lk, &f) {
		if is.Pointer == at("lessons", li, "activities", ai) {
			t.Fatalf("prefixed words counted unknown: %v", is)
		}
	}
	// An unmatched token counts as unknown.
	text = repeatJoin(vocab8[0].text, 9) + " \u05D0\u05D1\u05D2\u05D3"
	li, ai = addAct(&u, ActivityStory, text)
	lk = goldenLookup{Lexemes: lex, Audio: audiosOf(u)}.fake()
	got := validateBrief(&u, lk, &f)
	var cov bool
	for _, is := range got {
		if is.Rule == RuleBriefCoverage && is.Pointer == at("lessons", li, "activities", ai) && strings.Contains(is.Msg, "9/10") {
			cov = true
		}
	}
	if !cov {
		t.Fatalf("unmatched token should count unknown: %v", ruleSet(got))
	}
}

func TestBriefCoverageCountsDeclaredNewWords(t *testing.T) {
	word := vocab8[0].text
	other := naal
	u := Unit{
		Title: "Cafe", Situation: "Ordering at a cafe", Concept: "A greeting",
		NewWords: []string{word},
		Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			{Type: ActivityShadow, Text: word},
			{Type: ActivityListenPick, Text: word},
		}}},
	}
	lex := map[string]glex{
		normalize(word): {ID: 1, POS: "noun", Gender: "m", Gloss: "hello", Bare: stripNiqqud(word)},
	}
	lk := goldenLookup{Lexemes: lex, Audio: []string{normalize(word)}}.fake()
	f := BriefFacts{
		Tier: "established", MinKnownShare: 0.95,
		BriefLexemeIDs: map[int64]bool{}, KnownLexemeIDs: map[int64]bool{},
	}
	if got := validateBrief(&u, lk, &f); hasRule(got, RuleBriefCoverage) {
		t.Fatalf("a unit newWord with an exact lexicon entry should cover a one-word listen_pick: %v", ruleSet(got))
	}

	// The same word stays new for the brief's intro rule.
	f.NewWords = []BriefNewWord{{LexemeID: 1, Text: word, MinUses: 1}}
	f.BriefLexemeIDs[1] = true
	introduced := u
	introduced.Lessons = []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityListenPick, Text: word},
	}}}
	if got := validateBrief(&introduced, lk, &f); !hasRule(got, RuleBriefIntro) {
		t.Fatalf("a newWord is still new for the intro: %v", ruleSet(got))
	}
	if hasRule(validateBrief(&introduced, lk, &f), RuleBriefCoverage) {
		t.Fatal("coverage should still pass once the word is declared")
	}

	// A word in none of the three sets fails coverage.
	f = BriefFacts{
		Tier: "established", MinKnownShare: 0.95,
		BriefLexemeIDs: map[int64]bool{}, KnownLexemeIDs: map[int64]bool{},
	}
	lex[normalize(other)] = glex{ID: 2, POS: "noun", Gender: "f", Gloss: "shoe", Bare: stripNiqqud(other)}
	outsider := u
	outsider.Lessons = []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityShadow, Text: word},
		{Type: ActivityListenPick, Text: word},
		{Type: ActivityListenPick, Text: other},
	}}}
	lk = goldenLookup{Lexemes: lex, Audio: []string{normalize(word), normalize(other)}}.fake()
	got := validateBrief(&outsider, lk, &f)
	var cov []Issue
	for _, is := range got {
		if is.Rule == RuleBriefCoverage {
			cov = append(cov, is)
		}
	}
	if len(cov) != 1 || cov[0].Pointer != at("lessons", 0, "activities", 2) || cov[0].Severity != SeverityBlock {
		t.Fatalf("outsider coverage: %v", ruleSet(got))
	}

	// A newWord whose only lexicon row is the unpointed spelling does not count.
	const gar = "גָּר"
	bare := stripNiqqud(gar)
	if bare == gar {
		t.Fatal("bare form")
	}
	pointed := Unit{
		Title: "Home", Situation: "Saying where you live", Concept: "I live here",
		NewWords: []string{gar},
		Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			{Type: ActivityShadow, Text: gar},
			{Type: ActivityListenPick, Text: gar},
		}}},
	}
	bareLex := map[string]glex{
		normalize(bare): {ID: 3, POS: "verb", Gloss: "lives", Bare: bare},
	}
	blk := goldenLookup{Lexemes: bareLex, Audio: []string{normalize(gar)}}.fake()
	bf := BriefFacts{
		Tier: "established", MinKnownShare: 0.95,
		BriefLexemeIDs: map[int64]bool{}, KnownLexemeIDs: map[int64]bool{},
	}
	probe := &scan{ctx: context.Background(), lk: blk, cache: map[string]*probe{}}
	if !probe.inLexicon(gar) || probe.probeLex(normalize(gar)) {
		t.Fatal("fixture: the unpointed row should satisfy inLexicon and miss the exact key")
	}
	got = validateBrief(&pointed, blk, &bf)
	if !hasRule(got, RuleBriefCoverage) {
		t.Fatalf("unpointed newWord should fail coverage: %v", ruleSet(got))
	}
	for _, is := range got {
		if is.Rule == RuleBriefCoverage && (is.Pointer != at("lessons", 0, "activities", 1) || is.Severity != SeverityBlock) {
			t.Fatalf("unpointed coverage issue %+v", is)
		}
	}
}

func TestDropFirstDageshAndCutPrefix(t *testing.T) {
	if got := dropFirstDagesh("\u05DC\u05BC\u05B6\u05D7\u05B6\u05BC\u05DD"); got != "\u05DC\u05B6\u05D7\u05B6\u05BC\u05DD" {
		t.Fatalf("dropFirstDagesh = %+q", got)
	}
	if got := dropFirstDagesh("abc"); got != "abc" {
		t.Fatalf("no letter: %q", got)
	}
	rest, letter, ok := cutPrefix("\u05D4\u05B7\u05DC\u05B6\u05D7\u05B6\u05DD")
	if !ok || letter != '\u05D4' || rest != "\u05DC\u05B6\u05D7\u05B6\u05DD" {
		t.Fatalf("cutPrefix = %+q %q %v", rest, letter, ok)
	}
	if _, _, ok := cutPrefix("\u05D0\u05D1"); ok {
		t.Fatal("alef is not a prefix letter")
	}
}
