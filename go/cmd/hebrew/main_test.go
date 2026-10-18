package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
)

func TestHTTPTimeoutsUnderCloudflareLimit(t *testing.T) {
	const limit = 125 * time.Second
	srv := newServer("127.0.0.1:0", http.NewServeMux())
	timeouts := []struct {
		name string
		got  time.Duration
	}{
		{"ReadHeaderTimeout", srv.ReadHeaderTimeout},
		{"ReadTimeout", srv.ReadTimeout},
		{"WriteTimeout", srv.WriteTimeout},
		{"IdleTimeout", srv.IdleTimeout},
		{"healthPingTimeout", healthPingTimeout},
	}
	for _, tc := range timeouts {
		if tc.got <= 0 || tc.got >= limit {
			t.Fatalf("%s = %s, want > 0 and < %s", tc.name, tc.got, limit)
		}
	}
	if healthPingTimeout > httpWriteTimeout {
		t.Fatalf("health ping %s exceeds write timeout %s", healthPingTimeout, httpWriteTimeout)
	}
}

func TestListenAddr(t *testing.T) {
	if got := listenAddr("0.0.0.0", 8090); got != "0.0.0.0:8090" {
		t.Fatalf("default bind: %s", got)
	}
	if got := listenAddr("127.0.0.1", 8080); got != "127.0.0.1:8080" {
		t.Fatalf("local bind: %s", got)
	}
}

func TestHealthz(t *testing.T) {
	good := openNodeDB(t, "apply")
	defer good.Close()
	bad := openNodeDB(t, "empty")
	defer bad.Close()

	goodLedger := mustLedger(t, good)
	badLedger := mustLedgerFail(t, bad)

	okRec := httptest.NewRecorder()
	newMux(good, goodLedger, "").ServeHTTP(okRec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assertHealth(t, okRec, http.StatusOK, true)

	badRec := httptest.NewRecorder()
	newMux(bad, badLedger, "").ServeHTTP(badRec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assertHealth(t, badRec, http.StatusServiceUnavailable, false)

	cached := db.Ledger{OK: false, Error: "schema_migrations mismatch: cached"}
	cachedRec := httptest.NewRecorder()
	newMux(good, cached, "").ServeHTTP(cachedRec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assertHealth(t, cachedRec, http.StatusServiceUnavailable, false)
	if !strings.Contains(cachedRec.Body.String(), "cached") {
		t.Fatalf("healthz re-read the ledger: %s", cachedRec.Body.String())
	}

	good.Close()
	pingRec := httptest.NewRecorder()
	newMux(good, goodLedger, "").ServeHTTP(pingRec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assertHealth(t, pingRec, http.StatusServiceUnavailable, false)
	if !strings.Contains(pingRec.Body.String(), "db ping:") {
		t.Fatalf("closed db: %s", pingRec.Body.String())
	}

	postRec := httptest.NewRecorder()
	newMux(bad, badLedger, "").ServeHTTP(postRec, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz: %d", postRec.Code)
	}

	otherRec := httptest.NewRecorder()
	newMux(bad, badLedger, "").ServeHTTP(otherRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("GET /: %d", otherRec.Code)
	}
}

func TestSharedMountUsesHebrewRoot(t *testing.T) {
	root := t.TempDir()
	player := filepath.Join(root, "shared", "dist")
	if err := os.MkdirAll(player, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(player, "player.js"), []byte("player-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	conn := openNodeDB(t, "apply")
	defer conn.Close()
	ledger := mustLedger(t, conn)

	t.Setenv("HEBREW_DEV_WEB", "")
	off := httptest.NewRecorder()
	newMux(conn, ledger, root).ServeHTTP(off, httptest.NewRequest(http.MethodGet, "/shared/dist/player.js", nil))
	if off.Code != http.StatusNotFound {
		t.Fatalf("flag off: %d", off.Code)
	}

	t.Setenv("HEBREW_DEV_WEB", "1")
	on := httptest.NewRecorder()
	newMux(conn, ledger, root).ServeHTTP(on, httptest.NewRequest(http.MethodGet, "/shared/dist/player.js", nil))
	if on.Code != http.StatusOK || on.Body.String() != "player-bytes" {
		t.Fatalf("player.js: %d %q", on.Code, on.Body.String())
	}
	health := httptest.NewRecorder()
	newMux(conn, ledger, root).ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("healthz: %d %s", health.Code, health.Body.String())
	}
}

func mustLedger(t *testing.T, conn *sql.DB) db.Ledger {
	t.Helper()
	ledger, err := db.CheckLedger(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	return ledger
}

func mustLedgerFail(t *testing.T, conn *sql.DB) db.Ledger {
	t.Helper()
	ledger, err := db.CheckLedger(context.Background(), conn)
	if err == nil || ledger.OK {
		t.Fatal("expected ledger failure")
	}
	return ledger
}

func assertHealth(t *testing.T, rec *httptest.ResponseRecorder, status int, ok bool) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status: got %d want %d body %s", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: %s", ct)
	}
	var body struct {
		OK     bool      `json:"ok"`
		Ledger db.Ledger `json:"ledger"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.OK != ok || body.Ledger.OK != ok {
		t.Fatalf("body: %+v", body)
	}
	if ok && len(body.Ledger.Versions) != len(db.ExpectedMigrations) {
		t.Fatalf("versions: %d", len(body.Ledger.Versions))
	}
	if !ok && body.Ledger.Error == "" {
		t.Fatal("503 body has no ledger error")
	}
}

func openNodeDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join("..", "..", "internal", "db", "testdata", "apply-migrations.js")
	cmd := exec.Command("node", script, mode, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node %s: %v\n%s", mode, err, out)
	}
	conn, err := db.OpenReadOnly(filepath.Join(dir, "hebrew.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	return conn
}
