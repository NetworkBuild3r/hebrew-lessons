package mcp

import (
	"fmt"
	"strings"
)

func (a *App) diffPlan(id string, from, to Value, path string) (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("diff_plan", "plans"))
	}
	if id == "" {
		return planErr("not_found"), nil
	}
	left, err := a.resolveSide(id, from)
	if err != nil {
		return Value{}, err
	}
	if left.err != "" {
		return planErr(left.err), nil
	}
	right, err := a.resolveSide(id, to)
	if err != nil {
		return Value{}, err
	}
	if right.err != "" {
		return planErr(right.err), nil
	}
	entries, truncated := diffDocuments(left.doc, right.doc, path)
	return Object(
		Field{Key: "from", Val: sideValue(left)},
		Field{Key: "to", Val: sideValue(right)},
		Field{Key: "entries", Val: entries},
		Field{Key: "truncated", Val: Bool(truncated)},
	), nil
}

type side struct {
	err      string
	doc      Value
	revision *int64
	ref      Value
}

func sideValue(s side) Value {
	rev := Null()
	if s.revision != nil {
		rev = Int(*s.revision)
	}
	return Object(
		Field{Key: "ref", Val: s.ref},
		Field{Key: "revision", Val: rev},
	)
}

func (a *App) resolveSide(id string, ref Value) (side, error) {
	if ref.Kind == KindString && (ref.Str == "working" || ref.Str == "published") {
		row, ok, err := a.loadWorking(id)
		if err != nil {
			return side{}, err
		}
		if ref.Str == "working" {
			if !ok {
				return side{err: "not_found"}, nil
			}
			doc, err := Parse(row.document)
			if err != nil || doc.Kind != KindObject {
				return side{err: "invalid_plan"}, nil
			}
			doc = putField(doc, "revision", Int(row.revision))
			doc = putField(doc, "status", String(row.status))
			rev := row.revision
			return side{doc: doc, revision: &rev, ref: String("working")}, nil
		}
		if !ok {
			return side{err: "not_found"}, nil
		}
		if !row.snapshot.Valid || row.snapshot.String == "" {
			return side{err: "no_snapshot"}, nil
		}
		doc, err := Parse(row.snapshot.String)
		if err != nil || doc.Kind != KindObject {
			return side{err: "invalid_plan"}, nil
		}
		var revPtr *int64
		if rev, ok := doc.Get("revision"); ok && rev.IsInt() {
			f, _ := rev.Float()
			n := int64(f)
			revPtr = &n
		}
		return side{doc: doc, revision: revPtr, ref: String("published")}, nil
	}
	if ref.Kind == KindNumber && ref.IsInt() {
		f, _ := ref.Float()
		n := int64(f)
		if n < 1 {
			return side{err: "invalid_op"}, nil
		}
		got, err := a.getPlanRevision(id, n)
		if err != nil {
			return side{}, err
		}
		if errVal, ok := got.Get("error"); ok {
			return side{err: errVal.Str}, nil
		}
		doc, _ := got.Get("document")
		return side{doc: doc, revision: &n, ref: Int(n)}, nil
	}
	return side{err: "invalid_op"}, nil
}

func (a *App) readPart(id, path string) (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("read_part", "plans"))
	}
	if id == "" {
		return planErr("not_found"), nil
	}
	row, ok, err := a.loadWorking(id)
	if err != nil {
		return Value{}, err
	}
	if !ok {
		return planErr("not_found"), nil
	}
	doc, err := Parse(row.document)
	if err != nil || doc.Kind != KindObject {
		return planErr("invalid_plan", Field{Key: "revision", Val: Int(row.revision)}), nil
	}
	doc = putField(doc, "revision", Int(row.revision))
	doc = putField(doc, "status", String(row.status))
	doc = putField(doc, "audience", String(row.audience))
	if row.assignee.Valid {
		doc = putField(doc, "assigneeId", String(row.assignee.String))
	} else {
		doc = putField(doc, "assigneeId", Null())
	}
	found, located := locate(doc, path)
	if !located {
		p := path
		return planErr("not_found",
			Field{Key: "revision", Val: Int(row.revision)},
			Field{Key: "path", Val: String(p)},
		), nil
	}
	edit, err := a.editState(id, path)
	if err != nil {
		return Value{}, err
	}
	return Object(
		Field{Key: "part", Val: found},
		Field{Key: "revision", Val: Int(row.revision)},
		Field{Key: "editState", Val: edit},
	), nil
}

func locate(plan Value, pathStr string) (Value, bool) {
	if pathStr == "" || pathStr == "/" {
		return plan, true
	}
	segs := strings.FieldsFunc(pathStr, func(r rune) bool { return r == '/' })
	if len(segs) < 2 || segs[0] != "units" {
		return Value{}, false
	}
	units, ok := plan.Get("units")
	if !ok || units.Kind != KindArray {
		return Value{}, false
	}
	unit, ok := findID(units, segs[1])
	if !ok {
		return Value{}, false
	}
	if len(segs) == 2 {
		return unit, true
	}
	if segs[2] != "lessons" || len(segs) < 4 {
		return Value{}, false
	}
	lessons, ok := unit.Get("lessons")
	if !ok || lessons.Kind != KindArray {
		return Value{}, false
	}
	lesson, ok := findID(lessons, segs[3])
	if !ok {
		return Value{}, false
	}
	if len(segs) == 4 {
		return lesson, true
	}
	if segs[4] != "blocks" || len(segs) < 6 {
		return Value{}, false
	}
	blocks, ok := lesson.Get("blocks")
	if !ok || blocks.Kind != KindArray {
		return Value{}, false
	}
	block, ok := findID(blocks, segs[5])
	if !ok {
		return Value{}, false
	}
	if len(segs) == 6 {
		return block, true
	}
	bucket := segs[6]
	if !contains(diffBuckets, bucket) || len(segs) != 8 {
		return Value{}, false
	}
	items, ok := block.Get(bucket)
	if !ok || items.Kind != KindArray {
		return Value{}, false
	}
	item, ok := findID(items, segs[7])
	return item, ok
}

func findID(list Value, id string) (Value, bool) {
	for _, item := range list.Arr {
		got, ok := item.Get("id")
		if ok && got.Kind == KindString && got.Str == id {
			return item, true
		}
	}
	return Value{}, false
}

func (a *App) editState(planID, path string) (Value, error) {
	prefix := strings.Join(strings.FieldsFunc(path, func(r rune) bool { return r == '/' }), "/")
	var rows []mark
	var err error
	if prefix == "" {
		rows, err = a.marksForPlan(planID)
	} else {
		rows, err = a.marksForPrefix(planID, prefix)
	}
	if err != nil {
		if isNoTable(err) {
			rows = nil
		} else {
			return Value{}, err
		}
	}
	var human []mark
	for _, m := range rows {
		if m.lastKind == "human" {
			human = append(human, m)
		}
	}
	for i := 1; i < len(human); i++ {
		j := i
		for j > 0 && markLess(human[j], human[j-1]) {
			human[j], human[j-1] = human[j-1], human[j]
			j--
		}
	}
	const maxHuman = 50
	shown := human
	if len(shown) > maxHuman {
		shown = shown[:maxHuman]
	}
	items := make([]Value, 0, len(shown))
	for _, m := range shown {
		prev := Null()
		omit := false
		if m.prevJSON == nil {
			omit = true
		} else if parsed, err := Parse(*m.prevJSON); err != nil {
			omit = true
		} else {
			prev = parsed
		}
		writer := "unknown"
		if m.prevWriter != "" {
			writer = m.prevWriter
		}
		item := Object(
			Field{Key: "path", Val: String(m.path)},
			Field{Key: "field", Val: String(m.field)},
			Field{Key: "revision", Val: Int(m.humanRev)},
			Field{Key: "at", Val: String(m.humanAt)},
			Field{Key: "previousValue", Val: prev},
			Field{Key: "previousWriter", Val: String(writer)},
		)
		if omit {
			item = putField(item, "previousValueOmitted", Bool(true))
		}
		items = append(items, item)
	}
	last, err := a.lastHuman(planID)
	if err != nil && !isNoTable(err) {
		return Value{}, err
	}
	return Object(
		Field{Key: "advisory", Val: Bool(true)},
		Field{Key: "lastHumanEdit", Val: last},
		Field{Key: "humanEdited", Val: Array(items...)},
		Field{Key: "more", Val: Int(int64(max(0, len(human)-maxHuman)))},
	), nil
}

type mark struct {
	path       string
	field      string
	lastKind   string
	humanRev   int64
	humanAt    string
	prevJSON   *string
	prevWriter string
}

func markLess(a, b mark) bool {
	if a.humanRev != b.humanRev {
		return a.humanRev > b.humanRev
	}
	if a.path != b.path {
		return a.path < b.path
	}
	return a.field < b.field
}

func (a *App) marksForPlan(id string) ([]mark, error) {
	rows, err := a.db.Query(`SELECT path, field, last_actor_kind, human_revision, human_at, previous_value_json, previous_writer_kind FROM plan_field_marks WHERE plan_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMarks(rows)
}

func (a *App) marksForPrefix(id, prefix string) ([]mark, error) {
	rows, err := a.db.Query(`
		SELECT path, field, last_actor_kind, human_revision, human_at, previous_value_json, previous_writer_kind
		FROM plan_field_marks
		WHERE plan_id = ?
		  AND (path = ? OR substr(path, 1, length(?) + 1) = ? || '/')`, id, prefix, prefix, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMarks(rows)
}

type markRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanMarks(rows markRows) ([]mark, error) {
	var out []mark
	for rows.Next() {
		var m mark
		var revN sqlNullInt
		var at, prevS, writerS sqlNullStr
		if err := rows.Scan(&m.path, &m.field, &m.lastKind, &revN, &at, &prevS, &writerS); err != nil {
			return nil, err
		}
		if revN.valid {
			m.humanRev = revN.n
		}
		if at.valid {
			m.humanAt = at.s
		}
		if prevS.valid {
			s := prevS.s
			m.prevJSON = &s
		}
		if writerS.valid {
			m.prevWriter = writerS.s
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type sqlNullInt struct {
	n     int64
	valid bool
}

func (n *sqlNullInt) Scan(src any) error {
	if src == nil {
		n.valid = false
		return nil
	}
	switch v := src.(type) {
	case int64:
		n.n, n.valid = v, true
	case int:
		n.n, n.valid = int64(v), true
	default:
		return fmt.Errorf("human_revision type %T", src)
	}
	return nil
}

type sqlNullStr struct {
	s     string
	valid bool
}

func (n *sqlNullStr) Scan(src any) error {
	if src == nil {
		n.valid = false
		return nil
	}
	switch v := src.(type) {
	case string:
		n.s, n.valid = v, true
	case []byte:
		n.s, n.valid = string(v), true
	default:
		return fmt.Errorf("text type %T", src)
	}
	return nil
}

func (a *App) lastHuman(id string) (Value, error) {
	var rev sqlNullInt
	var at sqlNullStr
	err := a.db.QueryRow(`
		SELECT human_revision, human_at FROM plan_field_marks
		WHERE plan_id = ? AND human_revision IS NOT NULL
		ORDER BY human_revision DESC, human_at DESC LIMIT 1`, id).Scan(&rev, &at)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") || isNoTable(err) {
			return Null(), nil
		}
		return Null(), err
	}
	if !rev.valid {
		return Null(), nil
	}
	return Object(
		Field{Key: "revision", Val: Int(rev.n)},
		Field{Key: "at", Val: String(at.s)},
	), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
