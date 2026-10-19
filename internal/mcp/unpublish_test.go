package mcp_test

import (
	"errors"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// The unpublish path (INIT-015/SPEC-020, ac-4). The scope is the principal's. There
// is no argument that names a scope or a learner.

var _ mcp.LearnerUnits = (*apStoreCheck)(nil)

// apStoreCheck only proves the store satisfies the interface at compile time.
type apStoreCheck struct{ mcp.LearnerUnits }

func TestUnpublishLearnerOwnUnit(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	e.goodBrief(apBriefA, e.a)
	out := e.put(db.LearnerScope(e.a), nil, doc, apBriefA, true)
	if !out.Published {
		t.Fatalf("setup: %+v", out)
	}
	p := mcp.Principal{Actor: "client-1", UserID: e.a, Role: mcp.RoleLearner, Source: "oauth"}
	res, err := e.adapter.UnpublishLearner(t.Context(), p, out.UnitID)
	if err != nil || res.UnitID != out.UnitID || res.Unchanged || res.Revision != out.Revision {
		t.Fatalf("unpublish %+v %v", res, err)
	}
	if e.status(out.UnitID) != "draft" {
		t.Fatalf("status %s", e.status(out.UnitID))
	}
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND action = 'unpublish' AND actor = 'client-1' AND scope = ?`, out.UnitID, db.LearnerScope(e.a)); n != 1 {
		t.Fatalf("unpublish audit rows %d", n)
	}
	if e.audits() != 2 {
		t.Fatalf("audit rows %d, want the publish and the unpublish", e.audits())
	}
	// The brief stays spent: undo does not give a brief a second unit.
	if int64(e.briefUsed(apBriefA)) != out.UnitID {
		t.Fatal("unpublish released the brief")
	}
	again, err := e.adapter.UnpublishLearner(t.Context(), p, out.UnitID)
	if err != nil || !again.Unchanged || e.audits() != 2 {
		t.Fatalf("repeat %+v %v audits %d", again, err, e.audits())
	}
	// A blank actor takes the default name rather than an empty audit column.
	e.goodBrief(apBriefB, e.a)
	other := apDoc("Cafe two")
	e.audio(other)
	second := e.put(db.LearnerScope(e.a), nil, other, apBriefB, true)
	if !second.Published {
		t.Fatalf("setup second: %+v", second)
	}
	if _, err := e.adapter.UnpublishLearner(t.Context(), mcp.Principal{UserID: e.a}, second.UnitID); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND action = 'unpublish' AND actor = 'mcp'`, second.UnitID); n != 1 {
		t.Fatalf("default actor rows %d", n)
	}
}

func TestUnpublishRefusedOutsideScope(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	e.goodBrief(apBriefA, e.a)
	e.goodBrief(apBriefB, e.b)
	mine := e.put(db.LearnerScope(e.a), nil, doc, apBriefA, true)
	theirsDoc := apDoc("Theirs")
	e.audio(theirsDoc)
	theirs := e.put(db.LearnerScope(e.b), nil, theirsDoc, apBriefB, true)
	if !mine.Published || !theirs.Published {
		t.Fatalf("setup: %+v %+v", mine, theirs)
	}
	shared, err := e.adapter.PutUnit(t.Context(), nil, &units.Unit{Title: "Shared", Situation: "s", Concept: "c"}, mcp.ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.adapter.PublishUnit(t.Context(), shared.UnitID, nil); err != nil {
		t.Fatal(err)
	}
	e.exec(`INSERT INTO units (position, title, situation, concept, status, rev, doc, scope) VALUES (0, 'zero', 's', 'c', 'published', 1, '{}', 'learner:0')`)
	zero := int64(e.count(`SELECT id FROM units WHERE scope = 'learner:0'`))
	before := e.audits()

	a := mcp.Principal{Actor: "a", UserID: e.a, Role: mcp.RoleLearner}
	admin := mcp.Principal{Actor: "mcp", Role: mcp.RoleAdmin, Source: "token"} // the static token: no learner id
	for _, c := range []struct {
		name string
		p    mcp.Principal
		unit int64
		want error
	}{
		{"another learner's unit", a, theirs.UnitID, mcp.ErrUnitNotInScope},
		{"a shared unit", a, shared.UnitID, mcp.ErrUnitNotInScope},
		{"a missing unit", a, 99999, mcp.ErrUnitNotInScope},
		{"an admin with no learner id on a learner's unit", admin, mine.UnitID, mcp.ErrNoLearner},
		{"an admin with no learner id on learner:0", admin, zero, mcp.ErrNoLearner},
		{"an admin with no learner id on a shared unit", admin, shared.UnitID, mcp.ErrNoLearner},
		{"the other learner on the first learner's unit", mcp.Principal{Actor: "b", UserID: e.b, Role: mcp.RoleAdmin}, mine.UnitID, mcp.ErrUnitNotInScope},
	} {
		if _, err := e.adapter.UnpublishLearner(t.Context(), c.p, c.unit); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	if e.audits() != before {
		t.Fatalf("a refused unpublish wrote %d audit rows", e.audits()-before)
	}
	for _, id := range []int64{mine.UnitID, theirs.UnitID, shared.UnitID, zero} {
		if e.status(id) != "published" {
			t.Fatalf("unit %d status %s after refusals", id, e.status(id))
		}
	}
}
