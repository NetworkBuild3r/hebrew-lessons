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

func TestLoadKeepsAValidZone(t *testing.T) {
	loc := Load(" Asia/Jerusalem ")
	if loc == nil || loc.String() != "Asia/Jerusalem" {
		t.Fatalf("Load = %v", loc)
	}
	denver := Load(DefaultZone)
	if denver == nil || denver.String() != DefaultZone {
		t.Fatalf("Load(default) = %v", denver)
	}
	// 04:30 in Jerusalem is still the previous evening in Denver.
	now := time.Date(2026, 1, 15, 4, 30, 0, 0, loc)
	if Key(now, loc) == Key(now, denver) || DueCutoff(now, loc).Equal(DueCutoff(now, denver)) {
		t.Fatalf("Jerusalem key %s cutoff %s, Denver key %s cutoff %s", Key(now, loc), DueCutoff(now, loc), Key(now, denver), DueCutoff(now, denver))
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

func TestDecideSplitsReviewLearningAndNew(t *testing.T) {
	denver := mustLoc(t, DefaultZone)
	now := time.Date(2026, 10, 18, 22, 0, 0, 0, denver)
	cutoff := DueCutoff(now, denver)
	ahead := 20 * time.Minute
	inside := time.Date(2026, 10, 18, 22, 5, 0, 0, denver)
	outside := time.Date(2026, 10, 18, 22, 30, 0, 0, denver)
	exact := now.Add(ahead)
	justInside := exact.Add(-time.Nanosecond)
	pastExact := exact.Add(time.Nanosecond)
	tomorrow := time.Date(2026, 10, 19, 3, 0, 0, 0, denver)
	afterCutoff := time.Date(2026, 10, 19, 5, 0, 0, 0, denver)
	if !tomorrow.Before(cutoff) || !afterCutoff.After(cutoff) || !outside.Before(cutoff) {
		t.Fatalf("cutoff %s tomorrow %s outside %s after %s", cutoff, tomorrow, outside, afterCutoff)
	}
	one := func(state string, due time.Time) bool {
		t.Helper()
		got := Decide(now, cutoff, ahead, []DueCard{{State: state, Due: due}})
		if len(got.Due) != 1 {
			t.Fatalf("%s %s: len %d", state, due, len(got.Due))
		}
		return got.Due[0]
	}
	cases := []struct {
		name  string
		state string
		due   time.Time
		want  bool
	}{
		{"learning inside learn-ahead", "learning", inside, true},
		{"learning just inside the cap", "learning", justInside, true},
		{"learning at the learn-ahead limit", "learning", exact, true},
		{"learning one nanosecond past learn-ahead", "learning", pastExact, false},
		{"learning outside learn-ahead", "learning", outside, false},
		{"relearning inside learn-ahead", "relearning", inside, true},
		{"relearning just outside the cap", "relearning", pastExact, false},
		{"relearning outside learn-ahead", "relearning", outside, false},
		{"review before tomorrow's rollover", "review", tomorrow, true},
		{"review after the cutoff", "review", afterCutoff, false},
		{"new at the clock", "new", now, true},
		{"new before tomorrow's rollover", "new", tomorrow, false},
		{"new outside the clock stays off the day cutoff", "new", outside, false},
		{"new after the cutoff", "new", afterCutoff, false},
		{"empty state uses the clock", "", outside, false},
	}
	for _, tc := range cases {
		if got := one(tc.state, tc.due); got != tc.want {
			t.Errorf("%s: due = %v, want %v", tc.name, got, tc.want)
		}
	}

	// A learning card inside the cap waits while any other card is due.
	withReview := Decide(now, cutoff, ahead, []DueCard{
		{State: "learning", Due: inside},
		{State: "review", Due: now},
	})
	if withReview.Due[0] || !withReview.Due[1] {
		t.Fatalf("learn-ahead while a review card is due: %v", withReview.Due)
	}
	// A new card that is not yet due does not count as something else due.
	withFutureNew := Decide(now, cutoff, ahead, []DueCard{
		{State: "learning", Due: inside},
		{State: "new", Due: outside},
	})
	if !withFutureNew.Due[0] || withFutureNew.Due[1] {
		t.Fatalf("a future new card blocked learn-ahead: %v", withFutureNew.Due)
	}
	dueNew := Decide(now, cutoff, ahead, []DueCard{
		{State: "relearning", Due: inside},
		{State: "new", Due: now},
	})
	if dueNew.Due[0] || !dueNew.Due[1] {
		t.Fatalf("a due new card left the relearning card in learn-ahead: %v", dueNew.Due)
	}
	// The cap is the duration the caller passes.
	short := Decide(now, cutoff, 10*time.Minute, []DueCard{{State: "learning", Due: now.Add(15 * time.Minute)}})
	if short.Due[0] {
		t.Fatal("a 15 minute step was due under a 10 minute cap")
	}

	// 22:01 is before the 22:05 step. 22:05 is the step itself.
	at2201 := time.Date(2026, 10, 18, 22, 1, 0, 0, denver)
	step := time.Date(2026, 10, 18, 22, 5, 0, 0, denver)
	cut2201 := DueCutoff(at2201, denver)
	if Decide(at2201, cut2201, ahead, []DueCard{{State: "learning", Due: step}}).Due[0] != true {
		t.Fatal("22:01 did not serve a lone learning card due at 22:05")
	}
	blocked := Decide(at2201, cut2201, ahead, []DueCard{
		{State: "learning", Due: step},
		{State: "review", Due: at2201},
	})
	if blocked.Due[0] || !blocked.Due[1] {
		t.Fatalf("22:01 served learn-ahead beside a due review card: %v", blocked.Due)
	}
	at2205 := time.Date(2026, 10, 18, 22, 5, 0, 0, denver)
	dueNow := Decide(at2205, DueCutoff(at2205, denver), ahead, []DueCard{
		{State: "learning", Due: step},
		{State: "review", Due: at2205},
	})
	if !dueNow.Due[0] || !dueNow.Due[1] {
		t.Fatalf("22:05 left a due learning card out: %v", dueNow.Due)
	}
	at2159 := time.Date(2026, 10, 18, 21, 59, 0, 0, denver)
	if !Decide(at2159, DueCutoff(at2159, denver), ahead, []DueCard{{State: "learning", Due: at2159}}).Due[0] {
		t.Fatal("21:59 did not serve the step that is due now, before the Again")
	}

	// 03:59 and 04:01 each have a review card on either side of that cutoff.
	at359 := time.Date(2026, 10, 19, 3, 59, 0, 0, denver)
	at401 := time.Date(2026, 10, 19, 4, 1, 0, 0, denver)
	cut359 := DueCutoff(at359, denver)
	cut401 := DueCutoff(at401, denver)
	early := time.Date(2026, 10, 19, 3, 0, 0, 0, denver)
	laterToday := time.Date(2026, 10, 19, 5, 0, 0, 0, denver)
	nextEvening := time.Date(2026, 10, 20, 5, 0, 0, 0, denver)
	if early.After(cut359) || !laterToday.After(cut359) || laterToday.After(cut401) || !nextEvening.After(cut401) {
		t.Fatalf("cutoffs %s %s do not split 03:00, 05:00, and the next 05:00", cut359, cut401)
	}
	side359 := Decide(at359, cut359, ahead, []DueCard{
		{State: "review", Due: early},
		{State: "review", Due: laterToday},
	})
	if !side359.Due[0] || side359.Due[1] {
		t.Fatalf("03:59 sides: %v", side359.Due)
	}
	side401 := Decide(at401, cut401, ahead, []DueCard{
		{State: "review", Due: early},
		{State: "review", Due: laterToday},
		{State: "review", Due: nextEvening},
	})
	if !side401.Due[0] || !side401.Due[1] || side401.Due[2] {
		t.Fatalf("04:01 sides: %v", side401.Due)
	}
	if Decide(at359, cut359, ahead, nil).Day != "2026-10-18" || Decide(at401, cut401, ahead, nil).Day != "2026-10-19" {
		t.Fatalf("study day 03:59 %s 04:01 %s", Decide(at359, cut359, ahead, nil).Day, Decide(at401, cut401, ahead, nil).Day)
	}

	// 03:50: learn-ahead reaches past the 04:00 cutoff. The fetch has to
	// load that far, or Today would miss a learning card Review still serves.
	late := time.Date(2026, 10, 19, 3, 50, 0, 0, denver)
	lateCutoff := DueCutoff(late, denver)
	learningPast := time.Date(2026, 10, 19, 4, 5, 0, 0, denver)
	if !learningPast.After(lateCutoff) || learningPast.After(late.Add(ahead)) {
		t.Fatalf("04:05 is not past the cutoff and inside learn-ahead: cutoff %s", lateCutoff)
	}
	if !Decide(late, lateCutoff, ahead, []DueCard{{State: "learning", Due: learningPast}}).Due[0] {
		t.Fatal("a learning card 15 minutes away was not due")
	}
	if Decide(late, lateCutoff, ahead, []DueCard{{State: "review", Due: learningPast}}).Due[0] {
		t.Fatal("a review card past the cutoff was due")
	}
	if !DueFetchLimit(late, denver, ahead).Equal(late.Add(ahead)) {
		t.Fatalf("fetch limit %s, want %s", DueFetchLimit(late, denver, ahead), late.Add(ahead))
	}
	if !DueFetchLimit(late, denver, 0).Equal(lateCutoff) {
		t.Fatalf("zero cap fetch %s, want the cutoff %s", DueFetchLimit(late, denver, 0), lateCutoff)
	}
	if !DueFetchLimit(now, denver, ahead).Equal(cutoff) {
		t.Fatalf("fetch limit at 22:00 = %s, want the cutoff %s", DueFetchLimit(now, denver, ahead), cutoff)
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
