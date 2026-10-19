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
	lexemes   LexiconWriter
	lexLookup units.Lookup
	units     UnitWriter
}

type getContextInput struct {
	Position  int    `json:"position" jsonschema:"Course position you are about to write. Known lexemes are words introduced in units at a lower position. Zero is the first unit. Use the placement suggestedStartPosition when the learner has taken the vocabulary check."`
	LearnerID *int64 `json:"learnerId,omitempty" jsonschema:"Optional for an admin or the static token. Omit it to see shared units only. A learner account uses their own id when this is omitted. Another id is forbidden. A whole number is accepted, including 9.0, which is the integer 9. A fraction such as 9.5 is rejected."`
	TopN      int    `json:"topN,omitempty" jsonschema:"How many struggled lexemes to return. Defaults to 30. The store ranks them by misses in 7 days, then lapses, then Again rate. This tool keeps that order. It does not limit confusions, which already cover 7 days."`
}

type getContextOutput struct {
	Outline              []UnitOutline    `json:"outline"`
	KnownLexemes         []string         `json:"knownLexemes"`
	Budgets              Budgets          `json:"budgets"`
	Placement            placementView    `json:"placement"`
	Struggles            Struggles        `json:"struggles"`
	CompletedSharedUnits []UnitCompletion `json:"completedSharedUnits"`
}

// placementView is the get_context placement object.
// Taken false is the whole object when LatestPlacement reports no row.
// A row keeps zero values: level −1, suggestedStartPosition 0, finished
// false, sittings 0, and an empty stopReason are all meaningful.
// inProgress is true when the row is not finished and sittings is greater than 0.
type placementView struct {
	Taken                  bool        `json:"taken"`
	TakenAt                time.Time   `json:"takenAt,omitempty"`
	Finished               bool        `json:"finished,omitempty"`
	Sittings               int         `json:"sittings,omitempty"`
	InProgress             bool        `json:"inProgress,omitempty"`
	Level                  int         `json:"level,omitempty"`
	CardsRated             int         `json:"cardsRated,omitempty"`
	StopReason             string      `json:"stopReason,omitempty"`
	Bands                  []BandScore `json:"bands,omitempty"`
	EstimatedKnownWords    int         `json:"estimatedKnownWords,omitempty"`
	SuggestedStartPosition int         `json:"suggestedStartPosition,omitempty"`
	KnownLexemes           []string    `json:"knownLexemes,omitempty"`
	LearningLexemes        []string    `json:"learningLexemes,omitempty"`
}

// MarshalJSON emits {"taken":false} when there is no placement row.
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
	bands := p.Bands
	if bands == nil {
		bands = []BandScore{}
	}
	return json.Marshal(struct {
		Taken                  bool        `json:"taken"`
		TakenAt                time.Time   `json:"takenAt"`
		Finished               bool        `json:"finished"`
		Sittings               int         `json:"sittings"`
		InProgress             bool        `json:"inProgress"`
		Level                  int         `json:"level"`
		CardsRated             int         `json:"cardsRated"`
		StopReason             string      `json:"stopReason"`
		Bands                  []BandScore `json:"bands"`
		EstimatedKnownWords    int         `json:"estimatedKnownWords"`
		SuggestedStartPosition int         `json:"suggestedStartPosition"`
		KnownLexemes           []string    `json:"knownLexemes"`
		LearningLexemes        []string    `json:"learningLexemes"`
	}{
		Taken:                  true,
		TakenAt:                p.TakenAt,
		Finished:               p.Finished,
		Sittings:               p.Sittings,
		InProgress:             !p.Finished && p.Sittings > 0,
		Level:                  p.Level,
		CardsRated:             p.CardsRated,
		StopReason:             p.StopReason,
		Bands:                  bands,
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
	caller, err := s.gate(ctx, "get_context")
	if err != nil {
		return nil, out, err
	}
	if caller.Role == RoleLearner {
		if in.LearnerID != nil && *in.LearnerID != caller.UserID {
			return nil, out, errors.New(msgOtherLearner)
		}
		if in.LearnerID == nil {
			id := caller.UserID
			in.LearnerID = &id
		}
	}
	if in.Position < 0 {
		return nil, out, errBadPosition
	}
	if s.course == nil {
		return nil, out, errors.New("Course storage is not connected.")
	}
	if s.lexLookup == nil {
		return nil, out, errors.New("The lexicon lookup is not connected.")
	}
	if in.LearnerID != nil && *in.LearnerID < 0 {
		return nil, out, errors.New("learnerId must be 0 or greater.")
	}
	if in.TopN < 0 {
		return nil, out, errors.New("topN must be 0 or greater. Omit it to use 30.")
	}
	outline, err := s.course.UnitsInScope(ctx, in.LearnerID)
	if err != nil {
		return nil, out, storeErr("The course outline could not be read.", err)
	}
	courseKnown, err := s.lexLookup.KnownLexemes(ctx, in.Position)
	if err != nil {
		return nil, out, storeErr("The known-word list could not be read.", err)
	}
	knownList := sortedKnown(courseKnown)
	view := placementView{}
	struggles := emptyStruggles()
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
				Finished:               row.Finished,
				Sittings:               row.Sittings,
				InProgress:             !row.Finished && row.Sittings > 0,
				Level:                  row.Level,
				CardsRated:             row.CardsRated,
				StopReason:             row.StopReason,
				Bands:                  append([]BandScore(nil), row.BandScores...),
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
		stats, err := s.course.StruggleStats(ctx, userID, topN)
		if err != nil {
			return nil, out, storeErr("The struggle list could not be read.", err)
		}
		struggles = strugglesFrom(stats)
		completed, err = s.course.CompletedSharedUnits(ctx, userID)
		if err != nil {
			return nil, out, storeErr("The completed shared units could not be read.", err)
		}
		if completed == nil {
			completed = []UnitCompletion{}
		}
		knownList, err = s.course.LearnerKnownLexemes(ctx, userID)
		if err != nil {
			return nil, out, storeErr("The learner's known words could not be read.", err)
		}
		if knownList == nil {
			knownList = []string{}
		}
	}
	out = getContextOutput{
		Outline:              outline,
		KnownLexemes:         knownList,
		Budgets:              currentBudgets(),
		Placement:            view,
		Struggles:            struggles,
		CompletedSharedUnits: completed,
	}
	return nil, out, nil
}

func emptyStruggles() Struggles {
	return Struggles{
		Lexemes:    []StruggleLexeme{},
		Confusions: []StruggleConfusion{},
	}
}

func fillStruggles(in Struggles) Struggles {
	if in.Lexemes == nil {
		in.Lexemes = []StruggleLexeme{}
	}
	if in.Confusions == nil {
		in.Confusions = []StruggleConfusion{}
	}
	for i := range in.Lexemes {
		if in.Lexemes[i].ConfusionKeys == nil {
			in.Lexemes[i].ConfusionKeys = []string{}
		}
	}
	return in
}

// strugglesFrom emits the store's Top rows in Top order.
// It does not rank them again. Confusions are copied as returned.
func strugglesFrom(stats Struggle) Struggles {
	byText := make(map[string]StruggleLexeme, len(stats.Lexemes))
	for _, row := range stats.Lexemes {
		if _, ok := byText[row.TextNFC]; ok {
			continue
		}
		row.ConfusionKeys = append([]string(nil), row.ConfusionKeys...)
		if row.ConfusionKeys == nil {
			row.ConfusionKeys = []string{}
		}
		byText[row.TextNFC] = row
	}
	lexemes := make([]StruggleLexeme, 0, len(stats.Top))
	for _, text := range stats.Top {
		row, ok := byText[text]
		if !ok {
			continue
		}
		lexemes = append(lexemes, row)
	}
	confusions := append([]StruggleConfusion(nil), stats.Confusions...)
	return fillStruggles(Struggles{Lexemes: lexemes, Confusions: confusions})
}

func (s *service) lookup(ctx context.Context, _ *mcp.CallToolRequest, in lookupInput) (*mcp.CallToolResult, lookupOutput, error) {
	var out lookupOutput
	if _, err := s.gate(ctx, "lookup"); err != nil {
		return nil, out, err
	}
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
	if _, err := s.gate(ctx, "validate_unit"); err != nil {
		return nil, validateOutput{}, err
	}
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
	if _, err := s.gate(ctx, "put_unit"); err != nil {
		return nil, out, err
	}
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
	saved, err := s.units.PutUnit(ctx, in.UnitID, &doc, scope)
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
		Description: "Read the course before you draft a unit. A learner account is limited to their own learnerId; omitting it uses that id, and another id is forbidden. An admin, including the static token, may omit learnerId or pass any id. The outline rows have position, title, status, and scope (shared, or learner:<id>). Omit learnerId as an admin to see shared units only, with empty struggles and placement taken false. knownLexemes is then the pointed text introduced before position. Pass learnerId to add that learner's custom units, their placement, their struggles, and the shared units they have finished. knownLexemes is then that learner's words: the union of placement Good and Easy words and words from completed units, sorted pointed NFC. topN defaults to 30. Struggled lexemes are the store's Top rows, ordered by misses in 7 days, then lapses, then Again rate. Do not re-rank them. Each lexeme has text, gloss, reviews, lapses, againRate, misses7d, lastSeen, and confusionKeys. Confusions cover 7 days and are not limited by topN. Each has kind, a, b, count, and lastAt. b is empty for the agreement kind. Placement taken false means there is no row. A row includes takenAt, finished, sittings (20 cards each; a run can resume across days and has no time cap), inProgress (true when finished is false and sittings is greater than 0), level, cardsRated, stopReason (band_stop, cap_cards, exhausted, or empty), bands (per band: band, rated, again, hard, good, easy, dontKnow, goodPlusRate), estimatedKnownWords, suggestedStartPosition, knownLexemes (Good and Easy, pointed NFC, sorted), and learningLexemes (Again, Hard, and don't-know, pointed NFC, sorted). suggestedStartPosition 0 is the first unit. Read hebrew://rules for budgets and for the shared-versus-custom guidance. That guidance is not an enforced rule.",
		Annotations: readOnly,
	}, svc.getContext)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup",
		Description: "Find a lexicon entry by pointed Hebrew, unpointed Hebrew, or an English gloss. Use an entry's text_nfc as the spelling in a unit. A miss returns suggestions and is not an error. Add a missing entry with upsert_lexemes.",
		Annotations: readOnly,
	}, svc.lookup)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate_unit",
		Description: "Dry run. Checks the unit document at position with the authoring rules and returns ok, block and warn counts, and issues. ok means zero block issues. This call does not save. Each issue has a rule id, severity block or warn, a JSON pointer, and a plain-English message.",
		Annotations: readOnly,
	}, svc.validateUnit)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "put_unit",
		Description: "Save a draft revision. Admin only, including the static token: a learner call is a tool error, forbidden: admin only, even for scope learner:<their own id>. scope is required: shared, or learner:<id> for one learner. Scope is not inside the unit document. Send unitId to update; omit it to create. The same document, position, and scope do not create another revision. lesson_items are still rebuilt, so a unit saved before its words were in the lexicon gains those rows without a new revision. Changing scope does create a revision. The draft is saved even when issues block publishing. Warnings are not a reason to skip the save. JSON that cannot be read as a unit is rejected and nothing is written. This does not publish.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.putUnit)
	registerUpsertLexemes(server, svc)
}
