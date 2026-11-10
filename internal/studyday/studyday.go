// Package studyday is the one learner-day rule: parse a zone, resolve a learner's
// zone, and compute Today and DueCutoff. INIT-026/SPEC-003.
package studyday

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // Containers may ship no zoneinfo; the standard library carries one.
)

// Source values for Zone.Source.
const (
	SourceLearner = "learner"
	SourceApp     = "app"
)

// studyRolloverHour is the local hour when the study day rolls.
const studyRolloverHour = 4

// defaultAppZone is the server time zone when TZ is unset. Matches cmd/hebrew's
// historical defaultTimeZone (America/Chicago).
const defaultAppZone = "America/Chicago"

// ZoneStore is the store surface Resolver needs. *db.Store satisfies it.
type ZoneStore interface {
	UserTimeZone(ctx context.Context, id int64) (zone string, ok bool, err error)
	UserTimeZones(ctx context.Context) (map[int64]string, error)
	UserIDs(ctx context.Context) ([]int64, error)
}

// Zone is a resolved civil-day authority for one learner (or the app).
type Zone struct {
	Name   string
	Loc    *time.Location
	Source string // "learner" or "app"
}

// ParseZone loads an IANA zone name. It refuses empty, whitespace-only, "Local",
// and names longer than 64 characters. "UTC" and "Etc/UTC" are valid when named.
func ParseZone(name string) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("studyday: zone is required")
	}
	if name == "Local" {
		return nil, errors.New("studyday: zone Local is not allowed")
	}
	if len(name) > 64 {
		return nil, fmt.Errorf("studyday: zone length %d exceeds 64", len(name))
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("studyday: zone %q: %w", name, err)
	}
	return loc, nil
}

// AppZone reads the server time zone from TZ. An empty or whitespace value is
// America/Chicago. An unknown name is an error that mentions TZ.
func AppZone(tz string) (*time.Location, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		tz = defaultAppZone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("TZ: %w", err)
	}
	return loc, nil
}

// Resolver resolves a learner's study zone from the store, falling back to the
// app zone. There is no UTC default.
type Resolver struct {
	store ZoneStore
	app   Zone
}

// NewResolver builds a Resolver. app must be non-nil; there is no UTC default.
func NewResolver(store ZoneStore, app *time.Location) (*Resolver, error) {
	if app == nil {
		return nil, errors.New("studyday: app zone is required")
	}
	return &Resolver{
		store: store,
		app:   Zone{Name: app.String(), Loc: app, Source: SourceApp},
	}, nil
}

// For returns the learner's stored zone (Source learner), or the app zone
// (Source app) when none is stored. A stored name that no longer loads is an
// error, not a silent app-zone fallback.
func (r *Resolver) For(ctx context.Context, userID int64) (Zone, error) {
	if r == nil {
		return Zone{}, errors.New("studyday: resolver is nil")
	}
	name, ok, err := r.store.UserTimeZone(ctx, userID)
	if err != nil {
		return Zone{}, err
	}
	if !ok {
		return r.app, nil
	}
	loc, err := ParseZone(name)
	if err != nil {
		return Zone{}, fmt.Errorf("studyday: user %d: %w", userID, err)
	}
	return Zone{Name: name, Loc: loc, Source: SourceLearner}, nil
}

// All returns a zone for every user. Learners without a stored zone get the app
// zone. A stored name that no longer loads is an error.
func (r *Resolver) All(ctx context.Context) (map[int64]Zone, error) {
	if r == nil {
		return nil, errors.New("studyday: resolver is nil")
	}
	ids, err := r.store.UserIDs(ctx)
	if err != nil {
		return nil, err
	}
	stored, err := r.store.UserTimeZones(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]Zone, len(ids))
	for _, id := range ids {
		name, ok := stored[id]
		if !ok {
			out[id] = r.app
			continue
		}
		loc, err := ParseZone(name)
		if err != nil {
			return nil, fmt.Errorf("studyday: user %d: %w", id, err)
		}
		out[id] = Zone{Name: name, Loc: loc, Source: SourceLearner}
	}
	return out, nil
}

// Today is the calendar day YYYY-MM-DD of now in z.
func Today(now time.Time, z Zone) string {
	return now.In(z.Loc).Format(time.DateOnly)
}

// DueCutoff is the next 4:00 in z. Before 4:00 uses 4:00 this morning; at or
// after 4:00 uses 4:00 tomorrow. DST-safe via time.Date in z.Loc.
func DueCutoff(now time.Time, z Zone) time.Time {
	local := now.In(z.Loc)
	y, m, d := local.Date()
	start := time.Date(y, m, d, studyRolloverHour, 0, 0, 0, z.Loc)
	if !local.Before(start) {
		start = time.Date(y, m, d+1, studyRolloverHour, 0, 0, 0, z.Loc)
	}
	return start
}

// EffectiveDue is when a card can be asked. A due time that falls inside the
// same study day as the review that scheduled it is the next cutoff after that
// review (ADR-0033 D10). A card that has never been reviewed keeps its due.
// A zone with no location leaves due unchanged.
func EffectiveDue(due, reviewed time.Time, z Zone) time.Time {
	if reviewed.IsZero() || z.Loc == nil {
		return due
	}
	cutoff := DueCutoff(reviewed, z)
	if due.Before(cutoff) {
		return cutoff
	}
	return due
}
