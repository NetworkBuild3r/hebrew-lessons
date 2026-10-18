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
	TZ       string   `json:"tz"`
	Now      string   `json:"now"`
	Username string   `json:"username"`
	Rows     []dueRow `json:"rows"`
}

type dueRow struct {
	Word      string `json:"word"`
	Direction string `json:"direction"`
	Due       string `json:"due"`
	Suspended int    `json:"suspended"`
}

type dueOracle struct {
	ID             string `json:"id"`
	TZ             string `json:"tz"`
	Node           string `json:"node"`
	Count          int    `json:"count"`
	TimestampCount int    `json:"timestampCount"`
	WordLevelCount int    `json:"wordLevelCount"`
	NowMs          int64  `json:"nowMs"`
	NowDay         int64  `json:"nowDay"`
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
	if len(cases) == 0 {
		t.Fatal("no cases")
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
				"username":  item.Username,
				"word":      row.Word,
				"direction": row.Direction,
				"due":       row.Due,
				"suspended": row.Suspended,
			})
		}
	}
	dir, ids, conn := seedFixture(t, map[string]any{
		"users":   users,
		"reviews": reviews,
	})

	byTZ := map[string][]dueCase{}
	var zones []string
	for _, item := range cases {
		if _, ok := byTZ[item.TZ]; !ok {
			zones = append(zones, item.TZ)
		}
		byTZ[item.TZ] = append(byTZ[item.TZ], item)
	}

	oracles := map[string]dueOracle{}
	for _, tz := range zones {
		for _, line := range runDueOracle(t, dir, casesPath, tz) {
			if line.TZ != tz {
				t.Fatalf("oracle tz %s want %s", line.TZ, tz)
			}
			if !strings.HasPrefix(line.Node, "v22.") {
				t.Fatalf("node %s", line.Node)
			}
			oracles[line.ID] = line
		}
	}

	cfg := testConfig(t)
	ctx := context.Background()
	for _, item := range cases {
		oracle, ok := oracles[item.ID]
		if !ok {
			t.Fatalf("no oracle for %s", item.ID)
		}
		if oracle.TimestampCount > oracle.Count {
			t.Fatalf("%s timestamp %d > local %d", item.ID, oracle.TimestampCount, oracle.Count)
		}
		now, err := time.Parse(time.RFC3339, item.Now)
		if err != nil {
			t.Fatal(err)
		}
		if oracle.NowMs != now.UnixMilli() {
			t.Fatalf("%s nowMs node %d go %d", item.ID, oracle.NowMs, now.UnixMilli())
		}
		loc, err := time.LoadLocation(item.TZ)
		if err != nil {
			t.Fatal(err)
		}
		if day := localDayNumber(now.UnixMilli(), loc); day != oracle.NowDay {
			t.Fatalf("%s nowDay go %d node %d", item.ID, day, oracle.NowDay)
		}
		userID := ids[item.Username]
		if userID <= 0 {
			t.Fatalf("missing id for %s", item.Username)
		}
		svc := New(conn, cfg).WithLocation(loc).WithClock(func() time.Time { return now })
		got, err := svc.DueCount(ctx, userID)
		if err != nil {
			t.Fatalf("%s: %v", item.ID, err)
		}
		if got != oracle.Count {
			t.Fatalf("%s DueCount %d node %d timestamp %d wordLevel %d",
				item.ID, got, oracle.Count, oracle.TimestampCount, oracle.WordLevelCount)
		}
	}

	// These lock the local-day reading against the instant queue and the
	// word-level suspension flag. The numbers are the Node oracle's.
	want := []struct {
		id         string
		count      int
		timestamp  int
		wordLevel  int
		checkLevel bool
	}{
		{id: "utc-both", count: 2, timestamp: 2},
		{id: "partial-suspend", count: 1, timestamp: 1, wordLevel: 0, checkLevel: true},
		{id: "both-suspended", count: 0, timestamp: 0, wordLevel: 0, checkLevel: true},
		{id: "utc-future", count: 0, timestamp: 0},
		{id: "later-today-utc", count: 1, timestamp: 0},
		{id: "later-today-ny", count: 1, timestamp: 0},
		{id: "cross-utc", count: 1, timestamp: 1},
		{id: "cross-ny", count: 2, timestamp: 1},
		{id: "cross-kiritimati", count: 2, timestamp: 1},
		{id: "cross-pago", count: 2, timestamp: 1},
		{id: "invalid-due", count: 1, timestamp: 1},
		{id: "exactly-now", count: 1, timestamp: 1},
		{id: "dst-ny", count: 1, timestamp: 0},
		{id: "three-of-four", count: 3, timestamp: 3},
		{id: "no-rows", count: 0, timestamp: 0},
		{id: "spaced-word", count: 1, timestamp: 1},
		{id: "offset-now", count: 1, timestamp: 1},
	}
	if len(want) != len(cases) {
		t.Fatalf("locked cases %d file %d", len(want), len(cases))
	}
	for _, item := range want {
		oracle := oracles[item.id]
		if oracle.Count != item.count || oracle.TimestampCount != item.timestamp {
			t.Fatalf("%s oracle count %d timestamp %d want %d %d",
				item.id, oracle.Count, oracle.TimestampCount, item.count, item.timestamp)
		}
		if item.checkLevel && oracle.WordLevelCount != item.wordLevel {
			t.Fatalf("%s wordLevel %d want %d", item.id, oracle.WordLevelCount, item.wordLevel)
		}
	}

	unknown, err := New(conn, cfg).DueCount(ctx, 1_000_000_000)
	if err != nil || unknown != 0 {
		t.Fatalf("unknown user count %d err %v", unknown, err)
	}
	if _, err := New(conn, cfg).DueCount(ctx, 0); err != ErrBadUser {
		t.Fatalf("bad user: %v", err)
	}
}

func runDueOracle(t *testing.T, dir, casesPath, tz string) []dueOracle {
	t.Helper()
	cmd := exec.Command("node", filepath.Join("testdata", "due-oracle.js"), dir, casesPath)
	cmd.Env = nodeEnv(tz)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("oracle %s: %v\n%s", tz, err, stderr.String())
	}
	var out []dueOracle
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var item dueOracle
		if err := json.Unmarshal(line, &item); err != nil {
			t.Fatalf("oracle %s line %s: %v\nstderr %s", tz, line, err, stderr.String())
		}
		out = append(out, item)
	}
	return out
}
