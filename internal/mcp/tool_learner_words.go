package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type learnerWordsInput struct {
	LearnerID *int64     `json:"learnerId,omitempty" jsonschema:"Optional for a learner account, which may only read their own id. Omitting it uses that id. Another id is forbidden. Required for an admin, including the static token. A whole number is accepted, including 9.0. A fraction such as 9.5 is rejected."`
	Cursor    string     `json:"cursor,omitempty" jsonschema:"Opaque nextCursor from the previous page. Omit it for the first page."`
	Limit     int        `json:"limit,omitempty" jsonschema:"Page size. Defaults to 50 and will not exceed 200. Zero or a negative value uses the default."`
	State     string     `json:"state,omitempty" jsonschema:"Optional card state: new, learning, review, or relearning. Omit it for every state."`
	DueBefore *time.Time `json:"dueBefore,omitempty" jsonschema:"Optional UTC instant. Only cards due strictly before this time are returned."`
}

type learnerWordsOutput struct {
	Cards      []LearnerCard `json:"cards"`
	NextCursor *string       `json:"nextCursor"`
}

func (s *service) attachCoach(ctx context.Context, out *getContextOutput, learnerID *int64) error {
	out.Cards = []LearnerCard{}
	out.MixTarget = currentMixTarget()
	if learnerID == nil {
		return nil
	}
	if s.course == nil {
		return errors.New("Course storage is not connected.")
	}
	page, err := s.course.LearnerCards(ctx, CardQuery{UserID: *learnerID, Limit: contextCardCap})
	if err != nil {
		return coachStoreErr(err, "The learner's cards could not be read.")
	}
	if page.Cards == nil {
		page.Cards = []LearnerCard{}
	}
	out.Cards = page.Cards
	if page.NextCursor != "" {
		out.CardsTruncated = true
		out.CardsHint = cardsTruncatedHint
	}
	acc, err := s.course.RecentAccuracy(ctx, *learnerID)
	if err != nil {
		return storeErr("The learner's recent accuracy could not be read.", err)
	}
	out.RecentAccuracy = &acc
	return nil
}

func (s *service) getLearnerWords(ctx context.Context, _ *mcp.CallToolRequest, in learnerWordsInput) (*mcp.CallToolResult, learnerWordsOutput, error) {
	var out learnerWordsOutput
	caller, err := s.gate(ctx, "get_learner_words")
	if err != nil {
		return nil, out, err
	}
	learnerID, err := learnerForWords(caller, in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	if !validCardState(in.State) {
		return nil, out, errors.New("state must be new, learning, review, or relearning.")
	}
	if s.course == nil {
		return nil, out, errors.New("Course storage is not connected.")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = learnerWordsDefaultLimit
	}
	if limit > learnerWordsMaxLimit {
		limit = learnerWordsMaxLimit
	}
	page, err := s.course.LearnerCards(ctx, CardQuery{
		UserID:    learnerID,
		Cursor:    in.Cursor,
		Limit:     limit,
		State:     in.State,
		DueBefore: in.DueBefore,
	})
	if err != nil {
		return nil, out, coachStoreErr(err, "The learner's cards could not be read.")
	}
	cards := page.Cards
	if cards == nil {
		cards = []LearnerCard{}
	}
	var next *string
	if page.NextCursor != "" {
		cursor := page.NextCursor
		next = &cursor
	}
	return nil, learnerWordsOutput{Cards: cards, NextCursor: next}, nil
}

// learnerForWords applies the same learnerId rule as get_context, and
// requires an id from an admin. A learner who omits it reads their own cards.
func learnerForWords(caller Principal, id *int64) (int64, error) {
	if caller.Role == RoleLearner {
		if id != nil && *id != caller.UserID {
			return 0, errors.New(msgOtherLearner)
		}
		return caller.UserID, nil
	}
	if id == nil {
		return 0, errors.New("learnerId is required.")
	}
	if *id < 0 {
		return 0, errors.New("learnerId must be 0 or greater.")
	}
	return *id, nil
}

func coachStoreErr(err error, sentence string) error {
	if errors.Is(err, ErrBadCardCursor) {
		return ErrBadCardCursor
	}
	return storeErr(sentence, err)
}

func learnerWordsDescription() string {
	return fmt.Sprintf("Page one learner's FSRS cards. The card shape matches get_context cards: lexeme, translit, pos, gloss, skill, state, stability, difficulty, due, lapses, reps, lastRating, and lastReview. Times are UTC RFC3339. translit, lastRating, and lastReview are null when unknown. A learner account may omit learnerId and then reads their own cards; another id is forbidden. An admin, including the static token, must pass learnerId and may read anyone. limit defaults to %d and will not exceed %d. state is new, learning, review, or relearning. dueBefore keeps cards due strictly before that instant. Order is due soonest first, then most recently reviewed, then card id, so pages do not skip or repeat. nextCursor is null on the last page. Call this when get_context cardsTruncated is true. Recycle cards that are due. Reinforce words with high lapses.", learnerWordsDefaultLimit, learnerWordsMaxLimit)
}

func registerLearnerWords(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_learner_words",
		Description: learnerWordsDescription(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.getLearnerWords)
}
