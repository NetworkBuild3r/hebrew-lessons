package studyday

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestStudyDayRollover(t *testing.T) {
	denver := mustLoc(t, "America/Denver")
	jerusalem := mustLoc(t, "Asia/Jerusalem")

	springBefore := time.Date(2027, 3, 14, 3, 59, 0, 0, denver)
	if _, off := springBefore.Zone(); off != -6*3600 {
		t.Fatalf("2027-03-14 03:59 Denver offset = %d, want MDT (-6h)", off)
	}
	fallBefore := time.Date(2026, 11, 1, 3, 59, 0, 0, denver)
	if _, off := fallBefore.Zone(); off != -7*3600 {
		t.Fatalf("2026-11-01 03:59 Denver offset = %d, want MST (-7h)", off)
	}
	jerusalemMorning := time.Date(2026, 1, 15, 4, 0, 0, 0, jerusalem)
	if _, off := jerusalemMorning.Zone(); off != 2*3600 {
		t.Fatalf("2026-01-15 04:00 Jerusalem offset = %d, want IST (+2h)", off)
	}

	// Both 01:30s on the fall-back morning are still before 04:00.
	firstHalf := time.Date(2026, 11, 1, 7, 30, 0, 0, time.UTC)  // 01:30 MDT
	secondHalf := time.Date(2026, 11, 1, 8, 30, 0, 0, time.UTC) // 01:30 MST

	cases := []struct {
		name  string
		now   time.Time
		loc   *time.Location
		key   string
		start time.Time
		next  time.Time
	}{
		{
			name:  "denver 03:59 belongs to the previous day",
			now:   time.Date(2026, 10, 19, 3, 59, 0, 0, denver),
			loc:   denver,
			key:   "2026-10-18",
			start: time.Date(2026, 10, 18, 4, 0, 0, 0, denver),
			next:  time.Date(2026, 10, 19, 4, 0, 0, 0, denver),
		},
		{
			name:  "denver 04:00 starts the new day",
			now:   time.Date(2026, 10, 19, 4, 0, 0, 0, denver),
			loc:   denver,
			key:   "2026-10-19",
			start: time.Date(2026, 10, 19, 4, 0, 0, 0, denver),
			next:  time.Date(2026, 10, 20, 4, 0, 0, 0, denver),
		},
		{
			name:  "spring forward 03:59",
			now:   springBefore,
			loc:   denver,
			key:   "2027-03-13",
			start: time.Date(2027, 3, 13, 4, 0, 0, 0, denver),
			next:  time.Date(2027, 3, 14, 4, 0, 0, 0, denver),
		},
		{
			name:  "spring forward 04:00",
			now:   time.Date(2027, 3, 14, 4, 0, 0, 0, denver),
			loc:   denver,
			key:   "2027-03-14",
			start: time.Date(2027, 3, 14, 4, 0, 0, 0, denver),
			next:  time.Date(2027, 3, 15, 4, 0, 0, 0, denver),
		},
		{
			name:  "fall back 03:59",
			now:   fallBefore,
			loc:   denver,
			key:   "2026-10-31",
			start: time.Date(2026, 10, 31, 4, 0, 0, 0, denver),
			next:  time.Date(2026, 11, 1, 4, 0, 0, 0, denver),
		},
		{
			name:  "fall back 04:00",
			now:   time.Date(2026, 11, 1, 4, 0, 0, 0, denver),
			loc:   denver,
			key:   "2026-11-01",
			start: time.Date(2026, 11, 1, 4, 0, 0, 0, denver),
			next:  time.Date(2026, 11, 2, 4, 0, 0, 0, denver),
		},
		{
			name:  "fall back first 01:30",
			now:   firstHalf,
			loc:   denver,
			key:   "2026-10-31",
			start: time.Date(2026, 10, 31, 4, 0, 0, 0, denver),
			next:  time.Date(2026, 11, 1, 4, 0, 0, 0, denver),
		},
		{
			name:  "fall back second 01:30",
			now:   secondHalf,
			loc:   denver,
			key:   "2026-10-31",
			start: time.Date(2026, 10, 31, 4, 0, 0, 0, denver),
			next:  time.Date(2026, 11, 1, 4, 0, 0, 0, denver),
		},
		{
			name:  "jerusalem 03:59",
			now:   time.Date(2026, 1, 15, 3, 59, 0, 0, jerusalem),
			loc:   jerusalem,
			key:   "2026-01-14",
			start: time.Date(2026, 1, 14, 4, 0, 0, 0, jerusalem),
			next:  time.Date(2026, 1, 15, 4, 0, 0, 0, jerusalem),
		},
		{
			name:  "jerusalem 04:00",
			now:   jerusalemMorning,
			loc:   jerusalem,
			key:   "2026-01-15",
			start: jerusalemMorning,
			next:  time.Date(2026, 1, 16, 4, 0, 0, 0, jerusalem),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Key(tc.now, tc.loc); got != tc.key {
				t.Fatalf("Key(%s) = %s, want %s", tc.now, got, tc.key)
			}
			if got := Start(tc.now, tc.loc); !got.Equal(tc.start) {
				t.Fatalf("Start(%s) = %s, want %s", tc.now, got, tc.start)
			}
			if got := Next(tc.now, tc.loc); !got.Equal(tc.next) {
				t.Fatalf("Next(%s) = %s, want %s", tc.now, got, tc.next)
			}
		})
	}

	// 04:30 in Jerusalem is still the previous evening in Denver.
	instant := time.Date(2026, 1, 15, 4, 30, 0, 0, jerusalem)
	if got, want := Key(instant, jerusalem), "2026-01-15"; got != want {
		t.Fatalf("Jerusalem key = %s, want %s", got, want)
	}
	if got, want := Key(instant, denver), "2026-01-14"; got != want {
		t.Fatalf("Denver key at the same instant = %s, want %s", got, want)
	}

	// A nil location is Denver, including the 03:59 / 04:00 split.
	before := time.Date(2026, 10, 19, 3, 59, 0, 0, denver)
	at := time.Date(2026, 10, 19, 4, 0, 0, 0, denver)
	if Key(before, nil) != Key(before, denver) || Key(at, nil) != "2026-10-19" {
		t.Fatalf("nil loc keys %s %s", Key(before, nil), Key(at, nil))
	}
}

func TestInvalidTZFallsBackOnce(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	denver := Load(DefaultZone)
	if denver.String() != DefaultZone {
		t.Fatalf("Load(%s) = %s", DefaultZone, denver)
	}
	if Load("").String() != DefaultZone {
		t.Fatalf("empty tz = %s", Load(""))
	}

	bad := Load("Not/AZone")
	if bad.String() != DefaultZone {
		t.Fatalf("invalid tz = %s, want %s", bad, DefaultZone)
	}
	now := time.Date(2026, 10, 19, 3, 59, 0, 0, denver)
	if Key(now, bad) != Key(now, denver) || !Start(now, bad).Equal(Start(now, denver)) {
		t.Fatalf("fallback study day key %s start %s", Key(now, bad), Start(now, bad))
	}
	again := Load("Also/Missing")
	if again.String() != DefaultZone {
		t.Fatalf("second invalid tz = %s", again)
	}
	logs := buf.String()
	if strings.Count(logs, "invalid time zone") != 1 || !strings.Contains(logs, "Not/AZone") || strings.Contains(logs, "Also/Missing") {
		t.Fatalf("fallback log = %q", logs)
	}
}

func TestDueCutoffIsTheNextRollover(t *testing.T) {
	denver := mustLoc(t, DefaultZone)
	now := time.Date(2026, 10, 18, 22, 0, 0, 0, denver)
	if !DueCutoff(now, denver).Equal(Next(now, denver)) {
		t.Fatalf("cutoff %s, next rollover %s", DueCutoff(now, denver), Next(now, denver))
	}
	if !DueCutoff(now, nil).Equal(Next(now, nil)) {
		t.Fatal("a nil zone did not use the default zone")
	}
	// 23:30 and 03:59 share the next 4:00. 04:01 starts the following study day.
	night := time.Date(2026, 10, 18, 23, 30, 0, 0, denver)
	before := time.Date(2026, 10, 19, 3, 59, 0, 0, denver)
	after := time.Date(2026, 10, 19, 4, 1, 0, 0, denver)
	if !DueCutoff(night, denver).Equal(DueCutoff(before, denver)) {
		t.Fatal("23:30 and 03:59 do not share a cutoff")
	}
	if !DueCutoff(after, denver).After(DueCutoff(before, denver)) {
		t.Fatal("04:01 did not move the cutoff to the next rollover")
	}
}

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
