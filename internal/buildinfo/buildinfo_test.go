package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestResolveCommit(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	const rev = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		stamp, revision, want string
	}{
		{stamp: sha, revision: rev, want: sha},
		{stamp: "  " + sha + "  ", revision: rev, want: sha},
		{stamp: "unknown", revision: rev, want: rev},
		{stamp: "", revision: rev, want: rev},
		{stamp: "  unknown  ", revision: "  " + rev, want: rev},
		{stamp: "unknown", revision: "", want: "unknown"},
		{stamp: "unknown", revision: "   ", want: "unknown"},
		{stamp: "", revision: "", want: "unknown"},
		{stamp: "   ", revision: "", want: "unknown"},
	}
	for _, tc := range tests {
		if got := resolveCommit(tc.stamp, tc.revision); got != tc.want {
			t.Errorf("resolveCommit(%q, %q) = %q, want %q", tc.stamp, tc.revision, got, tc.want)
		}
	}
}

func TestRevisionFromSettings(t *testing.T) {
	const rev = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if got := revisionFromSettings([]debug.BuildSetting{
		{Key: "vcs.modified", Value: "false"},
		{Key: "vcs.revision", Value: "  " + rev + "\n"},
	}); got != rev {
		t.Fatalf("revision = %q", got)
	}
	if got := revisionFromSettings([]debug.BuildSetting{{Key: "vcs.revision", Value: "  "}}); got != "" {
		t.Fatalf("blank revision = %q", got)
	}
	if got := revisionFromSettings(nil); got != "" {
		t.Fatalf("nil settings = %q", got)
	}
}

func TestCurrentFallsBackToVCS(t *testing.T) {
	prevCommit, prevBuilt := Commit, BuiltAt
	t.Cleanup(func() {
		Commit = prevCommit
		BuiltAt = prevBuilt
	})

	Commit = "unknown"
	BuiltAt = "  2026-10-19T00:00:00Z  "
	got := Current()
	if got.BuiltAt != "2026-10-19T00:00:00Z" {
		t.Fatalf("builtAt = %q", got.BuiltAt)
	}
	if got.Go != strings.TrimSpace(got.Go) || !strings.HasPrefix(got.Go, "go") {
		t.Fatalf("go = %q", got.Go)
	}
	want := vcsRevision()
	if want == "" {
		want = "unknown"
	}
	if got.Commit != want {
		t.Fatalf("fallback commit = %q, want %q", got.Commit, want)
	}

	const stamped = "cccccccccccccccccccccccccccccccccccccccc"
	Commit = stamped
	got = Current()
	if got.Commit != stamped {
		t.Fatalf("stamped commit = %q, want %s (vcs %q)", got.Commit, stamped, vcsRevision())
	}
}
