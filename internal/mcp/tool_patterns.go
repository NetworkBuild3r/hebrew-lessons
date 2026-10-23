package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/richtext"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// patternStore is the grammar-pattern half of the database.
// A memory store does not implement it, and the tools then refuse to write.
type patternStore interface {
	UpsertPatterns(ctx context.Context, rows []db.PatternInput) error
	Patterns(ctx context.Context) ([]db.PatternInfo, error)
	Prereqs(ctx context.Context) (map[string][]string, error)
	PatternUnits(ctx context.Context) ([]db.PatternUnit, error)
	UpsertExplainer(ctx context.Context, row db.Explainer) error
	HasPattern(ctx context.Context, slug string) (bool, error)
	SetUnitMeta(ctx context.Context, id int64, pattern, worksheet *string) error
	AuditContent(ctx context.Context, actor, action, scope string) error
}

func registerPatternTools(server *mcp.Server, svc *service) {
	falseHint := false
	readOnly := &mcp.ToolAnnotations{
		ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
	write := &mcp.ToolAnnotations{
		ReadOnlyHint: false, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "upsert_patterns",
		Description: "Create or replace grammar patterns. Admin only. At most 100. Each has slug, title, summary, optional example, and prereqs (slugs that must be taught first). A cycle, a self-requirement, or an unknown prerequisite writes nothing. The write is logged.",
		Annotations: write,
	}, svc.upsertPatterns)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "upsert_explainer",
		Description: "Write the explainer for one pattern. Admin only. body is restricted markdown: h2, h3, paragraphs, bold, italic, lists, {he:...} and {he:...|tr}, and links to /learn, /roots, and /lesson only. Raw HTML, images, and any other link are refused and nothing is written. status is draft or published. The write is logged.",
		Annotations: write,
	}, svc.upsertExplainer)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_patterns",
		Description: "List grammar patterns: slug, title, prerequisites, and whether an explainer is published. Any token may call this.",
		Annotations: readOnly,
	}, svc.listPatterns)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_path_order",
		Description: "Admin only. Reports every unit whose pattern is taught before a pattern it requires, and a stable order of the units that keeps class order wherever that is valid.",
		Annotations: readOnly,
	}, svc.checkPathOrder)
}

type patternIn struct {
	Slug    string   `json:"slug" jsonschema:"Lowercase slug, letters, digits, and hyphens, up to 60 characters."`
	Title   string   `json:"title" jsonschema:"English title, up to 200 characters."`
	Summary string   `json:"summary" jsonschema:"One English sentence, up to 300 characters."`
	Example string   `json:"example,omitempty" jsonschema:"Optional pointed Hebrew example."`
	Prereqs []string `json:"prereqs,omitempty" jsonschema:"Slugs that must be taught before this one. Replaces the stored list."`
}

type upsertPatternsInput struct {
	Patterns []patternIn `json:"patterns" jsonschema:"Up to 100 patterns, written in one transaction."`
}

type upsertPatternsOutput struct {
	Written int `json:"written"`
}

func (s *service) upsertPatterns(ctx context.Context, _ *mcp.CallToolRequest, in upsertPatternsInput) (*mcp.CallToolResult, upsertPatternsOutput, error) {
	var out upsertPatternsOutput
	caller, err := s.gate(ctx, "upsert_patterns")
	if err != nil {
		return nil, out, err
	}
	store, err := s.patternDB()
	if err != nil {
		return nil, out, err
	}
	if len(in.Patterns) == 0 || len(in.Patterns) > 100 {
		return nil, out, errors.New("Send 1 to 100 patterns.")
	}
	rows := make([]db.PatternInput, len(in.Patterns))
	for i, p := range in.Patterns {
		rows[i] = db.PatternInput{Slug: p.Slug, Title: p.Title, Summary: p.Summary, Example: p.Example, Prereqs: p.Prereqs}
	}
	if err := store.UpsertPatterns(ctx, rows); err != nil {
		if errors.Is(err, db.ErrPatternCycle) {
			return nil, out, errors.New("Those prerequisites cycle, so nothing was written.")
		}
		return nil, out, err
	}
	if err := store.AuditContent(ctx, caller.Actor, "upsert_patterns", "shared"); err != nil {
		return nil, out, storeErr("The patterns were written, and the audit row was not.", err)
	}
	out.Written = len(rows)
	return nil, out, nil
}

type upsertExplainerInput struct {
	Slug   string `json:"slug" jsonschema:"Pattern slug the explainer belongs to."`
	Title  string `json:"title" jsonschema:"English title."`
	Body   string `json:"body" jsonschema:"Restricted markdown. Raw HTML is refused."`
	Status string `json:"status" jsonschema:"draft or published."`
}

type upsertExplainerOutput struct {
	Slug string `json:"slug"`
}

func (s *service) upsertExplainer(ctx context.Context, _ *mcp.CallToolRequest, in upsertExplainerInput) (*mcp.CallToolResult, upsertExplainerOutput, error) {
	var out upsertExplainerOutput
	caller, err := s.gate(ctx, "upsert_explainer")
	if err != nil {
		return nil, out, err
	}
	store, err := s.patternDB()
	if err != nil {
		return nil, out, err
	}
	if _, err := richtext.Parse(in.Body); err != nil {
		return nil, out, err
	}
	if err := store.UpsertExplainer(ctx, db.Explainer{
		Slug: in.Slug, Title: in.Title, Body: in.Body, Status: in.Status,
	}); err != nil {
		return nil, out, err
	}
	if err := store.AuditContent(ctx, caller.Actor, "upsert_explainer", "shared"); err != nil {
		return nil, out, storeErr("The explainer was written, and the audit row was not.", err)
	}
	out.Slug = in.Slug
	return nil, out, nil
}

type patternOut struct {
	Slug               string   `json:"slug"`
	Title              string   `json:"title"`
	Prereqs            []string `json:"prereqs"`
	ExplainerPublished bool     `json:"explainerPublished"`
}

type listPatternsOutput struct {
	Patterns []patternOut `json:"patterns"`
}

func (s *service) listPatterns(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listPatternsOutput, error) {
	var out listPatternsOutput
	if _, err := s.gate(ctx, "list_patterns"); err != nil {
		return nil, out, err
	}
	store, err := s.patternDB()
	if err != nil {
		return nil, out, err
	}
	rows, err := store.Patterns(ctx)
	if err != nil {
		return nil, out, storeErr("The patterns could not be read.", err)
	}
	out.Patterns = make([]patternOut, len(rows))
	for i, row := range rows {
		out.Patterns[i] = patternOut{
			Slug: row.Slug, Title: row.Title, Prereqs: row.Prereqs, ExplainerPublished: row.ExplainerPublished,
		}
	}
	return nil, out, nil
}

type pathOrderOutput struct {
	Issues []units.Issue `json:"issues"`
	Order  []int64       `json:"order"`
}

func (s *service) checkPathOrder(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, pathOrderOutput, error) {
	var out pathOrderOutput
	if _, err := s.gate(ctx, "check_path_order"); err != nil {
		return nil, out, err
	}
	course, prereqs, err := s.pathGraph(ctx)
	if err != nil {
		return nil, out, err
	}
	var issues []units.Issue
	for _, u := range course {
		if u.Pattern == "" {
			continue
		}
		found := units.PatternOrderIssues(u.Scope, u.Position, u.Pattern, course, prereqs)
		for i := range found {
			found[i].Pointer = fmt.Sprintf("/units/%d/pattern", u.ID)
		}
		issues = append(issues, found...)
	}
	out.Issues = issueList(issues)
	out.Order = units.StablePath(course, prereqs)
	if out.Order == nil {
		out.Order = []int64{}
	}
	return nil, out, nil
}

func (s *service) patternDB() (patternStore, error) {
	if s.patterns == nil {
		return nil, errors.New("Pattern storage is not connected, so nothing was written.")
	}
	return s.patterns, nil
}

func (s *service) pathGraph(ctx context.Context) ([]units.CourseUnit, map[string][]string, error) {
	store, err := s.patternDB()
	if err != nil {
		return nil, nil, err
	}
	rows, err := store.PatternUnits(ctx)
	if err != nil {
		return nil, nil, storeErr("The units could not be read.", err)
	}
	prereqs, err := store.Prereqs(ctx)
	if err != nil {
		return nil, nil, storeErr("The prerequisites could not be read.", err)
	}
	course := make([]units.CourseUnit, len(rows))
	for i, row := range rows {
		course[i] = units.CourseUnit{
			ID: row.ID, Position: row.Position, Scope: row.Scope, Title: row.Title, Pattern: row.Pattern,
		}
	}
	return course, prereqs, nil
}

// patternIssues adds unit.pattern_order and unit.worksheet_shape.
// A nil pointer means the caller omitted the field.
func (s *service) patternIssues(ctx context.Context, scope string, position int, pattern, worksheet *string, doc *units.Unit) []units.Issue {
	var out []units.Issue
	if pattern != nil && strings.TrimSpace(*pattern) != "" {
		slug := strings.TrimSpace(*pattern)
		if s.patterns == nil {
			out = append(out, unitsIssue(units.RuleSchemaLookup, "/pattern", "Pattern storage is not connected."))
		} else if ok, err := s.patterns.HasPattern(ctx, slug); err != nil {
			out = append(out, unitsIssue(units.RuleSchemaLookup, "/pattern", "The pattern could not be read."))
		} else if !ok {
			out = append(out, unitsIssue(units.RulePatternOrder, "/pattern", "Unknown pattern "+slug+"."))
		} else if course, prereqs, err := s.pathGraph(ctx); err != nil {
			out = append(out, unitsIssue(units.RuleSchemaLookup, "/pattern", "The path could not be read."))
		} else {
			out = append(out, units.PatternOrderIssues(scope, position, slug, course, prereqs)...)
		}
	}
	if worksheet != nil && strings.TrimSpace(*worksheet) != "" {
		out = append(out, units.WorksheetIssues(strings.TrimSpace(*worksheet), doc)...)
	}
	return out
}

func unitsIssue(rule, pointer, msg string) units.Issue {
	return units.Issue{Rule: rule, Severity: units.SeverityBlock, Pointer: pointer, Msg: msg}
}

// applyUnitMeta stores a known pattern and a known worksheet type.
// An unknown value is left unset. The draft is already saved.
func (s *service) applyUnitMeta(ctx context.Context, id int64, pattern, worksheet *string) error {
	if s.patterns == nil || (pattern == nil && worksheet == nil) {
		return nil
	}
	var setPattern, setSheet *string
	if pattern != nil {
		slug := strings.TrimSpace(*pattern)
		if slug == "" {
			setPattern = &slug
		} else if ok, err := s.patterns.HasPattern(ctx, slug); err != nil {
			return err
		} else if ok {
			setPattern = &slug
		}
	}
	if worksheet != nil {
		kind := strings.TrimSpace(*worksheet)
		if kind == "" || knownWorksheet(kind) {
			setSheet = &kind
		}
	}
	return s.patterns.SetUnitMeta(ctx, id, setPattern, setSheet)
}

func knownWorksheet(kind string) bool {
	switch kind {
	case units.SheetTopic, units.SheetReview, units.SheetVerb, units.SheetStory, units.SheetFoundations:
		return true
	default:
		return false
	}
}
