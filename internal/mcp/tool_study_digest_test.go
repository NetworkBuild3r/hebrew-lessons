package mcp_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// digestNow is Friday 2026-10-31 18:00 Denver (MDT), so today is 2026-10-31.
var digestNow = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

func digestAP(t *testing.T) *apEnv {
	t.Helper()
	e := newAP(t)
	app, err := studyday.AppZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := studyday.NewResolver(e.adapter, app)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetUserTimeZone(context.Background(), e.a, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	e.taughtOpts = mcp.Options{
		Validator: sqlTokens{a: e.a, b: e.b},
		Course:    e.adapter, Lexicon: e.adapter, Lexemes: e.adapter, Lookup: e.adapter, Units: e.adapter,
		StudyDay: resolver,
		Now:      func() time.Time { return digestNow },
	}
	return e
}

func (e *apEnv) asDigest(token string) *sdkmcp.ClientSession {
	e.t.Helper()
	return e.connect(e.taughtOpts, token)
}

func (e *apEnv) snapSkill(user int64, day, skill, state string, count int, retention float64) {
	e.t.Helper()
	e.exec(`INSERT INTO daily_snapshots (user_id, day, skill, state, count, est_known, retention)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, user, day, skill, state, count, float64(count), retention)
}

func (e *apEnv) seedDigestFixture() {
	e.t.Helper()
	denver := mustLoc(e.t, "America/Denver")
	z := studyday.Zone{Name: "America/Denver", Loc: denver}

	// Snapshots: today and 7 days ago for every skill.
	for _, skill := range []string{"listen", "read", "recall", "spell"} {
		e.snapSkill(e.a, "2026-10-31", skill, "known", 10, 0.88)
		e.snapSkill(e.a, "2026-10-31", skill, "solid", 5, 0.88)
		e.snapSkill(e.a, "2026-10-31", skill, "struggling", 3, 0.88)
		e.snapSkill(e.a, "2026-10-24", skill, "known", 8, 0.80)
		e.snapSkill(e.a, "2026-10-24", skill, "solid", 2, 0.80)
		e.snapSkill(e.a, "2026-10-24", skill, "struggling", 4, 0.80)
	}

	// Three struggling words (not the correction lexemes); mayim has a dominant letter pair.
	for _, text := range []string{apShalom, apBoker, apMayim} {
		e.attemptsAt(e.a, text, "listen", "RRWWWWWW", digestNow.Add(-2*time.Hour))
	}
	for i := range 3 {
		e.exec(`INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at) VALUES (?, 'letter_pair', 'ב/כ', ?, ?)`,
			e.a, e.ids[apMayim], db.FormatTime(digestNow.Add(-time.Duration(i+1)*time.Hour)))
	}

	// Correction lexeme is separate so right_since answers do not clear a struggle word.
	e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos) VALUES (?, ?, 'book', 'noun')`,
		units.Normalize("סֵפֶר"), units.Normalize("ספר"))
	sefer := int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, units.Normalize("סֵפֶר")))
	e.ids[units.Normalize("סֵפֶר")] = sefer

	// Two corrections; one answered right after its last_on day.
	onClosed := "2026-10-28"
	onOpen := "2026-10-27"
	e.exec(`INSERT INTO learner_mistakes (learner_id, wrong_he, right_he, note_en, lexeme_id, times, first_at, last_at, first_on, last_on, source)
		VALUES (?, ?, ?, 'closed note', ?, 2, ?, ?, ?, ?, 'tutor')`,
		e.a, units.Normalize("ספר"), units.Normalize("סֵפֶר"), sefer,
		db.FormatTime(digestNow.Add(-72*time.Hour)), db.FormatTime(digestNow.Add(-48*time.Hour)), onClosed, onClosed)
	e.exec(`INSERT INTO learner_mistakes (learner_id, wrong_he, right_he, note_en, lexeme_id, times, first_at, last_at, first_on, last_on, source)
		VALUES (?, ?, ?, 'open note', ?, 1, ?, ?, ?, ?, 'tutor')`,
		e.a, units.Normalize("תודה"), apToda, e.ids[apToda],
		db.FormatTime(digestNow.Add(-96*time.Hour)), db.FormatTime(digestNow.Add(-90*time.Hour)), onOpen, onOpen)

	// Same-day correct recall must NOT close the loop.
	sameDay := time.Date(2026, 10, 28, 15, 0, 0, 0, denver)
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'recall', 'test', 'review', '', '', 1, 0, 0, 1000, ?)`,
		e.a, sefer, db.FormatTime(sameDay))
	// After end of last_on (2026-10-29 00:00 Denver) counts.
	after := time.Date(2026, 10, 29, 8, 0, 0, 0, denver)
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'recall', 'test', 'review', '', '', 1, 0, 0, 1000, ?)`,
		e.a, sefer, db.FormatTime(after))

	// Two taught words in the last 14 days.
	due := studyday.DueCutoff(digestNow, z)
	for _, text := range []string{apToda, apMayim} {
		e.exec(`INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
			VALUES (?, ?, '2026-10-25', 'read,recall', ?)`, e.a, e.ids[text], db.FormatTime(digestNow))
		for _, skill := range []string{"read", "recall"} {
			e.exec(`INSERT OR IGNORE INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses)
				VALUES (?, ?, ?, 'new', ?, 0, 0, 0, 0)`, e.a, e.ids[text], skill, db.FormatTime(due))
		}
	}

	// Minutes window attempts (two close answers).
	t1 := time.Date(2026, 10, 30, 10, 0, 0, 0, denver)
	t2 := t1.Add(2 * time.Minute)
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'listen', 'test', 'review', '', '', 1, 0, 0, 0, ?)`, e.a, e.ids[apToda], db.FormatTime(t1))
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'listen', 'test', 'review', '', '', 1, 0, 0, 0, ?)`, e.a, e.ids[apToda], db.FormatTime(t2))

	e.exec(`INSERT INTO coach_notes (user_id, author, author_role, body, created_at) VALUES (?, 'coach', 'admin', 'A note', ?)`,
		e.a, db.FormatTime(digestNow))
	e.exec(`INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at)
		VALUES (?, '2026-10-20', '{"gained":[],"lost":[],"cleared":[],"retention":{"target":0.85},"minutesStudied":12,"bandSlips":[],"zone":"America/Denver"}', 'Week A', ?)`,
		e.a, db.FormatTime(digestNow))

	// Learner B secrets that must never appear.
	e.exec(`INSERT INTO learner_mistakes (learner_id, wrong_he, right_he, note_en, times, first_at, last_at, first_on, last_on, source)
		VALUES (?, 'בסוד', 'סוֹד', 'BSECRET-NOTE', 9, ?, ?, '2026-10-28', '2026-10-28', 'tutor')`,
		e.b, db.FormatTime(digestNow), db.FormatTime(digestNow))
	e.exec(`INSERT INTO coach_notes (user_id, author, author_role, body, created_at) VALUES (?, 'b', 'learner', 'BSECRET-BODY', ?)`,
		e.b, db.FormatTime(digestNow))
	e.exec(`INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at)
		VALUES (?, '2026-10-20', '{"secret":"B-payload","gained":[{"text":"BWORD"}]}', 'BSUMMARY', ?)`,
		e.b, db.FormatTime(digestNow))
	e.exec(`INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
		VALUES (?, ?, '2026-10-25', 'read', ?)`, e.b, e.ids[apShalom], db.FormatTime(digestNow))
}

// attemptsAt is like attempts but ends at a chosen instant.
func (e *apEnv) attemptsAt(user int64, text, skill, history string, end time.Time) {
	e.t.Helper()
	for i := range len(history) {
		at := end.Add(-time.Duration(len(history)-1-i) * time.Minute)
		correct := 0
		if history[i] == 'R' {
			correct = 1
		}
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', ?, 0, 0, 0, ?)`, user, e.ids[text], skill, correct, db.FormatTime(at))
	}
}

type digestOut struct {
	Zone       string `json:"zone"`
	ZoneSource string `json:"zoneSource"`
	Today      string `json:"today"`
	NextCutoff string `json:"nextCutoff"`
	AsOf          string `json:"asOf"`
	KnownAnySkill int    `json:"knownAnySkill"`
	Skills        []struct {
		Skill           string `json:"skill"`
		KnownSolidNow   int    `json:"knownSolidNow"`
		KnownSolid7dAgo int    `json:"knownSolid7dAgo"`
		StrugglingNow   int    `json:"strugglingNow"`
		AgoDay          string `json:"agoDay"`
	} `json:"skills"`
	Retention struct {
		BandLow  float64 `json:"bandLow"`
		BandHigh float64 `json:"bandHigh"`
		Skills   []struct {
			Skill      string `json:"skill"`
			BandStatus string `json:"bandStatus"`
			Days       []struct {
				Day    string   `json:"day"`
				Value  *float64 `json:"value"`
				Window string   `json:"window"`
				AsOf   string   `json:"asOf"`
			} `json:"days"`
		} `json:"skills"`
	} `json:"retention"`
	MinutesInApp int `json:"minutesInApp"`
	Struggle     []struct {
		Text         string `json:"text"`
		Gloss        string `json:"gloss"`
		Cause        string `json:"cause"`
		ConfusedWith *struct {
			Kind  string `json:"kind"`
			Key   string `json:"key"`
			Count int    `json:"count"`
		} `json:"confusedWith"`
	} `json:"struggle"`
	Corrections []struct {
		Wrong  string `json:"wrong"`
		Right  string `json:"right"`
		Note   string `json:"note"`
		On     string `json:"on"`
		Times  int    `json:"times"`
		Word   string `json:"word"`
		Status struct {
			Kind  string `json:"kind"`
			Count int    `json:"count"`
		} `json:"status"`
	} `json:"corrections"`
	ClosedLoop struct {
		Since      string `json:"since"`
		Total      int    `json:"total"`
		RightSince int    `json:"rightSince"`
	} `json:"closedLoop"`
	Taught []struct {
		Text  string `json:"text"`
		On    string `json:"on"`
		Cards []struct {
			Skill   string `json:"skill"`
			State   string `json:"state"`
			Reviews int    `json:"reviews"`
		} `json:"cards"`
	} `json:"taught"`
	Due struct {
		Now      int `json:"now"`
		ByCutoff int `json:"byCutoff"`
	} `json:"due"`
	CoachNotes []struct {
		Author     string `json:"author"`
		AuthorRole string `json:"authorRole"`
		Body       string `json:"body"`
	} `json:"coachNotes"`
	LastWeek *struct {
		WeekStart string `json:"weekStart"`
		Summary   string `json:"summary"`
		Payload   *struct {
			Zone           string `json:"zone"`
			MinutesStudied int    `json:"minutesStudied"`
		} `json:"payload"`
	} `json:"lastWeek"`
}

// ac-1 / ac-2 / ac-5: full digest sections match store queries; closed loop and shared minutes.
func TestStudyDigestSectionsAndClosedLoop(t *testing.T) {
	e := digestAP(t)
	e.seedDigestFixture()
	learner := e.asDigest("learner-a")
	raw := mustOK(t, learner, "get_study_digest", map[string]any{})
	d := decodeInto[digestOut](t, raw)

	if d.Zone != "America/Denver" || d.ZoneSource != "learner" || d.Today != "2026-10-31" {
		t.Fatalf("zone/today = %s/%s/%s", d.Zone, d.ZoneSource, d.Today)
	}
	wantCut := studyday.DueCutoff(digestNow, studyday.Zone{Name: "America/Denver", Loc: mustLoc(t, "America/Denver")})
	gotCut, err := time.Parse(time.RFC3339Nano, d.NextCutoff)
	if err != nil {
		gotCut, err = time.Parse(time.RFC3339, d.NextCutoff)
	}
	if err != nil || !gotCut.Equal(wantCut.UTC()) {
		t.Fatalf("nextCutoff = %v, want %v (%v)", d.NextCutoff, wantCut.UTC(), err)
	}
	if len(d.Skills) != 4 {
		t.Fatalf("skills = %d", len(d.Skills))
	}
	if d.AsOf == "" {
		t.Fatal("asOf missing")
	}
	// Live now counts (ADR-0027 D4): fixture has no known/solid cards, three listen struggles.
	for _, sk := range d.Skills {
		if sk.KnownSolid7dAgo != 10 || sk.AgoDay != "2026-10-24" {
			t.Fatalf("7d ago skill %+v", sk)
		}
		if sk.Skill == "listen" {
			if sk.KnownSolidNow != 0 || sk.StrugglingNow != 3 {
				t.Fatalf("live listen %+v", sk)
			}
			continue
		}
		if sk.KnownSolidNow != 0 || sk.StrugglingNow != 0 {
			t.Fatalf("live skill %+v", sk)
		}
	}
	for _, sk := range d.Retention.Skills {
		for _, day := range sk.Days {
			if day.Window != "30d due reviews" || day.AsOf != day.Day {
				t.Fatalf("retention day %+v", day)
			}
		}
	}
	if d.Retention.BandLow != 0.85 || d.Retention.BandHigh != 0.90 {
		t.Fatalf("retention band %+v", d.Retention)
	}
	if len(d.Struggle) != 3 {
		t.Fatalf("struggle = %d %s", len(d.Struggle), raw)
	}
	var mayimOK bool
	for _, w := range d.Struggle {
		if w.Text == apMayim && w.ConfusedWith != nil && w.ConfusedWith.Key == "ב/כ" && w.ConfusedWith.Count == 3 {
			mayimOK = true
		}
	}
	if !mayimOK {
		t.Fatalf("mayim confusedWith missing: %s", raw)
	}
	if len(d.Corrections) != 2 || d.ClosedLoop.Total != 2 || d.ClosedLoop.RightSince != 1 {
		t.Fatalf("corrections/closedLoop %+v / %+v", d.Corrections, d.ClosedLoop)
	}
	var closed, open bool
	for _, c := range d.Corrections {
		if c.On == "2026-10-28" && c.Status.Kind == "right_since" && c.Status.Count == 1 {
			closed = true
		}
		if c.On == "2026-10-27" && c.Status.Kind == "open" {
			open = true
		}
	}
	if !closed || !open {
		t.Fatalf("correction status: %s", raw)
	}
	if len(d.Taught) != 2 {
		t.Fatalf("taught = %d", len(d.Taught))
	}
	if len(d.CoachNotes) != 1 || d.CoachNotes[0].AuthorRole != "admin" {
		t.Fatalf("coachNotes %+v", d.CoachNotes)
	}
	if d.LastWeek == nil || d.LastWeek.WeekStart != "2026-10-20" || d.LastWeek.Summary != "Week A" {
		t.Fatalf("lastWeek %+v", d.LastWeek)
	}
	if d.Due.ByCutoff < d.Due.Now {
		t.Fatalf("due %+v", d.Due)
	}

	// ac-5: minutes match the shared ActiveMinutes path.
	denver := mustLoc(t, "America/Denver")
	from := time.Date(2026, 10, 25, 0, 0, 0, 0, denver)
	to := time.Date(2026, 11, 1, 0, 0, 0, 0, denver)
	attempts, err := e.store.AttemptTimes(context.Background(), e.a, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if want := jobs.ActiveMinutes(attempts); d.MinutesInApp != want {
		t.Fatalf("minutesInApp = %d, ActiveMinutes = %d", d.MinutesInApp, want)
	}
}

// ac-3: learner B never appears; naming B is refused.
func TestStudyDigestIsolation(t *testing.T) {
	e := digestAP(t)
	e.seedDigestFixture()
	a := e.asDigest("learner-a")
	raw := mustOK(t, a, "get_study_digest", map[string]any{})
	for _, banned := range []string{"BSECRET", "BWORD", "BSUMMARY", "בסוד", "B-payload"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("digest leaks %q: %s", banned, raw)
		}
	}
	if msg := mustFail(t, a, "get_study_digest", map[string]any{"learnerId": e.b}); msg != "learnerId must be your own id." {
		t.Fatalf("naming b: %q", msg)
	}
}

// ac-4: lists at caps stay within 16 KB.
func TestStudyDigestSizeCap(t *testing.T) {
	e := digestAP(t)
	if err := e.store.SetUserTimeZone(context.Background(), e.a, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	denver := mustLoc(t, "America/Denver")
	due := studyday.DueCutoff(digestNow, studyday.Zone{Name: "America/Denver", Loc: denver})
	// Cap-sized lists with modest string lengths; the 16 KB bound is on the JSON.
	for i := 0; i < 10; i++ {
		text := units.Normalize(fmt.Sprintf("ס%02d", i))
		e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos) VALUES (?, ?, ?, 'noun')`, text, text, fmt.Sprintf("g%d", i))
		id := int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, text))
		e.ids[text] = id
		e.attemptsAt(e.a, text, "listen", "RRWWWWWW", digestNow.Add(-time.Hour))
	}
	for i := 0; i < 20; i++ {
		e.exec(`INSERT INTO learner_mistakes (learner_id, wrong_he, right_he, note_en, times, first_at, last_at, first_on, last_on, source)
			VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?, 'tutor')`,
			e.a, units.Normalize(fmt.Sprintf("w%02d", i)), units.Normalize(fmt.Sprintf("r%02d", i)),
			fmt.Sprintf("n%d", i), db.FormatTime(digestNow), db.FormatTime(digestNow),
			fmt.Sprintf("2026-10-%02d", 10+(i%20)), fmt.Sprintf("2026-10-%02d", 10+(i%20)))
	}
	for i := 0; i < 30; i++ {
		text := units.Normalize(fmt.Sprintf("t%02d", i))
		e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos) VALUES (?, ?, 'g', 'noun')`, text, text)
		id := int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, text))
		e.exec(`INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
			VALUES (?, ?, '2026-10-25', 'read,recall', ?)`, e.a, id, db.FormatTime(digestNow))
		for _, skill := range []string{"read", "recall"} {
			e.exec(`INSERT OR IGNORE INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses)
				VALUES (?, ?, ?, 'new', ?, 0, 0, 0, 0)`, e.a, id, skill, db.FormatTime(due))
		}
	}
	raw := mustOK(t, e.asDigest("learner-a"), "get_study_digest", map[string]any{})
	if len(raw) > 16*1024 {
		t.Fatalf("digest JSON is %d bytes, want <= 16384", len(raw))
	}
}

// ac-6: hebrew://tutor is listed, names the tools, holds no secret.
func TestTutorResource(t *testing.T) {
	s := resConnect(t)
	res, err := s.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var found *sdkmcp.Resource
	for _, r := range res.Resources {
		if r.URI == "hebrew://tutor" {
			found = r
		}
	}
	if found == nil || found.MIMEType != "text/markdown" {
		t.Fatalf("resources %+v", res.Resources)
	}
	text := resRead(t, s, "hebrew://tutor")
	for _, name := range []string{"get_study_digest", "record_taught_words", "record_mistakes", "remove_taught_words"} {
		if !strings.Contains(text, "`"+name+"`") {
			t.Errorf("tutor guide never names %s", name)
		}
	}
	for _, want := range []string{"95–98%", "try again", "transcript", "recordings", "other people's names", "RSCH-003"} {
		if !strings.Contains(text, want) {
			t.Errorf("tutor guide missing %q", want)
		}
	}
	lower := strings.ToLower(text)
	for _, banned := range []string{"bearer", "secret", "password", "@", "mcp_auth", "elevenlabs", "api key", "api_key", "apikey", "authorization", "sk-", "http://", "https://", "localhost", "cookie"} {
		if strings.Contains(lower, banned) {
			t.Errorf("tutor guide mentions %q", banned)
		}
	}
}
