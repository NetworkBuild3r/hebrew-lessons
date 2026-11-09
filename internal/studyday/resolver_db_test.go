package studyday_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// ac-2: For returns Source learner with a stored zone and Source app without.
// A stored invalid name returns an error. INIT-026/SPEC-003.
// This file is package studyday_test so it can open a database. db imports
// studyday, and a test inside package studyday cannot import db.
func TestResolverFor(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	withZone, err := store.InsertUser(ctx, "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	without, err := store.InsertUser(ctx, "b@example.com", "sub-b", "learner")
	if err != nil {
		t.Fatal(err)
	}
	bad, err := store.InsertUser(ctx, "c@example.com", "sub-c", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserTimeZone(ctx, withZone, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserTimeZone(ctx, bad, "Not/AZone"); err != nil {
		t.Fatal(err)
	}

	app, err := studyday.AppZone("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	r, err := studyday.NewResolver(store, app)
	if err != nil {
		t.Fatal(err)
	}

	z, err := r.For(ctx, withZone)
	if err != nil {
		t.Fatal(err)
	}
	if z.Source != studyday.SourceLearner || z.Name != "America/Denver" || z.Loc == nil {
		t.Fatalf("learner zone = %+v", z)
	}

	z, err = r.For(ctx, without)
	if err != nil {
		t.Fatal(err)
	}
	if z.Source != studyday.SourceApp || z.Name != "America/Chicago" {
		t.Fatalf("app fallback = %+v", z)
	}

	if _, err := r.For(ctx, bad); err == nil {
		t.Fatal("invalid stored zone was accepted")
	}
}

func TestResolverAll(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	a, err := store.InsertUser(ctx, "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.InsertUser(ctx, "b@example.com", "sub-b", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserTimeZone(ctx, a, "America/Denver"); err != nil {
		t.Fatal(err)
	}

	app, err := studyday.AppZone("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	r, err := studyday.NewResolver(store, app)
	if err != nil {
		t.Fatal(err)
	}
	all, err := r.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("All len = %d, want 2", len(all))
	}
	if all[a].Source != studyday.SourceLearner || all[a].Name != "America/Denver" {
		t.Fatalf("a = %+v", all[a])
	}
	if all[b].Source != studyday.SourceApp || all[b].Name != "America/Chicago" {
		t.Fatalf("b = %+v", all[b])
	}
}
