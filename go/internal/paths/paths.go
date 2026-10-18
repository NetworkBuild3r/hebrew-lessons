// Package paths resolves the data, lessons, and database locations the way
// server/paths.js does.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const dbFilename = "hebrew.sqlite"

// Layout is the set of absolute paths the server uses.
type Layout struct {
	Root       string
	DataDir    string
	LessonsDir string
	DBPath     string
	MediaDir   string
}

// Resolve returns absolute paths. An empty hebrewRoot is discovered by walking
// up from the working directory until server/paths.js is found (Node's default
// root is the parent of server/). Relative dataDir and lessonsDir resolve
// against the working directory, matching path.resolve.
func Resolve(hebrewRoot, dataDir, lessonsDir string) (Layout, error) {
	root, err := resolveRoot(hebrewRoot)
	if err != nil {
		return Layout{}, err
	}

	lessons, err := resolveDir(lessonsDir, filepath.Join(root, "worksheets"))
	if err != nil {
		return Layout{}, err
	}
	data, err := resolveDir(dataDir, filepath.Join(root, "data"))
	if err != nil {
		return Layout{}, err
	}
	if err := assertDataDirNotServed(root, data, lessons); err != nil {
		return Layout{}, err
	}
	return Layout{
		Root:       root,
		DataDir:    data,
		LessonsDir: lessons,
		DBPath:     filepath.Join(data, dbFilename),
		MediaDir:   filepath.Join(data, "media"),
	}, nil
}

func resolveRoot(hebrewRoot string) (string, error) {
	if hebrewRoot != "" {
		abs, err := filepath.Abs(hebrewRoot)
		if err != nil {
			return "", err
		}
		return filepath.Clean(abs), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findRoot(cwd)
}

func resolveDir(value, fallback string) (string, error) {
	if value == "" {
		abs, err := filepath.Abs(fallback)
		if err != nil {
			return "", err
		}
		return filepath.Clean(abs), nil
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func findRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		marker := filepath.Join(dir, "server", "paths.js")
		info, statErr := os.Stat(marker)
		if statErr == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("paths: cannot find repo root from %s (set HEBREW_ROOT)", start)
		}
		dir = parent
	}
}

func assertDataDirNotServed(root, data, lessons string) error {
	served := []string{
		filepath.Join(root, "shared"),
		filepath.Join(root, "review"),
		lessons,
	}
	for _, rootPath := range served {
		if isInsideOrEqual(data, rootPath) {
			return fmt.Errorf(
				"DATA_DIR must not equal or lie inside a served root (shared/, review/, or LESSONS_DIR). Got: %s (served: %s)",
				data, rootPath,
			)
		}
	}
	return nil
}

func isInsideOrEqual(candidate, parent string) bool {
	resolved := filepath.Clean(candidate)
	base := filepath.Clean(parent)
	if resolved == base {
		return true
	}
	prefix := base
	if !strings.HasSuffix(base, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(resolved, prefix)
}
