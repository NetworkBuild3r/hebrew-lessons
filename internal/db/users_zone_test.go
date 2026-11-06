package db

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// ac-4: SetUserTimeZone then UserTimeZone round-trips America/Denver.
// UserTimeZones returns only users that have a zone.
func TestUserTimeZoneRoundTrip(t *testing.T) {
	s := openAt(t, t.TempDir(), nil)
	ctx := context.Background()
	a := insertUser(t, s, "a@example.com", "sub-a", "learner")
	b := insertUser(t, s, "b@example.com", "sub-b", "learner")
	_ = insertUser(t, s, "c@example.com", "sub-c", "learner")

	zone, ok, err := s.UserTimeZone(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if ok || zone != "" {
		t.Fatalf("unset zone: zone=%q ok=%v", zone, ok)
	}

	if err := s.SetUserTimeZone(ctx, a, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	zone, ok, err = s.UserTimeZone(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || zone != "America/Denver" {
		t.Fatalf("after set: zone=%q ok=%v", zone, ok)
	}

	if err := s.SetUserTimeZone(ctx, b, "Europe/London"); err != nil {
		t.Fatal(err)
	}
	got, err := s.UserTimeZones(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]string{a: "America/Denver", b: "Europe/London"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UserTimeZones = %v, want %v", got, want)
	}

	if err := s.SetUserTimeZone(ctx, a, ""); err == nil {
		t.Fatal("empty zone was accepted")
	}
	if err := s.SetUserTimeZone(ctx, a, strings.Repeat("z", 65)); err == nil {
		t.Fatal("65-character zone was accepted")
	}
	zone, ok, err = s.UserTimeZone(ctx, a)
	if err != nil || !ok || zone != "America/Denver" {
		t.Fatalf("zone after refused writes: zone=%q ok=%v err=%v", zone, ok, err)
	}
	if err := s.SetUserTimeZone(ctx, 9999, "America/Denver"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user: err=%v, want ErrNotFound", err)
	}
}

// INIT-032/SPEC-007: SetUserTimeZoneIfUnset writes only while time_zone IS NULL.
func TestSetUserTimeZoneIfUnset(t *testing.T) {
	s := openAt(t, t.TempDir(), nil)
	ctx := context.Background()
	id := insertUser(t, s, "zone-if-unset@example.com", "sub-zone-if-unset", "learner")

	if err := s.SetUserTimeZoneIfUnset(ctx, id, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	zone, ok, err := s.UserTimeZone(ctx, id)
	if err != nil || !ok || zone != "America/Denver" {
		t.Fatalf("after first set: zone=%q ok=%v err=%v", zone, ok, err)
	}

	if err := s.SetUserTimeZoneIfUnset(ctx, id, "America/New_York"); err != nil {
		t.Fatal(err)
	}
	zone, ok, err = s.UserTimeZone(ctx, id)
	if err != nil || !ok || zone != "America/Denver" {
		t.Fatalf("second set overwrote: zone=%q ok=%v err=%v", zone, ok, err)
	}

	if err := s.SetUserTimeZone(ctx, id, "America/Phoenix"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserTimeZoneIfUnset(ctx, id, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	zone, ok, err = s.UserTimeZone(ctx, id)
	if err != nil || !ok || zone != "America/Phoenix" {
		t.Fatalf("after stated zone: zone=%q ok=%v err=%v", zone, ok, err)
	}

	if err := s.SetUserTimeZoneIfUnset(ctx, id, ""); err == nil {
		t.Fatal("empty zone was accepted")
	}
	if err := s.SetUserTimeZoneIfUnset(ctx, id, strings.Repeat("z", 65)); err == nil {
		t.Fatal("65-character zone was accepted")
	}
}
