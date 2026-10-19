package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultSearchLimit = 8
	maxSearchLimit     = 20
)

var (
	errBadPosition = errors.New("Position must be 0 or greater.")
	errMissingDoc  = errors.New("The unit document is missing.")
)

type service struct {
	course    CourseReader
	lexicon   LexiconReader
	lexLookup units.Lookup
	units     UnitWriter
}

type getContextInput struct {
	Position  int    `json:"position" jsonschema:"Course position you are about to write. Known lexemes are words introduced in units at a lower position. Zero is the first unit. Use the placement suggestedStartPosition when the learner has taken the vocabulary check."`
	LearnerID *int64 `json:"learnerId,omitempty" jsonschema:"Optional. The learner whose custom units, placement, struggle, and completed shared units to include. Omit it to see shared units only, with no learner data. OAuth will later default this to the signed-in user."`
	TopN      int    `json:"topN,omitempty" jsonschema:"How many struggled words and confusion pairs to return. Defaults to 20."`
}

type getContextOutput struct {
	Outline              []UnitOutline    `json:"outline"`
	KnownLexemes         []string         `json:"knownLexemes"`
	Budgets              Budgets          `json:"budgets"`
	Placement            placementView    `json:"placement"`
	Struggle             Struggle         `json:"struggle"`
	CompletedSharedUnits []UnitCompletion `json:"completedSharedUnits"`
}

// placementView is the get_context placement object.
// Taken false is the whole object when LatestPlacement reports no run.
// A finished run keeps zero values: level −1, suggestedStartPosition 0,
// and reliable false are all meaningful.
type placementView struct {
	Taken                  bool        `json:"taken"`
	TakenAt                time.Time   `json:"takenAt,omitempty"`
	Reliable               bool        `json:"reliable,omitempty"`
	FalseAlarmRate         float64     `json:"falseAlarmRate,omitempty"`
	Level                  int         `json:"level,omitempty"`
	BandScores             []BandScore `json:"bandScores,omitempty"`
	EstimatedKnownWords    int         `json:"estimatedKnownWords,omitempty"`
	SuggestedStartPosition int         `json:"suggestedStartPosition,omitempty"`
	KnownLexemes           []string    `json:"knownLexemes,omitempty"`
	LearningLexemes        []string    `json:"learningLexemes,omitempty"`
}

// MarshalJSON emits {"taken":false} when there is no finished run.
// A finished run includes every field, including zeros.
func (p placementView) MarshalJSON() ([]byte, error) {
	if !p.Taken {
		return []byte(`{"taken":false}`), nil
	}
	known := p.KnownLexemes
	if known == nil {
		known = []string{}
	}
	learning := p.LearningLexemes
	if learning == nil {
		learning = []string{}
	}
	bands := p.BandScores
	if bands == nil {
		bands = []BandScore{}
	}
	return json.Marshal(struct {
		Taken                  bool        `json:"taken"`
		TakenAt                time.Time   `json:"takenAt"`
		Reliable               bool        `json:"reliable"`
		FalseAlarmRate         float64     `json:"falseAlarmRate"`
		Level                  int         `json:"level"`
		BandScores             []BandScore `json:"bandScores"`
		EstimatedKnownWords    int         `json:"estimatedKnownWords"`
		SuggestedStartPosition int         `json:"suggestedStartPosition"`
		KnownLexemes           []string    `json:"knownLexemes"`
		LearningLexemes        []string    `json:"learningLexemes"`
	}{
		Taken:                  true,
		TakenAt:                p.TakenAt,
		Reliable:               p.Reliable,
		FalseAlarmRate:         p.FalseAlarmRate,
		Level:                  p.Level,
		BandScores:             bands,
		EstimatedKnownWords:    p.EstimatedKnownWords,
		SuggestedStartPosition: p.SuggestedStartPosition,
		KnownLexemes:           known,
		LearningLexemes:        learning,
	})
}

type lookupInput struct {
	Query string `json:"query" jsonschema:"Pointed Hebrew, unpointed Hebrew, or an English gloss. A miss returns suggestions and is not an error."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum rows to return. Defaults to 8 and will not exceed 20."`
}

type lookupOutput struct {
	Hits        []Lexeme `json:"hits"`
	Suggestions []Lexeme `json:"suggestions"`
}

type validateInput struct {
	Position int        `json:"position" jsonschema:"Course position. Words from lower positions count as already known. This is not a field inside doc."`
	Doc      units.Unit `json:"doc" jsonschema:"Unit document. Read hebrew://schema/unit. This call writes nothing."`
}

type validateOutput struct {
	OK     bool          `json:"ok"`
	Blocks int           `json:"blocks"`
	Warns  int           `json:"warns"`
	Issues []units.Issue `json:"issues"`
}

type putInput struct {
	UnitID   *int64     `json:"unitId,omitempty" jsonschema:"Id from an earlier put_unit. Omit it to create a unit. Send the same id and the same document to avoid a new revision."`
	Position int        `json:"position" jsonschema:"Course position, 0 or greater. This is not a field inside doc. Changing it saves a new draft revision."`
	Scope    string     `json:"scope" jsonschema:"Required. shared for every learner, or learner:<id> for one learner. <id> is the numeric learner id, such as learner:7. Scope is metadata, not a field of doc. Changing it saves a new draft revision."`
	Doc      units.Unit `json:"doc" jsonschema:"Unit document. JSON that cannot be read as a unit is rejected. Warnings and block issues are returned, and the draft is still saved. Publishing is a later tool."`
}

type putOutput struct {
	UnitID    int64         `json:"unitId"`
	Revision  int           `json:"revision"`
	Unchanged bool          `json:"unchanged"`
	Issues    []units.Issue `json:"issues"`
}

func (s *service) getContext(ctx context.Context, _ *mcp.CallToolRequest, in getContextInput) (*mcp.CallToolResult, getContextOutput, error) {
	var out getContextOutput
	if in.Position < 0 {
		return nil, out, errBadPosition
	}
	if s.course == nil {
		return nil, out, errors.New("Course storage is not connected.")
	}
	if s.lexLookup == nil {
		return nil, out, errors.New("The lexicon lookup is not connected.")
	}
	outline, err := s.course.ListUnits(ctx)
	if err != nil {
		return nil, out, storeErr("The course outline could not be read.", err)
	}
	known, err := s.lexLookup.KnownLexemes(ctx, in.Position)
	if err != nil {
		return nil, out, storeErr("The known-word list could not be read.", err)
	}
	if in.LearnerID != nil && *in.LearnerID < 0 {
		return nil, out, errors.New("learnerId must be 0 or greater.")
	}
	if in.TopN < 0 {
		return nil, out, errors.New("topN must be 0 or greater. Omit it to use 20.")
	}
	outline = visibleUnits(outline, in.LearnerID)
	view := placementView{}
	struggle := emptyStruggle()
	completed := []UnitCompletion{}
	if in.LearnerID != nil {
		userID := *in.LearnerID
		row, taken, err := s.course.LatestPlacement(ctx, userID)
		if err != nil {
			return nil, out, storeErr("The placement result could not be read.", err)
		}
		if taken {
			words, err := s.course.PlacementKnownLexemes(ctx, userID)
			if err != nil {
				return nil, out, storeErr("The placement words could not be read.", err)
			}
			learning, err := s.course.PlacementLearningLexemes(ctx, userID)
			if err != nil {
				return nil, out, storeErr("The words still being learned could not be read.", err)
			}
			view = placementView{
				Taken:                  true,
				TakenAt:                row.TakenAt,
				Reliable:               row.Reliable,
				FalseAlarmRate:         row.FalseAlarmRate,
				Level:                  row.Level,
				BandScores:             append([]BandScore(nil), row.BandScores...),
				EstimatedKnownWords:    row.EstimatedKnownWords,
				SuggestedStartPosition: row.SuggestedStartPosition,
				KnownLexemes:           append([]string(nil), words...),
				LearningLexemes:        append([]string(nil), learning...),
			}
			slices.Sort(view.KnownLexemes)
			slices.Sort(view.LearningLexemes)
		}
		topN := in.TopN
		if topN == 0 {
			topN = defaultStruggleTop
		}
		struggle, err = s.course.StruggleStats(ctx, userID, topN)
		if err != nil {
			return nil, out, storeErr("The struggle list could not be read.", err)
		}
		struggle = fillStruggle(struggle)
		completed, err = s.course.CompletedSharedUnits(ctx, userID)
		if err != nil {
			return nil, out, storeErr("The completed shared units could not be read.", err)
		}
		if completed == nil {
			completed = []UnitCompletion{}
		}
	}
	out = getContextOutput{
		Outline:              outline,
		KnownLexemes:         sortedKnown(known),
		Budgets:              currentBudgets(),
		Placement:            view,
		Struggle:             struggle,
		CompletedSharedUnits: completed,
	}
	return nil, out, nil
}

func visibleUnits(outline []UnitOutline, learner *int64) []UnitOutline {
	out := make([]UnitOutline, 0, len(outline))
	for _, row := range outline {
		if row.Scope == ScopeShared || (learner != nil && row.Scope == fmt.Sprintf("learner:%d", *learner)) {
			out = append(out, row)
		}
	}
	return out
}

func emptyStruggle() Struggle {
	return Struggle{
		Lexemes:    []StruggleLexeme{},
		Confusions: []ConfusionPair{},
		Top:        []string{},
	}
}

func fillStruggle(in Struggle) Struggle {
	if in.Lexemes == nil {
		in.Lexemes = []StruggleLexeme{}
	}
	if in.Confusions == nil {
		in.Confusions = []ConfusionPair{}
	}
	if in.Top == nil {
		in.Top = []string{}
	}
	return in
}

func (s *service) lookup(ctx context.Context, _ *mcp.CallToolRequest, in lookupInput) (*mcp.CallToolResult, lookupOutput, error) {
	var out lookupOutput
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, out, errors.New("Query is empty. Send pointed Hebrew, unpointed Hebrew, or an English gloss.")
	}
	if s.lexicon == nil {
		return nil, out, errors.New("The lexicon is not connected.")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	hits, suggestions, err := s.lexicon.SearchLexemes(ctx, query, limit)
	if err != nil {
		return nil, out, storeErr("The lexicon search could not be completed.", err)
	}
	if hits == nil {
		hits = []Lexeme{}
	}
	if suggestions == nil {
		suggestions = []Lexeme{}
	}
	out.Hits = hits
	out.Suggestions = suggestions
	return nil, out, nil
}

func (s *service) validateUnit(ctx context.Context, _ *mcp.CallToolRequest, in validateInput) (*mcp.CallToolResult, validateOutput, error) {
	if in.Position < 0 {
		return nil, validateOutput{}, errBadPosition
	}
	doc := in.Doc
	doc.Position = in.Position
	issues := units.Validate(ctx, &doc, s.lexLookup)
	return nil, summarize(issues), nil
}

func (s *service) putUnit(ctx context.Context, _ *mcp.CallToolRequest, in putInput) (*mcp.CallToolResult, putOutput, error) {
	var out putOutput
	if in.Position < 0 {
		return nil, out, errBadPosition
	}
	if in.UnitID != nil && *in.UnitID <= 0 {
		return nil, out, errors.New("unitId must be a positive id from an earlier put_unit. Omit it to create a unit.")
	}
	scope, err := CanonicalScope(in.Scope)
	if err != nil {
		return nil, out, err
	}
	if s.units == nil {
		return nil, out, errors.New("Unit storage is not connected, so the draft was not saved.")
	}
	doc := in.Doc
	doc.Position = in.Position
	issues := units.Validate(ctx, &doc, s.lexLookup)
	saved, err := s.units.PutDraftUnit(ctx, PutDraft{
		UnitID:   in.UnitID,
		Position: in.Position,
		Doc:      &doc,
		Scope:    scope,
		Actor:    actorFrom(ctx),
	})
	if errors.Is(err, ErrUnitNotFound) {
		return nil, out, fmt.Errorf("No unit has id %d.", *in.UnitID)
	}
	if err != nil {
		return nil, out, storeErr("The draft could not be saved.", err)
	}
	out.UnitID = saved.UnitID
	out.Revision = saved.Revision
	out.Unchanged = saved.Unchanged
	out.Issues = issueList(issues)
	return nil, out, nil
}

func summarize(issues []units.Issue) validateOutput {
	var blocks, warns int
	for _, issue := range issues {
		switch issue.Severity {
		case units.SeverityBlock:
			blocks++
		case units.SeverityWarn:
			warns++
		}
	}
	return validateOutput{
		OK:     blocks == 0,
		Blocks: blocks,
		Warns:  warns,
		Issues: issueList(issues),
	}
}

func issueList(issues []units.Issue) []units.Issue {
	if len(issues) == 0 {
		return []units.Issue{}
	}
	return issues
}

func sortedKnown(known map[string]bool) []string {
	out := make([]string, 0, len(known))
	for text, ok := range known {
		if ok && strings.TrimSpace(text) != "" {
			out = append(out, text)
		}
	}
	slices.Sort(out)
	return out
}

func storeErr(sentence string, err error) error {
	slog.Error("mcp store", "err", err)
	return errors.New(sentence)
}

func registerTools(server *mcp.Server, svc *service) {
	falseHint := false
	readOnly := &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		IdempotentHint:  true,
		DestructiveHint: &falseHint,
		OpenWorldHint:   &falseHint,
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_context",
		Description: "Read the course before you draft a unit. The outline rows have position, title, status, and scope (shared, or learner:<id>). Omit learnerId to see shared units only, with empty struggle and no placement. Pass learnerId to add that learner's custom units, their placement, their struggle, and the shared units they have finished. topN defaults to 20 and limits struggled words and confusion pairs. Struggle order is lapses + misses in 7 days + again rate (0 to 1), highest first, then text. Placement is an adaptive deck: each rated word has a real FSRS card. taken false means there is no finished run. A finished run includes takenAt, reliable, falseAlarmRate, level (highest passed band, or -1), bandScores (per band: band, shown, again, hard, good, easy, dontknow, goodPlusRate), estimatedKnownWords, suggestedStartPosition, knownLexemes (Good and Easy across bands, pointed NFC, sorted), and learningLexemes (Hard, Again, and don't-know across bands, pointed NFC, sorted). suggestedStartPosition 0 is the first unit. Read hebrew://rules for budgets and for the shared-versus-custom guidance. That guidance is not an enforced rule.",
		Annotations: readOnly,
	}, svc.getContext)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup",
		Description: "Find a lexicon entry by pointed Hebrew, unpointed Hebrew, or an English gloss. Use an entry's text_nfc as the spelling in a unit. A miss returns suggestions and is not an error. Do not invent a lexeme here; upsert_lexemes is a later tool.",
		Annotations: readOnly,
	}, svc.lookup)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate_unit",
		Description: "Dry run. Checks the unit document at position with the authoring rules and returns ok, block and warn counts, and issues. ok means zero block issues. This call does not save. Each issue has a rule id, severity block or warn, a JSON pointer, and a plain-English message.",
		Annotations: readOnly,
	}, svc.validateUnit)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "put_unit",
		Description: "Save a draft revision. scope is required: shared, or learner:<id> for one learner. Scope is not inside the unit document. Send unitId to update; omit it to create. The same document, position, and scope do not create another revision. Changing scope does. The draft is saved even when issues block publishing. Warnings are not a reason to skip the save. JSON that cannot be read as a unit is rejected and nothing is written. This does not publish.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.putUnit)
}
