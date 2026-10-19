package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// This file holds the store side of learner auto-publish (INIT-015/SPEC-020):
// publish with a brief, unpublish, and the learner's report. Each function is one
// transaction, and each success writes one publish_audit row in that transaction,
// so a state change and its audit row cannot come apart.
//
// Who may act is decided here from ids the caller took from the principal or the
// unit's scope. No function reads a scope from request input.
//
// What is audited. publish_audit.action is one of auto_publish, unpublish, and
// report_hide, and the table has no action for a refused attempt. So every
// successful publish, unpublish, and hide has a row, and a refused or blocked
// attempt has none. A blocked publish is answered to the caller with its issues,
// and the publish path logs a refused one.

var (
	// ErrUnitNotInScope is returned for a unit that is missing, another learner's, or
	// shared, when the caller may only act on their own learner-scoped unit. The cases
	// are not told apart, so an id cannot be probed.
	ErrUnitNotInScope = errors.New("db: unit not found in the learner's scope")
	// ErrNoLearner means the learner id is not a real learner id (zero or less).
	ErrNoLearner = errors.New("db: no learner")
	// ErrBriefNotFound covers a brief that is missing, or another learner's.
	ErrBriefNotFound = errors.New("db: brief not found")
	// ErrBriefExpired means the brief is past its time to live.
	ErrBriefExpired = errors.New("db: brief expired")
	// ErrBriefUsed means the brief already built a unit.
	ErrBriefUsed = errors.New("db: brief already used")
	// ErrBadReport means the reason or note of a report is not acceptable.
	ErrBadReport = errors.New("db: bad report")
)

// Audit actions, as publish_audit.action allows.
const (
	AuditAutoPublish = "auto_publish"
	AuditUnpublish   = "unpublish"
	AuditReportHide  = "report_hide"
)

// Report limits, as unit_reports allows.
const (
	MaxReportReasonRunes = 100
	MaxReportNoteRunes   = 500
)

// LearnerScope is the units.scope of one learner's custom units.
func LearnerScope(learnerID int64) string {
	return "learner:" + strconv.FormatInt(learnerID, 10)
}

// AutoPublish asks AutoPublishLearnerUnit to publish one revision of a learner's
// unit against a brief.
type AutoPublish struct {
	UnitID   int64
	Revision int
	// LearnerID owns the unit and the brief. The unit's scope must be learner:<LearnerID>
	// and the brief must be this learner's. The caller takes it from the unit scope that
	// the principal was allowed to write, never from the request body.
	LearnerID int64
	BriefID   string
	// BriefNotBefore is the oldest created_at a brief may have. The caller passes now
	// minus the brief's time to live.
	BriefNotBefore time.Time
	// Actor is the client that published: the principal's actor.
	Actor string
	// IssuesJSON is the validator's issues, warnings included, as a JSON array.
	// Empty means [].
	IssuesJSON string
}

// AutoPublishLearnerUnit publishes revision of the unit, marks the brief used by
// the unit, and writes the audit row, in one transaction. Either all three happen
// or none do. The brief is claimed first with one UPDATE, so of any number of
// concurrent calls with the same brief exactly one succeeds, and the others get
// ErrBriefUsed. It does not validate: the caller has run the validator on this
// revision and passes the issues for the record.
//
// It returns ErrUnitNotInScope for a unit that is not this learner's, and the
// brief errors above. A publish of a revision that is already published still
// claims the brief and writes the row, because the brief changed state.
func (s *Store) AutoPublishLearnerUnit(ctx context.Context, p AutoPublish) (PutResult, error) {
	if p.LearnerID <= 0 {
		return PutResult{}, ErrNoLearner
	}
	if p.UnitID <= 0 || p.Revision < 1 {
		return PutResult{}, fmt.Errorf("db: auto publish: unit %d revision %d", p.UnitID, p.Revision)
	}
	if strings.TrimSpace(p.BriefID) == "" {
		return PutResult{}, ErrBriefNotFound
	}
	if strings.TrimSpace(p.Actor) == "" {
		return PutResult{}, errors.New("db: auto publish: actor is empty")
	}
	issues := p.IssuesJSON
	if issues == "" {
		issues = "[]"
	}
	if !json.Valid([]byte(issues)) {
		return PutResult{}, errors.New("db: auto publish: issues are not JSON")
	}
	conn, err := s.connOrErr()
	if err != nil {
		return PutResult{}, err
	}
	if s.clock == nil {
		return PutResult{}, errors.New("db: clock is not configured")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return PutResult{}, fmt.Errorf("db: begin auto publish: %w", err)
	}
	defer tx.Rollback()

	scope, _, _, err := unitHeadInTx(ctx, tx, p.UnitID)
	if err != nil {
		return PutResult{}, err
	}
	if scope != LearnerScope(p.LearnerID) {
		return PutResult{}, ErrUnitNotInScope
	}
	if err := claimBrief(ctx, tx, p.BriefID, p.LearnerID, p.UnitID, p.BriefNotBefore); err != nil {
		return PutResult{}, err
	}
	rev := p.Revision
	res, err := publishInTx(ctx, tx, p.UnitID, &rev)
	if err != nil {
		return PutResult{}, err
	}
	if err := insertAudit(ctx, tx, auditRow{
		unitID: p.UnitID, revision: res.Revision, scope: scope, actor: p.Actor,
		action: AuditAutoPublish, briefID: p.BriefID, issues: issues, at: FormatTime(s.clock.Now()),
	}); err != nil {
		return PutResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return PutResult{}, fmt.Errorf("db: commit auto publish: %w", err)
	}
	return res, nil
}

// claimBrief sets used_by_unit_id when the brief is this learner's, unused, and not
// expired. It is a single UPDATE, so two claims cannot both match. When it matches
// nothing, a second read says why.
func claimBrief(ctx context.Context, tx *sql.Tx, briefID string, learnerID, unitID int64, notBefore time.Time) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE lesson_briefs SET used_by_unit_id = ?
		WHERE id = ? AND user_id = ? AND used_by_unit_id IS NULL AND created_at > ?`,
		unitID, briefID, learnerID, FormatTime(notBefore))
	if err != nil {
		return fmt.Errorf("db: claim brief: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: claim brief: %w", err)
	}
	if n == 1 {
		return nil
	}
	var used sql.NullInt64
	err = tx.QueryRowContext(ctx,
		`SELECT used_by_unit_id FROM lesson_briefs WHERE id = ? AND user_id = ?`, briefID, learnerID).Scan(&used)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrBriefNotFound
	case err != nil:
		return fmt.Errorf("db: claim brief: %w", err)
	case used.Valid:
		return ErrBriefUsed
	default:
		return ErrBriefExpired
	}
}

// UnpublishResult is what UnpublishLearnerUnit reports. Unchanged is true when
// the unit was not published, so nothing changed and no audit row was written.
type UnpublishResult struct {
	UnitID    int64
	Revision  int
	Unchanged bool
}

// UnpublishLearnerUnit takes one of the learner's own units off their Today and
// writes an unpublish audit row, in one transaction. learnerID comes from the
// principal. The unit must be scoped learner:<learnerID>; a shared unit, another
// learner's unit, and a missing unit are all ErrUnitNotInScope, with nothing
// written. The unit goes back to draft, which is the state a save leaves it in.
// It does not reopen the brief: a brief builds one unit.
func (s *Store) UnpublishLearnerUnit(ctx context.Context, learnerID, unitID int64, actor string) (UnpublishResult, error) {
	if learnerID <= 0 {
		return UnpublishResult{}, ErrNoLearner
	}
	if strings.TrimSpace(actor) == "" {
		return UnpublishResult{}, errors.New("db: unpublish: actor is empty")
	}
	conn, err := s.connOrErr()
	if err != nil {
		return UnpublishResult{}, err
	}
	if s.clock == nil {
		return UnpublishResult{}, errors.New("db: clock is not configured")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return UnpublishResult{}, fmt.Errorf("db: begin unpublish: %w", err)
	}
	defer tx.Rollback()

	scope, status, rev, err := unitHeadInTx(ctx, tx, unitID)
	if errors.Is(err, ErrUnitNotFound) || err == nil && scope != LearnerScope(learnerID) {
		return UnpublishResult{}, ErrUnitNotInScope
	}
	if err != nil {
		return UnpublishResult{}, err
	}
	if status != "published" {
		return UnpublishResult{UnitID: unitID, Revision: rev, Unchanged: true}, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE units SET status = 'draft' WHERE id = ?`, unitID); err != nil {
		return UnpublishResult{}, fmt.Errorf("db: unpublish: %w", err)
	}
	if err := insertAudit(ctx, tx, auditRow{
		unitID: unitID, revision: rev, scope: scope, actor: actor,
		action: AuditUnpublish, issues: "[]", at: FormatTime(s.clock.Now()),
	}); err != nil {
		return UnpublishResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return UnpublishResult{}, fmt.Errorf("db: commit unpublish: %w", err)
	}
	return UnpublishResult{UnitID: unitID, Revision: rev}, nil
}

// Report is a learner's "Report a problem" on a unit.
type Report struct {
	// UserID is the reporter. The web handler takes it from the session.
	UserID int64
	UnitID int64
	// Reason is a short reason or a reason code, 1 to MaxReportReasonRunes characters.
	Reason string
	// Note is optional free text, at most MaxReportNoteRunes characters.
	Note string
}

// ReportResult is what ReportUnit reports. Hidden is true when this report changed
// what the reporter sees: their own unit was taken down, or a shared unit became
// hidden for them. A repeat report is recorded and Hidden is false.
type ReportResult struct {
	ReportID int64
	Hidden   bool
}

// ReportUnit records a report in unit_reports and hides the unit for the reporter,
// in one transaction. It is the store half of SPEC-017's report handler.
//
//   - A unit scoped to the reporter is unpublished, because it is theirs: the unit
//     goes back to draft for them and for anyone else, which is only them.
//   - A shared unit is hidden for the reporter only. The shared unit is not touched.
//     The hide is the unit_reports row: HiddenUnitIDs lists the units a learner has
//     reported, and the Today and lesson lists must leave those out. No new table is
//     needed, but no list reads HiddenUnitIDs yet. That filter is web code, outside
//     SPEC-020, so until it lands a reported shared unit still shows for the reporter.
//   - A unit that is another learner's, a shared draft, or missing is ErrUnitNotInScope.
//
// Each change of state writes a report_hide audit row whose actor is learner:<id>.
func (s *Store) ReportUnit(ctx context.Context, r Report) (ReportResult, error) {
	if r.UserID <= 0 {
		return ReportResult{}, ErrNoLearner
	}
	reason, note, err := cleanReport(r.Reason, r.Note)
	if err != nil {
		return ReportResult{}, err
	}
	conn, err := s.connOrErr()
	if err != nil {
		return ReportResult{}, err
	}
	if s.clock == nil {
		return ReportResult{}, errors.New("db: clock is not configured")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return ReportResult{}, fmt.Errorf("db: begin report: %w", err)
	}
	defer tx.Rollback()

	scope, status, rev, err := unitHeadInTx(ctx, tx, r.UnitID)
	if errors.Is(err, ErrUnitNotFound) {
		return ReportResult{}, ErrUnitNotInScope
	}
	if err != nil {
		return ReportResult{}, err
	}
	own := scope == LearnerScope(r.UserID)
	shared := scope == "shared" && status == "published"
	if !own && !shared {
		return ReportResult{}, ErrUnitNotInScope
	}
	var before int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM unit_reports WHERE unit_id = ? AND user_id = ?`, r.UnitID, r.UserID).Scan(&before); err != nil {
		return ReportResult{}, fmt.Errorf("db: report: %w", err)
	}
	now := FormatTime(s.clock.Now())
	res, err := tx.ExecContext(ctx,
		`INSERT INTO unit_reports (unit_id, user_id, reason, note, created_at) VALUES (?, ?, ?, ?, ?)`,
		r.UnitID, r.UserID, reason, note, now)
	if err != nil {
		return ReportResult{}, fmt.Errorf("db: report: %w", err)
	}
	reportID, err := res.LastInsertId()
	if err != nil {
		return ReportResult{}, fmt.Errorf("db: report: %w", err)
	}
	hidden := false
	switch {
	case own && status == "published":
		if _, err := tx.ExecContext(ctx, `UPDATE units SET status = 'draft' WHERE id = ?`, r.UnitID); err != nil {
			return ReportResult{}, fmt.Errorf("db: report: %w", err)
		}
		hidden = true
	case shared && before == 0:
		hidden = true
	}
	if hidden {
		if err := insertAudit(ctx, tx, auditRow{
			unitID: r.UnitID, revision: rev, scope: scope, actor: LearnerScope(r.UserID),
			action: AuditReportHide, issues: "[]", at: now,
		}); err != nil {
			return ReportResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ReportResult{}, fmt.Errorf("db: commit report: %w", err)
	}
	return ReportResult{ReportID: reportID, Hidden: hidden}, nil
}

// HiddenUnitIDs lists the units this learner has reported, in id order. A reported
// shared unit is hidden for this learner only, so a list of units for them leaves
// these out.
func (s *Store) HiddenUnitIDs(ctx context.Context, userID int64) ([]int64, error) {
	if userID <= 0 {
		return nil, ErrNoLearner
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx,
		`SELECT DISTINCT unit_id FROM unit_reports WHERE user_id = ? ORDER BY unit_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: hidden units: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db: hidden units: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: hidden units: %w", err)
	}
	return out, nil
}

func cleanReport(reason, note string) (string, string, error) {
	reason, note = strings.TrimSpace(reason), strings.TrimSpace(note)
	switch {
	case !utf8.ValidString(reason) || !utf8.ValidString(note):
		return "", "", fmt.Errorf("%w: not valid UTF-8", ErrBadReport)
	case reason == "":
		return "", "", fmt.Errorf("%w: the reason is empty", ErrBadReport)
	case utf8.RuneCountInString(reason) > MaxReportReasonRunes:
		return "", "", fmt.Errorf("%w: the reason is longer than %d characters", ErrBadReport, MaxReportReasonRunes)
	case utf8.RuneCountInString(note) > MaxReportNoteRunes:
		return "", "", fmt.Errorf("%w: the note is longer than %d characters", ErrBadReport, MaxReportNoteRunes)
	}
	for _, s := range []string{reason, note} {
		for _, c := range s {
			if unicode.IsControl(c) && c != '\n' && c != '\t' {
				return "", "", fmt.Errorf("%w: contains a control character", ErrBadReport)
			}
		}
	}
	return reason, note, nil
}

type auditRow struct {
	unitID   int64
	revision int
	scope    string
	actor    string
	action   string
	briefID  string
	issues   string
	at       string
}

func insertAudit(ctx context.Context, tx *sql.Tx, a auditRow) error {
	var brief any
	if a.briefID != "" {
		brief = a.briefID
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO publish_audit (unit_id, revision, scope, actor, action, brief_id, issues_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.unitID, a.revision, a.scope, a.actor, a.action, brief, a.issues, a.at); err != nil {
		return fmt.Errorf("db: publish audit: %w", err)
	}
	return nil
}

// unitHeadInTx reads the scope, status, and current revision of one unit.
// ErrUnitNotFound means no such unit.
func unitHeadInTx(ctx context.Context, tx *sql.Tx, id int64) (scope, status string, rev int, err error) {
	err = tx.QueryRowContext(ctx, `SELECT scope, status, rev FROM units WHERE id = ?`, id).Scan(&scope, &status, &rev)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", 0, ErrUnitNotFound
	}
	if err != nil {
		return "", "", 0, fmt.Errorf("db: read unit: %w", err)
	}
	return scope, status, rev, nil
}
