package smoke

import (
	"fmt"
	"net/url"
	"strings"
)

// Staging and local hosts may be smoked without SMOKE_ALLOW_PROD.
// Anything else, including production, refuses to run unless the flag is 1.
var smokeAllowedHosts = map[string]bool{
	"hebrew-next.ibhacked.us":     true,
	"hebrew-mcp-next.ibhacked.us": true,
	"localhost":                   true,
	"127.0.0.1":                   true,
}

var smokeProdHosts = map[string]bool{
	"hebrew.ibhacked.us":     true,
	"hebrew-mcp.ibhacked.us": true,
}

// refuseProdSmoke fails when SMOKE_WEB_URL or SMOKE_MCP_URL names a host
// outside the staging allow-list and SMOKE_ALLOW_PROD is not exactly "1".
// An empty URL is allowed: the smoke tests skip when the variable is unset.
func refuseProdSmoke(webURL, mcpURL, allow string) error {
	checks := []struct {
		name  string
		value string
	}{
		{name: "SMOKE_WEB_URL", value: webURL},
		{name: "SMOKE_MCP_URL", value: mcpURL},
	}
	for _, check := range checks {
		if err := smokeHostAllowed(check.name, check.value, allow); err != nil {
			return err
		}
	}
	return nil
}

func smokeHostAllowed(name, raw, allow string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("smoke: %s %q is not an absolute http(s) URL; refusing to run", name, raw)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if smokeAllowedHosts[host] {
		return nil
	}
	if allow == "1" {
		return nil
	}
	if smokeProdHosts[host] {
		return fmt.Errorf("smoke: %s points at production host %s; set SMOKE_ALLOW_PROD=1 to run against production", name, host)
	}
	return fmt.Errorf("smoke: %s host %s is not a staging host (hebrew-next.ibhacked.us, hebrew-mcp-next.ibhacked.us, localhost, 127.0.0.1); set SMOKE_ALLOW_PROD=1 to run anyway", name, host)
}
