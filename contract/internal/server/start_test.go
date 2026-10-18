package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestWaitReady(t *testing.T) {
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer web.Close()
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer mcp.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := WaitReady(ctx, web.URL, mcp.URL); err != nil {
		t.Fatal(err)
	}
}

func TestWaitReadyTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := WaitReady(ctx, "http://127.0.0.1:1", "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestStartAndClose(t *testing.T) {
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer web.Close()
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mcp.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proc, err := Start(ctx, Options{
		Command: "sleep 30",
		WebURL:  web.URL,
		MCPURL:  mcp.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := proc.DataDir
	proc.Close()
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("temp DATA_DIR was kept")
	}
}
