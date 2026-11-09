package mcp

// INIT-037/SPEC-009: published-unit block counts on get_context, and a
// read-only sweep of published units under ModeWrite with no brief.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// UnitHealthAllowlist is the MCP tools a read-only published-unit health
// sweep may call. A test fails if a write tool is added here.
// INIT-037/SPEC-009.
var UnitHealthAllowlist = []string{
	"get_context",
	"get_unit",
	"lookup",
	"validate_unit",
}

// unitHealthWriteTools must never appear on UnitHealthAllowlist.
var unitHealthWriteTools = []string{
	"put_unit",
	"publish_unit",
	"unpublish_unit",
	"upsert_lexemes",
	"upsert_verb_forms",
	"upsert_noun_forms",
	"upsert_patterns",
	"upsert_common_mistakes",
	"upsert_explainer",
	"upsert_verse_notes",
}

// RuleCatalogVersion is a stable stamp of units.Rules. Block-count caches
// key on it so a rule change invalidates old counts. INIT-037/SPEC-009.
func RuleCatalogVersion() string {
	return ruleCatalogVersionOnce()
}

var ruleCatalogVersionOnce = sync.OnceValue(func() string {
	raw, err := json.Marshal(units.Rules)
	if err != nil {
		return fmt.Sprintf("rules-len-%d", len(units.Rules))
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
})

// HealthRow is one published unit's ModeWrite block tally for the sweep.
// State is the learner progress state when known, otherwise the unit status.
// INIT-037/SPEC-009.
type HealthRow struct {
	ID       int64    `json:"id"`
	Position int      `json:"position"`
	State    string   `json:"state"`
	Blocks   int      `json:"blocks"`
	Rules    []string `json:"rules"`
}

// blockCacheEntry is one cached ModeWrite block tally for a published revision.
type blockCacheEntry struct {
	Blocks int
	Rules  []string
}

// healthCache keys document-local ModeWrite block counts by unit id,
// revision, and rule-catalog version. Path shape checks (story_repeat)
// are not cached here; they are cheap and depend on earlier units.
type healthCache struct {
	mu    sync.Mutex
	byKey map[string]blockCacheEntry
}

func (c *healthCache) get(unitID int64, revision int, catalog string) (blockCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byKey == nil {
		return blockCacheEntry{}, false
	}
	e, ok := c.byKey[healthCacheKey(unitID, revision, catalog)]
	return e, ok
}

func (c *healthCache) put(unitID int64, revision int, catalog string, e blockCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byKey == nil {
		c.byKey = map[string]blockCacheEntry{}
	}
	c.byKey[healthCacheKey(unitID, revision, catalog)] = e
}

func healthCacheKey(unitID int64, revision int, catalog string) string {
	return fmt.Sprintf("%d:%d:%s", unitID, revision, catalog)
}

// WriteModeBlocks is the SPEC-009 ModeWrite block tally. Defined once in
// units so Today (SPEC-010) and get_context share one function.
func WriteModeBlocks(ctx context.Context, doc *units.Unit, lk units.Lookup, earlierFirstStories, prevActivityTypes []string) (blocks int, rules []string) {
	return units.WriteModeBlocks(ctx, doc, lk, earlierFirstStories, prevActivityTypes)
}

// documentLocalBlocks is ModeWrite with no brief and no path shape context.
// Cached on get_context by revision and rule-catalog version.
func documentLocalBlocks(ctx context.Context, doc *units.Unit, lk units.Lookup) (int, []string) {
	return units.DocumentLocalBlocks(ctx, doc, lk)
}

// activityTypes is the activity-type sequence units.same_shape compares.
func activityTypes(doc *units.Unit) []string {
	return units.ActivityTypes(doc)
}

// attachUnitBlocks fills outline[].blocks for every row. Published documents
// are checked under ModeWrite with no brief. Document-local counts are cached
// by revision and rule-catalog version; story_repeat uses earlier first
// stories on the path. A draft has blocks 0. INIT-037/SPEC-009.
func (s *service) attachUnitBlocks(ctx context.Context, out *getContextOutput) error {
	if out == nil || len(out.Outline) == 0 {
		return nil
	}
	if s.units == nil || s.lexLookup == nil {
		for i := range out.Outline {
			out.Outline[i].Blocks = 0
		}
		return nil
	}
	catalog := RuleCatalogVersion()
	var earlierStories []string
	for i := range out.Outline {
		row := &out.Outline[i]
		if row.Status != "published" {
			row.Blocks = 0
			continue
		}
		snap, err := s.units.LoadUnit(ctx, row.ID, nil)
		if err != nil {
			return storeErr("The unit could not be read for block counts.", err)
		}
		if snap.Status != "published" {
			row.Blocks = 0
			continue
		}
		doc := snap.Doc
		local, ok := s.blockCache.get(snap.UnitID, snap.Revision, catalog)
		if !ok {
			n, rules := documentLocalBlocks(ctx, &doc, s.lexLookup)
			local = blockCacheEntry{Blocks: n, Rules: rules}
			s.blockCache.put(snap.UnitID, snap.Revision, catalog, local)
		}
		// Path shape: story_repeat blocks when the first story matches an earlier unit.
		storyIssues := units.StoryRepeatIssues(&doc, earlierStories, units.ModeWrite)
		row.Blocks = local.Blocks + len(storyIssues)

		if story := units.FirstStoryText(&doc); story != "" {
			earlierStories = append(earlierStories, story)
		}
	}
	return nil
}

// SweepPublished lists every published unit in scope with its ModeWrite block
// count and rule ids. Shape comparison uses the previous unit's activity
// sequence and earlier first stories, without a brief. It never writes.
// learnerID nil is shared units only. INIT-037/SPEC-009.
func SweepPublished(ctx context.Context, course CourseReader, unitsStore UnitWriter, lk units.Lookup, learnerID *int64) ([]HealthRow, error) {
	if course == nil {
		return nil, fmt.Errorf("course storage is not connected")
	}
	if unitsStore == nil {
		return nil, fmt.Errorf("unit storage is not connected")
	}
	outline, err := course.UnitsInScope(ctx, learnerID)
	if err != nil {
		return nil, err
	}
	progress := map[int64]string{}
	if learnerID != nil {
		if reader, ok := course.(unitProgressReader); ok {
			rows, err := reader.UnitProgress(ctx, *learnerID)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				progress[row.UnitID] = row.State
			}
		}
	}
	var earlierStories []string
	var prevTypes []string
	var out []HealthRow
	for _, row := range outline {
		if row.Status != "published" {
			continue
		}
		snap, err := unitsStore.LoadUnit(ctx, row.ID, nil)
		if err != nil {
			return nil, err
		}
		if snap.Status != "published" {
			continue
		}
		doc := snap.Doc
		blocks, rules := WriteModeBlocks(ctx, &doc, lk, earlierStories, prevTypes)
		state := progress[row.ID]
		if state == "" {
			state = snap.Status
		}
		out = append(out, HealthRow{
			ID:       snap.UnitID,
			Position: snap.Position,
			State:    state,
			Blocks:   blocks,
			Rules:    rules,
		})
		if story := units.FirstStoryText(&doc); story != "" {
			earlierStories = append(earlierStories, story)
		}
		prevTypes = activityTypes(&doc)
	}
	return out, nil
}

// UnhealthyPublished returns SweepPublished rows with blocks > 0, or with
// unit.same_shape among the rule ids (shape copy without a brief).
func UnhealthyPublished(rows []HealthRow) []HealthRow {
	var out []HealthRow
	for _, row := range rows {
		if row.Blocks > 0 || hasRuleID(row.Rules, units.RuleUnitSameShape) {
			out = append(out, row)
		}
	}
	return out
}

func hasRuleID(rules []string, id string) bool {
	for _, r := range rules {
		if r == id {
			return true
		}
	}
	return false
}
