package mcp

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestSentenceTranslationBlocksNewWrites(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{TextNFC: coffee, Bare: "קפה", Gloss: "coffee", POS: "noun", Gender: "m", AudioPresent: true})
	line := hello + " " + coffee
	mem.SetAudio(units.Normalize(line))
	mem.SetAudio(units.Normalize(coffee))
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	doc := sentenceDoc(line, "", "")
	dry := call[validateOutput](t, session, "validate_unit", validateInput{Position: 0, Doc: doc})
	if dry.OK || !translationBlock(dry.Issues) {
		t.Fatalf("validate_unit %+v", dry)
	}
	saved := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: doc})
	if saved.UnitID == 0 || saved.Published || saved.Revision != 1 || !translationBlock(saved.Issues) {
		t.Fatalf("put_unit %+v", saved)
	}
	if mem.Status(saved.UnitID) != "draft" {
		t.Fatalf("status %s", mem.Status(saved.UnitID))
	}

	one := call[validateOutput](t, session, "validate_unit", validateInput{Position: 0, Doc: helloDoc()})
	if !one.OK || hasRule(one.Issues, units.RuleUnitTranslationMissing) {
		t.Fatalf("single-token listen_pick %+v", one)
	}

	filled := sentenceDoc(line, coffee, "hello, coffee")
	ok := call[validateOutput](t, session, "validate_unit", validateInput{Position: 0, Doc: filled})
	if !ok.OK || hasRule(ok.Issues, units.RuleUnitTranslationMissing) || hasRule(ok.Issues, units.RuleUnitTranslationDistractors) {
		t.Fatalf("translated sentence %+v", ok)
	}

	pub, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: saved.UnitID})
	if !res.IsError || pub.Status != "draft" || !translationBlock(pub.Issues) || mem.Status(saved.UnitID) != "draft" {
		t.Fatalf("new draft publish isError=%v out=%+v status=%s", res.IsError, pub, mem.Status(saved.UnitID))
	}
}

func TestPublishedRevisionKeepsAMissingTranslation(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{TextNFC: coffee, Bare: "קפה", Gloss: "coffee", POS: "noun", Gender: "m", AudioPresent: true})
	line := hello + " " + coffee
	mem.SetAudio(units.Normalize(line))
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	doc := sentenceDoc(line, coffee, "")
	saved := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: doc})
	mem.SetStatus(saved.UnitID, "published")

	got := call[getUnitOutput](t, session, "get_unit", getUnitInput{UnitID: saved.UnitID})
	act := got.Doc.Lessons[0].Activities[0]
	if got.Status != "published" || act.Text != line || act.Target != coffee || act.Translation != "" {
		t.Fatalf("get_unit %+v", act)
	}

	again, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: saved.UnitID})
	if res.IsError || !again.Unchanged || again.Status != "published" || translationBlock(again.Issues) || !translationWarn(again.Issues) {
		t.Fatalf("republish isError=%v out=%+v", res.IsError, again)
	}
	if mem.Status(saved.UnitID) != "published" {
		t.Fatalf("status %s", mem.Status(saved.UnitID))
	}
}

func TestOlderUnpublishedRevisionCannotPublishTranslation(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{TextNFC: coffee, Bare: "קפה", Gloss: "coffee", POS: "noun", Gender: "m", AudioPresent: true})
	line := hello + " " + coffee
	mem.SetAudio(units.Normalize(line))
	mem.SetAudio(units.Normalize(coffee))
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	saved := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: sentenceDoc(line, "", "")})
	if saved.UnitID == 0 || saved.Revision != 1 || saved.Published || !translationBlock(saved.Issues) || mem.Status(saved.UnitID) != "draft" {
		t.Fatalf("rev 1 %+v status %s", saved, mem.Status(saved.UnitID))
	}
	id := saved.UnitID
	next := call[putOutput](t, session, "put_unit", putInput{UnitID: &id, Scope: ScopeShared, Position: 0, Doc: sentenceDoc(line, "", "hello, coffee")})
	if next.UnitID != id || next.Revision != 2 || translationBlock(next.Issues) {
		t.Fatalf("rev 2 %+v", next)
	}
	pub, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: id})
	if res.IsError || pub.Revision != 2 || pub.Status != "published" || mem.Status(id) != "published" {
		t.Fatalf("publish current isError=%v out=%+v status=%s", res.IsError, pub, mem.Status(id))
	}

	one := 1
	old, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: id, Revision: &one})
	if !res.IsError || !translationBlock(old.Issues) || mem.Status(id) != "published" {
		t.Fatalf("older revision isError=%v out=%+v status=%s", res.IsError, old, mem.Status(id))
	}
	got := call[getUnitOutput](t, session, "get_unit", getUnitInput{UnitID: id})
	act := got.Doc.Lessons[0].Activities[0]
	if got.Revision != 2 || got.Status != "published" || act.Translation != "hello, coffee" || act.Text != line {
		t.Fatalf("current published revision %+v activity %+v", got, act)
	}
}

func sentenceDoc(line, target, translation string) units.Unit {
	acts := []units.Activity{{
		Type: units.ActivityListenPick, Text: line, Target: target, Translation: translation,
	}}
	if translation != "" {
		for _, english := range []string{"coffee", "tea", "water"} {
			acts = append(acts, units.Activity{Type: units.ActivityRecall, Text: coffee, Translation: english})
		}
	}
	return units.Unit{
		Title: "Cafe", Situation: "Ordering a drink", Concept: "Greeting",
		NewWords: []string{hello, coffee},
		Lessons:  []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
	}
}

func translationBlock(issues []units.Issue) bool {
	for _, is := range issues {
		if is.Rule == units.RuleUnitTranslationMissing && is.Severity == units.SeverityBlock && strings.Contains(is.Msg, "several words") {
			return true
		}
	}
	return false
}

func translationWarn(issues []units.Issue) bool {
	for _, is := range issues {
		if is.Rule == units.RuleUnitTranslationMissing && is.Severity == units.SeverityWarn && strings.Contains(is.Msg, "several words") {
			return true
		}
	}
	return false
}
