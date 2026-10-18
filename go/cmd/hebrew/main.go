// Command hebrew loads config, opens the database read-only, checks the
// migration ledger, and serves GET /healthz.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/config"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
)

// HTTP timeouts stay under Cloudflare's 125s proxy read timeout (ADR-0016).
const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	httpWriteTimeout      = 30 * time.Second
	httpIdleTimeout       = 90 * time.Second

	// listenHost matches Node's HOST default (server/web.js). HOST is not read.
	listenHost = "127.0.0.1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "hebrew: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	conn, err := db.OpenReadOnly(cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	ledger, err := db.CheckLedger(context.Background(), conn)
	if err != nil {
		log.Printf("ledger: %v", err)
	} else {
		log.Printf("ledger ok (%d versions)", len(ledger.Versions))
	}

	srv := newServer(net.JoinHostPort(listenHost, strconv.Itoa(cfg.Port)), newMux(conn))
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	log.Printf("listening on %s", ln.Addr())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
}

func newMux(conn *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler(conn))
	return mux
}

func healthHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ledger, err := db.CheckLedger(r.Context(), conn)
		ok := err == nil && ledger.OK
		status := http.StatusOK
		if !ok {
			status = http.StatusServiceUnavailable
			if ledger.Error == "" && err != nil {
				ledger.OK = false
				ledger.Error = err.Error()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		encodeErr := json.NewEncoder(w).Encode(struct {
			OK     bool      `json:"ok"`
			Ledger db.Ledger `json:"ledger"`
		}{OK: ok, Ledger: ledger})
		if encodeErr != nil {
			log.Printf("healthz encode: %v", encodeErr)
		}
	}
}
