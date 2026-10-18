package units

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestTextNormalizeAndTokens(t *testing.T) {
	pointed := "שָׁלוֹם"
	if got := normalize("  " + norm.NFD.String(pointed) + "   " + pointed + "  "); got != pointed+" "+pointed {
		t.Fatalf("normalize = %q", got)
	}
	toks := tokenize("שָׁלוֹם־בֹּקֶר, 12!")
	if strings.Join(toks, "|") != "שָׁלוֹם|בֹּקֶר|12" {
		t.Fatalf("tokens = %#v", toks)
	}
	if isPointed("שׁלום") || isPointed("דוד") {
		t.Fatal("shin dot or bare letters counted as niqqud")
	}
	if !isPointed("שָׁלוֹם") || !isPointed("בּ") {
		t.Fatal("vowel or dagesh should count as niqqud")
	}
	for _, tok := range []string{"12", "ה׳", "י״ב", "ט״ו", "ט״ז", "תשפ״ז", "תתקע״ה"} {
		if !isNumberTok(tok) {
			t.Fatalf("%q should be a numeral", tok)
		}
	}
	for _, tok := range []string{"שָׁלוֹם", "צ׳יפס", "ג׳ירפה", "צה״ל", "ארה״ב", "ביה״ס", "א״א", "י״ה", "י״ו", "ך׳", "י\"ב", "ה'"} {
		if isNumberTok(tok) {
			t.Fatalf("%q should be a word, not a numeral", tok)
		}
	}
	if stripNiqqud("בֹּקֶר") != stripNiqqud("בָּקָר") {
		t.Fatalf("homograph skeletons differ: %q vs %q", stripNiqqud("בֹּקֶר"), stripNiqqud("בָּקָר"))
	}
	if stripNiqqud("שׁ") == stripNiqqud("שׂ") {
		t.Fatal("shin and sin dots must stay on the skeleton")
	}
}

func TestNFDAndSpacesAreNotUnknown(t *testing.T) {
	u, lk := v1()
	u.Lessons[0].Activities[0].Text = "  " + norm.NFD.String(vocab8[0].text) + "   "
	if got := Validate(context.Background(), &u, lk); len(got) != 0 {
		t.Fatalf("NFD and spaces produced %#v", got)
	}
}

func TestWrongJSONType(t *testing.T) {
	var u Unit
	err := json.Unmarshal([]byte(`{"title":1,"newWords":"שָׁלוֹם","lessons":{}}`), &u)
	if err == nil {
		t.Fatal("wrong JSON types should fail before Validate")
	}
}

func TestNilAndLookupFailure(t *testing.T) {
	if got := Validate(context.Background(), nil, fakeLookup{}); len(got) != 1 || got[0].Rule != RuleSchemaRequired || got[0].Pointer != "" {
		t.Fatalf("nil doc = %#v", got)
	}
	u, _ := v1()
	if got := Validate(context.Background(), &u, nil); len(got) != 1 || got[0].Rule != RuleSchemaLookup {
		t.Fatalf("nil lookup = %#v", got)
	}
	_, lk := v1()
	lk.err = errors.New("boom")
	got := Validate(context.Background(), &u, lk)
	if len(got) != 1 || got[0].Rule != RuleSchemaLookup || !strings.Contains(got[0].Msg, "boom") {
		t.Fatalf("lookup error = %#v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, lk = v1()
	got = Validate(ctx, &u, lk)
	if len(got) != 1 || got[0].Rule != RuleSchemaLookup {
		t.Fatalf("canceled = %#v", got)
	}
}

func TestValidateDeterministic(t *testing.T) {
	u, lk := v1()
	u.Title = ""
	a := Validate(context.Background(), &u, lk)
	b := Validate(context.Background(), &u, lk)
	if len(a) == 0 || !sameIssues(a, stripMsg(b)) {
		t.Fatalf("not deterministic\n%#v\n%#v", a, b)
	}
}

func TestGenderBothAndPluralSignals(t *testing.T) {
	u, lk := v1()
	e := lk.lex[normalize(vocab8[0].text)]
	e.Gender = "BOTH"
	lk.lex[normalize(vocab8[0].text)] = e
	if got := Validate(context.Background(), &u, lk); len(got) != 0 {
		t.Fatalf("gender both = %#v", got)
	}

	u, lk = v1()
	e = lk.lex[normalize(vocab8[0].text)]
	e.Gender = "neuter"
	lk.lex[normalize(vocab8[0].text)] = e
	got := Validate(context.Background(), &u, lk)
	if len(got) != 1 || got[0].Rule != RuleNounGender || got[0].Severity != SeverityBlock {
		t.Fatalf("bad gender = %#v", got)
	}

	u, lk = v1()
	e = lk.lex[normalize(sus)]
	e.IrregularPlural = true
	lk.lex[normalize(sus)] = e
	got = Validate(context.Background(), &u, lk)
	if len(got) != 1 || got[0].Rule != RuleNounPlural || got[0].Severity != SeverityWarn {
		t.Fatalf("flag without form = %#v", got)
	}

	u, lk = v1()
	e = lk.lex[normalize(sus)]
	e.Plural = susim
	lk.lex[normalize(sus)] = e
	got = Validate(context.Background(), &u, lk)
	if len(got) != 1 || got[0].Rule != RuleNounPlural {
		t.Fatalf("stored plural without flag = %#v", got)
	}
}

func TestDuplicateNewWord(t *testing.T) {
	u, lk := v1()
	u.NewWords = append(u.NewWords, u.NewWords[0])
	got := stripMsg(Validate(context.Background(), &u, lk))
	want := []Issue{
		iss(RuleBudgetUnit, SeverityBlock, "/newWords/8"),
		iss(RuleSchemaRef, SeverityBlock, "/newWords/8"),
	}
	if !sameIssues(got, want) {
		t.Fatalf("duplicate = %s", mustJSON(got))
	}
}

func TestRulesCatalog(t *testing.T) {
	if len(Rules) == 0 {
		t.Fatal("empty catalog")
	}
	seen := map[string]bool{}
	for i, r := range Rules {
		if r.ID == "" || r.Description == "" {
			t.Fatalf("blank rule %#v", r)
		}
		if r.Severity != SeverityBlock && r.Severity != SeverityWarn {
			t.Fatalf("severity %s", r.Severity)
		}
		if seen[r.ID] {
			t.Fatalf("duplicate %s", r.ID)
		}
		seen[r.ID] = true
		if i > 0 && Rules[i-1].ID >= r.ID {
			t.Fatalf("catalog not sorted at %s", r.ID)
		}
	}
	for _, id := range []string{
		RuleAudioMissing, RuleBudgetLesson, RuleBudgetLessonFew, RuleBudgetUnit,
		RuleCoverageListening, RuleCoverageStory, RuleLexemeUnknown,
		RuleNounGender, RuleNounPlural, RuleRecycleCount, RuleRecycleSpread,
		RuleSchemaActivity, RuleSchemaCheckpoint, RuleSchemaKind,
		RuleSchemaLookup, RuleSchemaRef, RuleSchemaRequired, RuleStoryUnmapped, RuleStoryUnpointed,
	} {
		if !seen[id] {
			t.Fatalf("catalog missing %s", id)
		}
	}
}

func TestPlayerGuidance(t *testing.T) {
	want := []string{
		"N1", "N2", "N3", "N4", "N5",
		"T1", "T2", "T3", "T4",
		"D1", "D2", "D3", "D4",
		"R1", "R2", "R3", "R4",
		"A1", "A2", "A3", "A4",
		"G1", "G2", "G3", "G4", "G5",
		"C1", "C2", "C3", "C4",
	}
	got := map[string]Guidance{}
	for _, g := range PlayerGuidance {
		if g.ID == "" || g.Statement == "" {
			t.Fatalf("blank guidance %#v", g)
		}
		switch g.Basis {
		case "E", "P", "E/P":
		default:
			t.Fatalf("basis %q on %s", g.Basis, g.ID)
		}
		if _, ok := got[g.ID]; ok {
			t.Fatalf("duplicate guidance %s", g.ID)
		}
		got[g.ID] = g
		for _, rule := range Rules {
			if rule.ID == g.ID {
				t.Fatalf("guidance id %s is also a validator rule", g.ID)
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("guidance len %d", len(got))
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Fatalf("missing guidance %s", id)
		}
	}
	r1 := strings.ToLower(got["R1"].Statement)
	if !strings.Contains(r1, "2 or more") || !strings.Contains(r1, "root") {
		t.Fatalf("R1 = %s", got["R1"].Statement)
	}
	r3 := strings.ToLower(got["R3"].Statement)
	if !strings.Contains(r3, "2 or more") || !strings.Contains(r3, "root") {
		t.Fatalf("R3 = %s", got["R3"].Statement)
	}
	r2 := strings.ToLower(got["R2"].Statement)
	if !strings.Contains(r2, "3 or more") || !strings.Contains(r2, "binyan") {
		t.Fatalf("R2 = %s", got["R2"].Statement)
	}
	c1 := strings.ToLower(got["C1"].Statement)
	if !strings.Contains(c1, "minimal pair") || !strings.Contains(c1, "confusable") {
		t.Fatalf("C1 = %s", got["C1"].Statement)
	}
	if got["D2"].Basis != "P" {
		t.Fatalf("D2 basis %s", got["D2"].Basis)
	}
}

func TestFewNewWordsWarnsAndCheckpointsAreExempt(t *testing.T) {
	u, lk := v1()
	start := len(u.Lessons)
	u.Lessons = append(u.Lessons,
		Lesson{Kind: KindLesson, Activities: []Activity{{Type: ActivityRecall, Text: vocab8[0].text}}},
		Lesson{Kind: KindCheckpoint, Activities: []Activity{{Type: ActivityRecall, Text: vocab8[0].text}}},
	)
	got := Validate(context.Background(), &u, lk)
	want := []Issue{iss(RuleBudgetLessonFew, SeverityWarn, at("lessons", start))}
	if !sameIssues(got, want) {
		t.Fatalf("zero-word lesson = %s", mustJSON(stripMsg(got)))
	}
	if got[0].Severity != SeverityWarn || !strings.Contains(got[0].Msg, "fewer than 2") {
		t.Fatalf("warn message = %#v", got[0])
	}
}

func TestKnownLexemesUsesPosition(t *testing.T) {
	u, lk := v1()
	u.Position = 12
	const sefer = "סֵפֶר"
	lk.lex[normalize(sefer)] = glex{ID: 99, POS: "noun", Gender: "m", Bare: stripNiqqud(sefer)}
	var storyAt int
	for i, act := range u.Lessons[0].Activities {
		if act.Type != ActivityStory {
			continue
		}
		storyAt = i
		text := act.Text + " " + sefer
		u.Lessons[0].Activities[i].Text = text
		lk.audio[normalize(text)] = true
	}
	rec := &posLookup{fakeLookup: lk, flags: map[string]bool{sefer: false}}
	got := Validate(context.Background(), &u, rec)
	if rec.calls != 1 || rec.before != 12 {
		t.Fatalf("KnownLexemes before=%d calls=%d", rec.before, rec.calls)
	}
	want := []Issue{iss(RuleCoverageStory, SeverityBlock, at("lessons", 0, "activities", storyAt))}
	if !sameIssues(got, want) {
		t.Fatalf("false known value = %s", mustJSON(stripMsg(got)))
	}
	rec.flags[sefer] = true
	if got = Validate(context.Background(), &u, rec); len(got) != 0 {
		t.Fatalf("true known value = %s", mustJSON(stripMsg(got)))
	}
	if rec.before != 12 {
		t.Fatalf("position = %d", rec.before)
	}
}

type posLookup struct {
	fakeLookup
	before int
	calls  int
	flags  map[string]bool
}

func (p *posLookup) KnownLexemes(ctx context.Context, before int) (map[string]bool, error) {
	p.calls++
	p.before = before
	out, err := p.fakeLookup.KnownLexemes(ctx, before)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]bool{}
	}
	for text, ok := range p.flags {
		out[text] = ok
	}
	return out, nil
}

func TestGereshLoanwordIsNotANumber(t *testing.T) {
	u, lk := v1()
	text := strings.Join([]string{"צ׳יפס", "ה׳", "תשפ״ז"}, " ")
	li := len(u.Lessons) - 1
	ai := addActAt(&u, li, ActivityStory, text)
	lk.audio[normalize(text)] = true
	got := Validate(context.Background(), &u, lk)
	want := []Issue{
		iss(RuleCoverageStory, SeverityBlock, at("lessons", li, "activities", ai)),
		iss(RuleStoryUnmapped, SeverityBlock, at("lessons", li, "activities", ai, "text")),
		iss(RuleStoryUnpointed, SeverityBlock, at("lessons", li, "activities", ai, "text")),
	}
	if !sameIssues(got, want) {
		t.Fatalf("loanword = %s", mustJSON(stripMsg(got)))
	}
	joined := joinMsgs(got)
	if !strings.Contains(joined, "צ׳יפס") || strings.Contains(joined, "ה׳") || strings.Contains(joined, "תשפ״ז") {
		t.Fatalf("messages = %s", joined)
	}
}

func TestHasAudioUsesNormalizedPointedText(t *testing.T) {
	u, lk := v1()
	raw := "  " + norm.NFD.String(vocab8[0].text) + "   "
	want := normalize(raw)
	if want != vocab8[0].text || stripNiqqud(want) == want {
		t.Fatalf("normalized key = %q", want)
	}
	u.Lessons[0].Activities[0].Text = raw
	rec := &audioLookup{fakeLookup: lk}
	if got := Validate(context.Background(), &u, rec); len(got) != 0 {
		t.Fatalf("issues %s", mustJSON(got))
	}
	found := false
	for _, key := range rec.keys {
		if key == raw || key == stripNiqqud(want) {
			t.Fatalf("HasAudio key %q", key)
		}
		if key == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("keys %q, want %q", rec.keys, want)
	}
}

type audioLookup struct {
	fakeLookup
	keys []string
}

func (a *audioLookup) HasAudio(ctx context.Context, text string) (bool, error) {
	a.keys = append(a.keys, text)
	if a.err != nil {
		return false, a.err
	}
	return a.audio[text], nil
}

func TestGuidanceIsNotEnforced(t *testing.T) {
	u, lk := v1()
	for _, is := range Validate(context.Background(), &u, lk) {
		if len(is.Rule) >= 2 && is.Rule[0] >= 'A' && is.Rule[0] <= 'Z' {
			t.Fatalf("validator emitted guidance id %s", is.Rule)
		}
	}
}
