package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionTimesKeepSubsecondPrecisionAndOrder(t *testing.T) {
	boundary := time.Date(2026, 6, 15, 12, 0, 1, 0, time.UTC)
	early := boundary.Add(-time.Nanosecond)
	late := boundary.Add(time.Nanosecond)
	s := openStore(t, fixedClock{early})
	ctx := context.Background()

	user, err := s.CreateUser(ctx, "sess@example.com", "sub-sess", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if !user.CreatedAt.Equal(early) {
		t.Fatalf("created_at round-trip = %s, want %s", user.CreatedAt, early)
	}
	again, err := s.UserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatal(err)
	}
	if !again.CreatedAt.Equal(early) {
		t.Fatalf("created_at reread = %s, want %s", again.CreatedAt, early)
	}

	if err := s.InsertSession(ctx, "early", user.ID, "csrf-early", early); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertSession(ctx, "on-boundary", user.ID, "csrf-bound", boundary); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertSession(ctx, "late", user.ID, "csrf-late", late); err != nil {
		t.Fatal(err)
	}

	gotEarly, _, err := s.SessionUser(ctx, "early")
	if err != nil {
		t.Fatal(err)
	}
	if !gotEarly.ExpiresAt.Equal(early) {
		t.Fatalf("early expires_at = %s, want %s", gotEarly.ExpiresAt, early)
	}
	gotLate, _, err := s.SessionUser(ctx, "late")
	if err != nil {
		t.Fatal(err)
	}
	if !gotLate.ExpiresAt.Equal(late) {
		t.Fatalf("late expires_at = %s, want %s", gotLate.ExpiresAt, late)
	}

	rows, err := s.conn.QueryContext(ctx, `SELECT id_hash, expires_at FROM sessions ORDER BY expires_at`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var hashes []string
	var prev string
	for rows.Next() {
		var hash, exp string
		if err := rows.Scan(&hash, &exp); err != nil {
			t.Fatal(err)
		}
		if prev != "" && prev >= exp {
			t.Fatalf("expires_at text order %s >= %s", prev, exp)
		}
		if exp != FormatTime(map[string]time.Time{"early": early, "on-boundary": boundary, "late": late}[hash]) {
			t.Fatalf("%s expires_at = %s", hash, exp)
		}
		prev = exp
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(hashes) != 3 || hashes[0] != "early" || hashes[1] != "on-boundary" || hashes[2] != "late" {
		t.Fatalf("order = %v", hashes)
	}

	// expires_at <= boundary drops the exact second and everything before it.
	// One nanosecond later stays, which a second-truncated stamp would not.
	if err := s.DeleteExpiredSessions(ctx, boundary); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SessionUser(ctx, "early"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("early session error = %v, want ErrNotFound", err)
	}
	if _, _, err := s.SessionUser(ctx, "on-boundary"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("boundary session error = %v, want ErrNotFound", err)
	}
	kept, _, err := s.SessionUser(ctx, "late")
	if err != nil {
		t.Fatal(err)
	}
	if !kept.ExpiresAt.Equal(late) {
		t.Fatalf("kept expires_at = %s, want %s", kept.ExpiresAt, late)
	}

	slid := late.Add(2 * time.Nanosecond)
	if err := s.UpdateSessionExpiry(ctx, "late", slid); err != nil {
		t.Fatal(err)
	}
	slidRow, _, err := s.SessionUser(ctx, "late")
	if err != nil {
		t.Fatal(err)
	}
	if !slidRow.ExpiresAt.Equal(slid) {
		t.Fatalf("slid expires_at = %s, want %s", slidRow.ExpiresAt, slid)
	}
}
