package app

// INIT-032/SPEC-009: cross-surface agreement for learner numbers (ADR-0027).
// One fixture learner is read through Home, Progress, and MCP tools over httptest.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const agreeToken = "agree-test-token"

type agreeFix struct {
	app    *App
	userID int64
	cookie *http.Cookie
	when   time.Time
	zone   string
	sum    learner.Summary
}

func TestNumbersAgreeAcrossSurfaces(t *testing.T) {
	fx := seedAgreeFixture(t)
	id := fx.userID

	home := getAgree(t, fx, "/")
	progress := getAgree(t, fx, "/progress")

	session := connectAppMCP(t, fx.app.MCP(), agreeToken)
	digest := callAgree[mcp.StudyDigest](t, session, "get_study_digest", map[string]any{"learnerId": id})
	ctxRaw := callAppTool(t, session, "get_context", map[string]any{"position": 0, "learnerId": id})
	var ctxFull struct {
		Model     *mcp.ModelSummary `json:"model"`
		Placement struct {
			Taken     bool `json:"taken"`
			Level     int  `json:"level"`
			Listening *struct {
				Tested bool `json:"tested"`
				Level  int  `json:"level"`
			} `json:"listening"`
		} `json:"placement"`
	}
	decodeStructured(t, ctxRaw, &ctxFull)
	if ctxFull.Model == nil {
		t.Fatal("get_context.model is nil")
	}
	modelSum := *ctxFull.Model

	progMCP := callAgree[mcp.Progress](t, session, "get_progress", map[string]any{"learnerId": id})

	// ---- Today's count is Hebrew-from-English proved (ADR-0033 D6) ----
	wantProved := skillLadder(fx.sum, fsrs.SkillRecall).Proved
	assertHomeKnown(t, home, wantProved)
	if !strings.Contains(progress, "Hebrew from English") || !strings.Contains(progress, "Understand by ear") {
		t.Fatal("Progress missing the ladder headlines")
	}
	if got := rungCount(t, progress, fsrs.SkillRecall, "proved"); got != wantProved {
		t.Errorf("progress recall proved=%d want %d", got, wantProved)
	}
	if digest.KnownAnySkill != fx.sum.KnownAnySkill {
		t.Errorf("digest knownAnySkill=%d want %d", digest.KnownAnySkill, fx.sum.KnownAnySkill)
	}
	if modelSum.KnownAnySkill != fx.sum.KnownAnySkill {
		t.Errorf("get_context.model knownAnySkill=%d want %d", modelSum.KnownAnySkill, fx.sum.KnownAnySkill)
	}

	// ---- per-skill known+solid, estimate, source ----
	standingEarly, err := learner.New(fx.app.Store(), fixedClock{fx.when}).Standing(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	askable := map[string]bool{}
	for _, d := range standingEarly.Directions {
		askable[d.Skill] = d.Askable
	}
	for _, sk := range fx.sum.Skills {
		wantKS := sk.Counts[learner.Known] + sk.Counts[learner.Solid]
		row := overviewRow(t, progress, sk.Skill)
		if got := rungCountIn(t, row, "proved"); got != sk.Ladder.Proved {
			t.Errorf("progress %s proved=%d want %d", sk.Skill, got, sk.Ladder.Proved)
		}
		if !askable[sk.Skill] && sk.Skill == fsrs.SkillListen && !strings.Contains(row, "No audio yet") {
			t.Errorf("%s not_askable row missing No audio yet:\n%s", sk.Skill, row)
		}
		var dig mcp.DigestSkill
		for _, d := range digest.Skills {
			if d.Skill == sk.Skill {
				dig = d
			}
		}
		if dig.KnownSolidNow != wantKS {
			t.Errorf("digest %s knownSolidNow=%d want %d", sk.Skill, dig.KnownSolidNow, wantKS)
		}
		if dig.Direction != learner.DirectionLabel(sk.Skill) {
			t.Errorf("digest %s direction=%q", sk.Skill, dig.Direction)
		}
		var mod mcp.ModelSkillSummary
		for _, m := range modelSum.Skills {
			if m.Skill == sk.Skill {
				mod = m
			}
		}
		modKS := mod.Counts[string(learner.Known)] + mod.Counts[string(learner.Solid)]
		if modKS != wantKS {
			t.Errorf("model %s known+solid=%d want %d", sk.Skill, modKS, wantKS)
		}
		if math.Abs(mod.EstimatedKnown-sk.EstimatedKnown) > 1e-6 {
			t.Errorf("model %s estimate=%v want %v", sk.Skill, mod.EstimatedKnown, sk.EstimatedKnown)
		}
		if mod.EstimateSource != sk.EstimateSource {
			t.Errorf("model %s estimateSource=%q want %q", sk.Skill, mod.EstimateSource, sk.EstimateSource)
		}
	}

	// ---- retention window + asOf ----
	if modelSum.Retention == nil {
		t.Fatal("model retention is nil; fixture needs a due review")
	}
	if modelSum.Retention.Window != mcp.RetentionWindow30d || modelSum.Retention.AsOf == "" {
		t.Fatalf("model retention %+v", modelSum.Retention)
	}
	if progMCP.Retention.Window != mcp.RetentionWindow30d {
		t.Fatalf("get_progress retention window=%q", progMCP.Retention.Window)
	}
	for _, sk := range digest.Retention.Skills {
		for _, d := range sk.Days {
			if d.Window != "" && d.Window != mcp.RetentionWindow30d {
				t.Errorf("digest retention window=%q", d.Window)
			}
		}
	}

	// ---- placement per-part level (run 4 shape: listening 2, run level 1) ----
	if !ctxFull.Placement.Taken || ctxFull.Placement.Level != 1 {
		t.Fatalf("get_context placement level=%d taken=%v want level 1", ctxFull.Placement.Level, ctxFull.Placement.Taken)
	}
	if ctxFull.Placement.Listening == nil || !ctxFull.Placement.Listening.Tested || ctxFull.Placement.Listening.Level != 2 {
		t.Fatalf("get_context listening part %+v, want tested level 2", ctxFull.Placement.Listening)
	}
	if len(progMCP.Placements) == 0 {
		t.Fatal("get_progress has no placements")
	}
	p := progMCP.Placements[0]
	if p.Level != 1 {
		t.Fatalf("get_progress placement.level=%d want 1", p.Level)
	}
	if p.ListeningPart == nil || !p.ListeningPart.Tested || p.ListeningPart.Level != 2 {
		t.Fatalf("get_progress listening %+v, want level 2", p.ListeningPart)
	}
	if !strings.Contains(progress, "Listening level 2") {
		t.Errorf("Progress placement card missing Listening level 2:\n%s", sectionPlace(progress))
	}

	// ---- live now, not missing today snapshot ----
	if dig := digestSkill(digest, "read"); dig.KnownSolidNow == 0 {
		t.Fatal("digest read knownSolidNow is 0; live model should see reading cards (snapshot today is absent)")
	}
	if progMCP.SnapshotsThrough == studyday.Today(fx.when, mustZone(t, fx.zone)) {
		// Yesterday only — through should be yesterday, not inventing today.
	}
	today := studyday.Today(fx.when, mustZone(t, fx.zone))
	if progMCP.SnapshotsThrough == today {
		t.Fatalf("snapshotsThrough=%q is today; fixture has no today snapshot", progMCP.SnapshotsThrough)
	}

	// INIT-033/SPEC-008: chart, headline, verdicts, MCP standing, compact context, and
	// digest history all say the same thing as Model.Standing.
	surf := readStandingSurfaces(t, fx, session, progress, digest, ctxFull.Model.Standing)
	if len(surf.words) == 0 {
		t.Fatal("get_learner_model returned no words")
	}
	for _, d := range standingDisagreements(t, surf, standingEarly) {
		t.Error(d)
	}
}

// standingSurfaces is what each learner-facing surface says about the learner's standing.
type standingSurfaces struct {
	progress string // /progress HTML: headline, chart table, data-verdict per direction
	digest   []mcp.DigestHistoryPoint
	summary  mcp.LearnerModelSummaryLite
	standing mcp.StandingView
	compact  *mcp.CompactStanding
	words    []mcp.ModelWord
}

func readStandingSurfaces(t *testing.T, fx agreeFix, session *mcpsdk.ClientSession, progress string, digest mcp.StudyDigest, compact *mcp.CompactStanding) standingSurfaces {
	t.Helper()
	page := callAgree[struct {
		Words    []mcp.ModelWord             `json:"words"`
		Summary  mcp.LearnerModelSummaryLite `json:"summary"`
		Standing mcp.StandingView            `json:"standing"`
	}](t, session, "get_learner_model", map[string]any{"learnerId": fx.userID, "limit": 200})
	return standingSurfaces{
		progress: progress, digest: digest.History, summary: page.Summary,
		standing: page.Standing, compact: compact, words: page.Words,
	}
}

// standingDisagreements names every surface that differs from Model.Standing. Empty means
// they agree.
func standingDisagreements(t *testing.T, s standingSurfaces, model learner.Standing) []string {
	t.Helper()
	var out []string
	want := model.KnownAnySkill
	wantProved := directionLadder(model, fsrs.SkillRecall).Proved
	if got := rungCount(t, s.progress, fsrs.SkillRecall, "proved"); got != wantProved {
		out = append(out, fmt.Sprintf("progress recall proved=%d, model=%d", got, wantProved))
	}
	if got := chartKnownLast(t, s.progress); got != wantProved {
		out = append(out, fmt.Sprintf("progress chart last point=%d, model recall proved=%d", got, wantProved))
	}
	if s.summary.KnownAnySkill != want {
		out = append(out, fmt.Sprintf("get_learner_model summary knownAnySkill=%d, model=%d", s.summary.KnownAnySkill, want))
	}
	if s.standing.KnownAnySkill != want {
		out = append(out, fmt.Sprintf("get_learner_model standing knownAnySkill=%d, model=%d", s.standing.KnownAnySkill, want))
	}
	if len(s.digest) == 0 {
		out = append(out, "digest history is empty")
	} else if last := s.digest[len(s.digest)-1]; last.KnownAnySkill != want || !last.Live {
		out = append(out, fmt.Sprintf("digest history last=%+v, want live knownAnySkill=%d", last, want))
	}
	if len(s.standing.Directions) != len(model.Directions) {
		out = append(out, fmt.Sprintf("get_learner_model directions=%d, model=%d", len(s.standing.Directions), len(model.Directions)))
	}
	for i, d := range model.Directions {
		if i < len(s.standing.Directions) {
			if got := s.standing.Directions[i]; got.Skill != d.Skill || got.Verdict != d.Verdict || got.Direction != d.Direction {
				out = append(out, fmt.Sprintf("get_learner_model direction %s=%+v, model=%+v", d.Skill, got, d))
			}
		}
		if s.compact == nil {
			continue
		}
		var compact mcp.CompactDirection
		for _, c := range s.compact.Directions {
			if c.Skill == d.Skill {
				compact = c
			}
		}
		if compact.Verdict != d.Verdict {
			out = append(out, fmt.Sprintf("get_context standing %s verdict=%q, model %q", d.Skill, compact.Verdict, d.Verdict))
		}
	}
	if s.compact == nil {
		out = append(out, "get_context.model.standing is nil")
	}
	return out
}

// TestNumbersAgreePlantedMismatch reads the real surfaces, changes exactly one of them, and
// requires the agreement check to name it. The unedited surfaces must agree.
func TestNumbersAgreePlantedMismatch(t *testing.T) {
	fx := seedAgreeFixture(t)
	progress := getAgree(t, fx, "/progress")
	session := connectAppMCP(t, fx.app.MCP(), agreeToken)
	digest := callAgree[mcp.StudyDigest](t, session, "get_study_digest", map[string]any{"learnerId": fx.userID})
	var ctxFull struct {
		Model *mcp.ModelSummary `json:"model"`
	}
	decodeStructured(t, callAppTool(t, session, "get_context", map[string]any{"position": 0, "learnerId": fx.userID}), &ctxFull)
	if ctxFull.Model == nil {
		t.Fatal("get_context.model is nil")
	}
	model, err := learner.New(fx.app.Store(), fixedClock{fx.when}).Standing(context.Background(), fx.userID)
	if err != nil {
		t.Fatal(err)
	}
	real := readStandingSurfaces(t, fx, session, progress, digest, ctxFull.Model.Standing)
	if d := standingDisagreements(t, real, model); len(d) != 0 {
		t.Fatalf("unedited surfaces disagree: %v", d)
	}

	wantProved := directionLadder(model, fsrs.SkillRecall).Proved
	want := strconv.Itoa(wantProved)
	chart := regexp.MustCompile(`(?s)<section class="c-card pchart"[^>]*data-chart="est".*?</section>`).FindStringIndex(progress)
	if chart == nil {
		t.Fatal("no est chart")
	}
	card := progress[chart[0]:chart[1]]
	cut := strings.LastIndex(card, ">"+want+"<")
	if cut < 0 {
		t.Fatalf("chart has no cell %s", want)
	}
	editedCard := card[:cut] + ">" + strconv.Itoa(wantProved+1) + "<" + card[cut+len(want)+2:]
	provedOld := `data-rung="proved"`
	if !strings.Contains(overviewRow(t, progress, fsrs.SkillRecall), provedOld) {
		t.Fatal("progress recall row has no proved rung")
	}
	editedDigest := append([]mcp.DigestHistoryPoint(nil), digest.History...)
	editedDigest[len(editedDigest)-1].KnownAnySkill++

	for _, tc := range []struct {
		name, mention string
		edit          func(s standingSurfaces) standingSurfaces
	}{
		{"chart last point", "progress chart", func(s standingSurfaces) standingSurfaces {
			s.progress = progress[:chart[0]] + editedCard + progress[chart[1]:]
			return s
		}},
		{"one proved rung", "proved", func(s standingSurfaces) standingSurfaces {
			row := overviewRow(t, progress, fsrs.SkillRecall)
			planted := strings.Replace(row, "("+want+")", "("+strconv.Itoa(wantProved+1)+")", 1)
			s.progress = strings.Replace(progress, row, planted, 1)
			return s
		}},
		{"digest last point", "digest history", func(s standingSurfaces) standingSurfaces {
			s.digest = editedDigest
			return s
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := standingDisagreements(t, tc.edit(real), model)
			if len(got) != 1 || !strings.Contains(got[0], tc.mention) {
				t.Fatalf("planted %s: disagreements=%v, want exactly one naming %q", tc.name, got, tc.mention)
			}
		})
	}
}

// TestNumbersAgreeOldFormulasDiffer proves the three regressions the agreement
// test fences: weakest-skill headline, snapshot-as-now, all-skill placement.
func TestNumbersAgreeOldFormulasDiffer(t *testing.T) {
	fx := seedAgreeFixture(t)

	// 1) Weakest-skill headline (old Progress) differs from KnownAnySkill.
	weak := learner.WeakestSkillHeadline(fx.sum)
	if weak == fx.sum.KnownAnySkill {
		t.Fatalf("weakest headline %d equals KnownAnySkill; fixture needs uneven skills", weak)
	}

	// 2) Same-day snapshot "now" would be 0 (no today row); live knownSolidNow is not.
	today := studyday.Today(fx.when, mustZone(t, fx.zone))
	snapNow := 0 // missing today snapshot
	liveRead := 0
	for _, sk := range fx.sum.Skills {
		if sk.Skill == fsrs.SkillRead {
			liveRead = sk.Counts[learner.Known] + sk.Counts[learner.Solid]
		}
	}
	if liveRead == 0 {
		t.Fatal("live read known+solid is 0")
	}
	if snapNow == liveRead {
		t.Fatalf("snapshot-now fence collapsed: both %d (today=%s)", liveRead, today)
	}

	// 3) Applying listen bands to every skill would inflate spell; spell is evidence-only.
	bands, err := fx.app.Store().PlacementSkillBands(context.Background(), fx.userID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	listen, ok := bands[fsrs.SkillListen]
	if !ok || len(listen.Bands) == 0 {
		t.Fatal("fixture missing listen PlacementSkillBands")
	}
	var spellKnown int
	for _, sk := range fx.sum.Skills {
		if sk.Skill == fsrs.SkillSpell {
			spellKnown = sk.Counts[learner.Known] + sk.Counts[learner.Solid]
			if sk.EstimateSource != learner.EstimateSourceEvidence {
				t.Fatalf("spell EstimateSource=%q, want evidence", sk.EstimateSource)
			}
		}
	}
	mutated := learner.EstimateKnown(listen.Bands, true, [5]int{}, spellKnown)
	if math.Abs(mutated-float64(spellKnown)) < 1e-9 {
		t.Fatal("listen bands applied to spell still equal evidence-only estimate")
	}
	for _, sk := range fx.sum.Skills {
		if sk.Skill == fsrs.SkillSpell && math.Abs(sk.EstimatedKnown-mutated) < 1e-9 {
			t.Fatalf("spell estimate equals all-skill placement mutation %v", mutated)
		}
	}
}

func seedAgreeFixture(t *testing.T) agreeFix {
	t.Helper()
	when := time.Date(2026, 11, 6, 19, 0, 0, 0, time.UTC)
	zone := "America/Denver"
	a := newTestApp(t, Config{
		DataDir:       t.TempDir(),
		Clock:         fixedClock{when},
		MCPAuthToken:  agreeToken,
		PublicOrigin:  "https://learn.example",
		AllowedEmails: []string{"agree@example.com"},
	})
	ctx := context.Background()
	user, err := a.Store().CreateUser(ctx, "agree@example.com", "sub-agree", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Store().SetUserTimeZone(ctx, user.ID, zone); err != nil {
		t.Fatal(err)
	}

	// Cards in four skills (known/solid), uneven so weakest headline stays low.
	texts := []string{"אֶרֶץ", "שָׁלוֹם", "כֶּלֶב", "מַיִם", "לֶחֶם", "סֵפֶר", "יָד", "עַיִן"}
	ids := make([]int64, len(texts))
	for i, text := range texts {
		ids[i] = mustLexeme(t, a.Store(), text)
	}
	// Read: 4 known; listen: 1; recall: 2; spell: 1 after type_it.
	// ids[0] is also learning in recall so Overall (weakest) hides it from the
	// old headline while KnownAnySkill still counts the solid/known read.
	for i := 0; i < 4; i++ {
		seedKnownCard(t, a.Store(), user.ID, ids[i], fsrs.SkillRead, when)
	}
	seedKnownCard(t, a.Store(), user.ID, ids[4], fsrs.SkillListen, when)
	seedKnownCard(t, a.Store(), user.ID, ids[5], fsrs.SkillRecall, when)
	seedKnownCard(t, a.Store(), user.ID, ids[6], fsrs.SkillRecall, when)
	last := when.AddDate(0, 0, -1)
	learnID, err := a.Store().InsertCard(ctx, db.Card{
		UserID: user.ID, LexemeID: ids[0], Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: when, Stability: 2, Difficulty: 5, Reps: 1, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store().ExecContext(ctx, `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
		VALUES (?, 3, ?, 0, 1, 0)`, learnID, db.FormatTime(last)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store().ExecContext(ctx, `
		INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'recall', 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
		user.ID, ids[0], db.FormatTime(last)); err != nil {
		t.Fatal(err)
	}

	// Implied review on a read card (Good-only Recorder.Implied).
	readCard, ok, err := a.Store().CardID(ctx, user.ID, ids[0], fsrs.SkillRead)
	if err != nil || !ok {
		t.Fatalf("read card: ok=%v err=%v", ok, err)
	}
	rec := evidence.New(a.Store(), fsrs.New(a.Store(), fixedClock{when}), fixedClock{when})
	if err := rec.Implied(ctx, user.ID, readCard, fsrs.Good); err != nil {
		t.Fatal(err)
	}

	// Correct type_it credits spelling (card + implied).
	if _, err := a.Store().InsertCard(ctx, db.Card{
		UserID: user.ID, LexemeID: ids[7], Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: when.Add(-time.Hour), Stability: 5, Difficulty: 5, Reps: 2,
		LastReview: when.Add(-48 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := rec.Attempt(ctx, evidence.Attempt{
		UserID: user.ID, LexemeID: ids[7], Skill: fsrs.SkillRecall, Activity: "type_it",
		Source: evidence.SourceReview, Prompt: "eye", Response: texts[7], Correct: true,
		LettersPassed: true, Latency: 3 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}

	// Listening-only placement shaped like run 4.
	seedListeningRun4(t, a.Store(), user.ID, when.Add(-time.Hour))

	// Snapshot yesterday only — no today.
	z := mustZone(t, zone)
	yesterday := civilAdd(studyday.Today(when, z), -1)
	if err := a.Store().InDailyTx(ctx, func(tx *db.DailyTx) error {
		return tx.InsertSnapshot(ctx, user.ID, yesterday, []db.DailySnapshotRow{
			{Skill: "read", State: "known", Count: 2, EstKnown: 2},
			{Skill: "listen", State: "known", Count: 1, EstKnown: 1},
		})
	}); err != nil {
		t.Fatal(err)
	}

	sum, err := learner.New(a.Store(), fixedClock{when}).Summary(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	cookie := sessionCookie(t, a.Store(), user.ID, when)
	return agreeFix{app: a, userID: user.ID, cookie: cookie, when: when, zone: zone, sum: sum}
}

func sessionCookie(t *testing.T, store *db.Store, userID int64, now time.Time) *http.Cookie {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf)
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), userID, "csrf-agree", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "hebrew_session", Value: base64.RawURLEncoding.EncodeToString(buf)}
}

func getAgree(t *testing.T, fx agreeFix, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "learn.example"
	req.AddCookie(fx.cookie)
	rec := httptest.NewRecorder()
	fx.app.Web().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func callAgree[T any](t *testing.T, session *mcpsdk.ClientSession, name string, args any) T {
	t.Helper()
	res := callAppTool(t, session, name, args)
	var out T
	decodeStructured(t, res, &out)
	return out
}

func skillLadder(sum learner.Summary, skill string) learner.LadderCounts {
	for _, sk := range sum.Skills {
		if sk.Skill == skill {
			return sk.Ladder
		}
	}
	return learner.LadderCounts{}
}

func directionLadder(model learner.Standing, skill string) learner.LadderCounts {
	for _, d := range model.Directions {
		if d.Skill == skill {
			return d.Ladder
		}
	}
	return learner.LadderCounts{}
}

func rungCount(t *testing.T, body, skill, rung string) int {
	t.Helper()
	return rungCountIn(t, overviewRow(t, body, skill), rung)
}

func rungCountIn(t *testing.T, row, rung string) int {
	t.Helper()
	re := regexp.MustCompile(`data-rung="` + regexp.QuoteMeta(rung) + `".*?\(([0-9]+)\)`)
	m := re.FindStringSubmatch(row)
	if m == nil {
		t.Fatalf("no %s rung in row:\n%s", rung, row)
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func assertHomeKnown(t *testing.T, body string, want int) {
	t.Helper()
	wantS := strconv.Itoa(want)
	if !strings.Contains(body, `data-known="`+wantS+`"`) {
		t.Fatalf("Home data-known missing %s", wantS)
	}
	re := regexp.MustCompile(`(?s)today-known.*?class="count"><bdi[^>]*>([0-9]+)</bdi>`)
	if m := re.FindStringSubmatch(body); m == nil || m[1] != wantS {
		t.Fatalf("Home count %v want %s", m, wantS)
	}
}

func overviewRow(t *testing.T, body, skill string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<li class="pknow__skill" data-skill="` + regexp.QuoteMeta(skill) + `"[^>]*>.*?</li>`)
	row := re.FindString(body)
	if row == "" {
		t.Fatalf("no overview row for %s", skill)
	}
	return row
}

func chartKnownLast(t *testing.T, body string) int {
	t.Helper()
	// Words-you-know chart table: last numeric cell of the known series equals the headline.
	card := regexp.MustCompile(`(?s)<section class="c-card pchart"[^>]*data-chart="est".*?</section>`).FindString(body)
	if card == "" {
		t.Fatal("missing Words you know over time chart")
	}
	rows := regexp.MustCompile(`(?s)<tr[^>]*>.*?</tr>`).FindAllString(card, -1)
	if len(rows) < 2 {
		t.Fatalf("chart table rows=%d", len(rows))
	}
	last := rows[len(rows)-1]
	nums := regexp.MustCompile(`>([0-9]+)<`).FindAllStringSubmatch(last, -1)
	if len(nums) == 0 {
		t.Fatalf("no numbers in last chart row:\n%s", last)
	}
	n, _ := strconv.Atoi(nums[len(nums)-1][1])
	return n
}

func sectionPlace(body string) string {
	re := regexp.MustCompile(`(?s)<section class="c-card pplace".*?</section>`)
	return re.FindString(body)
}

func digestSkill(d mcp.StudyDigest, skill string) mcp.DigestSkill {
	for _, sk := range d.Skills {
		if sk.Skill == skill {
			return sk
		}
	}
	return mcp.DigestSkill{}
}

func mustLexeme(t *testing.T, store *db.Store, text string) int64 {
	t.Helper()
	id, err := testharness.InsertLexeme(context.Background(), store, text)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(context.Background(), `
		UPDATE lexemes SET gloss_en = ?, bare = ?, gender = 'm', translit = 'x' WHERE id = ?`,
		"g-"+text, text, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedKnownCard(t *testing.T, store *db.Store, user, lexeme int64, skill string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	last := now.AddDate(0, 0, -1)
	id, err := store.InsertCard(ctx, db.Card{
		UserID: user, LexemeID: lexeme, Skill: skill, State: fsrs.StateReview,
		Due: now, Stability: 10, Difficulty: 5, Reps: 3, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		at := last.Add(-time.Duration(2-i) * 24 * time.Hour)
		if _, err := store.ExecContext(ctx, `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
			VALUES (?, 3, ?, 1, 1, 0)`, id, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			user, lexeme, skill, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
}

// seedListeningRun4 matches mcpstore/spec005 run-4 shape (listening level 2, run level 1).
func seedListeningRun4(t *testing.T, store *db.Store, userID int64, finishedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	started := finishedAt.Add(-time.Minute)
	runID, err := store.StartPlacementV3(ctx, userID, started, 4, db.RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	lex, err := store.UpsertPlacementLexeme(ctx, "אֶרֶץ", "land", "noun", "eretz", "")
	if err != nil {
		t.Fatal(err)
	}
	seq := 1
	claimAgreePlacement(t, store, runID, seq, db.PartLetters, "", 0, "letter", "right", 0, started)
	seq++
	seedListenBand(t, store, runID, &seq, 1, 8, 4, lex, started)
	seedListenBand(t, store, runID, &seq, 2, 5, 1, lex, started)
	seedListenBand(t, store, runID, &seq, 3, 5, 7, lex, started)

	rates := []float64{
		max(0, (8.0-4.0/3.0)/12.0),
		max(0, (5.0-1.0/3.0)/6.0),
		max(0, (5.0-7.0/3.0)/12.0),
	}
	body, err := json.Marshal(map[string]any{
		"v": 3, "kind": "initial",
		"listening": map[string]any{"tested": true, "level": 2, "estimate": 500},
		"reading":   map[string]any{"tested": false, "level": 0, "estimate": 0},
		"bands": []map[string]any{
			{"band": 0, "rated": 12, "good": 8, "again": 4, "good_plus_rate": rates[0]},
			{"band": 1, "rated": 6, "good": 5, "again": 1, "good_plus_rate": rates[1]},
			{"band": 2, "rated": 12, "good": 5, "again": 7, "good_plus_rate": rates[2]},
		},
		"estimated_known_words": 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(ctx, runID, finishedAt, 1, "band_stop", string(body)); err != nil {
		t.Fatal(err)
	}
}

func claimAgreePlacement(t *testing.T, store *db.Store, runID int64, seq int, part, skill string, band int, itemID, outcome string, lexemeID int64, at time.Time) {
	t.Helper()
	ctx := context.Background()
	it := db.PlacementItem{
		RunID: runID, Seq: seq, Part: part, Skill: skill, Band: band, ItemID: itemID,
		Options: `{"options":[]}`, ShownAt: at,
	}
	if err := store.InsertPlacementItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	ok, err := store.ClaimPlacementItem(ctx, runID, seq, outcome, "", lexemeID, at, 0)
	if err != nil || !ok {
		t.Fatalf("claim %s: ok=%v err=%v", itemID, ok, err)
	}
}

func seedListenBand(t *testing.T, store *db.Store, runID int64, seq *int, band, right, wrong int, lex int64, at time.Time) {
	t.Helper()
	for i := 0; i < right; i++ {
		claimAgreePlacement(t, store, runID, *seq, db.PartListening, "listen", band, fmt.Sprintf("r-b%d-%d", band, i), "right", lex, at)
		*seq++
	}
	for i := 0; i < wrong; i++ {
		claimAgreePlacement(t, store, runID, *seq, db.PartListening, "listen", band, fmt.Sprintf("w-b%d-%d", band, i), "wrong", lex, at)
		*seq++
	}
}

func mustZone(t *testing.T, name string) studyday.Zone {
	t.Helper()
	loc, err := studyday.ParseZone(name)
	if err != nil {
		t.Fatal(err)
	}
	return studyday.Zone{Name: name, Loc: loc, Source: studyday.SourceLearner}
}

func civilAdd(day string, delta int) string {
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return d.AddDate(0, 0, delta).Format("2006-01-02")
}
