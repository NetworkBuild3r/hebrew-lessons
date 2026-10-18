package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type lessonsCheck struct{}

func (lessonsCheck) Name() string { return "lessons" }

type lessonDetail struct {
	Root       string          `json:"root"`
	Files      int             `json:"files"`
	Mismatches int             `json:"mismatches"`
	Examples   []lessonExample `json:"examples"`
	Truncated  bool            `json:"truncated,omitempty"`
	Pending    string          `json:"pending,omitempty"`
}

type lessonExample struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func (lessonsCheck) Run(ctx context.Context, env Env) Result {
	files, missing, err := discoverLessons(env.LessonsDir)
	if err != nil {
		return Result{Name: "lessons", Status: statusFail, Summary: err.Error()}
	}
	if len(files) == 0 && len(missing) == 0 {
		return Result{
			Name:    "lessons",
			Status:  statusFail,
			Summary: "no lesson files under " + env.LessonsDir,
			Detail:  lessonDetail{Root: env.LessonsDir, Examples: []lessonExample{}},
		}
	}
	detail := lessonDetail{Root: env.LessonsDir, Examples: []lessonExample{}}
	for _, path := range missing {
		detail.Mismatches++
		detail.Examples = append(detail.Examples, lessonExample{
			Path:   lessonRel(env.LessonsDir, path),
			Reason: "missing worksheet.js",
		})
	}
	if env.Deps.RoundTrip == nil {
		if len(missing) > 0 {
			detail.Files = len(files)
			shown, cut := clip(detail.Examples, exampleLimit)
			detail.Examples = shown
			detail.Truncated = cut
			summary := fmt.Sprintf("files=%d mismatches=%d", len(files), detail.Mismatches)
			if len(files) > 0 {
				summary += "; round-trip not wired (" + lessondataDep + ")"
			}
			return Result{Name: "lessons", Status: statusFail, Summary: summary, Detail: detail}
		}
		return Result{
			Name:   "lessons",
			Status: statusPending,
			Summary: fmt.Sprintf(
				"not wired (%s); %d lesson files not checked",
				lessondataDep, len(files),
			),
			Detail: lessonDetail{
				Root:     env.LessonsDir,
				Files:    len(files),
				Examples: []lessonExample{},
				Pending:  lessondataDep,
			},
		}
	}
	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return Result{Name: "lessons", Status: statusFail, Summary: err.Error()}
		}
		stored, err := os.ReadFile(path)
		if err != nil {
			return Result{Name: "lessons", Status: statusFail, Summary: err.Error()}
		}
		written, err := env.Deps.RoundTrip(path, stored)
		rel := lessonRel(env.LessonsDir, path)
		if err != nil {
			detail.Mismatches++
			detail.Examples = append(detail.Examples, lessonExample{
				Path:   rel,
				Reason: err.Error(),
			})
			continue
		}
		if !bytes.Equal(stored, written) {
			detail.Mismatches++
			detail.Examples = append(detail.Examples, lessonExample{
				Path: rel,
				Reason: fmt.Sprintf(
					"round-trip bytes differ (stored %d, written %d)",
					len(stored), len(written),
				),
			})
		}
	}
	detail.Files = len(files)
	shown, cut := clip(detail.Examples, exampleLimit)
	detail.Examples = shown
	detail.Truncated = cut
	if detail.Examples == nil {
		detail.Examples = []lessonExample{}
	}
	status := statusPass
	if detail.Mismatches > 0 || len(files) == 0 {
		status = statusFail
	}
	summary := fmt.Sprintf("files=%d mismatches=%d", detail.Files, detail.Mismatches)
	return Result{Name: "lessons", Status: status, Summary: summary, Detail: detail}
}

// discoverLessons lists <LessonsDir>/<name>/worksheet.js. Names starting
// with '.' or '_' are skipped, matching server/vocab-index.js listLessonNames.
// A lesson directory with no worksheet.js is reported in missing.
func discoverLessons(root string) (files []string, missing []string, err error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, fmt.Errorf("lessons: read %s: %w", root, err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(root, name, "worksheet.js")
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, path)
				continue
			}
			return nil, nil, fmt.Errorf("lessons: stat %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			missing = append(missing, path)
			continue
		}
		files = append(files, path)
	}
	return files, missing, nil
}

func lessonRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
