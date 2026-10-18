package path

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type dueCase struct {
	ID       string   `json:"id"`
	Now      string   `json:"now"`
	Username string   `json:"username"`
	Rows     []dueRow `json:"rows"`
}

type dueRow struct {
	Word       string   `json:"word"`
	Direction  string   `json:"direction"`
	Due        string   `json:"due"`
	Suspended  int      `json:"suspended"`
	LastAt     string   `json:"lastAt,omitempty"`
	LastResult string   `json:"lastResult,omitempty"`
	Sources    []string `json:"sources,omitempty"`
}

type dueCard struct {
	Word      string `json:"word"`
	Direction string `json:"direction"`
}

type dueOracle struct {
	ID           string    `json:"id"`
	Node         string    `json:"node"`
	Count        int       `json:"count"`
	InstantCount int       `json:"instantCount"`
	Cards        []dueCard `json:"cards"`
}

func TestDueCountGolden(t *testing.T) {
	casesPath := filepath.Join("testdata", "due-cases.jsonl")
	info, err := os.Stat(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() >= 1<<20 {
		t.Fatalf("golden file is %d bytes", info.Size())
	}
	raw, err := os.ReadFile(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(raw) {
		t.Fatal("golden file is not valid UTF-8")
	}
	var cases []dueCase
	for i, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var item dueCase
		if err := json.Unmarshal(line, &item); err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		cases = append(cases, item)
	}

	users := make([]string, 0, len(cases))
	var reviews []map[string]any
	seenUser := map[string]struct{}{}
	for _, item := range cases {
		if _, ok := seenUser[item.Username]; ok {
			t.Fatalf("duplicate user %s", item.Username)
		}
		seenUser[item.Username] = struct{}{}
		users = append(users, item.Username)
		for _, row := range item.Rows {
			reviews = append(reviews, map[string]any{
				"username":   item.Username,
				"word":       row.Word,
				"direction":  row.Direction,
				"due":        row.Due,
				"suspended":  row.Suspended,
				"lastAt":     row.LastAt,
				"lastResult": row.LastResult,
				"sources":    row.Sources,
			})
		}
	}
	dir, ids, conn := seedFixture(t, map[string]any{
		"users":   users,
		"reviews": reviews,
	})
	oracles := map[string]dueOracle{}
	for _, line := range runDueOracle(t, dir, casesPath) {
		if !strings.HasPrefix(line.Node, "v22.14.") {
			t.Fatalf("node %s", line.Node)
		}
		oracles[line.ID] = line
	}

	cfg := testConfig(t)
	ctx := context.Background()
	for _, item := range cases {
		oracle, ok := oracles[item.ID]
		if !ok {
			t.Fatalf("no oracle for %s", item.ID)
		}
		if oracle.Count > oracle.InstantCount {
			t.Fatalf("%s deck %d exceeds uncapped %d", item.ID, oracle.Count, oracle.InstantCount)
		}
		now, err := time.Parse(time.RFC3339Nano, item.Now)
		if err != nil {
			now, err = time.Parse(time.RFC3339, item.Now)
			if err != nil {
				t.Fatal(err)
			}
		}
		userID := ids[item.Username]
		svc := New(conn, cfg).WithClock(func() time.Time { return now })
		got, err := svc.dueQueue(ctx, userID, now)
		if err != nil {
			t.Fatalf("%s: %v", item.ID, err)
		}
		if len(got) != oracle.Count {
			t.Fatalf("%s DueCount %d review page %d uncapped %d",
				item.ID, len(got), oracle.Count, oracle.InstantCount)
		}
		if len(got) != len(oracle.Cards) {
			t.Fatalf("%s cards %d vs %d", item.ID, len(got), len(oracle.Cards))
		}
		for i, card := range oracle.Cards {
			if got[i].word != card.Word || got[i].direction != card.Direction {
				t.Fatalf("%s card %d go %s %s node %s %s",
					item.ID, i, got[i].word, got[i].direction, card.Word, card.Direction)
			}
		}
	}

	// Locks the instant reading and the Review page caps. The numbers are
	// what cards.deck returned.
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
		oracle := oracles[id]
		if oracle.Count != item.count || oracle.InstantCount != item.instant {
			t.Fatalf("%s review %d uncapped %d want %d %d",
				id, oracle.Count, oracle.InstantCount, item.count, item.instant)
		}
	}
	var interleaved []string
	for _, card := range oracles["interleave"].Cards {
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

func runDueOracle(t *testing.T, dir, casesPath string) []dueOracle {
	t.Helper()
	cmd := exec.Command("node", filepath.Join("testdata", "due-oracle.js"), dir, casesPath)
	cmd.Env = nodeEnv("UTC")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	var out []dueOracle
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var item dueOracle
		if err := json.Unmarshal(line, &item); err != nil {
			t.Fatalf("oracle line %s: %v\nstderr %s", line, err, stderr.String())
		}
		out = append(out, item)
	}
	return out
}
