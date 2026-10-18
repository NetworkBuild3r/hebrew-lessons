package auth

import (
	"context"
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

func TestSafeNextPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "/"},
		{"   ", "/"},
		{"  /progress/  ", "/progress/"},
		{"/review/", "/review/"},
		{"/words/?q=1", "/words/?q=1"},
		{"evil", "/"},
		{"//evil.example", "/"},
		{"/\\evil", "/"},
		{`\evil`, "/"},
		{"https://evil.example/", "/"},
		{"javascript:alert(1)", "/"},
		{"/\x00secret", "/"},
		{"/login/?next=%2F", "/login/?next=%2F"},
	}
	for _, tc := range cases {
		if got := SafeNextPath(tc.in); got != tc.want {
			t.Errorf("SafeNextPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestReadSessionToken(t *testing.T) {
	header := "theme=light; hebrew_session=one; __Host-hebrew_session=two; hebrew_session=later"
	if got := ReadSessionToken(header, false); got != "one" {
		t.Fatalf("insecure primary: %q", got)
	}
	if got := ReadSessionToken(header, true); got != "two" {
		t.Fatalf("secure primary: %q", got)
	}
	if got := ReadSessionToken("__Host-hebrew_session=host-only", false); got != "host-only" {
		t.Fatalf("fallback to host cookie: %q", got)
	}
	if got := ReadSessionToken("hebrew_session=plain", true); got != "plain" {
		t.Fatalf("fallback to base cookie: %q", got)
	}
	encoded := "hebrew_session=" + encodeURIComponent("tok-\u05e9")
	if got := ReadSessionToken(encoded, false); got != "tok-\u05e9" {
		t.Fatalf("decoded: %q", got)
	}
	if got := ReadSessionToken(`hebrew_session="quoted"`, false); got != "quoted" {
		t.Fatalf("quoted: %q", got)
	}
	if got := ReadSessionToken("hebrew_session=", false); got != "" {
		t.Fatalf("empty value: %q", got)
	}
}

func TestRequireSession(t *testing.T) {
	next := RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("html redirect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/words/", nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status %d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/login/?next=%2Fwords%2F" {
			t.Fatalf("location %q", loc)
		}
	})

	t.Run("trailing slash without accept", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/words/", nil)
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("hostile path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = `/\evil`
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, req)
		if loc := rec.Header().Get("Location"); loc != "/login/?next=%2F" {
			t.Fatalf("location %q", loc)
		}
	})

	t.Run("json unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/me/library", nil)
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != `{"error":"Unauthorized","code":"no_session"}` {
			t.Fatalf("body %s", rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("cache %q", rec.Header().Get("Cache-Control"))
		}
	})

	t.Run("passes with user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/me/library", nil)
		req = req.WithContext(NewContext(req.Context(), &User{ID: 1, Role: "user"}, "hash"))
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

func TestResolveMatchesNode(t *testing.T) {
	dataDir := t.TempDir()
	outPath := filepath.Join(dataDir, "sessions.jsonl")
	cmd := exec.Command("node", filepath.Join("testdata", "sessions.js"), dataDir, outPath)
	cmd.Env = nodeEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node sessions: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	var meta struct {
		DB  string `json:"db"`
		Now string `json:"now"`
	}
	var cases []sessionCase
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var kind struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal([]byte(line), &kind); err != nil {
			t.Fatal(err)
		}
		switch kind.Kind {
		case "meta":
			if err := json.Unmarshal([]byte(line), &meta); err != nil {
				t.Fatal(err)
			}
		case "case":
			var item sessionCase
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				t.Fatal(err)
			}
			cases = append(cases, item)
		default:
			t.Fatalf("kind %s", kind.Kind)
		}
	}
	now, err := time.Parse(time.RFC3339Nano, meta.Now)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.OpenReadOnly(meta.DB)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var learnerToken string
	for _, item := range cases {
		t.Run(item.Name, func(t *testing.T) {
			if item.Token != "" && HashToken(item.Token) != item.TokenHash {
				t.Fatalf("hash %s want %s", HashToken(item.Token), item.TokenHash)
			}
			got, err := Resolve(context.Background(), conn, item.Token, now)
			if err != nil {
				t.Fatal(err)
			}
			if item.User == nil {
				if got != nil {
					t.Fatalf("got user %+v", got)
				}
			} else {
				if got == nil {
					t.Fatal("got nil user")
				}
				if got.ID != item.User.ID || got.Username != item.User.Username || got.Role != item.User.Role || got.Status != item.User.Status {
					t.Fatalf("got %+v want %+v", got, item.User)
				}
				if !sameString(got.DisplayName, item.User.DisplayName) || !sameString(got.GoogleEmail, item.User.GoogleEmail) {
					t.Fatalf("names got %+v want %+v", got, item.User)
				}
				if got.MustChangePassword != item.User.MustChangePassword {
					t.Fatalf("must change: %+v", got)
				}
			}
			if item.LastSeenAt != "" {
				var seen string
				if err := conn.QueryRowContext(context.Background(),
					`SELECT last_seen_at FROM sessions WHERE token_hash = ?`, item.TokenHash,
				).Scan(&seen); err != nil {
					t.Fatal(err)
				}
				if seen != item.LastSeenAt {
					t.Fatalf("last_seen_at changed from %s to %s", item.LastSeenAt, seen)
				}
			}
		})
		if item.Name == "learner" {
			learnerToken = item.Token
		}
	}

	handler := Middleware(conn, false)(RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Role != "user" || TokenHashFromContext(r.Context()) == "" {
			t.Errorf("context user %+v hash %q", user, TokenHashFromContext(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Cookie", "hebrew_session="+learnerToken)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("middleware status %d body %s", rec.Code, rec.Body.String())
	}

	secure := Middleware(conn, true)(RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			t.Error("missing user from host cookie")
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Cookie", "__Host-hebrew_session="+learnerToken)
	req.Header.Set("Accept", "application/json")
	rec = httptest.NewRecorder()
	secure.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("host cookie status %d", rec.Code)
	}
}

type sessionCase struct {
	Name       string `json:"name"`
	Token      string `json:"token"`
	TokenHash  string `json:"tokenHash"`
	LastSeenAt string `json:"lastSeenAt"`
	User       *User  `json:"user"`
}

func sameString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func nodeEnv() []string {
	out := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "TZ=") {
			continue
		}
		out = append(out, value)
	}
	return append(out, "TZ=UTC")
}

func TestResolveRejectsNil(t *testing.T) {
	if _, err := Resolve(context.Background(), nil, "token", time.Now()); err == nil {
		t.Fatal("expected nil database error")
	}
	//lint:ignore SA1012 nil context is the case under test
	if _, err := Resolve(nil, nil, "token", time.Now()); err == nil {
		t.Fatal("expected nil context error")
	}
}
