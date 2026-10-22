package db

import (
	"context"
	"io/fs"
	"reflect"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// The v3 branch on origin shipped 007_oauth_client_issued.sql before the INIT-015 migrations
// existed, and every push to v3 is deployed. So a real database can already be at versions
// 1 to 5 and 7. The learner model, evidence, and placement tables must still arrive: they must
// not share the version of a migration that is already recorded as applied.
func TestMigrateFromTheDeployedV3Database(t *testing.T) {
	conn := m6At005(t)
	body, err := fs.ReadFile(migrations.FS, "007_oauth_client_issued.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "007_oauth_client_issued.sql", 7, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("apply 007: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 7}) {
		t.Fatalf("seed database versions = %v, want 1..5 and 7", got)
	}

	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 15}) {
		t.Fatalf("versions after migrate = %v, want 1..12", got)
	}
	// One table from each INIT-015 migration is there, so none was skipped as already applied.
	for _, table := range []string{"attempts", "daily_snapshots", "weekly_reports", "job_state", "placement_v3_runs", "placement_items"} {
		if !tableExists(t, conn, table) {
			t.Errorf("table %s is missing after the upgrade", table)
		}
	}
	// The migration that was already applied is untouched.
	if cols := columnNames(t, conn, "oauth_clients"); !cols["issued"] {
		t.Errorf("oauth_clients has no issued column: %v", cols)
	}
	// Running it again changes nothing.
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if got := m6Applied(t, conn); len(got) != 13 {
		t.Fatalf("versions after a second migrate = %v", got)
	}
}
