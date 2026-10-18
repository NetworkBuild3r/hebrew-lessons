package contract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/fixturecheck"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/golden"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/sqlite"
)

// Filled by TestMain: the schema after migrate, then again after the public-API seed.
var (
	schemaAfterBoot string
	schemaAfterSeed string
)

func TestDBSchema(t *testing.T) {
	if schemaAfterBoot == "" || schemaAfterSeed == "" {
		t.Fatal("schema was not captured")
	}
	if schemaAfterBoot != schemaAfterSeed {
		t.Fatalf("seed changed the schema\nafter boot:\n%s\nafter seed:\n%s", schemaAfterBoot, schemaAfterSeed)
	}
	golden.CompareBytes(t, "golden/db/schema.sql", []byte(schemaAfterBoot))

	root := golden.Root(t)
	fixture := filepath.Join(root, "fixtures", "db", "seeded.sqlite")
	dumped, err := sqlite.Dump(fixture)
	if err != nil {
		t.Fatalf("fixture db: %v (set UPDATE_GOLDEN=1 to regenerate fixtures/db/seeded.sqlite)", err)
	}
	if dumped != schemaAfterBoot {
		t.Fatalf("committed fixture schema differs from the live server\n%s", goldenMismatch(dumped, schemaAfterBoot))
	}
	fixturecheck.File(t, fixture)
	fixturecheck.File(t, filepath.Join(os.Getenv("CONTRACT_DATA_DIR"), "hebrew.sqlite"))
	if dir := os.Getenv("CONTRACT_LESSONS_DIR"); dir != "" {
		fixturecheck.Names(t, dir)
		fixturecheck.Dir(t, dir)
	}
}

func goldenMismatch(got, want string) string {
	// Reuse the line diff by going through CompareBytes only on failure text.
	gb := []byte(got)
	wb := []byte(want)
	n := len(gb)
	if len(wb) < n {
		n = len(wb)
	}
	line := 1
	last := 0
	for i := 0; i < n; i++ {
		if gb[i] != wb[i] {
			return "differ at byte " + itoa(i) + " line " + itoa(line)
		}
		if gb[i] == '\n' {
			line++
			last = i
		}
	}
	_ = last
	return "length got " + itoa(len(got)) + " want " + itoa(len(want))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
