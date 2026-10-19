package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddedTrees(t *testing.T) {
	static, err := Static()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(static, "README.txt"); err != nil {
		t.Fatal(err)
	}
	views, err := fs.Sub(Assets, "views")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(views, "README.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestHealthRoute(t *testing.T) {
	h := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d", rr.Code)
	}
}
