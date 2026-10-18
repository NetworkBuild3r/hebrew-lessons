package mcp

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Queries use the read-only *sql.DB from go/internal/db. Nothing here writes.
// A missing table is reported as an explicit unavailable result, not Node's
// "no such table" string from an unmigrated process.

func dbUnavailable(tool, table string) string {
	return tool + " was not run: table " + table + " is not readable (go/internal/db is read-only and does not migrate). Nothing was substituted."
}

func (a *App) listUsers() (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("list_users", "users"))
	}
	rows, err := a.db.Query(`SELECT username, display_name, status FROM users ORDER BY username ASC`)
	if err != nil {
		return Value{}, sqlErr("list_users", "users", err)
	}
	defer rows.Close()
	var out []Value
	for rows.Next() {
		var username, status string
		var display sql.NullString
		if err := rows.Scan(&username, &display, &status); err != nil {
			return Value{}, err
		}
		if status != "active" {
			continue
		}
		name := Null()
		if display.Valid {
			name = String(display.String)
		}
		out = append(out, Object(
			Field{Key: "username", Val: String(username)},
			Field{Key: "displayName", Val: name},
		))
	}
	if err := rows.Err(); err != nil {
		return Value{}, err
	}
	if out == nil {
		out = []Value{}
	}
	return Array(out...), nil
}

func (a *App) listGrammarConcepts() (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("list_grammar_concepts", "grammar_concepts"))
	}
	rows, err := a.db.Query(`SELECT key, family, label_en, label_he, colour, sort FROM grammar_concepts ORDER BY sort ASC, key ASC`)
	if err != nil {
		return Value{}, sqlErr("list_grammar_concepts", "grammar_concepts", err)
	}
	defer rows.Close()
	var concepts []Value
	for rows.Next() {
		var key, family, en, he, colour string
		var sort int64
		if err := rows.Scan(&key, &family, &en, &he, &colour, &sort); err != nil {
			return Value{}, err
		}
		concepts = append(concepts, Object(
			Field{Key: "key", Val: String(key)},
			Field{Key: "family", Val: String(family)},
			Field{Key: "label_en", Val: String(en)},
			Field{Key: "label_he", Val: String(he)},
			Field{Key: "colour", Val: String(colour)},
			Field{Key: "sort", Val: Int(sort)},
		))
	}
	if err := rows.Err(); err != nil {
		return Value{}, err
	}
	if concepts == nil {
		concepts = []Value{}
	}
	// Node's listGrammarConcepts also INSERT OR IGNOREs future cell rows.
	// A read-only handle cannot. Migration 010 is expected to have seeded them.
	return Object(Field{Key: "concepts", Val: Array(concepts...)}), nil
}

func (a *App) listPlans(limit int) (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("list_plans", "plans"))
	}
	if limit < 1 || limit > 200 {
		limit = 200
	}
	rows, err := a.db.Query(`
		SELECT id, revision, status, audience, assignee_id, updated_at,
		       CASE WHEN json_valid(working_copy) THEN json_extract(working_copy, '$.title') END,
		       (snapshot IS NOT NULL AND snapshot != ''),
		       CASE WHEN snapshot IS NOT NULL AND json_valid(snapshot) THEN json_extract(snapshot, '$.revision') END
		FROM plans
		ORDER BY updated_at DESC, id
		LIMIT ?`, limit+1)
	if err != nil {
		return Value{}, sqlErr("list_plans", "plans", err)
	}
	defer rows.Close()
	type row struct {
		id, status, audience, updated string
		assignee                      sql.NullString
		title                         sql.NullString
		revision                      int64
		hasSnap                       int64
		snapRev                       sql.NullInt64
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.revision, &r.status, &r.audience, &r.assignee, &r.updated, &r.title, &r.hasSnap, &r.snapRev); err != nil {
			return Value{}, err
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		return Value{}, err
	}
	counts, err := a.humanCounts()
	if err != nil {
		return Value{}, err
	}
	truncated := len(got) > limit
	if truncated {
		got = got[:limit]
	}
	plans := make([]Value, 0, len(got))
	for _, r := range got {
		published := r.hasSnap == 1
		unpublished := published && (!r.snapRev.Valid || r.revision > r.snapRev.Int64)
		title := Null()
		if r.title.Valid {
			title = String(r.title.String)
		}
		plans = append(plans, Object(
			Field{Key: "id", Val: String(r.id)},
			Field{Key: "title", Val: title},
			Field{Key: "audience", Val: String(r.audience)},
			Field{Key: "assigned", Val: Bool(r.assignee.Valid)},
			Field{Key: "status", Val: String(r.status)},
			Field{Key: "revision", Val: Int(r.revision)},
			Field{Key: "updatedAt", Val: String(r.updated)},
			Field{Key: "published", Val: Bool(published)},
			Field{Key: "unpublishedChanges", Val: Bool(unpublished)},
			Field{Key: "humanEditedFields", Val: Int(counts[r.id])},
		))
	}
	return Object(
		Field{Key: "plans", Val: Array(plans...)},
		Field{Key: "truncated", Val: Bool(truncated)},
	), nil
}

func (a *App) humanCounts() (map[string]int64, error) {
	counts := map[string]int64{}
	rows, err := a.db.Query(`SELECT plan_id, COUNT(*) FROM plan_field_marks WHERE last_actor_kind = 'human' GROUP BY plan_id`)
	if err != nil {
		if isNoTable(err) {
			return counts, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		counts[id] = n
	}
	return counts, rows.Err()
}

func sqlErr(tool, table string, err error) error {
	if isNoTable(err) {
		return fmt.Errorf("%s", dbUnavailable(tool, table))
	}
	return err
}

func isNoTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}

type revisionMeta struct {
	revision  int64
	op        string
	path      string
	field     string
	summary   string
	actorKind string
	actorName string
	tool      sql.NullString
	client    sql.NullString
	at        string
}

func (a *App) listPlanRevisions(id string, since, before *int64, actorKind, pathPrefix string, limit int) (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("list_plan_revisions", "plan_revisions"))
	}
	if id == "" {
		return planErr("not_found"), nil
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := a.db.Query(`
		SELECT revision, op, path, field, summary, actor_kind, actor_name, tool, client, created_at
		FROM plan_revisions
		WHERE plan_id = ?
		  AND (? IS NULL OR revision > ?)
		  AND (? IS NULL OR revision < ?)
		  AND (? IS NULL OR actor_kind = ?)
		  AND (? IS NULL OR substr(path, 1, length(?)) = ?)
		ORDER BY revision DESC
		LIMIT ?`,
		id,
		nullableInt(since), nullableInt(since),
		nullableInt(before), nullableInt(before),
		nullableString(actorKind), nullableString(actorKind),
		nullableString(pathPrefix), nullableString(pathPrefix), nullableString(pathPrefix),
		limit+1,
	)
	if err != nil {
		return Value{}, sqlErr("list_plan_revisions", "plan_revisions", err)
	}
	defer rows.Close()
	var metas []revisionMeta
	for rows.Next() {
		m, err := scanMeta(rows)
		if err != nil {
			return Value{}, err
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return Value{}, err
	}
	if len(metas) == 0 {
		ok, err := a.planExists(id)
		if err != nil {
			return Value{}, err
		}
		if !ok {
			return planErr("not_found"), nil
		}
	}
	truncated := len(metas) > limit
	if truncated {
		metas = metas[:limit]
	}
	revs := make([]Value, 0, len(metas))
	for _, m := range metas {
		revs = append(revs, shapeMeta(m))
	}
	return Object(
		Field{Key: "revisions", Val: Array(revs...)},
		Field{Key: "truncated", Val: Bool(truncated)},
	), nil
}

func (a *App) getPlanRevision(id string, revision int64) (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("get_plan_revision", "plan_revisions"))
	}
	if id == "" || revision < 1 {
		return planErr("not_found"), nil
	}
	row := a.db.QueryRow(`
		SELECT revision, op, path, field, summary, actor_kind, actor_name, tool, client, created_at,
		       document_gz, document_sha256
		FROM plan_revisions WHERE plan_id = ? AND revision = ?`, id, revision)
	var m revisionMeta
	var gz []byte
	var sum string
	var path, field sql.NullString
	var tool, client sql.NullString
	err := row.Scan(&m.revision, &m.op, &path, &field, &m.summary, &m.actorKind, &m.actorName, &tool, &client, &m.at, &gz, &sum)
	if err == sql.ErrNoRows {
		return planErr("not_found"), nil
	}
	if err != nil {
		return Value{}, sqlErr("get_plan_revision", "plan_revisions", err)
	}
	m.path = path.String
	m.field = field.String
	m.tool = tool
	m.client = client
	text, err := gunzip(gz)
	if err != nil || sha256Hex(text) != sum {
		return planErr("revision_corrupt"), nil
	}
	doc, err := Parse(text)
	if err != nil {
		return planErr("revision_corrupt"), nil
	}
	out := shapeMeta(m)
	out.Obj = append(out.Obj, Field{Key: "document", Val: doc})
	return out, nil
}

func (a *App) planExists(id string) (bool, error) {
	var n int
	err := a.db.QueryRow(`SELECT COUNT(*) FROM plans WHERE id = ?`, id).Scan(&n)
	if err != nil {
		return false, sqlErr("list_plan_revisions", "plans", err)
	}
	return n > 0, nil
}

func scanMeta(rows *sql.Rows) (revisionMeta, error) {
	var m revisionMeta
	var path, field sql.NullString
	err := rows.Scan(&m.revision, &m.op, &path, &field, &m.summary, &m.actorKind, &m.actorName, &m.tool, &m.client, &m.at)
	m.path = path.String
	m.field = field.String
	return m, err
}

func shapeMeta(m revisionMeta) Value {
	name := m.actorName
	if m.actorKind == "human" {
		name = "human"
	}
	tool := Null()
	if m.tool.Valid {
		tool = String(m.tool.String)
	}
	client := Null()
	if m.client.Valid {
		client = String(m.client.String)
	}
	return Object(
		Field{Key: "revision", Val: Int(m.revision)},
		Field{Key: "op", Val: String(m.op)},
		Field{Key: "path", Val: String(m.path)},
		Field{Key: "field", Val: String(m.field)},
		Field{Key: "summary", Val: String(m.summary)},
		Field{Key: "actorKind", Val: String(m.actorKind)},
		Field{Key: "actorName", Val: String(name)},
		Field{Key: "tool", Val: tool},
		Field{Key: "client", Val: client},
		Field{Key: "at", Val: String(m.at)},
	)
}

func nullableInt(n *int64) any {
	if n == nil {
		return nil
	}
	return *n
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func gunzip(gz []byte) (string, error) {
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return "", err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func planErr(code string, extra ...Field) Value {
	fields := []Field{{Key: "error", Val: String(code)}}
	fields = append(fields, extra...)
	return Object(fields...)
}

type workingCopy struct {
	id       string
	revision int64
	status   string
	audience string
	assignee sql.NullString
	document string
	snapshot sql.NullString
}

func (a *App) loadWorking(id string) (workingCopy, bool, error) {
	var w workingCopy
	err := a.db.QueryRow(`
		SELECT id, revision, status, audience, assignee_id, working_copy, snapshot
		FROM plans WHERE id = ?`, id).Scan(&w.id, &w.revision, &w.status, &w.audience, &w.assignee, &w.document, &w.snapshot)
	if err == sql.ErrNoRows {
		return workingCopy{}, false, nil
	}
	if err != nil {
		return workingCopy{}, false, err
	}
	return w, true, nil
}
