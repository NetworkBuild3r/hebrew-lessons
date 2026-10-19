// Command hebrew serves the learner app and the authoring MCP from one process.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/app"
)

const shutdownTimeout = 5 * time.Second

func main() {
	if err := run(os.Args[1:]); err != nil {
		code, quiet := statusCode(err)
		if !quiet {
			fmt.Fprintf(os.Stderr, "hebrew: %v\n", err)
		}
		os.Exit(code)
	}
}

// statusCode maps an error to a process status. A validation refusal
// implements ExitCode and Quiet so the summary line is the only log.
// Every other error is status 1.
func statusCode(err error) (code int, quiet bool) {
	if err == nil {
		return 0, true
	}
	var eb interface {
		ExitCode() int
		Quiet() bool
	}
	if errors.As(err, &eb) {
		return eb.ExitCode(), eb.Quiet()
	}
	return 1, false
}

func run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "import-audio":
			return runImport(args[1:])
		case "import-tanakh":
			return runImportTanakh(args[1:])
		case "jobs":
			return runJobs(args[1:])
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
	}
	return runServer()
}

func runServer() error {
	cfg, err := configFromEnv()
	if err != nil {
		return err
	}
	setLogLevel(cfg.LogLevel)
	warnOptionalConfig(cfg)

	application, err := app.NewApp(cfg)
	if err != nil {
		return err
	}

	slog.Info("starting",
		"web", cfg.HTTPAddr,
		"mcp", cfg.MCPAddr,
		"data_dir", cfg.DataDir,
		"public_origin", cfg.PublicOrigin,
		"cookie_secure", cfg.CookieSecure,
		"trusted_proxies", len(cfg.TrustedProxies),
		"tts", application.TTS() != nil,
	)

	stopJobs, err := startJobs(cfg, application)
	if err != nil {
		application.Close()
		return err
	}
	err = serve(cfg, application)
	stopJobs()
	closeErr := application.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func serve(cfg app.Config, application *app.App) error {
	webSrv := newServer(cfg.HTTPAddr, application.Web())
	mcpSrv := newServer(cfg.MCPAddr, application.MCP())

	webLn, err := net.Listen("tcp", webSrv.Addr)
	if err != nil {
		return fmt.Errorf("listen web %s: %w", webSrv.Addr, err)
	}
	mcpLn, err := net.Listen("tcp", mcpSrv.Addr)
	if err != nil {
		webLn.Close()
		return fmt.Errorf("listen mcp %s: %w", mcpSrv.Addr, err)
	}
	slog.Info("listening", "web", webLn.Addr().String(), "mcp", mcpLn.Addr().String())

	errCh := make(chan error, 2)
	go func() { errCh <- serveOne(webSrv, webLn) }()
	go func() { errCh <- serveOne(mcpSrv, mcpLn) }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case err := <-errCh:
		stop()
		shutdownServers(webSrv, mcpSrv)
		if err != nil {
			return err
		}
		return nil
	}
	return shutdownServers(webSrv, mcpSrv)
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
}

func serveOne(srv *http.Server, ln net.Listener) error {
	err := srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func shutdownServers(servers ...*http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	var err error
	for _, srv := range servers {
		if e := srv.Shutdown(ctx); e != nil && err == nil {
			err = e
		}
	}
	return err
}
