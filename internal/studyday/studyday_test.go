package studyday

import (
	"os"
	"strings"
	"testing"
	"time"
)

// ac-1: ParseZone accepts America/Denver and UTC, refuses "", "  ", Local,
// Not/AZone, and a 65-character name. INIT-026/SPEC-003.
func TestParseZone(t *testing.T) {
	for _, name := range []string{"America/Denver", "UTC", "Etc/UTC"} {
		loc, err := ParseZone(name)
		if err != nil || loc == nil {
			t.Fatalf("ParseZone(%q) = %v, %v", name, loc, err)
		}
		if loc.String() != name && !(name == "UTC" && loc.String() == "UTC") {
			// LoadLocation("UTC") returns a location whose String is "UTC".
			if name != "UTC" && name != "Etc/UTC" {
				t.Fatalf("ParseZone(%q).String() = %q", name, loc.String())
			}
		}
	}
	for _, name := range []string{"", "  ", "Local", "Not/AZone", strings.Repeat("z", 65)} {
		if _, err := ParseZone(name); err == nil {
			t.Fatalf("ParseZone(%q) was accepted", name)
		}
	}
}

func TestAppZone(t *testing.T) {
	loc, err := AppZone("")
	if err != nil || loc.String() != "America/Chicago" {
		t.Fatalf("default = %v, %v", loc, err)
	}
	loc, err = AppZone(" Europe/Paris ")
	if err != nil || loc.String() != "Europe/Paris" {
		t.Fatalf("Europe/Paris = %v, %v", loc, err)
	}
	if _, err := AppZone("Not/AZone"); err == nil || !strings.Contains(err.Error(), "TZ") {
		t.Fatalf("bad zone: err = %v", err)
	}
}

// ac-3: NewResolver with a nil app zone returns an error.
func TestNewResolverNilApp(t *testing.T) {
	if _, err := NewResolver(nil, nil); err == nil {
		t.Fatal("nil app zone was accepted")
	}
}

// ac-4: Today at 2026-11-01T05:30:00Z in America/Denver is 2026-10-31.
// DueCutoff keeps every case of TestStudyDayCutoffIsTheNextFourDenver.
func TestTodayAndDueCutoffDenver(t *testing.T) {
	loc, err := ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	z := Zone{Name: "America/Denver", Loc: loc, Source: SourceLearner}

	now := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
	if got := Today(now, z); got != "2026-10-31" {
		t.Fatalf("Today = %q, want 2026-10-31", got)
	}

	at := func(y int, m time.Month, d, hh, mm int) time.Time {
		return time.Date(y, m, d, hh, mm, 0, 0, loc)
	}
	for _, tc := range []struct{ now, want time.Time }{
		{at(2026, 10, 21, 3, 59), at(2026, 10, 21, 4, 0)},
		{at(2026, 10, 21, 4, 0), at(2026, 10, 22, 4, 0)},
		{at(2026, 10, 20, 22, 0), at(2026, 10, 21, 4, 0)},
		{at(2026, 10, 21, 3, 30), at(2026, 10, 21, 4, 0)},
	} {
		if got := DueCutoff(tc.now, z); !got.Equal(tc.want) {
			t.Fatalf("%s cutoff = %s, want %s", tc.now, got, tc.want)
		}
	}
	early, late := at(2026, 10, 20, 17, 0), at(2026, 10, 20, 23, 0)
	if !DueCutoff(early, z).Equal(DueCutoff(late, z)) {
		t.Fatal("17:00 and 23:00 Denver are one study day")
	}
}

// ac-3: no time.UTC in production studyday sources except a named-zone branch.
func TestNoUTCDefaultInSources(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if !strings.Contains(line, "time.UTC") {
				continue
			}
			// Allowed only on a branch that loaded the named zone "UTC".
			if strings.Contains(line, `"UTC"`) || strings.Contains(line, `"Etc/UTC"`) {
				continue
			}
			t.Errorf("%s:%d: time.UTC is banned outside a named-zone branch:\n%s", name, i+1, line)
		}
	}
}
