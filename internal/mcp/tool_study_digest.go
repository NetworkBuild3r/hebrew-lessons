package mcp

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// INIT-026/SPEC-009: get_study_digest and the hebrew://tutor resource.

const resourceTutor = "hebrew://tutor"

const studyDigestDescription = "Read one learner's study digest for a tutor conversation: zone, today, next study-day cutoff, live skill counts, retention against the 85–90% band, minutes in app, struggle words, dated corrections with closed-loop status, recently taught words, due cards, coach notes, history of words-you-know, and the newest weekly report. " +
	"Every number is computed by the app. Do not recompute these numbers. " +
	"A learner account reads its own digest, and omitting learnerId uses its own id. Another id is forbidden. " +
	"An admin, including the static token, must pass learnerId and may read any learner. A learner id that does not exist is an error. " +
	"zone and zoneSource name the IANA zone used for today and every day in the reply (learner when stored, otherwise app). There is no UTC invent-today. " +
	"asOf is the request instant for the live counts. knownAnySkill is words known or solid in at least one skill. " +
	"skills holds, per skill, direction (ADR-0028 D4 label), knownSolidNow and strugglingNow from the live learner model, knownSolid7dAgo from the snapshot on agoDay (seven learner days back), and agoDay names that day. Skill keys stay listen, read, recall, spell. " +
	"history is the last 14 daily_totals days plus today's live knownAnySkill point, each with ruleVersion (3 under current rules; today's live point uses the current RuleVersion). The last point equals knownAnySkill. " +
	"retention holds, per skill, the last seven learner days against the 85–90% band; each day value has window 30d due reviews and asOf for that day. " +
	"minutesInApp is the last seven learner days of answering time, using the same idle-gap rule as the weekly job. " +
	"struggle is at most ten struggling words with cause and confusedWith. " +
	"corrections is at most twenty from the last thirty learner days by last_on; status is right_since with a count once an in-app correct recall or spell happens after the end of last_on in the learner zone, otherwise open. closedLoop summarises those rows. " +
	"taught is at most thirty from the last fourteen learner days. due is cards due now and by nextCutoff. " +
	"coachNotes is the newest five with authorRole. lastWeek is the newest weekly payload when one exists; its retention uses window week. " +
	"At most 60 calls a minute across the learner tools."

//go:embed resources/tutor.md
var tutorGuide string

// DigestReader is the store half of get_study_digest. Memory does not implement it.
type DigestReader interface {
	StudyDigest(ctx context.Context, learnerID int64, now time.Time, z studyday.Zone) (StudyDigest, error)
}

type studyDigestInput struct {
	LearnerID *int64 `json:"learnerId,omitempty" jsonschema:"Required for an admin, including the static token. A learner account may omit it, and may only use its own id. A whole number is accepted, including 9.0. A fraction such as 9.5 is rejected."`
}

// StudyDigest is the get_study_digest reply. INIT-026/SPEC-009; live now counts INIT-032/SPEC-005.
type StudyDigest struct {
	Zone       string    `json:"zone"`
	ZoneSource string    `json:"zoneSource"`
	Today      string    `json:"today"`
	NextCutoff time.Time `json:"nextCutoff"`
	// AsOf is the request instant for knownSolidNow, strugglingNow, and knownAnySkill (ADR-0027 D4).
	AsOf time.Time `json:"asOf" jsonschema:"UTC RFC3339 instant when the live skill counts were read."`
	// KnownAnySkill is words known or solid in at least one skill (ADR-0027 D1).
	KnownAnySkill int                  `json:"knownAnySkill" jsonschema:"Words known or solid in at least one skill."`
	Skills        []DigestSkill        `json:"skills"`
	History       []DigestHistoryPoint `json:"history" jsonschema:"Last 14 daily_totals days plus today's live knownAnySkill point, each with ruleVersion."`
	Retention     DigestRetention      `json:"retention"`
	MinutesInApp  int                  `json:"minutesInApp"`
	Struggle      []DigestStruggleWord `json:"struggle"`
	Corrections   []DigestCorrection   `json:"corrections"`
	ClosedLoop    DigestClosedLoop     `json:"closedLoop"`
	Taught        []DigestTaughtWord   `json:"taught"`
	Due           DigestDue            `json:"due"`
	CoachNotes    []CoachNote          `json:"coachNotes"`
	LastWeek      *ReportView          `json:"lastWeek,omitempty"`
}

// DigestSkill is known+solid and struggling counts for one skill.
// knownSolidNow and strugglingNow are live; knownSolid7dAgo is from the agoDay snapshot.
type DigestSkill struct {
	Skill           string `json:"skill"`
	Direction       string `json:"direction,omitempty" jsonschema:"ADR-0028 D4 label for the skill."`
	KnownSolidNow   int    `json:"knownSolidNow" jsonschema:"Live known plus solid for this skill from the learner model."`
	KnownSolid7dAgo int    `json:"knownSolid7dAgo" jsonschema:"Known plus solid on agoDay from the daily snapshot."`
	StrugglingNow   int    `json:"strugglingNow" jsonschema:"Live struggling count for this skill from the learner model."`
	AgoDay          string `json:"agoDay" jsonschema:"The learner civil day seven days before today that knownSolid7dAgo is from."`
}

// DigestRetention is per-skill retention over the last seven learner days against 85–90%.
type DigestRetention struct {
	BandLow  float64                `json:"bandLow"`
	BandHigh float64                `json:"bandHigh"`
	Skills   []DigestRetentionSkill `json:"skills"`
}

// DigestRetentionSkill is one skill's day values and band status from the newest day with data.
type DigestRetentionSkill struct {
	Skill      string               `json:"skill"`
	Days       []DigestRetentionDay `json:"days"`
	BandStatus string               `json:"bandStatus" jsonschema:"below, in_band, above, or none when there is no retention."`
}

// DigestRetentionDay is one day's retention for a skill.
type DigestRetentionDay struct {
	Day    string   `json:"day"`
	Value  *float64 `json:"value"`
	Window string   `json:"window" jsonschema:"30d due reviews."`
	AsOf   string   `json:"asOf" jsonschema:"The learner civil day this share is as of (same as day)."`
}

// DigestStruggleWord is one struggling word for a tutor.
type DigestStruggleWord struct {
	Text         string        `json:"text"`
	Gloss        string        `json:"gloss"`
	Cause        string        `json:"cause,omitempty"`
	ConfusedWith *ConfusedWith `json:"confusedWith,omitempty"`
	LastSeen     *time.Time    `json:"lastSeen"`
}

// DigestCorrection is one dated personal correction.
type DigestCorrection struct {
	Wrong  string                 `json:"wrong"`
	Right  string                 `json:"right"`
	Note   string                 `json:"note,omitempty"`
	On     string                 `json:"on"`
	Times  int                    `json:"times"`
	Word   string                 `json:"word,omitempty" jsonschema:"Pointed Hebrew of the linked lexeme when set."`
	Status DigestCorrectionStatus `json:"status"`
}

// DigestCorrectionStatus is open, or right_since with a count of later correct recall/spell attempts.
type DigestCorrectionStatus struct {
	Kind  string `json:"kind" jsonschema:"open or right_since."`
	Count int    `json:"count,omitempty" jsonschema:"Present only when kind is right_since."`
}

// DigestClosedLoop summarises the corrections window.
type DigestClosedLoop struct {
	Since      string `json:"since"`
	Total      int    `json:"total"`
	RightSince int    `json:"rightSince"`
}

// DigestTaughtWord is one outside-taught word and its cards.
type DigestTaughtWord struct {
	Text  string             `json:"text"`
	On    string             `json:"on"`
	Cards []DigestTaughtCard `json:"cards"`
}

// DigestTaughtCard is one skill card created or already present for a taught word.
type DigestTaughtCard struct {
	Skill   string `json:"skill"`
	State   string `json:"state"`
	Reviews int    `json:"reviews" jsonschema:"review_log rows for this card since taught_on."`
}

// DigestDue is due-card counts now and by the next study-day cutoff.
type DigestDue struct {
	Now      int `json:"now"`
	ByCutoff int `json:"byCutoff"`
}

func (s *service) getStudyDigest(ctx context.Context, _ *mcp.CallToolRequest, in studyDigestInput) (*mcp.CallToolResult, StudyDigest, error) {
	var out StudyDigest
	_, id, _, err := s.learnerTool(ctx, "get_study_digest", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	if s.digest == nil {
		return nil, out, errors.New("The study digest is not connected.")
	}
	z, err := s.learnerZone(ctx, id)
	if err != nil {
		return nil, out, err
	}
	out, err = s.digest.StudyDigest(ctx, id, s.now(), z)
	if err != nil {
		return nil, out, storeErr("The learner's study digest could not be read.", err)
	}
	if out.Skills == nil {
		out.Skills = []DigestSkill{}
	}
	if out.History == nil {
		out.History = []DigestHistoryPoint{}
	}
	if out.Retention.Skills == nil {
		out.Retention.Skills = []DigestRetentionSkill{}
	}
	if out.Struggle == nil {
		out.Struggle = []DigestStruggleWord{}
	}
	if out.Corrections == nil {
		out.Corrections = []DigestCorrection{}
	}
	if out.Taught == nil {
		out.Taught = []DigestTaughtWord{}
	}
	if out.CoachNotes == nil {
		out.CoachNotes = []CoachNote{}
	}
	return nil, out, nil
}

func registerStudyDigest(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_study_digest",
		Description: studyDigestDescription,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.getStudyDigest)
}
