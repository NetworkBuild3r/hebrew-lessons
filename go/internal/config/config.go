// Package config loads the environment the Go server shares with Node.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/paths"
)

// DefaultPort is the listening port when PORT is unset. Node defaults to
// 8080 and MCP to 8787; 8090 keeps a local process off both. Staging sets
// PORT=8080.
const DefaultPort = 8090

// DefaultHost is the listen address when HOST is unset. 0.0.0.0 accepts
// connections from the pod network, which Kubernetes probes and the Service
// need. Set HOST=127.0.0.1 for local use.
const DefaultHost = "0.0.0.0"

// Config is the typed environment for this process.
type Config struct {
	HebrewRoot     string
	DataDir        string
	LessonsDir     string
	DBPath         string
	MediaDir       string
	Host           string
	Port           int
	TrustedProxies []net.IPNet
	CookieSecure   bool
}

// Load reads the process environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	layout, err := paths.Resolve(
		getenv("HEBREW_ROOT"),
		getenv("DATA_DIR"),
		getenv("LESSONS_DIR"),
	)
	if err != nil {
		return Config{}, err
	}
	port, err := parsePort(getenv("PORT"))
	if err != nil {
		return Config{}, err
	}
	proxies, err := parseTrustedProxies(getenv("TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, err
	}
	return Config{
		HebrewRoot:     layout.Root,
		DataDir:        layout.DataDir,
		LessonsDir:     layout.LessonsDir,
		DBPath:         layout.DBPath,
		MediaDir:       layout.MediaDir,
		Host:           parseHost(getenv("HOST")),
		Port:           port,
		TrustedProxies: proxies,
		CookieSecure:   getenv("COOKIE_SECURE") == "1",
	}, nil
}

func parseHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultHost
	}
	return raw
}

func parsePort(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultPort, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 0 || port > 65535 {
		return 0, fmt.Errorf("config: PORT %q is not a TCP port", raw)
	}
	return port, nil
}

// parseTrustedProxies parses a comma-separated list of CIDRs or host
// addresses. Empty means trust nobody (ADR-0016). A host address is one
// address: IPv4 /32 or IPv6 /128.
func parseTrustedProxies(raw string) ([]net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []net.IPNet
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			ip := net.ParseIP(part)
			if ip == nil {
				return nil, fmt.Errorf("config: TRUSTED_PROXIES %q is not an IP or CIDR", part)
			}
			bits := 128
			if v4 := ip.To4(); v4 != nil {
				ip = v4
				bits = 32
			}
			out = append(out, net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("config: TRUSTED_PROXIES %q: %w", part, err)
		}
		out = append(out, *network)
	}
	return out, nil
}
