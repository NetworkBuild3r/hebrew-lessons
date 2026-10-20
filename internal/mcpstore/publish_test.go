package mcpstore

import (
	"errors"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

func TestPublishUnitSkipsDraftsInKnownLexemes(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := t.Context()
	store, _ := openAdapter(t, fixedClock{when})
	doc := unitDoc("Hello", hello)
	doc.Position = 0
	saved, err := store.PutUnit(ctx, nil, &doc, mcp.ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	known, err := store.KnownLexemes(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if known[hello] {
		t.Fatalf("draft counted %+v", known)
	}
	missing := 4
	if _, err := store.PublishUnit(ctx, saved.UnitID, &missing); !errors.Is(err, mcp.ErrRevisionNotFound) {
		t.Fatalf("missing revision %v", err)
	}
	if _, err := store.LoadUnit(ctx, saved.UnitID, &missing); !errors.Is(err, mcp.ErrRevisionNotFound) {
		t.Fatalf("load missing %v", err)
	}
	published, err := store.PublishUnit(ctx, saved.UnitID, nil)
	if err != nil || published.Unchanged || published.Revision != 1 || published.UnitID != saved.UnitID {
		t.Fatalf("publish %+v %v", published, err)
	}
	known, err = store.KnownLexemes(ctx, 1)
	if err != nil || !known[hello] || len(known) != 1 {
		t.Fatalf("published %+v %v", known, err)
	}
	again, err := store.PublishUnit(ctx, saved.UnitID, nil)
	if err != nil || !again.Unchanged || again.Revision != 1 {
		t.Fatalf("republish %+v %v", again, err)
	}
	view, err := store.LoadUnit(ctx, saved.UnitID, nil)
	if err != nil || view.Status != "published" || view.Revision != 1 || view.Doc.Title != "Hello" {
		t.Fatalf("load %+v %v", view, err)
	}
}
