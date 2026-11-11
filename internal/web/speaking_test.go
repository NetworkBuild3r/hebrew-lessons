package web

import (
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
)

func TestSpeakingPrepIsThisLearnersClassesOnly(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	store, h := openShellFlags(t, fixedClock(now), nil, nil, featuresOn())
	a := addLearner(t, store, "a@example.com", "sub-a", now)
	b := addLearner(t, store, "b@example.com", "sub-b", now)
	mine, err := lessons.ImportSheet(lessons.Sheet{
		Ref: "class-a", ClassOn: "2026-11-01", Kind: "class", Title: "Morning",
		Body: "tutor: שָׁלוֹם || peace\n",
	}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.SaveClassSheet(t.Context(), a.user.ID, mine); err != nil {
		t.Fatal(err)
	}
	theirs, err := lessons.ImportSheet(lessons.Sheet{
		Ref: "class-b", ClassOn: "2026-11-02", Kind: "class",
		Body: "tutor: סוֹד || secret\n",
	}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.SaveClassSheet(t.Context(), b.user.ID, theirs); err != nil {
		t.Fatal(err)
	}
	body := getLearner(t, h, a, "/me/speaking")
	if !strings.Contains(body, "שָׁלוֹם") || strings.Contains(body, "סוֹד") {
		t.Fatal(body)
	}
	if !strings.Contains(body, "2026-11-01") {
		t.Fatal(body)
	}
}
