package smoke

import (
	"strings"
	"testing"
)

func TestSmokeHostAllowed(t *testing.T) {
	t.Parallel()
	allowed := []string{
		"",
		"   ",
		"https://hebrew-next.ibhacked.us",
		"https://hebrew-next.ibhacked.us/",
		"https://Hebrew-Next.ibhacked.us",
		"https://hebrew-next.ibhacked.us.",
		"https://hebrew-mcp-next.ibhacked.us/mcp",
		"http://localhost",
		"http://localhost:8080",
		"http://127.0.0.1",
		"http://127.0.0.1:8787/mcp",
	}
	for _, raw := range allowed {
		if err := smokeHostAllowed("SMOKE_WEB_URL", raw, ""); err != nil {
			t.Errorf("%q: %v", raw, err)
		}
	}

	prod := []string{
		"https://hebrew.ibhacked.us",
		"https://hebrew.ibhacked.us/healthz",
		"https://HEBREW.ibhacked.us",
		"https://hebrew-mcp.ibhacked.us/mcp",
	}
	for _, raw := range prod {
		err := smokeHostAllowed("SMOKE_WEB_URL", raw, "")
		if err == nil || !strings.Contains(err.Error(), "production") {
			t.Errorf("%q err = %v, want a production refusal", raw, err)
		}
		if err := smokeHostAllowed("SMOKE_MCP_URL", raw, "1"); err != nil {
			t.Errorf("%q with SMOKE_ALLOW_PROD=1: %v", raw, err)
		}
		if err := smokeHostAllowed("SMOKE_WEB_URL", raw, "true"); err == nil {
			t.Errorf("%q with SMOKE_ALLOW_PROD=true should still refuse", raw)
		}
	}

	other := []string{
		"https://example.com",
		"https://hebrew.ibhacked.us.evil.example",
		"https://not-hebrew-next.ibhacked.us",
	}
	for _, raw := range other {
		err := smokeHostAllowed("SMOKE_WEB_URL", raw, "")
		if err == nil || !strings.Contains(err.Error(), "not a staging host") {
			t.Errorf("%q err = %v, want a non-staging refusal", raw, err)
		}
	}

	for _, raw := range []string{"hebrew.ibhacked.us", "ftp://hebrew-next.ibhacked.us", "://nope", "/relative"} {
		if err := smokeHostAllowed("SMOKE_WEB_URL", raw, "1"); err == nil {
			t.Errorf("%q should fail before the allow flag is considered", raw)
		}
	}

	if err := refuseProdSmoke("", "https://hebrew-mcp.ibhacked.us/mcp", ""); err == nil || !strings.Contains(err.Error(), "SMOKE_MCP_URL") {
		t.Fatalf("mcp prod err = %v", err)
	}
	if err := refuseProdSmoke("https://hebrew-next.ibhacked.us", "http://127.0.0.1:8787", ""); err != nil {
		t.Fatal(err)
	}
}
