package path

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
)

type dueCard struct {
	Word      string `json:"word"`
	Direction string `json:"direction"`
}

type dueOracle struct {
	ID           string    `json:"id"`
	Now          string    `json:"now"`
	Username     string    `json:"username"`
	Node         string    `json:"node"`
	Unicode      string    `json:"unicode"`
	Count        int       `json:"count"`
	InstantCount int       `json:"instantCount"`
	Cards        []dueCard `json:"cards"`
}

func TestDueCountGolden(t *testing.T) {
	goldenPath := filepath.Join("testdata", "golden", "due.jsonl")
	info, err := os.Stat(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() >= 1<<20 {
		t.Fatalf("golden file is %d bytes", info.Size())
	}
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(raw) {
		t.Fatal("golden file is not valid UTF-8")
	}
	var oracles []dueOracle
	for i, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var item dueOracle
		if err := json.Unmarshal(line, &item); err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		if item.Node != "v22.14.0" || item.Unicode != "16.0" {
			t.Fatalf("%s golden node %s unicode %s", item.ID, item.Node, item.Unicode)
		}
		oracles = append(oracles, item)
	}
	if len(oracles) == 0 {
		t.Fatal("golden file is empty")
	}

	conn, err := db.OpenReadOnly(filepath.Join("testdata", "golden", "due.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	cfg := testConfig(t)
	ctx := context.Background()
	byID := map[string]dueOracle{}
	for _, oracle := range oracles {
		if _, ok := byID[oracle.ID]; ok {
			t.Fatalf("duplicate golden %s", oracle.ID)
		}
		byID[oracle.ID] = oracle
		if oracle.Count > oracle.InstantCount {
			t.Fatalf("%s deck %d exceeds uncapped %d", oracle.ID, oracle.Count, oracle.InstantCount)
		}
		now, err := time.Parse(time.RFC3339Nano, oracle.Now)
		if err != nil {
			now, err = time.Parse(time.RFC3339, oracle.Now)
			if err != nil {
				t.Fatal(err)
			}
		}
		userID := userIDByName(t, ctx, conn, oracle.Username)
		svc := New(conn, cfg).WithClock(func() time.Time { return now })
		got, err := svc.dueQueue(ctx, userID, now)
		if err != nil {
			t.Fatalf("%s: %v", oracle.ID, err)
		}
		if len(got) != oracle.Count {
			t.Fatalf("%s DueCount %d review page %d uncapped %d",
				oracle.ID, len(got), oracle.Count, oracle.InstantCount)
		}
		if len(got) != len(oracle.Cards) {
			t.Fatalf("%s cards %d vs %d", oracle.ID, len(got), len(oracle.Cards))
		}
		for i, card := range oracle.Cards {
			if got[i].word != card.Word || got[i].direction != card.Direction {
				t.Fatalf("%s card %d go %s %s node %s %s",
					oracle.ID, i, got[i].word, got[i].direction, card.Word, card.Direction)
			}
		}
	}

	// Locks the instant reading and the Review page caps. The numbers are
	// what cards.deck wrote into the golden.
	want := map[string]struct{ count, instant int }{
		"later-today-before": {0, 0},
		"later-today-after":  {1, 1},
		"exactly-now":        {1, 1},
		"cross-ny":           {1, 1},
		"due-cap":            {30, 31},
		"new-cap":            {10, 11},
		"new-second":         {11, 11},
		"partial-suspend":    {1, 1},
		"interleave":         {4, 4},
	}
	for id, item := range want {
		oracle, ok := byID[id]
		if !ok {
			t.Fatalf("missing golden %s", id)
		}
		if oracle.Count != item.count || oracle.InstantCount != item.instant {
			t.Fatalf("%s review %d uncapped %d want %d %d",
				id, oracle.Count, oracle.InstantCount, item.count, item.instant)
		}
	}
	var interleaved []string
	for _, card := range byID["interleave"].Cards {
		interleaved = append(interleaved, card.Word)
	}
	if stringsJoin(interleaved) != "a1,a2,b1,a3" {
		t.Fatalf("interleave %v", interleaved)
	}

	unknown, err := New(conn, cfg).DueCount(ctx, 1_000_000_000)
	if err != nil || unknown != 0 {
		t.Fatalf("unknown user count %d err %v", unknown, err)
	}
	if _, err := New(conn, cfg).DueCount(ctx, 0); err != ErrBadUser {
		t.Fatalf("bad user: %v", err)
	}
}

func userIDByName(t *testing.T, ctx context.Context, conn *sql.DB, username string) int64 {
	t.Helper()
	var id int64
	err := conn.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, username).Scan(&id)
	if err != nil {
		t.Fatalf("user %s: %v", username, err)
	}
	return id
}
