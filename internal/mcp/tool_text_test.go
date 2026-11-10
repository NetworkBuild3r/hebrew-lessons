package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// INIT-038/SPEC-007: server instructions and tool descriptions name ruleVersion
// and do not restate the known rule or a hard-coded rule version.
func TestToolTextPointsAtRuleVersion(t *testing.T) {
	if !strings.Contains(instructions, "ruleVersion") {
		t.Fatal("instructions do not point at ruleVersion")
	}
	session := connectMCP(t, Handler(Options{AuthToken: testToken}), testToken)
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString(instructions)
	b.WriteString("\n")
	foundContext, foundDigest := false, false
	for _, tool := range tools.Tools {
		b.WriteString(tool.Description)
		b.WriteString("\n")
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteString("\n")
		if tool.Name == "get_context" {
			foundContext = true
			if !strings.Contains(tool.Description, "ruleVersion") {
				t.Fatal("get_context description does not point at ruleVersion")
			}
		}
		if tool.Name == "get_study_digest" {
			foundDigest = true
			if !strings.Contains(tool.Description, "ruleVersion") {
				t.Fatal("get_study_digest description does not point at ruleVersion")
			}
		}
	}
	if !foundContext || !foundDigest {
		t.Fatalf("missing tools context=%v digest=%v", foundContext, foundDigest)
	}
	text := b.String()
	for _, forbidden := range []string{
		"4 under current",
		"Good or Easy",
		"one checked-success study day",
		"strict every-skill",
		"strict every-practiced",
		"rule version 4",
		"Rule version 4",
		"known or solid",
		"two study days",
		"recall probability",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("tool text restates %q", forbidden)
		}
	}
}
