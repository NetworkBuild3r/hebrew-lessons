package mcpstore

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

func day(d string, skill string, known, solid int, retention *float64) mcp.SnapshotDay {
	return mcp.SnapshotDay{Day: d, Skills: []mcp.SkillDay{{
		Skill: skill, Counts: map[string]int{"known": known, "solid": solid}, Retention: retention,
	}}}
}

func TestSkillChangesGainedAndLostAreNetAndNeverBothSet(t *testing.T) {
	up := skillChanges([]mcp.SnapshotDay{day("2026-10-01", "listen", 3, 1, nil), day("2026-10-08", "listen", 4, 3, nil)})
	if len(up) != 1 || up[0].KnownAtStart != 4 || up[0].KnownAtEnd != 7 || up[0].Gained != 3 || up[0].Lost != 0 {
		t.Fatalf("up %+v", up)
	}
	down := skillChanges([]mcp.SnapshotDay{day("2026-10-01", "listen", 6, 0, nil), day("2026-10-08", "listen", 2, 1, nil)})
	if len(down) != 1 || down[0].Gained != 0 || down[0].Lost != 3 {
		t.Fatalf("down %+v", down)
	}
	if one := skillChanges([]mcp.SnapshotDay{day("2026-10-01", "listen", 6, 0, nil)}); len(one) != 1 || one[0].Gained != 0 || one[0].Lost != 0 {
		t.Fatalf("one day %+v", one)
	}
	if none := skillChanges(nil); none == nil || len(none) != 0 {
		t.Fatalf("no days %+v", none)
	}
	// A skill that is on only one of the two days has no change to report.
	mixed := skillChanges([]mcp.SnapshotDay{day("2026-10-01", "listen", 1, 0, nil), day("2026-10-08", "read", 1, 0, nil)})
	if len(mixed) != 0 {
		t.Fatalf("mixed %+v", mixed)
	}
}

func TestRetentionIsAgainstTheTarget(t *testing.T) {
	r := func(v float64) *float64 { return &v }
	if v := retentionOf(nil); v.Actual != nil || v.Gap != nil || v.Target != 0.85 {
		t.Fatalf("no data %+v", v)
	}
	// The newest day with any retention is used, and a day with none is skipped.
	days := []mcp.SnapshotDay{day("2026-10-01", "listen", 1, 0, r(0.5)), day("2026-10-02", "listen", 1, 0, r(0.9)), day("2026-10-03", "listen", 1, 0, nil)}
	v := retentionOf(days)
	if v.Actual == nil || *v.Actual != 0.9 || v.Day != "2026-10-02" || v.Gap == nil || *v.Gap < 0.049 || *v.Gap > 0.051 {
		t.Fatalf("retention %+v", v)
	}
}

func TestSnapshotRowsGroupByDayAndSkillInOrder(t *testing.T) {
	ret := 0.7
	days := snapshotDays([]db.SnapshotRow{
		{Day: "2026-10-01", Skill: "read", State: "known", Count: 2, EstKnown: 5, Retention: &ret},
		{Day: "2026-10-01", Skill: "listen", State: "known", Count: 1, EstKnown: 4},
		{Day: "2026-10-01", Skill: "listen", State: "solid", Count: 3, EstKnown: 4},
		{Day: "2026-10-02", Skill: "listen", State: "known", Count: 9, EstKnown: 6},
	})
	if len(days) != 2 || len(days[0].Skills) != 2 || days[0].Skills[0].Skill != "listen" || days[0].Skills[1].Skill != "read" {
		t.Fatalf("days %+v", days)
	}
	if days[0].Skills[0].Counts["known"] != 1 || days[0].Skills[0].Counts["solid"] != 3 || days[0].Skills[1].Retention == nil {
		t.Fatalf("counts %+v", days[0])
	}
}

func TestPlacementViewReadsV3AndV2Results(t *testing.T) {
	v3 := placementView(db.PlacementHistoryRow{Kind: "initial", Level: 3, Result: `{"v":3,"listening":{"tested":true,"estimate":410},"reading":{"tested":false,"estimate":0},"estimated_known_words":410}`})
	if v3.Kind != "initial" || v3.Listening == nil || *v3.Listening != 410 || v3.Reading != nil {
		t.Fatalf("v3 %+v", v3)
	}
	v2 := placementView(db.PlacementHistoryRow{Level: -1, Result: `{"bands":[],"estimated_known_words":80}`})
	if v2.Kind != "v2" || v2.EstimatedKW != 80 || v2.Listening != nil {
		t.Fatalf("v2 %+v", v2)
	}
	// A result that is not JSON gives a view with no estimates, not an error.
	if bad := placementView(db.PlacementHistoryRow{Result: `not json`}); bad.Listening != nil || bad.EstimatedKW != 0 {
		t.Fatalf("bad %+v", bad)
	}
}
