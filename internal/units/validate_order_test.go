package units

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// INIT-029/SPEC-002: order activity schema and validator rules.

func orderLine(words ...string) string {
	return strings.Join(words, " ")
}

func setOrderAct(u *Unit, li, ai int, text, translation string, accept []string) {
	u.Lessons[li].Activities[ai].Type = ActivityOrder
	u.Lessons[li].Activities[ai].Text = text
	u.Lessons[li].Activities[ai].Translation = translation
	u.Lessons[li].Activities[ai].Accept = accept
	u.Lessons[li].Activities[ai].Target = ""
	u.Lessons[li].Activities[ai].PromptEN = ""
	u.Lessons[li].Activities[ai].Lines = nil
}

func TestOrderActivityValidPasses(t *testing.T) {
	w0, w1, w2 := vocab8[0].text, vocab8[1].text, vocab8[2].text
	line := orderLine(w0, w1, w2)
	alt := orderLine(w1, w0, w2)

	u, lk := v1()
	li, ai := addAct(&u, ActivityOrder, line)
	setOrderAct(&u, li, ai, line, "hello morning thanks", []string{alt})
	lk.audio[normalize(line)] = true
	if got := Validate(context.Background(), &u, lk); len(got) != 0 {
		t.Fatalf("Validate: %#v", got)
	}

	u2, lk2, f := briefFixture()
	li2, ai2 := addAct(&u2, ActivityOrder, line)
	setOrderAct(&u2, li2, ai2, line, "hello morning thanks", []string{alt})
	lk2 = goldenLookup{Lexemes: lk2.lexMap(), Audio: audiosOf(u2)}.fake()
	if got := ValidateWithBrief(context.Background(), &u2, lk2, &f); len(got) != 0 {
		t.Fatalf("ValidateWithBrief: %v", ruleSet(got))
	}
}

func TestOrderRulesPassAndFail(t *testing.T) {
	w0, w1, w2, w3 := vocab8[0].text, vocab8[1].text, vocab8[2].text, vocab8[3].text
	good := orderLine(w0, w1, w2)
	alt := orderLine(w2, w1, w0)

	type check struct {
		name   string
		rule   string
		pass   bool
		mutate func(u *Unit, li, ai int)
	}
	cases := []check{
		{
			name: "order_tokens pass",
			rule: RuleUnitOrderTokens,
			pass: true,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "ok", nil)
			},
		},
		{
			name: "order_tokens fail too few",
			rule: RuleUnitOrderTokens,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, orderLine(w0, w1), "ok", nil)
			},
		},
		{
			name: "order_tokens fail too many",
			rule: RuleUnitOrderTokens,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, orderLine(w0, w1, w2, w3, w0, w1, w2, w3, w0), "ok", nil)
			},
		},
		{
			name: "order_translation pass",
			rule: RuleUnitOrderTranslation,
			pass: true,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "a short English cue", nil)
			},
		},
		{
			name: "order_translation fail missing",
			rule: RuleUnitOrderTranslation,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "", nil)
			},
		},
		{
			name: "order_translation fail Hebrew",
			rule: RuleUnitOrderTranslation,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "hello "+w0, nil)
			},
		},
		{
			name: "order_translation fail too long",
			rule: RuleUnitOrderTranslation,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, strings.Repeat("a", TranslationMaxLen+1), nil)
			},
		},
		{
			name: "order_accept pass",
			rule: RuleUnitOrderAccept,
			pass: true,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "ok", []string{alt})
			},
		},
		{
			name: "order_accept fail duplicate of text",
			rule: RuleUnitOrderAccept,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "ok", []string{good})
			},
		},
		{
			name: "order_accept fail adds token",
			rule: RuleUnitOrderAccept,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "ok", []string{orderLine(w0, w1, w2, w3)})
			},
		},
		{
			name: "order_accept fail drops token",
			rule: RuleUnitOrderAccept,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "ok", []string{orderLine(w0, w1)})
			},
		},
		{
			name: "order_accept fail repeats token",
			rule: RuleUnitOrderAccept,
			pass: false,
			mutate: func(u *Unit, li, ai int) {
				setOrderAct(u, li, ai, good, "ok", []string{orderLine(w0, w0, w1)})
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, lk := v1()
			li, ai := addAct(&u, ActivityOrder, good)
			c.mutate(&u, li, ai)
			text := u.Lessons[li].Activities[ai].Text
			lk.audio[normalize(text)] = true
			got := Validate(context.Background(), &u, lk)
			has := hasRule(got, c.rule)
			if c.pass && has {
				t.Fatalf("unexpected %s: %#v", c.rule, got)
			}
			if !c.pass && !has {
				t.Fatalf("missing %s: %#v", c.rule, got)
			}
		})
	}
}

func TestOrderKnownPassAndFail(t *testing.T) {
	w0, w1, w2 := vocab8[0].text, vocab8[1].text, vocab8[2].text
	line := orderLine(w0, w1, w2)

	t.Run("order_known pass after intro", func(t *testing.T) {
		u, lk, f := briefFixture()
		li, ai := addAct(&u, ActivityOrder, line)
		setOrderAct(&u, li, ai, line, "hello morning thanks", nil)
		lk = goldenLookup{Lexemes: lk.lexMap(), Audio: audiosOf(u)}.fake()
		got := ValidateWithBrief(context.Background(), &u, lk, &f)
		if hasRule(got, RuleBriefOrderKnown) {
			t.Fatalf("unexpected brief.order_known: %v", ruleSet(got))
		}
	})

	t.Run("order_known fail before intro", func(t *testing.T) {
		u, lk, f := briefFixture()
		// Prepend order before the lesson-0 shadows so the words are not yet introduced.
		act := Activity{Type: ActivityOrder, Text: line, Translation: "hello morning thanks"}
		u.Lessons[0].Activities = append([]Activity{act}, u.Lessons[0].Activities...)
		lk = goldenLookup{Lexemes: lk.lexMap(), Audio: audiosOf(u)}.fake()
		got := ValidateWithBrief(context.Background(), &u, lk, &f)
		if !hasRule(got, RuleBriefOrderKnown) {
			t.Fatalf("missing brief.order_known: %v", ruleSet(got))
		}
	})

	t.Run("order_known pass when known", func(t *testing.T) {
		u, lk, f := briefFixture()
		// Known words need no earlier intro.
		f.KnownLexemeIDs[1] = true
		f.KnownLexemeIDs[2] = true
		f.KnownLexemeIDs[3] = true
		act := Activity{Type: ActivityOrder, Text: line, Translation: "hello morning thanks"}
		u.Lessons[0].Activities = append([]Activity{act}, u.Lessons[0].Activities...)
		lk = goldenLookup{Lexemes: lk.lexMap(), Audio: audiosOf(u)}.fake()
		got := ValidateWithBrief(context.Background(), &u, lk, &f)
		if hasRule(got, RuleBriefOrderKnown) {
			t.Fatalf("unexpected brief.order_known for known words: %v", ruleSet(got))
		}
	})
}

func TestOrderRulesAreCatalogedAndDocumented(t *testing.T) {
	cat := map[string]Rule{}
	var order []string
	for _, r := range Rules {
		cat[r.ID] = r
		order = append(order, r.ID)
	}
	if !slices.IsSorted(order) {
		t.Fatalf("catalog is not sorted: %v", order)
	}
	for _, id := range []string{RuleUnitOrderTokens, RuleUnitOrderTranslation, RuleUnitOrderAccept, RuleBriefOrderKnown} {
		r, ok := cat[id]
		if !ok || r.Severity != SeverityBlock || r.Description == "" {
			t.Fatalf("catalog row for %s: %+v", id, r)
		}
	}
	if !knownActivity(ActivityOrder) {
		t.Fatal("order is not a known activity type")
	}
}
