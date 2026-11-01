package brief

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// ac-6 / ac-7: learner note is quoted with authorRole, JSON-escaped, and not in another brief.
func TestBriefQuotesLearnerCoachNote(t *testing.T) {
	e := newEnv(t)
	other := e.other()
	body := `<script>alert(1)</script>`
	e.exec(`INSERT INTO coach_notes (user_id, author, body, created_at, author_role) VALUES (?, '7', ?, ?, 'learner')`,
		e.user, body, db.FormatTime(testNow.Add(-time.Hour)))
	e.exec(`INSERT INTO coach_notes (user_id, author, body, created_at, author_role) VALUES (?, '9', 'secret-B', ?, 'learner')`,
		other, db.FormatTime(testNow.Add(-time.Hour)))

	br := e.build("")
	if len(br.CoachNotes) != 1 || br.CoachNotes[0].AuthorRole != "learner" || br.CoachNotes[0].Body != body {
		t.Fatalf("coach notes = %+v", br.CoachNotes)
	}
	found := false
	for _, n := range br.Notes {
		if strings.HasPrefix(n, "The learner wrote:") && strings.Contains(n, "alert(1)") {
			found = true
		}
		if strings.Contains(n, "secret-B") {
			t.Fatalf("other learner's note leaked into Notes: %q", n)
		}
	}
	if !found {
		t.Fatalf("notes missing quoted learner line: %#v", br.Notes)
	}

	raw, err := json.Marshal(br.Plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "<script>") {
		t.Fatalf("JSON did not escape script tags: %s", s)
	}
	if !strings.Contains(s, "alert(1)") {
		t.Fatalf("JSON lost the note body: %s", s)
	}

	brOther, err := e.b.Build(context.Background(), other, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range brOther.CoachNotes {
		if n.Body == body || strings.Contains(n.Body, "alert") {
			t.Fatalf("other brief has A's note: %+v", brOther.CoachNotes)
		}
	}
	for _, n := range brOther.Notes {
		if strings.Contains(n, "alert") || strings.Contains(n, body) {
			t.Fatalf("other brief Notes leaked A's text: %#v", brOther.Notes)
		}
	}
	if len(brOther.CoachNotes) != 1 || brOther.CoachNotes[0].Body != "secret-B" {
		t.Fatalf("other brief notes = %+v", brOther.CoachNotes)
	}
}
