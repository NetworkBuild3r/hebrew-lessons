package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
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
	}
	for _, tc := range timeouts {
		if tc.got <= 0 || tc.got >= limit {
			t.Fatalf("%s = %s, want > 0 and < %s", tc.name, tc.got, limit)
		}
	}
}

func TestHealthz(t *testing.T) {
	good := openNodeDB(t, "apply")
	defer good.Close()
	bad := openNodeDB(t, "empty")
	defer bad.Close()

	okRec := httptest.NewRecorder()
	newMux(good).ServeHTTP(okRec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assertHealth(t, okRec, http.StatusOK, true)

	badRec := httptest.NewRecorder()
	newMux(bad).ServeHTTP(badRec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assertHealth(t, badRec, http.StatusServiceUnavailable, false)

	postRec := httptest.NewRecorder()
	newMux(good).ServeHTTP(postRec, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz: %d", postRec.Code)
	}

	otherRec := httptest.NewRecorder()
	newMux(good).ServeHTTP(otherRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("GET /: %d", otherRec.Code)
	}
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
