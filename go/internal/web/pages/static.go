package pages

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// MountShared serves /shared/ from HEBREW_ROOT/shared when HEBREW_DEV_WEB=1.
//
// HEBREW_ROOT is the app root (the image sets it to /app). CSS, fonts, brand
// assets, and dist/player.js are files on that disk, not embedded: go:embed
// cannot reach outside this module. The call is a no-op unless the flag is
// set, same as Mount. Leave the flag unset until sessions exist. Node still
// publishes only /shared/fonts/ and /shared/brand/ without a session.
//
// cmd/hebrew calls this once, with the root config already resolved:
//
//	pages.MountShared(mux, cfg.HebrewRoot)
func MountShared(mux *http.ServeMux, hebrewRoot string) {
	if os.Getenv(DevEnv) != "1" {
		return
	}
	mux.Handle("GET /shared/", sharedFiles{root: sharedDir(hebrewRoot)})
}

func sharedDir(hebrewRoot string) string {
	if hebrewRoot == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(hebrewRoot, "shared"))
}

// sharedFiles reads regular files under root. Directory listings, dotfiles,
// and paths that resolve outside root (including through a symlink) are 404.
type sharedFiles struct {
	root string
}

func (s sharedFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")

	name, err := resolveShared(s.root, strings.TrimPrefix(r.URL.Path, "/shared/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", sharedContentType(name))
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, filepath.Base(name), info.ModTime(), f)
}

// resolveShared maps a URL path under /shared/ to one regular file inside root.
func resolveShared(root, rel string) (string, error) {
	rel = path.Clean("/" + rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." || !filepath.IsLocal(rel) {
		return "", os.ErrNotExist
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") {
			return "", os.ErrNotExist
		}
	}
	if root == "" {
		return "", os.ErrNotExist
	}
	root = filepath.Clean(root)
	full := filepath.Join(root, filepath.FromSlash(rel))
	if !insideDir(root, full) {
		return "", os.ErrNotExist
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", os.ErrNotExist
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", os.ErrNotExist
	}
	if !insideDir(resolvedRoot, resolved) {
		return "", os.ErrNotExist
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", os.ErrNotExist
	}
	return resolved, nil
}

func insideDir(parent, candidate string) bool {
	rel, err := filepath.Rel(parent, candidate)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// sharedContentType matches the types Node sends for files under /shared/.
func sharedContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".webmanifest":
		return "application/manifest+json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".webm":
		return "audio/webm"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}
