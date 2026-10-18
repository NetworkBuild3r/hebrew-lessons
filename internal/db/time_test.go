package db

import (
	"context"
	"testing"
	"time"
)

func TestFormatTimeTextCompareOrdersAcrossSecondAndSubsecond(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	userID := insertUser(t, s, "order@example.com", "sub-order", "learner")

	// boundary is an exact second. The neighbour one nanosecond earlier
	// crosses that second. The neighbour one nanosecond later is a
	// sub-second gap. RFC3339Nano writes the exact second with no fraction
	// ("...01Z") and the later instant with nine digits ("...01.000000001Z"),
	// and 'Z' sorts after '.', so the earlier instant would win a text
	// comparison. The fixed layout keeps both at the same width.
	boundary := time.Date(2026, 6, 15, 12, 0, 1, 0, time.UTC)
	times := []time.Time{
		boundary.Add(-time.Nanosecond),
		boundary,
		boundary.Add(time.Nanosecond),
		boundary.Add(time.Second),
	}
	earlyNano := boundary.Format(time.RFC3339Nano)
	laterNano := boundary.Add(time.Nanosecond).Format(time.RFC3339Nano)
	if earlyNano <= laterNano {
		t.Fatalf("RFC3339Nano fixture no longer mis-orders: %s <= %s", earlyNano, laterNano)
	}

	for i, tm := range times {
		text := FormatTime(tm)
		if len(text) != len(timestampLayout) {
			t.Fatalf("FormatTime(%s) = %s, width %d", tm, text, len(text))
		}
		if i > 0 {
			prev := FormatTime(times[i-1])
			if prev >= text {
				t.Fatalf("lexical order %s >= %s", prev, text)
			}
		}
		back, err := ParseTime(text)
		if err != nil {
			t.Fatal(err)
		}
		if !back.Equal(tm.UTC()) {
			t.Fatalf("ParseTime(%s) = %s", text, back)
		}
		lexemeID := insertLexeme(t, s, "due-"+text)
		mustExec(t, s, `INSERT INTO cards (user_id, lexeme_id, dir, state, due, stability, difficulty, reps, lapses, suspended)
			VALUES (?, ?, 'he_en', 'new', ?, 0, 0, 0, 0, 0)`, userID, lexemeID, text)
	}

	for _, cutoff := range times {
		rows, err := s.conn.QueryContext(ctx,
			`SELECT due FROM cards WHERE user_id = ? AND due <= ? ORDER BY due`,
			userID, FormatTime(cutoff),
		)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var due string
			if err := rows.Scan(&due); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			got = append(got, due)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()

		var want []string
		for _, tm := range times {
			if !tm.After(cutoff) {
				want = append(want, FormatTime(tm))
			}
		}
		if len(got) != len(want) {
			t.Fatalf("due <= %s: got %d rows %v, want %v", FormatTime(cutoff), len(got), got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("due <= %s: got %v, want %v", FormatTime(cutoff), got, want)
			}
		}
	}
}

func TestParseTimeAcceptsOlderLayouts(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Time
	}{
		{raw: "2020-01-01T00:00:00Z", want: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{raw: "2020-01-01T00:00:00.5Z", want: time.Date(2020, 1, 1, 0, 0, 0, 500000000, time.UTC)},
	}
	for _, tc := range cases {
		got, err := ParseTime(tc.raw)
		if err != nil {
			t.Fatalf("ParseTime(%s): %v", tc.raw, err)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("ParseTime(%s) = %s, want %s", tc.raw, got, tc.want)
		}
	}
	if _, err := ParseTime("not-a-time"); err == nil {
		t.Fatal("ParseTime accepted a non-time")
	}
}

func TestFormatTimeConvertsToUTC(t *testing.T) {
	local := time.Date(2026, 6, 15, 15, 0, 1, 0, time.FixedZone("IDT", 3*60*60))
	utc := local.UTC()
	if FormatTime(local) != FormatTime(utc) {
		t.Fatalf("FormatTime(local) = %s, FormatTime(utc) = %s", FormatTime(local), FormatTime(utc))
	}
	if !utc.Equal(time.Date(2026, 6, 15, 12, 0, 1, 0, time.UTC)) {
		t.Fatalf("utc = %s", utc)
	}
}
