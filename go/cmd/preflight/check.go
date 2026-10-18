// Package main is the read-only cutover preflight for a restored backup.
//
// It does not migrate, and it does not write the database or lesson files.
// Lexicon and lesson checks call Deps. A nil function is pending: the real
// go/internal/textkey and go/internal/lessondata packages are not on main
// yet (PR #37 and PR #25). See deps.go.
package main

import (
	"context"
	"database/sql"
)

const (
	statusPass    = "pass"
	statusFail    = "fail"
	statusPending = "pending"
)

// Check is one read-only cutover check. The command runs every check and
// exits non-zero unless every status is pass. There is no warn status.
type Check interface {
	Name() string
	Run(ctx context.Context, env Env) Result
}

// Env is the opened backup. DB is the read-only connection from
// go/internal/db. LessonsDir is the worksheets directory from
// go/internal/config (paths.Resolve).
type Env struct {
	DB         *sql.DB
	DBPath     string
	LessonsDir string
	Deps       Deps
}

// Deps are the two packages this command does not vendor.
//
// ExactKey is go/internal/textkey.ExactKey once PR #37 is on main.
// RoundTrip is go/internal/lessondata parse-then-Write once PR #25 is on main.
// A nil func leaves that check pending so this package still builds on main.
type Deps struct {
	ExactKey  func(string) (string, error)
	RoundTrip func(path string, src []byte) ([]byte, error)
}

// Result is one check. Detail is a check-specific struct with json tags.
type Result struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Detail  any    `json:"detail,omitempty"`
}

func checks() []Check {
	return []Check{ledgerCheck{}, lexiconCheck{}, lessonsCheck{}}
}
