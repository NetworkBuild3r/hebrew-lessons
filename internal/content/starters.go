// Package content holds the original starter explainers. The app inserts
// them when the table is empty, through the same store the explainer tool uses.
package content

import (
	"context"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/richtext"
)

// Starter is one original explainer. It names no person.
type Starter struct {
	Slug    string
	Title   string
	Summary string
	Example string
	Prereqs []string
	Body    string
}

// Starters are the five notes a new database shows on Learn.
func Starters() []Starter {
	return []Starter{
		{
			Slug: "present-tense", Title: "Present tense agreement",
			Summary: "The present verb changes with who is doing it.",
			Example: "אֲנִי לוֹמֵד",
			Body: `## The verb agrees with the person

In the present, the verb changes for who is speaking.

- One man: {he:לוֹמֵד|lomed}
- One woman: {he:לוֹמֶדֶת|lomedet}
- A group: {he:לוֹמְדִים|lomdim}

{he:אֲנִי לוֹמֵד|ani lomed}`,
		},
		{
			Slug: "past-tense-ti", Title: "Past tense with ti",
			Summary: "The ending ti on a past verb means I.",
			Example: "לָמַדְתִּי",
			Prereqs: []string{"present-tense"},
			Body: `## Past tense with ti

The ending {he:תִּי|ti} on a verb means the speaker did it.

{he:לָמַדְתִּי|lamadeti} is I learned.`,
		},
		{
			Slug: "going-to", Title: "Going to plus a verb",
			Summary: "Holekh before a verb says what is about to happen.",
			Example: "אֲנִי הוֹלֵךְ לִלְמוֹד",
			Prereqs: []string{"present-tense"},
			Body: `## Going to plus a verb

{he:הוֹלֵךְ|holekh} before a verb with {he:לְ|le} says someone is about to do it.

{he:אֲנִי הוֹלֵךְ לִלְמוֹד|ani holekh lilmod}`,
		},
		{
			Slug: "efshar", Title: "Efshar plus a verb",
			Summary: "Efshar before a verb means it is possible.",
			Example: "אֶפְשָׁר לִלְמוֹד",
			Prereqs: []string{"present-tense"},
			Body: `## Efshar plus a verb

{he:אֶפְשָׁר|efshar} before a verb means it is possible to do it.

{he:אֶפְשָׁר לִלְמוֹד|efshar lilmod}`,
		},
		{
			Slug: "preposition-endings", Title: "Prepositions with endings",
			Summary: "A preposition takes an ending for the person.",
			Example: "לִי",
			Body: `## Prepositions with endings

{he:לְ|le} takes an ending for the person.

- {he:לִי|li} to me
- {he:לְךָ|lekha} to you`,
		},
	}
}

// Ensure inserts the starters when no explainer is stored yet.
// Each body is parsed first, so a bad starter refuses to start the app.
func Ensure(ctx context.Context, store *db.Store) error {
	if store == nil {
		return nil
	}
	n, err := store.ExplainerCount(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	rows := Starters()
	patterns := make([]db.PatternInput, len(rows))
	for i, row := range rows {
		if _, err := richtext.Parse(row.Body); err != nil {
			return fmt.Errorf("content: %s: %w", row.Slug, err)
		}
		patterns[i] = db.PatternInput{
			Slug: row.Slug, Title: row.Title, Summary: row.Summary, Example: row.Example, Prereqs: row.Prereqs,
		}
	}
	if err := store.UpsertPatterns(ctx, patterns); err != nil {
		return err
	}
	if err := store.AuditContent(ctx, "starter", "upsert_patterns", "shared"); err != nil {
		return err
	}
	for _, row := range rows {
		if err := store.UpsertExplainer(ctx, db.Explainer{
			Slug: row.Slug, Title: row.Title, Body: row.Body, Status: "published",
		}); err != nil {
			return fmt.Errorf("content: %s: %w", row.Slug, err)
		}
		if err := store.AuditContent(ctx, "starter", "upsert_explainer", "shared"); err != nil {
			return err
		}
	}
	return nil
}
