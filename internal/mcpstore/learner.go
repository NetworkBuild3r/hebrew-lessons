package mcpstore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/brief"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

var _ mcp.LearnerUnits = (*Store)(nil)

// maxKnownPages bounds how far BriefFacts pages through a learner's known words.
// A page is learner.MaxPageSize words, so the bound is far past any real vocabulary.
const maxKnownPages = 200

// BriefFacts implements mcp.LearnerUnits. It reads the learner's own brief, and
// the learner's known and solid words from the learner model, and converts them
// to what units.ValidateWithBrief takes. units cannot import internal/brief (see
// the units package comment), so this is where a brief.Brief becomes a BriefFacts.
//
// brief.Builder.Get answers ErrNotFound for a brief that is missing, malformed, or
// another learner's, and the three cases are not told apart.
func (s *Store) BriefFacts(ctx context.Context, learnerID int64, briefID string) (units.BriefFacts, error) {
	if err := s.ready(ctx); err != nil {
		return units.BriefFacts{}, err
	}
	model := learner.New(s.db, nil)
	br, err := brief.New(s.db, model, nil).Get(ctx, learnerID, briefID)
	switch {
	case errors.Is(err, brief.ErrNotFound):
		return units.BriefFacts{}, mcp.ErrBriefNotFound
	case errors.Is(err, brief.ErrExpired):
		return units.BriefFacts{}, mcp.ErrBriefExpired
	case err != nil:
		return units.BriefFacts{}, err
	}
	known, err := knownLexemeIDs(ctx, model, learnerID)
	if err != nil {
		return units.BriefFacts{}, err
	}
	return factsFromBrief(br, known), nil
}

// knownLexemeIDs is every word the learner knows now, known or solid overall.
func knownLexemeIDs(ctx context.Context, model *learner.Model, learnerID int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	for _, state := range []learner.State{learner.Known, learner.Solid} {
		f := learner.Filter{State: state, Limit: learner.MaxPageSize}
		for page := 0; page < maxKnownPages; page++ {
			words, next, err := model.List(ctx, learnerID, f)
			if err != nil {
				return nil, err
			}
			for _, w := range words {
				out[w.LexemeID] = true
			}
			if next == "" {
				break
			}
			f.After = next
		}
	}
	return out, nil
}

// factsFromBrief converts a brief to the validator's view. known is the learner's
// known and solid word ids.
//
// The coverage figure is the brief's staged tier.min_known_share, not the flat
// targets.coverage of 0.95. SPEC-020 names 0.95, but a beginner brief could never
// meet it, and north-star 7.3 stages it. When a brief has no tier figure, the
// flat target is used, and the validator falls back to 0.95 if that is missing too.
func factsFromBrief(br brief.Brief, known map[int64]bool) units.BriefFacts {
	f := units.BriefFacts{
		Tier:           br.Tier.Name,
		MinKnownShare:  br.Tier.MinKnownShare,
		MaxTextTokens:  br.Tier.MaxTextTokens,
		Used:           br.UsedByUnitID != nil,
		BriefLexemeIDs: map[int64]bool{},
		KnownLexemeIDs: known,
	}
	if f.MinKnownShare <= 0 {
		f.MinKnownShare = br.Targets.Coverage
	}
	if br.UsedByUnitID != nil {
		f.UsedByUnitID = *br.UsedByUnitID
	}
	for _, w := range br.NewWords {
		uses := w.MinUses
		if uses <= 0 {
			uses = br.Targets.NewWordMinUses
		}
		f.NewWords = append(f.NewWords, units.BriefNewWord{LexemeID: w.LexemeID, Text: w.Text, MinUses: uses})
		f.BriefLexemeIDs[w.LexemeID] = true
	}
	for _, w := range br.StruggleWords {
		f.StruggleWords = append(f.StruggleWords, units.BriefStruggleWord{
			LexemeID: w.LexemeID, Text: w.Text, RequiredCount: w.RequiredCount, Skill: w.Skill,
		})
		f.BriefLexemeIDs[w.LexemeID] = true
	}
	for _, w := range br.ComfortableWords {
		f.BriefLexemeIDs[w.LexemeID] = true
	}
	for _, m := range br.FixMistakes {
		f.FixMistakes = append(f.FixMistakes, units.BriefFix{Right: m.Right})
	}
	return f
}

// AutoPublishLearner implements mcp.LearnerUnits. The publish, the brief claim, and
// the audit row are one transaction in db.AutoPublishLearnerUnit.
func (s *Store) AutoPublishLearner(ctx context.Context, req mcp.AutoPublishRequest) (mcp.PutResult, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.PutResult{}, err
	}
	issues := req.Issues
	if issues == nil {
		issues = []units.Issue{}
	}
	raw, err := json.Marshal(issues)
	if err != nil {
		return mcp.PutResult{}, err
	}
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = defaultActor
	}
	res, err := s.db.AutoPublishLearnerUnit(ctx, db.AutoPublish{
		UnitID:         req.UnitID,
		Revision:       req.Revision,
		LearnerID:      req.LearnerID,
		BriefID:        req.BriefID,
		BriefNotBefore: s.now().Add(-brief.TTL),
		Actor:          actor,
		IssuesJSON:     string(raw),
	})
	switch {
	case errors.Is(err, db.ErrBriefUsed):
		return mcp.PutResult{}, mcp.ErrBriefUsed
	case errors.Is(err, db.ErrBriefNotFound):
		return mcp.PutResult{}, mcp.ErrBriefNotFound
	case errors.Is(err, db.ErrBriefExpired):
		return mcp.PutResult{}, mcp.ErrBriefExpired
	case errors.Is(err, db.ErrUnitNotInScope):
		return mcp.PutResult{}, mcp.ErrUnitNotInScope
	case errors.Is(err, db.ErrNoLearner):
		return mcp.PutResult{}, mcp.ErrNoLearner
	case errors.Is(err, db.ErrUnitNotFound):
		return mcp.PutResult{}, mcp.ErrUnitNotFound
	case errors.Is(err, db.ErrRevisionNotFound):
		return mcp.PutResult{}, mcp.ErrRevisionNotFound
	case err != nil:
		return mcp.PutResult{}, err
	}
	return mcp.PutResult{UnitID: res.UnitID, Revision: res.Revision, Unchanged: res.Unchanged}, nil
}

// UnpublishLearner implements mcp.LearnerUnits. The scope is learner:<p.UserID>,
// taken from the principal and from nothing the caller sent. A principal with no
// learner id, as the static token has, has no scope and is refused.
func (s *Store) UnpublishLearner(ctx context.Context, p mcp.Principal, unitID int64) (mcp.UnpublishResult, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.UnpublishResult{}, err
	}
	actor := strings.TrimSpace(p.Actor)
	if actor == "" {
		actor = defaultActor
	}
	res, err := s.db.UnpublishLearnerUnit(ctx, p.UserID, unitID, actor)
	switch {
	case errors.Is(err, db.ErrNoLearner):
		return mcp.UnpublishResult{}, mcp.ErrNoLearner
	case errors.Is(err, db.ErrUnitNotInScope):
		return mcp.UnpublishResult{}, mcp.ErrUnitNotInScope
	case err != nil:
		return mcp.UnpublishResult{}, err
	}
	return mcp.UnpublishResult{UnitID: res.UnitID, Revision: res.Revision, Unchanged: res.Unchanged}, nil
}
