package app

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestAppStoreCreatesUsers(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir()})
	ctx := context.Background()

	learner, err := a.Store().CreateUser(ctx, "Learner@Example.com", "sub-learner", "learner")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := a.Store().CreateUser(ctx, "admin@example.com", "sub-admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if learner.Role != "learner" || learner.Email != "learner@example.com" || learner.GoogleSub != "sub-learner" {
		t.Fatalf("learner %+v", learner)
	}
	if admin.Role != "admin" || admin.ID == learner.ID {
		t.Fatalf("admin %+v learner %+v", admin, learner)
	}

	got, err := a.Store().UserByEmail(ctx, "learner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != learner.ID || got.Role != "learner" {
		t.Fatalf("lookup %+v", got)
	}
	if _, err := a.Store().CreateUser(ctx, "teacher@example.com", "sub-teacher", "teacher"); err == nil {
		t.Fatal("expected role error")
	}
}

func TestStartupWarnings(t *testing.T) {
	// The placement-audio count is logged from a goroutine. A locked buffer
	// keeps that write from racing the reads below.
	var buf lockedBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	_ = newTestApp(t, Config{
		DataDir:      t.TempDir(),
		PublicOrigin: "https://hebrew.example",
		CookieSecure: false,
	})
	log := buf.String()
	if !strings.Contains(log, "COOKIE_SECURE is off") {
		t.Fatalf("cookie warning missing: %s", log)
	}
	if !strings.Contains(log, "TRUSTED_PROXIES is empty") {
		t.Fatalf("proxy warning missing: %s", log)
	}

	buf.Reset()
	_ = newTestApp(t, Config{
		DataDir:        t.TempDir(),
		PublicOrigin:   "https://hebrew.example",
		CookieSecure:   false,
		Dev:            true,
		TrustedProxies: []string{"127.0.0.1"},
	})
	log = buf.String()
	if strings.Contains(log, "COOKIE_SECURE is off") || strings.Contains(log, "TRUSTED_PROXIES is empty") {
		t.Fatalf("warnings should be quiet: %s", log)
	}
}
