package brief

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// addUnit inserts a unit row so a brief can name it.
func addUnit(t *testing.T, e *env, position int) int64 {
	t.Helper()
	res := e.exec(`INSERT INTO units (position, title, situation, concept, status, rev, doc) VALUES (?, 't', 's', 'c', 'draft', 1, '{}')`, position)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

const idAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// A brief id is 128 random bits written as 22 base64url characters.
func TestIDFormat(t *testing.T) {
	b := &Builder{}
	id, err := b.newID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 22 {
		t.Fatalf("id %q has %d characters, want 22", id, len(id))
	}
	for _, r := range id {
		if !strings.ContainsRune(idAlphabet, r) {
			t.Fatalf("id %q has %q outside the base64url alphabet", id, r)
		}
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(id)
	if err != nil || len(raw) != 16 {
		t.Fatalf("id decodes to %d bytes (%v), want 16", len(raw), err)
	}
	if !validID(id) {
		t.Error("an id from newID is not valid")
	}
}

// Unguessability: no collisions across many ids, and every one of the 128 bits is set about
// half the time, which a counter or a timestamp would not do.
func TestIDsAreRandom(t *testing.T) {
	const n = 100_000
	b := &Builder{}
	seen := make(map[string]struct{}, n)
	var ones [128]int
	for range n {
		id, err := b.newID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 22 || !validID(id) {
			t.Fatalf("bad id %q", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("collision on %q", id)
		}
		seen[id] = struct{}{}
		raw, _ := base64.RawURLEncoding.Strict().DecodeString(id)
		for bit := range 128 {
			if raw[bit/8]>>(7-bit%8)&1 == 1 {
				ones[bit]++
			}
		}
	}
	for bit, c := range ones {
		// 5 standard deviations of a fair coin over n tosses is 0.0079.
		if share := float64(c) / n; share < 0.49 || share > 0.51 {
			t.Errorf("bit %d is set in %.4f of ids, want about 0.5", bit, share)
		}
	}
}

func TestIDsFromBuildAreRandomAndStored(t *testing.T) {
	e := newEnv(t)
	seen := map[string]bool{}
	for range 300 {
		br := e.build("")
		if seen[br.ID] {
			t.Fatalf("id %q repeated", br.ID)
		}
		seen[br.ID] = true
		if len(br.ID) != 22 || !validID(br.ID) {
			t.Fatalf("id %q is not a 22-character id", br.ID)
		}
	}
	if n := briefCount(t, e); n != 300 {
		t.Errorf("rows = %d, want 300", n)
	}
}

// A failing random source stops the build. It never falls back to a weaker id.
func TestRandomSourceFailureStoresNothing(t *testing.T) {
	for name, r := range map[string]io.Reader{
		"short":  strings.NewReader("only 5"),
		"empty":  strings.NewReader(""),
		"errors": errReader{},
	} {
		e := newEnv(t, WithRand(r))
		if _, err := e.b.Build(context.Background(), e.user, ""); err == nil {
			t.Errorf("%s source: Build succeeded", name)
		}
		if n := briefCount(t, e); n != 0 {
			t.Errorf("%s source: %d rows stored", name, n)
		}
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

// The id is the one the random source gave, so it is not derived from anything else.
func TestIDComesFromTheRandomSource(t *testing.T) {
	var raw [16]byte
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	e := newEnv(t, WithRand(strings.NewReader(string(raw[:]))))
	br := e.build("")
	if want := base64.RawURLEncoding.EncodeToString(raw[:]); br.ID != want {
		t.Errorf("id = %q, want %q", br.ID, want)
	}
}

func TestMalformedIDsAreNotFound(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	ctx := context.Background()
	for _, id := range []string{
		"", "x", br.ID[:21], br.ID + "A", strings.ToUpper(br.ID) + "", strings.Repeat("A", 22),
		"' OR 1=1 --" + strings.Repeat("a", 11), br.ID[:21] + "!", " " + br.ID[:21],
		"AAAAAAAAAAAAAAAAAAAAAB", // 22 characters, but not the canonical encoding of 16 bytes.
	} {
		if id == br.ID {
			continue
		}
		if _, err := e.b.Get(ctx, e.user, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) = %v, want ErrNotFound", id, err)
		}
		if err := e.b.MarkUsed(ctx, e.user, id, 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("MarkUsed(%q) = %v, want ErrNotFound", id, err)
		}
	}
	if _, err := e.b.Get(ctx, 0, br.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get as learner 0 = %v", err)
	}
	if _, err := e.b.Get(ctx, -1, br.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get as learner -1 = %v", err)
	}
}

// Get returns what Build returned.
func TestGetRoundTrip(t *testing.T) {
	f := newFixture(t)
	f.exec(`INSERT INTO coach_notes (user_id, author, body, created_at) VALUES (?, 'muse', 'ח/כ', ?)`, f.user, testNow.Format("2006-01-02T15:04:05.000000000Z"))
	br := f.build("Food")
	got, err := f.b.Get(context.Background(), f.user, br.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != br.ID || got.UserID != f.user || !got.CreatedAt.Equal(br.CreatedAt) || !got.ExpiresAt.Equal(testNow.Add(TTL)) {
		t.Errorf("got %+v, built %+v", got, br)
	}
	if got.UsedByUnitID != nil {
		t.Errorf("a new brief is used by %d", *got.UsedByUnitID)
	}
	if !reflect.DeepEqual(got.Plan, br.Plan) {
		t.Errorf("plan differs after the round trip:\n got %+v\nwant %+v", got.Plan, br.Plan)
	}
}

// ac-4: Get with another learner's brief id returns not found.
func TestAC4OneLearnerCannotReadAnothersBrief(t *testing.T) {
	f := newFixture(t)
	mine := f.build("")
	other := f.other()
	ctx := context.Background()

	_, err := f.b.Get(ctx, other, mine.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get as another learner = %v, want ErrNotFound", err)
	}
	// The same error as an id that does not exist, so the two cannot be told apart.
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	_, errMissing := f.b.Get(ctx, other, base64.RawURLEncoding.EncodeToString(raw[:]))
	if err.Error() != errMissing.Error() || !errors.Is(errMissing, ErrNotFound) {
		t.Errorf("other learner's id gave %q, a missing id gave %q", err, errMissing)
	}
	// The other learner cannot use it either, and it stays unused for its owner.
	unit := addUnit(t, f.env, 1)
	if err := f.b.MarkUsed(ctx, other, mine.ID, unit); !errors.Is(err, ErrNotFound) {
		t.Errorf("MarkUsed as another learner = %v, want ErrNotFound", err)
	}
	got, err := f.b.Get(ctx, f.user, mine.ID)
	if err != nil || got.UsedByUnitID != nil {
		t.Errorf("owner's brief = %+v (%v), want it untouched", got.UsedByUnitID, err)
	}
	// The other learner's own brief is theirs alone.
	theirs, err := f.b.Build(ctx, other, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.b.Get(ctx, f.user, theirs.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get of the other learner's brief by the first learner = %v", err)
	}
	if _, err := f.b.Get(ctx, other, theirs.ID); err != nil {
		t.Errorf("a learner cannot read their own brief: %v", err)
	}
	// A brief is built from its own learner's evidence only: the new learner has none.
	if len(theirs.ComfortableWords) != 0 || len(theirs.StruggleWords) != 0 {
		t.Errorf("another learner's evidence leaked into %+v", theirs.Plan)
	}
}

func TestExpiry(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	ctx := context.Background()
	if !br.ExpiresAt.Equal(testNow.Add(7 * 24 * time.Hour)) {
		t.Fatalf("expires %v, want 7 days after %v", br.ExpiresAt, testNow)
	}
	unit := addUnit(t, e, 1)

	e.clk.t = testNow.Add(TTL - time.Nanosecond)
	if _, err := e.b.Get(ctx, e.user, br.ID); err != nil {
		t.Errorf("Get just before expiry: %v", err)
	}
	e.clk.t = testNow.Add(TTL)
	if _, err := e.b.Get(ctx, e.user, br.ID); !errors.Is(err, ErrExpired) {
		t.Errorf("Get at expiry = %v, want ErrExpired", err)
	}
	if err := e.b.MarkUsed(ctx, e.user, br.ID, unit); !errors.Is(err, ErrExpired) {
		t.Errorf("MarkUsed at expiry = %v, want ErrExpired", err)
	}
	e.clk.t = testNow.Add(30 * 24 * time.Hour)
	if _, err := e.b.Get(ctx, e.user, br.ID); !errors.Is(err, ErrExpired) {
		t.Errorf("Get a month later = %v", err)
	}

	// Just before expiry a brief can still be used.
	e.clk.t = testNow
	fresh := e.build("")
	e.clk.t = testNow.Add(TTL - time.Nanosecond)
	if err := e.b.MarkUsed(ctx, e.user, fresh.ID, unit); err != nil {
		t.Errorf("MarkUsed just before expiry: %v", err)
	}
	// An expired brief is still not readable by another learner as anything but not found.
	e.clk.t = testNow.Add(TTL * 2)
	if _, err := e.b.Get(ctx, e.other(), br.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another learner's Get of an expired brief = %v, want ErrNotFound", err)
	}
}

func TestSingleUse(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	ctx := context.Background()
	first, second := addUnit(t, e, 1), addUnit(t, e, 2)

	if err := e.b.MarkUsed(ctx, e.user, br.ID, first); err != nil {
		t.Fatal(err)
	}
	got, err := e.b.Get(ctx, e.user, br.ID)
	if err != nil {
		t.Fatalf("Get of a used brief: %v", err)
	}
	if got.UsedByUnitID == nil || *got.UsedByUnitID != first {
		t.Fatalf("used by %v, want %d", got.UsedByUnitID, first)
	}
	// A second use is refused, by the same unit or another, and the first unit stays.
	for _, unit := range []int64{first, second} {
		if err := e.b.MarkUsed(ctx, e.user, br.ID, unit); !errors.Is(err, ErrUsed) {
			t.Errorf("MarkUsed again with unit %d = %v, want ErrUsed", unit, err)
		}
	}
	got, _ = e.b.Get(ctx, e.user, br.ID)
	if got.UsedByUnitID == nil || *got.UsedByUnitID != first {
		t.Errorf("used by %v after refused reuse, want %d", got.UsedByUnitID, first)
	}
	// The used brief's JSON says so.
	if !strings.Contains(mustJSON(t, got), fmt.Sprintf(`"used_by_unit_id":%d`, first)) {
		t.Error("the used brief's JSON lacks used_by_unit_id")
	}
	// Another brief is unaffected.
	other := e.build("")
	if err := e.b.MarkUsed(ctx, e.user, other.ID, second); err != nil {
		t.Errorf("a second brief could not be used: %v", err)
	}
}

func TestMarkUsedRefusesBadUnits(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	ctx := context.Background()
	for _, unit := range []int64{0, -3} {
		if err := e.b.MarkUsed(ctx, e.user, br.ID, unit); err == nil {
			t.Errorf("MarkUsed with unit %d succeeded", unit)
		}
	}
	// A unit that does not exist is refused by the foreign key, and the brief stays unused.
	err := e.b.MarkUsed(ctx, e.user, br.ID, 424242)
	if err == nil || errors.Is(err, ErrUsed) || errors.Is(err, ErrNotFound) {
		t.Errorf("MarkUsed with a missing unit = %v, want a database error", err)
	}
	if got, err := e.b.Get(ctx, e.user, br.ID); err != nil || got.UsedByUnitID != nil {
		t.Errorf("brief = %v (%v), want it unused", got.UsedByUnitID, err)
	}
	if err := e.b.MarkUsed(ctx, e.user, br.ID, addUnit(t, e, 1)); err != nil {
		t.Errorf("a real unit after a refused one: %v", err)
	}
}

// Two units racing for one brief: exactly one wins.
func TestSingleUseUnderConcurrency(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	const n = 8
	units := make([]int64, n)
	for i := range units {
		units[i] = addUnit(t, e, i)
	}
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = e.b.MarkUsed(context.Background(), e.user, br.ID, units[i])
		}()
	}
	wg.Wait()
	winners := 0
	for i, err := range results {
		switch {
		case err == nil:
			winners++
			got, _ := e.b.Get(context.Background(), e.user, br.ID)
			if got.UsedByUnitID == nil || *got.UsedByUnitID != units[i] {
				t.Errorf("winner %d, but the brief is used by %v", units[i], got.UsedByUnitID)
			}
		case !errors.Is(err, ErrUsed):
			t.Errorf("loser got %v, want ErrUsed", err)
		}
	}
	if winners != 1 {
		t.Errorf("%d callers won, want 1", winners)
	}
}

// Deleting the learner deletes their briefs, and deleting the unit clears the use.
func TestRowsFollowTheirParents(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	unit := addUnit(t, e, 1)
	if err := e.b.MarkUsed(context.Background(), e.user, br.ID, unit); err != nil {
		t.Fatal(err)
	}
	e.exec(`DELETE FROM units WHERE id = ?`, unit)
	got, err := e.b.Get(context.Background(), e.user, br.ID)
	if err != nil || got.UsedByUnitID != nil {
		t.Errorf("after the unit was deleted: %v (%v)", got.UsedByUnitID, err)
	}
	e.exec(`DELETE FROM users WHERE id = ?`, e.user)
	if n := briefCount(t, e); n != 0 {
		t.Errorf("%d briefs left after the learner was deleted", n)
	}
}

func TestUnsupportedVersionIsRefused(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	e.exec(`UPDATE lesson_briefs SET payload_json = '{"v":2}' WHERE id = ?`, br.ID)
	if _, err := e.b.Get(context.Background(), e.user, br.ID); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("Get of a v2 payload = %v, want a version error", err)
	}
}

func TestBuilderNeedsItsParts(t *testing.T) {
	var nilBuilder *Builder
	if _, err := nilBuilder.Build(context.Background(), 1, ""); err == nil {
		t.Error("a nil builder built a brief")
	}
	if _, err := (&Builder{}).Get(context.Background(), 1, "x"); err == nil {
		t.Error("an empty builder read a brief")
	}
	e := newEnv(t)
	if _, err := e.b.Build(context.Background(), 0, ""); err == nil {
		t.Error("built a brief for learner 0")
	}
	// A learner that does not exist: the foreign key refuses the row.
	if _, err := e.b.Build(context.Background(), 9999, ""); err == nil {
		t.Error("built a brief for a learner that does not exist")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
