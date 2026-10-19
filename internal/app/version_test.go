package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/buildinfo"
)

func TestVersionJSONAndNoStore(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir(), MCPAuthToken: "test-token"})
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	want := buildinfo.Current()
	for _, name := range []string{"web", "mcp"} {
		h := a.Web()
		if name == "mcp" {
			h = a.MCP()
		}
		req := httptest.NewRequest(http.MethodGet, "/version", nil)
		req.Header.Set("Authorization", "Bearer not-the-token")
		req.AddCookie(&http.Cookie{Name: "hebrew_session", Value: "nope"})
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d body %s", name, rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s content-type %q", name, ct)
		}
		if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("%s cache-control %q", name, cc)
		}
		dec := json.NewDecoder(rr.Body)
		dec.DisallowUnknownFields()
		var body buildinfo.Version
		if err := dec.Decode(&body); err != nil {
			t.Fatalf("%s json: %v", name, err)
		}
		if body != want {
			t.Fatalf("%s version = %+v, want %+v", name, body, want)
		}
		if body.Go != runtime.Version() {
			t.Fatalf("%s go = %q, runtime %q", name, body.Go, runtime.Version())
		}
		if body.Commit == "" {
			t.Fatalf("%s commit is empty", name)
		}
	}
}

func TestVersionDoesNotReplaceOtherRoutes(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir()})
	web := httptest.NewRecorder()
	a.Web().ServeHTTP(web, httptest.NewRequest(http.MethodPost, "/version", nil))
	if web.Code == http.StatusOK || strings.Contains(web.Body.String(), `"builtAt"`) {
		t.Fatalf("web POST /version = %d %s", web.Code, web.Body.String())
	}
	mcp := httptest.NewRecorder()
	a.MCP().ServeHTTP(mcp, httptest.NewRequest(http.MethodPost, "/version", nil))
	if mcp.Code != http.StatusNotFound || mcp.Body.String() != "Not found" {
		t.Fatalf("mcp POST /version = %d %q", mcp.Code, mcp.Body.String())
	}
}

func TestHealthzKeepsStatusAndAddsCommit(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir()})
	want := buildinfo.Current().Commit
	for _, h := range []http.Handler{a.Web(), a.MCP()} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body %s", rr.Code, rr.Body.String())
		}
		assertHealthCommit(t, rr.Body.Bytes(), true, want)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	for _, h := range []http.Handler{a.Web(), a.MCP()} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("closed status = %d body %s", rr.Code, rr.Body.String())
		}
		assertHealthCommit(t, rr.Body.Bytes(), false, want)
	}
}

func assertHealthCommit(t *testing.T, raw []byte, ok bool, commit string) {
	t.Helper()
	var legacy struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.OK != ok {
		t.Fatalf("ok = %v, want %v", legacy.OK, ok)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var body struct {
		OK     bool   `json:"ok"`
		Commit string `json:"commit"`
	}
	if err := dec.Decode(&body); err != nil {
		t.Fatalf("health json: %v (%s)", err, raw)
	}
	if body.OK != ok || body.Commit != commit {
		t.Fatalf("health = %+v, want ok=%v commit=%s", body, ok, commit)
	}
}
