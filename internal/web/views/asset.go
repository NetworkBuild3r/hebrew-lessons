package views

import (
	"strings"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/buildinfo"
)

var (
	assetOnce    sync.Once
	assetVersion string
)

// Asset is the URL of a stylesheet or script the app serves. It carries the build's commit as
// a query, so a new build is a new URL. The CDN and the browser keep the old file under the
// old URL, and a deploy can never leave a page with new markup and the last build's styles.
// A build with no stamp (a local run) has no query.
func Asset(path string) string {
	assetOnce.Do(func() {
		commit := strings.TrimSpace(buildinfo.Current().Commit)
		if commit != "" && commit != "unknown" {
			assetVersion = commit
			if len(assetVersion) > 12 {
				assetVersion = assetVersion[:12]
			}
		}
	})
	if assetVersion == "" {
		return path
	}
	return path + "?v=" + assetVersion
}
