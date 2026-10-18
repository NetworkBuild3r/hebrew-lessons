// Command hebrew loads config, opens the database read-only, checks the
// migration ledger, and serves GET /healthz. HEBREW_DEV_WEB=1 also serves
// the guides and words pages and /shared/ from HEBREW_ROOT/shared.
// /words/ reads a fixture library until learner/library.List is wired.
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
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/pages"
)

// HTTP timeouts stay under Cloudflare's 125s proxy read timeout (ADR-0016).
const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	httpWriteTimeout      = 30 * time.Second
	httpIdleTimeout       = 90 * time.Second

	// healthPingTimeout bounds the cheap DB ping on GET /healthz. The ledger
	// is cached at startup, so a probe does not re-read schema_migrations.
	// Stay well under the HTTP write timeout and Cloudflare's 125s limit.
	healthPingTimeout = 2 * time.Second
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
		if ledger.Error == "" {
			ledger.OK = false
			ledger.Error = err.Error()
		}
	} else {
		log.Printf("ledger ok (%d versions)", len(ledger.Versions))
	}

	srv := newServer(listenAddr(cfg.Host, cfg.Port), newMux(conn, ledger, cfg.HebrewRoot))
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

func listenAddr(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func newMux(conn *sql.DB, ledger db.Ledger, hebrewRoot string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler(conn, ledger))
	pages.Mount(mux, conn)
	pages.MountShared(mux, hebrewRoot)
	return mux
}

func healthHandler(conn *sql.DB, ledger db.Ledger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()
		pingErr := conn.PingContext(ctx)

		out := ledger
		ok := ledger.OK && pingErr == nil
		if ledger.OK && pingErr != nil {
			out.OK = false
			out.Error = "db ping: " + pingErr.Error()
		}
		status := http.StatusOK
		if !ok {
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		encodeErr := json.NewEncoder(w).Encode(struct {
			OK     bool      `json:"ok"`
			Ledger db.Ledger `json:"ledger"`
		}{OK: ok, Ledger: out})
		if encodeErr != nil {
			log.Printf("healthz encode: %v", encodeErr)
		}
	}
}
