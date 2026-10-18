// Package server boots the process under test. run.sh is the supported
// entry point. Start exists so a later job can spawn the server from Go
// with the same env contract.
package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Proc is a running server and the temp data directory it was given.
type Proc struct {
	Cmd     *exec.Cmd
	DataDir string
	owned   bool
}

// Options configures one boot.
type Options struct {
	Command string
	Dir     string
	Env     []string
	WebURL  string
	MCPURL  string
	DataDir string
	Stdout  io.Writer
	Stderr  io.Writer
}

// Start runs Command and waits until both URLs answer.
func Start(ctx context.Context, opt Options) (*Proc, error) {
	if opt.Command == "" {
		return nil, fmt.Errorf("server command is empty")
	}
	data := opt.DataDir
	owned := false
	if data == "" {
		var err error
		data, err = os.MkdirTemp("", "hebrew-contract-")
		if err != nil {
			return nil, err
		}
		owned = true
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", opt.Command)
	cmd.Dir = opt.Dir
	cmd.Env = opt.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "DATA_DIR="+data, "TZ=UTC")
	if opt.Stdout != nil {
		cmd.Stdout = opt.Stdout
	}
	if opt.Stderr != nil {
		cmd.Stderr = opt.Stderr
	}
	if err := cmd.Start(); err != nil {
		if owned {
			os.RemoveAll(data)
		}
		return nil, err
	}
	p := &Proc{Cmd: cmd, DataDir: data, owned: owned}
	if err := WaitReady(ctx, opt.WebURL, opt.MCPURL); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

// WaitReady returns when webURL and mcpURL both produce an HTTP response.
// MCP is ready at 401; the suite does not require a successful RPC to boot.
func WaitReady(ctx context.Context, webURL, mcpURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	var last error
	for {
		webOK := probe(ctx, client, http.MethodGet, webURL, nil) == nil
		mcpOK := probe(ctx, client, http.MethodPost, mcpURL, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)) == nil
		if webOK && mcpOK {
			return nil
		}
		last = fmt.Errorf("web ready %v mcp ready %v", webOK, mcpOK)
		select {
		case <-ctx.Done():
			if last != nil {
				return fmt.Errorf("%w: %v", ctx.Err(), last)
			}
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func probe(ctx context.Context, client *http.Client, method, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

// Close stops the process and removes a data directory this package created.
func (p *Proc) Close() {
	if p == nil || p.Cmd == nil || p.Cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-p.Cmd.Process.Pid, syscall.SIGKILL)
	_, _ = p.Cmd.Process.Wait()
	if p.owned {
		os.RemoveAll(p.DataDir)
	}
}
