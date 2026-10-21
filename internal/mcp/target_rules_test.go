package mcp

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestGradedTargetBlocksNewWrites(t *testing.T) {
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

	doc := recallLine(line, "")
	dry := call[validateOutput](t, session, "validate_unit", validateInput{Position: 0, Doc: doc})
	if dry.OK || !targetBlock(dry.Issues) {
		t.Fatalf("validate_unit %+v", dry)
	}
	saved := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: doc})
	if saved.UnitID == 0 || saved.Published || saved.Revision != 1 || !targetBlock(saved.Issues) {
		t.Fatalf("put_unit %+v", saved)
	}
	if mem.Status(saved.UnitID) != "draft" {
		t.Fatalf("status %s", mem.Status(saved.UnitID))
	}

	one := call[validateOutput](t, session, "validate_unit", validateInput{Position: 0, Doc: helloDoc()})
	if !one.OK || hasRule(one.Issues, units.RuleActivityTarget) {
		t.Fatalf("single-token recall %+v", one)
	}

	exact := recallLine(line, coffee)
	ok := call[validateOutput](t, session, "validate_unit", validateInput{Position: 0, Doc: exact})
	if !ok.OK || hasRule(ok.Issues, units.RuleActivityTarget) {
		t.Fatalf("exact target %+v", ok)
	}

	pub, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: saved.UnitID})
	if !res.IsError || pub.Status != "draft" || !targetBlock(pub.Issues) || mem.Status(saved.UnitID) != "draft" {
		t.Fatalf("new draft publish isError=%v out=%+v status=%s", res.IsError, pub, mem.Status(saved.UnitID))
	}
}

func TestPublishedRevisionKeepsAMissingTarget(t *testing.T) {
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

	doc := recallLine(line, "")
	saved := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: doc})
	mem.SetStatus(saved.UnitID, "published")

	got := call[getUnitOutput](t, session, "get_unit", getUnitInput{UnitID: saved.UnitID})
	if got.Status != "published" || got.Doc.Lessons[0].Activities[0].Text != line || got.Doc.Lessons[0].Activities[0].Target != "" {
		t.Fatalf("get_unit %+v", got.Doc.Lessons)
	}

	again, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: saved.UnitID})
	if res.IsError || !again.Unchanged || again.Status != "published" || targetBlock(again.Issues) || !targetWarn(again.Issues) {
		t.Fatalf("republish isError=%v out=%+v", res.IsError, again)
	}
	if mem.Status(saved.UnitID) != "published" {
		t.Fatalf("status %s", mem.Status(saved.UnitID))
	}
}

func recallLine(line, target string) units.Unit {
	return units.Unit{
		Title: "Cafe", Situation: "Ordering a drink", Concept: "Greeting",
		NewWords: []string{coffee},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
			Type: units.ActivityRecall, Text: line, Target: target,
		}}}},
	}
}

func targetBlock(issues []units.Issue) bool {
	for _, is := range issues {
		if is.Rule == units.RuleActivityTarget && is.Severity == units.SeverityBlock && strings.Contains(is.Msg, "several words") {
			return true
		}
	}
	return false
}

func TestOlderUnpublishedRevisionCannotPublish(t *testing.T) {
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

	saved := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: recallLine(line, "")})
	if saved.UnitID == 0 || saved.Revision != 1 || saved.Published || !targetBlock(saved.Issues) || mem.Status(saved.UnitID) != "draft" {
		t.Fatalf("rev 1 %+v status %s", saved, mem.Status(saved.UnitID))
	}
	id := saved.UnitID
	next := call[putOutput](t, session, "put_unit", putInput{UnitID: &id, Scope: ScopeShared, Position: 0, Doc: recallLine(line, coffee)})
	if next.UnitID != id || next.Revision != 2 || targetBlock(next.Issues) {
		t.Fatalf("rev 2 %+v", next)
	}
	pub, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: id})
	if res.IsError || pub.Revision != 2 || pub.Status != "published" || mem.Status(id) != "published" {
		t.Fatalf("publish current isError=%v out=%+v status=%s", res.IsError, pub, mem.Status(id))
	}

	one := 1
	old, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: id, Revision: &one})
	if !res.IsError || !targetBlock(old.Issues) || mem.Status(id) != "published" {
		t.Fatalf("older revision isError=%v out=%+v status=%s", res.IsError, old, mem.Status(id))
	}
	got := call[getUnitOutput](t, session, "get_unit", getUnitInput{UnitID: id})
	act := got.Doc.Lessons[0].Activities[0]
	if got.Revision != 2 || got.Status != "published" || act.Target != coffee || act.Text != line {
		t.Fatalf("current published revision %+v activity %+v", got, act)
	}
}

func targetWarn(issues []units.Issue) bool {
	for _, is := range issues {
		if is.Rule == units.RuleActivityTarget && is.Severity == units.SeverityWarn && strings.Contains(is.Msg, "several words") {
			return true
		}
	}
	return false
}
