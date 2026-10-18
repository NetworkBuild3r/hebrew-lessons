package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/config"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
)

// App is one MCP process. The HTTP front in http.go owns the bytes on the
// wire. sdk is constructed with the same 42 raw schemas and with logging
// turned off, and it is not the request handler: the SDK transport cannot
// emit Node's SSE key order, protocol version 2024-10-07, or isError tool
// failures. One process-wide registry matches Node, whose rate limit is
// also process-global.
type App struct {
	lessonsDir string
	db         *sql.DB
	audit      AuditWriter
	learner    LearnerReader
	clock      func() time.Time
	readsMu    sync.Mutex
	reads      *readWindow
	sdk        *sdkmcp.Server
	schemas    map[string]schema
	toolOrder  []string
}

// Options configures a process. Nil DB means lesson tools still answer and
// database tools say the table is not readable.
type Options struct {
	LessonsDir string
	DB         *sql.DB
	Audit      AuditWriter
	Learner    LearnerReader
	Clock      func() time.Time
}

// New loads the Node tools/list schemas and registers them on an SDK server.
func New(opt Options) (*App, error) {
	schemas, order, err := loadSchemas(toolsListResult)
	if err != nil {
		return nil, err
	}
	a := &App{
		lessonsDir: opt.LessonsDir,
		db:         opt.DB,
		audit:      opt.Audit,
		learner:    opt.Learner,
		clock:      opt.Clock,
		schemas:    schemas,
		toolOrder:  order,
	}
	a.sdk = newSDKServer(a)
	return a, nil
}

func loadSchemas(raw []byte) (map[string]schema, []string, error) {
	root, err := Parse(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, nil, err
	}
	tools, ok := root.Get("tools")
	if !ok || tools.Kind != KindArray {
		return nil, nil, errors.New("mcp: tools/list golden has no tools array")
	}
	out := make(map[string]schema, len(tools.Arr))
	order := make([]string, 0, len(tools.Arr))
	for _, tool := range tools.Arr {
		name, ok := tool.Get("name")
		if !ok || name.Kind != KindString || name.Str == "" {
			return nil, nil, errors.New("mcp: tool missing name")
		}
		input, ok := tool.Get("inputSchema")
		if !ok {
			return nil, nil, errors.New("mcp: tool " + name.Str + " missing inputSchema")
		}
		out[name.Str] = parseSchema(input)
		order = append(order, name.Str)
	}
	return out, order, nil
}

func newSDKServer(a *App) *sdkmcp.Server {
	// Capabilities is non-nil so the SDK does not advertise logging.
	// 2026-07-28 is excluded. 2024-10-07 is accepted on the HTTP front;
	// the SDK list cannot be widened to include it.
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "hebrew-lessons",
		Version: "1.0.0",
	}, &sdkmcp.ServerOptions{
		Capabilities: &sdkmcp.ServerCapabilities{
			Tools: &sdkmcp.ToolCapabilities{ListChanged: true},
		},
		SupportedProtocolVersions: []string{
			"2025-11-25",
			"2025-06-18",
			"2025-03-26",
			"2024-11-05",
		},
	})
	srv.AddReceivingMiddleware(toolErrorIsResult)
	root, err := Parse(strings.TrimSpace(string(toolsListResult)))
	if err != nil {
		panic(err)
	}
	tools, _ := root.Get("tools")
	for _, tool := range tools.Arr {
		name, _ := tool.Get("name")
		input, _ := tool.Get("inputSchema")
		desc, _ := tool.Get("description")
		title, _ := tool.Get("title")
		schema := json.RawMessage(Stringify(input, 0))
		copied := name.Str
		srv.AddTool(&sdkmcp.Tool{
			Name:        copied,
			Title:       title.Str,
			Description: desc.Str,
			InputSchema: schema,
		}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return a.sdkCall(copied, req)
		})
	}
	return srv
}

func (a *App) sdkCall(name string, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	args := Object()
	if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
		parsed, err := Parse(string(req.Params.Arguments))
		if err != nil {
			return nil, err
		}
		if parsed.Kind == KindObject {
			args = parsed
		} else if parsed.Kind != KindNull && parsed.Kind != 0 {
			return nil, errors.New("arguments must be an object")
		}
	}
	text, isErr := a.callTool(name, args)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
		IsError: isErr,
	}, nil
}

// toolErrorIsResult turns a tools/call failure into an isError result.
// The HTTP front already does this. The middleware covers the SDK server
// if a session is ever attached to it.
func toolErrorIsResult(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
	return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
		result, err := next(ctx, method, req)
		if err == nil || method != "tools/call" {
			return result, err
		}
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}
}

// ProtocolServer is the SDK registry. It is not mounted on /mcp.
func (a *App) ProtocolServer() *sdkmcp.Server { return a.sdk }

// Listen serves /mcp. HOST defaults to 127.0.0.1 and MCP_PORT to 8787,
// matching mcp/http.js. config.Load still resolves DATA_DIR and LESSONS_DIR.
// A missing database is logged and the process still serves lesson tools.
// A ledger mismatch is logged and the process still serves.
func Listen() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port, err := mcpPort()
	if err != nil {
		return err
	}
	if os.Getenv("MCP_AUTH_TOKEN") == "" && !loopbackHost(host) {
		//lint:ignore ST1005 Node mcp/http.js throws this sentence, period included.
		return errors.New("MCP_AUTH_TOKEN is required when HOST is not loopback.")
	}

	var conn *sql.DB
	opened, openErr := db.OpenReadOnly(cfg.DBPath)
	if openErr != nil {
		log.Printf("database: %v", openErr)
	} else {
		conn = opened
		defer conn.Close()
		ledger, lerr := db.CheckLedger(context.Background(), conn)
		if lerr != nil {
			log.Printf("ledger: %v", lerr)
		} else {
			log.Printf("ledger ok (%d versions)", len(ledger.Versions))
		}
	}

	app, err := New(Options{LessonsDir: cfg.LessonsDir, DB: conn})
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              net.JoinHostPort(host, strconv.Itoa(port)),
		Handler:           app.Handler(),
		ReadHeaderTimeout: 60 * time.Second,
		// No ReadTimeout or WriteTimeout. Node's requestTimeout is 300s,
		// which would close a GET /mcp stream. Keep-alives are written
		// every 15s so the stream stays inside Cloudflare's 125s idle limit
		// without a write deadline cutting it off.
		IdleTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	log.Printf("mcp listening on %s", ln.Addr())

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

func mcpPort() (int, error) {
	raw := os.Getenv("MCP_PORT")
	if raw == "" {
		return 8787, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 0 || port > 65535 {
		return 0, errors.New("MCP_PORT " + raw + " is not a TCP port")
	}
	return port, nil
}

func loopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}
