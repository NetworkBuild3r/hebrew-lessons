package db

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// Migration 012 adds the index the learner-list scan walks. The scan's ORDER BY
// includes id, so an index that stops at created_at makes SQLite sort every attempt.
func TestMigrate012AttemptListIndexAvoidsASort(t *testing.T) {
	conn := openRaw(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	if got := indexColumns(t, conn, "attempts_user_lexeme_skill_id"); !reflect.DeepEqual(got, []string{"user_id", "lexeme_id", "skill", "created_at", "id", "correct"}) {
		t.Fatalf("index columns = %v", got)
	}
	rows, err := conn.QueryContext(context.Background(), `
		EXPLAIN QUERY PLAN
		SELECT lexeme_id, skill, correct, created_at
		FROM attempts INDEXED BY attempts_user_lexeme_skill_id
		WHERE user_id = ?
		ORDER BY lexeme_id DESC, skill DESC, created_at DESC, id DESC`, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail)
		plan.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	got := plan.String()
	if strings.Contains(got, "TEMP B-TREE") {
		t.Fatalf("attempt scan sorts:\n%s", got)
	}
	if !strings.Contains(got, "attempts_user_lexeme_skill_id") {
		t.Fatalf("attempt scan plan:\n%s", got)
	}
}
