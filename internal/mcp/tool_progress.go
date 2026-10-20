package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const progressDescription = "Read one learner's progress over a range of days: snapshots, regression events, what was gained and lost, retention against the target, placement history, and weekly reports. " +
	"A learner account reads its own progress, and omitting learnerId uses its own id. Another id is forbidden. " +
	"An admin, including the static token, must pass learnerId and may read any learner. A learner id that does not exist is an error. " +
	"from and to are days written YYYY-MM-DD, both included, in UTC. to defaults to today and from defaults to 29 days before to. " +
	"from after to, a day that is not a real date, or a range of more than 366 days is an error. " +
	"snapshots has one entry per day that has data, each with the words per state, estimatedKnown, and retention for each skill. " +
	"changes has, for each skill, the known words at the first and last snapshot in the range and the net gained or lost. A week with no snapshot has no change. " +
	"regressions lists the words that fell back, newest first and at most 200, and any band that slipped. " +
	"retention is the real retention of the newest snapshot in the range, its gap from the target, and the day it is from. The target is 0.85. " +
	"placements lists the finished placement runs, newest first and at most 20, whatever the range. " +
	"weeklyReports lists the summaries of the weeks that start inside the range. " +
	"A learner with no snapshots gets empty lists and a null actual retention, not an error. " +
	"At most 60 calls a minute across the learner tools."

type progressInput struct {
	LearnerID *int64 `json:"learnerId,omitempty" jsonschema:"Required for an admin, including the static token. A learner account may omit it, and may only use its own id. A whole number is accepted, including 9.0. A fraction such as 9.5 is rejected."`
	From      string `json:"from,omitempty" jsonschema:"First day, YYYY-MM-DD, UTC. Defaults to 29 days before to."`
	To        string `json:"to,omitempty" jsonschema:"Last day, YYYY-MM-DD, UTC. Defaults to today. The range from to to is at most 366 days."`
}

type progressOutput struct {
	From          string           `json:"from"`
	To            string           `json:"to"`
	Snapshots     []SnapshotDay    `json:"snapshots"`
	Changes       []SkillChange    `json:"changes"`
	Regressions   []RegressionView `json:"regressions"`
	Retention     RetentionView    `json:"retention"`
	Placements    []PlacementView  `json:"placements"`
	WeeklyReports []ReportView     `json:"weeklyReports"`
}

func (s *service) getProgress(ctx context.Context, _ *mcp.CallToolRequest, in progressInput) (*mcp.CallToolResult, progressOutput, error) {
	var out progressOutput
	_, id, data, err := s.learnerTool(ctx, "get_progress", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	from, to, err := progressRange(in.From, in.To, s.now())
	if err != nil {
		return nil, out, err
	}
	p, err := data.Progress(ctx, id, from, to)
	if err != nil {
		return nil, out, storeErr("The learner's progress could not be read.", err)
	}
	out = progressOutput{
		From: from, To: to,
		Snapshots: p.Snapshots, Changes: p.Changes, Regressions: p.Regressions,
		Retention: p.Retention, Placements: p.Placements, WeeklyReports: p.WeeklyReports,
	}
	if out.Snapshots == nil {
		out.Snapshots = []SnapshotDay{}
	}
	if out.Changes == nil {
		out.Changes = []SkillChange{}
	}
	if out.Regressions == nil {
		out.Regressions = []RegressionView{}
	}
	if out.Placements == nil {
		out.Placements = []PlacementView{}
	}
	if out.WeeklyReports == nil {
		out.WeeklyReports = []ReportView{}
	}
	return nil, out, nil
}

// progressRange checks the days and fills the defaults. A day is YYYY-MM-DD and a real date.
func progressRange(fromIn, toIn string, now time.Time) (from, to string, err error) {
	today := now.UTC().Truncate(24 * time.Hour)
	toDay := today
	if t := strings.TrimSpace(toIn); t != "" {
		if toDay, err = parseDay(t); err != nil {
			return "", "", errors.New("to must be a day written YYYY-MM-DD.")
		}
	}
	fromDay := toDay.AddDate(0, 0, -(defaultProgressDays - 1))
	if f := strings.TrimSpace(fromIn); f != "" {
		if fromDay, err = parseDay(f); err != nil {
			return "", "", errors.New("from must be a day written YYYY-MM-DD.")
		}
	}
	if fromDay.After(toDay) {
		return "", "", errors.New("from must not be after to.")
	}
	if toDay.Sub(fromDay) > time.Duration(maxProgressDays)*24*time.Hour {
		return "", "", fmt.Errorf("The range from from to to is at most %d days.", maxProgressDays)
	}
	return fromDay.Format(time.DateOnly), toDay.Format(time.DateOnly), nil
}

// parseDay reads a day written YYYY-MM-DD. The layout is strict: two-digit month and day.
func parseDay(s string) (time.Time, error) {
	if len(s) != len(time.DateOnly) {
		return time.Time{}, errors.New("bad day")
	}
	return time.Parse(time.DateOnly, s)
}

func registerProgress(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_progress",
		Description: progressDescription,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.getProgress)
}
