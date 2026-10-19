// Package buildinfo reports the git commit and build time stamped into
// the binary.
//
// CI sets Commit and BuiltAt with -ldflags -X. A build that leaves Commit
// empty or "unknown" uses vcs.revision from runtime/debug.ReadBuildInfo
// when the toolchain recorded one. Docker builds omit .git, so that
// fallback is empty there unless the ldflags stamp is set.
package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

// Commit is the git SHA of the source that produced this binary.
// The Docker image and CI pass it with:
//
//	-X github.com/NetworkBuild3r/hebrew-lessons/internal/buildinfo.Commit=$GIT_SHA
//
// "unknown" means the stamp was not set.
var Commit = "unknown"

// BuiltAt is the UTC time the binary was built, when CI recorded one.
// Empty means it was not set. The stamp is:
//
//	-X github.com/NetworkBuild3r/hebrew-lessons/internal/buildinfo.BuiltAt=$BUILT_AT
var BuiltAt string

// Version is the JSON body of GET /version.
type Version struct {
	Commit  string `json:"commit"`
	BuiltAt string `json:"builtAt"`
	Go      string `json:"go"`
}

// Current returns the stamped version, or vcs.revision when Commit was
// left unset. Go is runtime.Version, for example go1.26.0.
func Current() Version {
	return Version{
		Commit:  resolveCommit(Commit, vcsRevision()),
		BuiltAt: strings.TrimSpace(BuiltAt),
		Go:      runtime.Version(),
	}
}

// resolveCommit prefers a real stamp. Empty and "unknown" fall through
// to the VCS revision. With neither, the result is "unknown".
func resolveCommit(stamp, revision string) string {
	stamp = strings.TrimSpace(stamp)
	if stamp != "" && stamp != "unknown" {
		return stamp
	}
	revision = strings.TrimSpace(revision)
	if revision != "" {
		return revision
	}
	if stamp == "" {
		return "unknown"
	}
	return stamp
}

func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return ""
	}
	return revisionFromSettings(info.Settings)
}

func revisionFromSettings(settings []debug.BuildSetting) string {
	for _, s := range settings {
		if s.Key == "vcs.revision" {
			return strings.TrimSpace(s.Value)
		}
	}
	return ""
}
