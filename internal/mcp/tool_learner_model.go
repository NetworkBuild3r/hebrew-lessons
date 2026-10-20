package mcp

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// learnerModelDefaultLimit is the page size when limit is omitted or not positive.
	learnerModelDefaultLimit = 50
	// learnerModelMaxLimit is the largest page. A bigger limit is cut to it.
	learnerModelMaxLimit = 200
)

const learnerModelDescription = "Read what the app knows about one learner's words, one page at a time. " +
	"A learner account reads its own words, and omitting learnerId uses its own id. Another id is forbidden. " +
	"An admin, including the static token, must pass learnerId and may read any learner. A learner id that does not exist is an error. " +
	"Each word has its pointed text, gloss, band (0 to 4, or null when the word is not in the placement deck), its overall state, and a state for each of the four skills: listen, read, recall, and spell. " +
	"The overall state is the weakest skill that has evidence: unseen, seen, struggling, regressed, learning, known, or solid. " +
	"Each skill has attempts, accuracy over the newest answers, lastSeen, and stability in days. " +
	"cause says why a struggling or regressed word is failing: decoding, production, spelling, confusion, or band_slip. It is left out when no pattern fits. " +
	"state keeps words whose overall state matches. With skill set, state is the state of that skill. band keeps one band. " +
	"limit defaults to 50 and is cut to 200. limit in the reply is the page size that was used. " +
	"nextCursor is null on the last page. Send it back as cursor, with the same filters, to read the next page. " +
	"An unknown state, skill, or band, a cursor that this server did not issue, or a cursor longer than 256 characters is an error. " +
	"Workflow: call get_lesson_brief for the word plan, and call this tool to look at the words behind it. " +
	"At most 60 calls a minute across the learner tools."

type learnerModelInput struct {
	LearnerID *int64 `json:"learnerId,omitempty" jsonschema:"Required for an admin, including the static token. A learner account may omit it, and may only use its own id. A whole number is accepted, including 9.0. A fraction such as 9.5 is rejected."`
	State     string `json:"state,omitempty" jsonschema:"Keep words in this state: unseen, seen, struggling, regressed, learning, known, or solid. Omit it for every state."`
	Skill     string `json:"skill,omitempty" jsonschema:"listen, read, recall, or spell. With state, the state of this skill is filtered. Without state it has no effect on which words are returned."`
	Band      *int   `json:"band,omitempty" jsonschema:"Keep words of one placement band, 0 to 4. Words that are not in the placement deck have no band and are left out when this is set."`
	Cursor    string `json:"cursor,omitempty" jsonschema:"Opaque nextCursor from the previous page. Omit it for the first page."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Page size. Defaults to 50 and is cut to 200. Zero or a negative value uses the default."`
}

type learnerModelOutput struct {
	Words      []ModelWord `json:"words"`
	NextCursor *string     `json:"nextCursor"`
	Limit      int         `json:"limit" jsonschema:"The page size that was used."`
}

func (s *service) getLearnerModel(ctx context.Context, _ *mcp.CallToolRequest, in learnerModelInput) (*mcp.CallToolResult, learnerModelOutput, error) {
	var out learnerModelOutput
	_, id, data, err := s.learnerTool(ctx, "get_learner_model", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	q, err := modelQueryFrom(in)
	if err != nil {
		return nil, out, err
	}
	page, err := data.LearnerModel(ctx, id, q)
	if errors.Is(err, ErrBadModelCursor) {
		return nil, out, ErrBadModelCursor
	}
	if err != nil {
		return nil, out, storeErr("The learner's words could not be read.", err)
	}
	out.Words = page.Words
	if out.Words == nil {
		out.Words = []ModelWord{}
	}
	for i := range out.Words {
		if out.Words[i].Skills == nil {
			out.Words[i].Skills = []ModelSkill{}
		}
	}
	if page.NextCursor != "" {
		next := page.NextCursor
		out.NextCursor = &next
	}
	out.Limit = q.Limit
	return nil, out, nil
}

// modelQueryFrom checks and clamps the filters. The store gets only values it can use.
func modelQueryFrom(in learnerModelInput) (ModelQuery, error) {
	q := ModelQuery{State: in.State, Skill: in.Skill, Band: in.Band, Cursor: in.Cursor, Limit: in.Limit}
	if q.State != "" && !learner.State(q.State).Valid() {
		return q, fmt.Errorf("state must be one of %s.", joinStates())
	}
	if q.Skill != "" && !slices.Contains(learner.Skills, q.Skill) {
		return q, errors.New("skill must be listen, read, recall, or spell.")
	}
	if q.Band != nil && (*q.Band < 0 || *q.Band > 4) {
		return q, errors.New("band must be 0 to 4.")
	}
	if len(q.Cursor) > maxCursorLen {
		return q, ErrBadModelCursor
	}
	if q.Limit <= 0 {
		q.Limit = learnerModelDefaultLimit
	}
	if q.Limit > learnerModelMaxLimit {
		q.Limit = learnerModelMaxLimit
	}
	return q, nil
}

func joinStates() string {
	var s string
	for i, st := range learner.States {
		if i > 0 {
			s += ", "
		}
		s += string(st)
	}
	return s
}

func registerLearnerModel(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_learner_model",
		Description: learnerModelDescription,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.getLearnerModel)
}
