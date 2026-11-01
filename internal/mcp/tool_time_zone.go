package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// INIT-026/SPEC-007: set_time_zone stores the learner's IANA zone that every
// reported civil day uses.

const timeZoneDescription = "Store one learner's IANA time zone, for example America/Denver. " +
	"That zone decides every day the app reports for the learner: progress ranges, weekly reports, and today. " +
	"A learner account sets its own zone, and omitting learnerId uses its own id. Another id is forbidden. " +
	"An admin, including the static token, must pass learnerId and may set any learner. A learner id that does not exist is an error. " +
	"zone is required. An empty value, Local, or a name that is not a real IANA zone is an error and nothing is written. " +
	"The reply is the stored zone and that learner's today in it. " +
	"At most 60 calls a minute across the learner tools."

type timeZoneInput struct {
	LearnerID *int64 `json:"learnerId,omitempty" jsonschema:"Required for an admin, including the static token. A learner account may omit it, and may only use its own id. A whole number is accepted, including 9.0. A fraction such as 9.5 is rejected."`
	Zone      string `json:"zone" jsonschema:"IANA time zone name, for example America/Denver."`
}

type timeZoneOutput struct {
	Zone  string `json:"zone"`
	Today string `json:"today" jsonschema:"The learner's civil day YYYY-MM-DD in the stored zone."`
}

func (s *service) setTimeZone(ctx context.Context, _ *mcp.CallToolRequest, in timeZoneInput) (*mcp.CallToolResult, timeZoneOutput, error) {
	var out timeZoneOutput
	_, id, data, err := s.learnerTool(ctx, "set_time_zone", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	zone := strings.TrimSpace(in.Zone)
	loc, err := studyday.ParseZone(zone)
	if err != nil {
		return nil, out, errors.New("zone must be an IANA time zone name, for example America/Denver.")
	}
	if err := data.SetUserTimeZone(ctx, id, zone); err != nil {
		return nil, out, storeErr("The learner's time zone could not be stored.", err)
	}
	z := studyday.Zone{Name: zone, Loc: loc, Source: studyday.SourceLearner}
	out = timeZoneOutput{Zone: zone, Today: studyday.Today(s.now(), z)}
	return nil, out, nil
}

func registerTimeZone(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_time_zone",
		Description: timeZoneDescription,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.setTimeZone)
}
