package mcp

import (
	"strings"
	"testing"
)

// INIT-038/SPEC-007. With the switch off, put_unit on learner:1 saves a draft
// and names ADR-0033, publish_unit refuses with that note, and a shared unit
// still publishes.
func TestLearnerAutoPublishOff(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	learner := call[putOutput](t, session, "put_unit", putInput{
		Scope: "learner:1", Position: 0, Doc: helloDoc(), Publish: true, BriefID: "0123456789012345678901",
	})
	if learner.Published || mem.Status(learner.UnitID) != "draft" || learner.PublishReason != LearnerAutoPublishNote {
		t.Fatalf("put %+v status %s", learner, mem.Status(learner.UnitID))
	}
	if !strings.Contains(learner.PublishReason, "ADR-0033") {
		t.Fatalf("note %q", learner.PublishReason)
	}

	_, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: learner.UnitID})
	if !res.IsError || !strings.Contains(resultText(res), LearnerAutoPublishNote) || mem.Status(learner.UnitID) != "draft" {
		t.Fatalf("publish_unit error %v %q status %s", res.IsError, resultText(res), mem.Status(learner.UnitID))
	}

	shared := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 1, Doc: helloDoc()})
	published := call[publishOutput](t, session, "publish_unit", publishInput{UnitID: shared.UnitID})
	if published.Status != "published" || mem.Status(shared.UnitID) != "published" {
		t.Fatalf("shared %+v status %s", published, mem.Status(shared.UnitID))
	}
}
