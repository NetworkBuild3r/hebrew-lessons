package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html"
	mrand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The placement screens (INIT-015/SPEC-012), over HTTP, on the real v3 engine and a real store. Every
// clip is a row in audio_assets; no audio is generated and no provider is called.

type placedUser struct {
	id     int64
	csrf   string
	cookie *http.Cookie
	// origin is where the user posts from. Empty is the tests' https://learn.example; the browser tests
	// set the origin of the server they start.
	origin string
}

// constSource gives every run the same seed, so two learners are dealt the same items.
type constSource uint64

func (c constSource) Uint64() uint64 { return uint64(c) }

func newPlacementRouter(t *testing.T) (http.Handler, *db.Store, placedUser, placedUser) {
	t.Helper()
	return newPlacementRouterWith(t, mrand.NewPCG(1, 7))
}

func newPlacementRouterWith(t *testing.T, src mrand.Source) (http.Handler, *db.Store, placedUser, placedUser) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedAudio(t, store)
	a := insertPlacedUser(t, store, "a@example.com", "sub-a", "csrf-a", now)
	b := insertPlacedUser(t, store, "b@example.com", "sub-b", "csrf-b", now)
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"a@example.com", "b@example.com", "c@example.com", "d@example.com", "u0@example.com", "u1@example.com", "u2@example.com", "u3@example.com", "u4@example.com", "u5@example.com", "u6@example.com", "u7@example.com", "u8@example.com", "u9@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := placement.NewEngine(store, evidence.New(store, fsrs.New(store, clk), clk), clk, placement.WithSource(src))
	return NewWithPlacement(nil, svc, engine, store), store, a, b
}

func insertPlacedUser(t *testing.T, store *db.Store, email, sub, csrf string, now time.Time) placedUser {
	t.Helper()
	user, err := store.CreateUser(context.Background(), email, sub, auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(id)
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), user.ID, csrf, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	raw := base64.RawURLEncoding.EncodeToString(id)
	return placedUser{id: user.ID, csrf: csrf, cookie: &http.Cookie{Name: "hebrew_session", Value: raw}}
}

// seedAudio gives every word of the deck, and every letter syllable, a clip row.
func seedAudio(t *testing.T, store *db.Store) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "placement", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		Text string `json:"text_nfc"`
	}
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, it := range items {
		texts = append(texts, it.Text)
	}
	letters, err := placement.Letters()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range letters {
		texts = append(texts, l.SyllableText)
	}
	for _, text := range texts {
		sum := sha256.Sum256([]byte("placement-audio:" + text))
		if _, err := store.InsertAudio(context.Background(), db.Asset{
			ID:      hex.EncodeToString(sum[:]),
			TextNFC: audio.NormalizeText(text),
			Voice:   "test",
			Model:   "test",
			Bytes:   1,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func (u placedUser) post(t *testing.T, h http.Handler, path string, form url.Values, token string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(form.Encode())
	req := httptest.NewRequest(http.MethodPost, path, body)
	origin := u.origin
	if origin == "" {
		origin = "https://learn.example"
	}
	req.Host = strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(u.cookie)
	if token != "" {
		req.Header.Set("X-CSRF-Token", token)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func hidden(t *testing.T, body, name string) string {
	t.Helper()
	re := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("hidden %s missing in %s", name, body)
	}
	return m[1]
}

// ---- a scripted learner over HTTP -------------------------------------------------------------

var (
	screenRE = regexp.MustCompile(`data-screen="([a-z]+)"`)
	phaseRE  = regexp.MustCompile(`data-phase="([a-z]+)"`)
)

func screenOf(body string) string {
	if m := screenRE.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}

func phaseOf(body string) string {
	if m := phaseRE.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}

// plScript says how the learner answers. letters is how many of the six letters are right. pick says
// what a word item gets: "right", "wrong", or "idk". typing is "take" or "skip" at the offer.
// typingRight types the word correctly on every typing card. Without it, only the first is right.
type plScript struct {
	letters     int
	pick        func(it db.PlacementItem, n int) string
	typing      string
	typingRight bool
}

// plShot is one response the server sent.
type plShot struct {
	path string
	body string
}

type plDriver struct {
	t       *testing.T
	h       http.Handler
	store   *db.Store
	u       placedUser
	body    string
	log     []plShot
	letters int
	words   int
	typed   int
	// plain posts with no HX-Request header, the way a browser with JavaScript off does.
	// The handler then returns the whole page. Leave it false for the htmx fragment path.
	plain bool
}

func newDriver(t *testing.T, h http.Handler, store *db.Store, u placedUser) *plDriver {
	return &plDriver{t: t, h: h, store: store, u: u}
}

func (d *plDriver) post(path string, form url.Values) string {
	d.t.Helper()
	form.Set("_csrf", d.u.csrf)
	rec := d.u.post(d.t, d.h, path, form, "", !d.plain)
	if rec.Code != http.StatusOK {
		d.t.Fatalf("POST %s %v = %d %s", path, form, rec.Code, rec.Body.String())
	}
	d.body = rec.Body.String()
	d.log = append(d.log, plShot{path: path, body: d.body})
	return d.body
}

func (d *plDriver) runID() string { return hidden(d.t, d.body, "run_id") }

type plOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Item string `json:"item"`
}

// open is the run's open item in the store, and the id of its right option. The test reads the answer
// where only the server keeps it.
func (d *plDriver) open() (db.PlacementItem, string, []plOption) {
	d.t.Helper()
	id, err := strconv.ParseInt(d.runID(), 10, 64)
	if err != nil {
		d.t.Fatal(err)
	}
	items, err := d.store.ListPlacementItems(context.Background(), id)
	if err != nil {
		d.t.Fatal(err)
	}
	for _, it := range items {
		if it.Outcome != "" {
			continue
		}
		var stored struct {
			Options []plOption `json:"options"`
		}
		if err := json.Unmarshal([]byte(it.Options), &stored); err != nil {
			d.t.Fatal(err)
		}
		right := ""
		if it.CorrectIdx.Valid {
			right = stored.Options[it.CorrectIdx.Int64].ID
		}
		return it, right, stored.Options
	}
	d.t.Fatal("the run has no open item")
	return db.PlacementItem{}, "", nil
}

// answer answers the open item as the script says, and returns what the server sent back.
func (d *plDriver) answer(sc plScript) string {
	d.t.Helper()
	it, right, opts := d.open()
	form := url.Values{"run_id": {d.runID()}, "item": {hidden(d.t, d.body, "item")}}
	if want := "p" + strconv.Itoa(it.Seq); form.Get("item") != want {
		d.t.Fatalf("the screen offers %s, the open item is %s", form.Get("item"), want)
	}
	switch it.Part {
	case "typing":
		w, _, _ := placement.Lookup(it.ItemID)
		d.typed++
		if sc.typingRight {
			form.Set("typed", unpointedHebrew(w.TextNFC))
			break
		}
		switch d.typed {
		case 1:
			form.Set("typed", unpointedHebrew(w.TextNFC))
		case 2:
			form.Set("dont_know", "1")
		default:
			form.Set("typed", "<i>nope</i>")
		}
	default:
		action := "right"
		if it.Part == "letters" {
			d.letters++
			if d.letters > sc.letters {
				action = "wrong"
			}
		} else {
			d.words++
			action = sc.pick(it, d.words)
		}
		switch action {
		case "right":
			form.Set("option", right)
		case "idk":
			form.Set("dont_know", "1")
		default:
			for _, o := range opts {
				if o.ID != right {
					form.Set("option", o.ID)
					break
				}
			}
		}
	}
	return d.post("/_hx/placement/answer", form)
}

func unpointedHebrew(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x0591 && r <= 0x05C7 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// run takes the whole test and returns the results screen.
func (d *plDriver) run(sc plScript) string {
	d.t.Helper()
	d.post("/_hx/placement/start", url.Values{})
	for i := 0; i < 600; i++ {
		switch screen := screenOf(d.body); screen {
		case views.PlaceResults:
			return d.body
		case views.PlaceLetters, views.PlaceListening, views.PlaceReading:
			if phaseOf(d.body) == views.PlacePicked {
				d.post("/_hx/placement/next", url.Values{"run_id": {d.runID()}})
				continue
			}
			d.answer(sc)
		case views.PlaceTyping:
			d.answer(sc)
		case views.PlaceBreak:
			d.post("/_hx/placement/next", url.Values{"run_id": {d.runID()}, "cont": {"1"}})
		case views.PlaceOffer:
			path := "/_hx/placement/typing"
			if sc.typing == "skip" {
				path = "/_hx/placement/finish"
			}
			d.post(path, url.Values{"run_id": {d.runID()}})
		default:
			d.t.Fatalf("unexpected screen %q: %s", screen, d.body)
		}
	}
	d.t.Fatal("the run did not finish")
	return ""
}

// toScreen answers the letters right and moves on until the wanted screen is up, asking.
func (d *plDriver) toScreen(want string) {
	d.t.Helper()
	d.post("/_hx/placement/start", url.Values{})
	sc := plScript{letters: 6, pick: func(db.PlacementItem, int) string { return "right" }}
	for i := 0; i < 400; i++ {
		screen := screenOf(d.body)
		if screen == want && phaseOf(d.body) != views.PlacePicked {
			return
		}
		switch screen {
		case views.PlaceLetters, views.PlaceListening, views.PlaceReading:
			if phaseOf(d.body) == views.PlacePicked {
				d.post("/_hx/placement/next", url.Values{"run_id": {d.runID()}})
			} else {
				d.answer(sc)
			}
		case views.PlaceBreak:
			d.post("/_hx/placement/next", url.Values{"run_id": {d.runID()}, "cont": {"1"}})
		default:
			d.t.Fatalf("never reached %s: %s", want, screen)
		}
	}
	d.t.Fatalf("never reached %s", want)
}

// A learner who knows the first two bands and guesses the rest wrong, says "I don't know" now and then,
// and takes the typing part. Every kind of screen appears in that run.
func mixedScript() plScript {
	return plScript{
		letters: 5,
		typing:  "take",
		pick: func(it db.PlacementItem, n int) string {
			switch {
			case n%7 == 0:
				return "idk"
			case it.Band <= 2:
				return "right"
			}
			return "wrong"
		},
	}
}

func countMatches(re *regexp.Regexp, s string) int { return len(re.FindAllString(s, -1)) }

var (
	primaryRE = regexp.MustCompile(`data-kind="primary"`)
	orbRE     = regexp.MustCompile(`data-gold=""`)
	hebrewRE  = regexp.MustCompile(`[\x{0590}-\x{05FF}]`)
)

// ---- ac-1, ac-2, ac-3: a full scripted run ----------------------------------------------------

func TestPlacementFullRun(t *testing.T) {
	if views.PlaceOptionCount != placement.OptionCount {
		t.Fatalf("views.PlaceOptionCount = %d, the engine deals %d options", views.PlaceOptionCount, placement.OptionCount)
	}
	h, store, a, _ := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	results := d.run(mixedScript())

	seen := map[string]bool{}
	for i, shot := range d.log {
		screen, phase := screenOf(shot.body), phaseOf(shot.body)
		seen[screen] = true
		if phase == views.PlacePicked {
			seen["picked"] = true
		}
		where := "response " + strconv.Itoa(i) + " (" + shot.path + ", " + screen + ")"
		primary, orb := countMatches(primaryRE, shot.body), countMatches(orbRE, shot.body)

		// ac-1: one gold action. A pick screen's gold is its audio orb; the answers are its action.
		switch screen {
		case views.PlaceLetters, views.PlaceListening:
			if primary != 0 || orb != 1 {
				t.Errorf("%s: %d primary buttons and %d gold orbs, want 0 and 1", where, primary, orb)
			}
		case views.PlaceReading:
			if primary != 0 || orb != 0 {
				t.Errorf("%s: %d primary buttons and %d gold orbs, want 0 and 0", where, primary, orb)
			}
		default:
			if primary != 1 || orb != 0 {
				t.Errorf("%s: %d primary buttons and %d gold orbs, want 1 and 0", where, primary, orb)
			}
		}
		if primary+orb > 1 {
			t.Errorf("%s: %d gold actions", where, primary+orb)
		}

		// SPEC-009: a screen that is tapped at once runs no view transition, or the first tap is lost.
		switch screen {
		case views.PlaceLetters, views.PlaceListening, views.PlaceReading, views.PlaceTyping:
			if !strings.Contains(shot.body, `class="c-answer"`) && !strings.Contains(shot.body, "data-no-transition") {
				t.Errorf("%s would run a view transition over its first tap", where)
			}
			if screen != views.PlaceTyping {
				if got := strings.Count(shot.body, `class="c-answer"`); got != views.PlaceOptionCount {
					t.Errorf("%s has %d tappable options of class c-answer, want %d", where, got, views.PlaceOptionCount)
				}
			}
		}

		// ac-2: nothing before the results is a verdict.
		if screen != views.PlaceResults {
			for _, bad := range []string{`data-state="correct"`, `data-state="wrong"`, "--go", "--stop", "feedback", "c-sheet", "c-answer__mark", `class="fb`} {
				if strings.Contains(shot.body, bad) {
					t.Errorf("%s shows %q before the results", where, bad)
				}
			}
		}

		// ac-3: listening has no Hebrew text node; reading has no transliteration (and no clip).
		switch screen {
		case views.PlaceListening:
			if hebrewRE.MatchString(shot.body) || strings.Contains(shot.body, `lang="he`) {
				t.Errorf("%s carries Hebrew", where)
			}
			if phase == views.PlaceAsk && !strings.Contains(shot.body, "<audio ") {
				t.Errorf("%s has no clip", where)
			}
		case views.PlaceReading:
			for _, bad := range []string{"he-Latn", "wb__tr", "translit", "<audio", "data-pl-play"} {
				if strings.Contains(shot.body, bad) {
					t.Errorf("%s carries %q", where, bad)
				}
			}
			if !strings.Contains(shot.body, `lang="he" dir="rtl"`) {
				t.Errorf("%s has no Hebrew node with lang and dir", where)
			}
		}
	}
	for _, want := range []string{views.PlaceLetters, views.PlaceListening, views.PlaceReading, views.PlaceBreak, views.PlaceOffer, views.PlaceTyping, views.PlaceResults, "picked"} {
		if !seen[want] {
			t.Errorf("the run never showed a %s screen", want)
		}
	}

	// The results: one gold way into the first lesson, the levels, and a count of misses that is true.
	misses := dbMisses(t, store, a.id)
	if len(misses) == 0 {
		t.Fatal("the script left no misses to review")
	}
	for _, want := range []string{"Your starting point", "Start my first lesson", "Listening", "Reading", "We’ll work on first", "Review my " + strconv.Itoa(len(misses)) + " misses", `class="pl__back" href="/">`} {
		if !strings.Contains(results, want) {
			t.Errorf("results are missing %q", want)
		}
	}
	if countMatches(primaryRE, results) != 1 {
		t.Errorf("results have %d primary buttons", countMatches(primaryRE, results))
	}
	if strings.Count(results, `class="pl-meter"`) != 2 {
		t.Error("results are missing a level meter")
	}
	page := getSignedIn(t, h, a.cookie, "/placement")
	if screenOf(page) != views.PlaceResults || !strings.Contains(page, "Start my first lesson") {
		t.Errorf("GET /placement after the run shows %q", screenOf(page))
	}
	if today := getSignedIn(t, h, a.cookie, "/"); strings.Contains(today, "Resume placement") || strings.Contains(today, views.PlacementFor("en").TakeTitle) {
		t.Error("a finished run still offers placement on Today")
	}
}

// dbMisses is the word items the learner did not get right, from the store.
func dbMisses(t *testing.T, store *db.Store, userID int64) []db.PlacementItem {
	t.Helper()
	run, err := store.LatestRun(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.ListPlacementItems(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var out []db.PlacementItem
	for _, it := range items {
		if it.Part != "letters" && it.Outcome != "" && it.Outcome != placement.OutcomeRight {
			out = append(out, it)
		}
	}
	return out
}

func TestPlacementLettersFailedSkipsReading(t *testing.T) {
	h, store, a, _ := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	results := d.run(plScript{letters: 2, typing: "skip", pick: func(db.PlacementItem, int) string { return "right" }})
	for _, shot := range d.log {
		if s := screenOf(shot.body); s == views.PlaceReading || s == views.PlaceOffer || s == views.PlaceBreak {
			t.Fatalf("a learner who failed the letters was shown %s", s)
		}
	}
	if !strings.Contains(results, "Letters first, then reading.") || !strings.Contains(results, "Not tested yet") {
		t.Fatalf("results do not say reading was not tested: %s", results)
	}
}

// ---- D1: the reply to an answer is the same whatever the answer was ---------------------------

func plNormalize(body, runID, csrf string) string {
	body = strings.ReplaceAll(body, `name="run_id" value="`+runID+`"`, `name="run_id" value="RUN"`)
	body = strings.ReplaceAll(body, `name="_csrf" value="`+csrf+`"`, `name="_csrf" value="CSRF"`)
	body = strings.ReplaceAll(body, ` data-state="selected"`, ` data-state="default"`)
	body = strings.ReplaceAll(body, ` data-dim=""`, "")
	body = strings.ReplaceAll(body, ` aria-pressed="true"`, "")
	return body
}

func TestPlacementPickedReplyIsTheSameWhateverTheAnswer(t *testing.T) {
	h, store, _, _ := newPlacementRouterWith(t, constSource(99))
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for s, screen := range []string{views.PlaceLetters, views.PlaceListening} {
		var users []placedUser
		for i := 0; i < 5; i++ {
			name := "u" + strconv.Itoa(s*5+i)
			users = append(users, insertPlacedUser(t, store, name+"@example.com", "sub-"+name, "csrf-"+name, now))
		}
		var normal []string
		var rightPicks int
		for i, u := range users {
			d := newDriver(t, h, store, u)
			if screen == views.PlaceLetters {
				d.post("/_hx/placement/start", url.Values{})
			} else {
				d.toScreen(views.PlaceListening)
			}
			it, right, opts := d.open()
			form := url.Values{"run_id": {d.runID()}, "item": {hidden(t, d.body, "item")}}
			if i < 4 {
				form.Set("option", opts[i].ID)
				if opts[i].ID == right {
					rightPicks++
				}
			} else {
				form.Set("dont_know", "1")
			}
			if it.Part != screen {
				t.Fatalf("expected a %s item, got %s", screen, it.Part)
			}
			reply := d.post("/_hx/placement/answer", form)
			if phaseOf(reply) != views.PlacePicked {
				t.Fatalf("the reply is not the picked screen: %s", reply)
			}
			normal = append(normal, plNormalize(reply, d.runID(), u.csrf))
		}
		if rightPicks != 1 {
			t.Fatalf("%s: %d of the four picks were right, want exactly one", screen, rightPicks)
		}
		for i := 1; i < len(normal); i++ {
			if normal[i] != normal[0] {
				t.Errorf("%s: the reply to pick %d differs from the reply to pick 0 beyond the chosen option:\n%s", screen, i, firstDifference(normal[0], normal[i]))
			}
		}
	}
}

// boostedGet is a GET the way htmx boost sends it.
func boostedGet(t *testing.T, h http.Handler, cookie *http.Cookie, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("boosted GET %s = %d", path, rec.Code)
	}
	return rec.Body.String()
}

// firstDifference shows where two bodies part, with a little before and after.
func firstDifference(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	from := max(0, i-60)
	return "...\n" + a[from:min(len(a), i+80)] + "\n--- vs ---\n" + b[from:min(len(b), i+80)]
}

func TestPlacementPickedScreenShowsOnlyTheChoice(t *testing.T) {
	h, store, a, _ := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	d.toScreen(views.PlaceListening)
	asked := d.body
	_, _, opts := d.open()
	reply := d.post("/_hx/placement/answer", url.Values{"run_id": {d.runID()}, "item": {hidden(t, d.body, "item")}, "option": {opts[2].ID}})
	if got := strings.Count(reply, `data-state="selected"`); got != 1 {
		t.Fatalf("%d options are Selected, want 1", got)
	}
	if got := strings.Count(reply, ` data-dim=""`); got != 3 {
		t.Fatalf("%d options are dimmed, want 3", got)
	}
	for _, want := range []string{"Answer saved. Next word coming.", `data-advance="800"`, `hx-post="/_hx/placement/next"`} {
		if !strings.Contains(reply, want) {
			t.Errorf("the picked screen is missing %q", want)
		}
	}
	if strings.Contains(reply, "<audio") || strings.Contains(reply, `name="option"`) {
		t.Error("the picked screen still plays a clip or takes an answer")
	}
	if strings.Contains(asked, "Answer saved") {
		t.Error("the question already says the answer is saved")
	}
}

// ---- ac-4, ac-5 markup ---------------------------------------------------------------------------

func TestPlacementListeningMarkup(t *testing.T) {
	h, store, a, _ := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	d.toScreen(views.PlaceListening)
	body := d.body
	if !regexp.MustCompile(`<audio [^>]*\sautoplay[\s>]`).MatchString(body) {
		t.Error("the listening <audio> has no autoplay")
	}
	if !strings.Contains(body, `data-pl-replace`) || !strings.Contains(body, `action="/_hx/placement/replace"`) {
		t.Error("the screen has no way to ask for a replacement clip")
	}
	// ac-5: a key for each option, "I don't know", and focus order that follows the screen.
	for i := 1; i <= 4; i++ {
		if !strings.Contains(body, `data-hotkey="`+strconv.Itoa(i)+`"`) || !strings.Contains(body, `aria-keyshortcuts="`+strconv.Itoa(i)+`"`) {
			t.Errorf("option %d has no key", i)
		}
	}
	order := []string{"data-pl-close", "data-pl-play", "data-pl-replay", "data-pl-slower", `value="o0"`, `value="o1"`, `value="o2"`, `value="o3"`, `name="dont_know"`}
	last := -1
	for _, marker := range order {
		at := strings.Index(body, marker)
		if at < 0 {
			t.Fatalf("%s is missing", marker)
		}
		if at < last {
			t.Errorf("%s comes before the control above it in the screen", marker)
		}
		last = at
	}
	if regexp.MustCompile(`tabindex="[1-9]`).MatchString(body) {
		t.Error("a positive tabindex breaks the focus order")
	}
	if !strings.Contains(body, `aria-label="Play the word"`) || !strings.Contains(body, `data-pl-play`) {
		t.Error("the orb has no accessible name")
	}
	js, err := os.ReadFile("static/js/placement.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`/^[1-4]$/`, `k === "?"`, `k === " "`, "autoplay", "NotAllowedError", "Tap to listen", "0.75", "form[data-pl-replace]"} {
		if !strings.Contains(string(js), want) {
			t.Errorf("placement.js does not handle %s", want)
		}
	}
}

func TestPlacementReadingMarkup(t *testing.T) {
	h, store, a, _ := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	d.toScreen(views.PlaceReading)
	it, _, _ := d.open()
	w, _, _ := placement.Lookup(it.ItemID)
	if !strings.Contains(d.body, `<bdi lang="he" dir="rtl">`+html.EscapeString(w.TextNFC)+`</bdi>`) {
		t.Errorf("the pointed word is not in a bdi with lang and dir: %s", d.body)
	}
	if w.Translit != "" && strings.Contains(d.body, html.EscapeString(w.Translit)) {
		t.Error("the reading screen shows the transliteration")
	}
	if !strings.Contains(d.body, "No sound in this part") {
		t.Error("the reading screen does not say it has no sound")
	}
}

// ---- ac-4: a clip that will not play is replaced by the server ------------------------------------

func TestPlacementReplacesABrokenClip(t *testing.T) {
	h, store, a, b := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	d.toScreen(views.PlaceListening)
	before := d.body
	item, _, _ := d.open()
	token := hidden(t, before, "item")
	clipBefore := regexp.MustCompile(`<audio [^>]*src="([^"]+)"`).FindStringSubmatch(before)[1]
	attemptsBefore := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, a.id)
	itemsBefore := countRows(t, store, `SELECT COUNT(*) FROM placement_items WHERE run_id = ?`, mustInt(t, d.runID()))

	// Someone else cannot swap a clip in this run.
	if rec := b.post(t, h, "/_hx/placement/replace", url.Values{"run_id": {d.runID()}, "item": {token}, "tries": {"1"}, "_csrf": {b.csrf}}, "", true); rec.Code != http.StatusNotFound {
		t.Fatalf("another learner's replace = %d", rec.Code)
	}

	got := d.post("/_hx/placement/replace", url.Values{"run_id": {d.runID()}, "item": {token}, "tries": {"1"}})
	if screenOf(got) != views.PlaceListening || phaseOf(got) != views.PlaceAsk {
		t.Fatalf("replace did not return a listening question: %s", got)
	}
	clipAfter := regexp.MustCompile(`<audio [^>]*src="([^"]+)"`).FindStringSubmatch(got)[1]
	if clipAfter == clipBefore {
		t.Error("the replacement plays the same clip")
	}
	if hidden(t, got, "item") != token {
		t.Error("the replacement changed the item id")
	}
	now, _, _ := d.open()
	if now.ItemID == item.ItemID || now.Seq != item.Seq || now.Outcome != "" {
		t.Errorf("the open item was not swapped in place: %+v then %+v", item, now)
	}
	if !strings.Contains(now.Options, `"skipped":["`+item.ItemID+`"]`) {
		t.Errorf("the swapped-out word is not remembered: %s", now.Options)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM placement_items WHERE run_id = ?`, mustInt(t, d.runID())); n != itemsBefore {
		t.Errorf("the run has %d items, was %d", n, itemsBefore)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, a.id); n != attemptsBefore {
		t.Errorf("a replaced clip wrote %d attempts", n-attemptsBefore)
	}
	if !strings.Contains(got, `name="tries" value="2"`) {
		t.Error("the replacement does not count its tries")
	}

	// After the last try the screen stays and says so. It never loops.
	last := d.post("/_hx/placement/replace", url.Values{"run_id": {d.runID()}, "item": {token}, "tries": {strconv.Itoa(views.PlaceMaxReplaceTries)}})
	if strings.Contains(last, "data-pl-replace") {
		t.Error("the last replacement still offers another")
	}
}

func TestPlacementNoReplacementSaysSo(t *testing.T) {
	h, store, a, _ := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	// A letters clip has no spare: the item stays, and the screen says its clip is broken.
	d.post("/_hx/placement/start", url.Values{})
	token := hidden(t, d.body, "item")
	if strings.Contains(d.body, "data-pl-replace") {
		t.Error("a letters screen offers a replacement it cannot make")
	}
	got := d.post("/_hx/placement/replace", url.Values{"run_id": {d.runID()}, "item": {token}, "tries": {"1"}})
	if hidden(t, got, "item") != token || screenOf(got) != views.PlaceLetters {
		t.Fatalf("the letters item changed: %s", got)
	}
	if !strings.Contains(got, `data-audio-state="error"`) || !strings.Contains(got, "We couldn’t play this clip.") || strings.Contains(got, "data-pl-replace") {
		t.Errorf("the screen does not say the clip is broken: %s", got)
	}
}

func countRows(t *testing.T, store *db.Store, query string, args ...any) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// ---- the intro, the gate, CSRF, ownership ---------------------------------------------------------

func TestPlacementGateAndIntro(t *testing.T) {
	h, store, a, _ := newPlacementRouter(t)
	en := views.PlacementFor("en")

	anon := httptest.NewRecorder()
	h.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, "/placement", nil))
	if anon.Code != http.StatusSeeOther || anon.Header().Get("Location") != "/signin" {
		t.Fatalf("signed-out GET = %d %q", anon.Code, anon.Header().Get("Location"))
	}
	if strings.Contains(anon.Body.String(), en.Heading) {
		t.Fatal("signed-out GET rendered the page")
	}
	hxReq := httptest.NewRequest(http.MethodGet, "/placement", nil)
	hxReq.Header.Set("HX-Request", "true")
	hx := httptest.NewRecorder()
	h.ServeHTTP(hx, hxReq)
	if hx.Code != http.StatusUnauthorized || hx.Header().Get("HX-Redirect") != "/signin" || strings.Contains(hx.Body.String(), `id="placement"`) {
		t.Fatalf("htmx GET = %d %q", hx.Code, hx.Header().Get("HX-Redirect"))
	}
	for _, path := range []string{"/_hx/placement/answer", "/_hx/placement/next", "/_hx/placement/start"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d; reads do not live under /_hx/", path, rec.Code)
		}
	}

	page := getSignedIn(t, h, a.cookie, "/placement")
	for _, want := range []string{
		`<html lang="en" dir="ltr"`, html.EscapeString(en.Heading), "about 10–12 minutes", "Headphones help",
		"Listening", "Reading", "Typing", `hx-post="/_hx/placement/start"`, `hx-select="unset"`,
		`hx-swap="outerHTML transition:true"`, `src="/static/js/placement.js"`, `href="/static/css/placement.css"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the intro is missing %q", want)
		}
	}
	if screenOf(page) != views.PlaceIntro || countMatches(primaryRE, page) != 1 {
		t.Errorf("the intro is %q with %d primary buttons", screenOf(page), countMatches(primaryRE, page))
	}
	if strings.Contains(page, `aria-current="page"`) {
		t.Error("placement should not mark a dock item current")
	}
	run, err := store.LatestRun(context.Background(), a.id)
	if err == nil && run.ID != 0 {
		t.Error("GET /placement opened a run")
	}

	// A boosted GET sends HX-Request, and the body selects #app out of the answer, so a GET is always a page.
	if body := boostedGet(t, h, a.cookie, "/placement"); !strings.Contains(body, `<div id="app">`) {
		t.Error("a boosted GET /placement is not a whole page")
	}

	frag := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", true)
	if frag.Code != http.StatusOK || strings.Contains(frag.Body.String(), "<html") || !strings.Contains(frag.Body.String(), `id="placement"`) {
		t.Fatalf("an htmx start is not a fragment: %d", frag.Code)
	}
	full := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", false)
	if full.Code != http.StatusOK || !strings.Contains(full.Body.String(), "<html") {
		t.Fatalf("a plain start is not a page: %d", full.Code)
	}
}

func TestPlacementCSRF(t *testing.T) {
	h, store, a, _ := newPlacementRouter(t)
	for _, path := range []string{"/_hx/placement/start", "/_hx/placement/answer", "/_hx/placement/next", "/_hx/placement/typing", "/_hx/placement/finish", "/_hx/placement/replace"} {
		if rec := a.post(t, h, path, url.Values{}, "", false); rec.Code != http.StatusForbidden {
			t.Errorf("%s without a token = %d", path, rec.Code)
		}
		if rec := a.post(t, h, path, url.Values{}, "not-the-token", false); rec.Code != http.StatusForbidden {
			t.Errorf("%s with a bad token = %d", path, rec.Code)
		}
	}
	foreign := httptest.NewRequest(http.MethodPost, "/_hx/placement/start", strings.NewReader(""))
	foreign.Host = "learn.example"
	foreign.Header.Set("Origin", "https://evil.example")
	foreign.Header.Set("X-CSRF-Token", a.csrf)
	foreign.AddCookie(a.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, foreign)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a foreign origin = %d", rec.Code)
	}
	if run, err := store.LatestRun(context.Background(), a.id); err == nil && run.ID != 0 {
		t.Error("a rejected write opened a run")
	}
	// The token in the header works, as it does for the htmx body.
	ok := a.post(t, h, "/_hx/placement/start", url.Values{}, a.csrf, true)
	if ok.Code != http.StatusOK {
		t.Errorf("header token = %d", ok.Code)
	}
	// Every form on a screen carries the token the app already uses.
	if !strings.Contains(ok.Body.String(), `name="_csrf" value="`+a.csrf+`"`) {
		t.Error("the screen's form has no CSRF field")
	}
}

func TestPlacementOwnershipAndStaleAnswers(t *testing.T) {
	h, store, a, b := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	d.post("/_hx/placement/start", url.Values{})
	runID, token := d.runID(), hidden(t, d.body, "item")
	_, right, _ := d.open()

	for _, path := range []string{"/_hx/placement/answer", "/_hx/placement/next", "/_hx/placement/typing", "/_hx/placement/finish", "/_hx/placement/replace"} {
		rec := b.post(t, h, path, url.Values{"run_id": {runID}, "item": {token}, "option": {right}, "_csrf": {b.csrf}}, "", true)
		if rec.Code != http.StatusNotFound {
			t.Errorf("another learner's %s = %d, want 404", path, rec.Code)
		}
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM placement_items WHERE run_id = ? AND outcome IS NOT NULL`, mustInt(t, runID)); n != 0 {
		t.Fatalf("a forbidden answer was stored (%d)", n)
	}

	// A stale or made-up item id shows where the run is. It stores nothing and says nothing about the answer.
	got := d.post("/_hx/placement/answer", url.Values{"run_id": {runID}, "item": {"p99"}, "option": {right}})
	if screenOf(got) != views.PlaceLetters || phaseOf(got) != views.PlaceAsk || hidden(t, got, "item") != token {
		t.Errorf("a stale post did not show the open item: %s", got)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM placement_items WHERE run_id = ? AND outcome IS NOT NULL`, mustInt(t, runID)); n != 0 {
		t.Errorf("a stale answer was stored (%d)", n)
	}
	// An option the item never offered is refused.
	if rec := a.post(t, h, "/_hx/placement/answer", url.Values{"run_id": {runID}, "item": {token}, "option": {"o9"}, "_csrf": {a.csrf}}, "", true); rec.Code != http.StatusBadRequest {
		t.Errorf("an unknown option = %d", rec.Code)
	}
	// Two answers at once store one: the second post is a stale one.
	first := d.post("/_hx/placement/answer", url.Values{"run_id": {runID}, "item": {token}, "option": {right}})
	second := d.post("/_hx/placement/answer", url.Values{"run_id": {runID}, "item": {token}, "option": {right}})
	if phaseOf(first) != views.PlacePicked || phaseOf(second) == views.PlacePicked {
		t.Errorf("a repeated answer was taken twice")
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM placement_items WHERE run_id = ? AND outcome IS NOT NULL`, mustInt(t, runID)); n != 1 {
		t.Errorf("%d answers stored after a double post, want 1", n)
	}
	// A long typed answer is refused before it reaches the engine.
	if rec := a.post(t, h, "/_hx/placement/answer", url.Values{"run_id": {runID}, "item": {"p2"}, "typed": {strings.Repeat("א", 200)}, "_csrf": {a.csrf}}, "", true); rec.Code != http.StatusBadRequest {
		t.Errorf("a 200-letter answer = %d", rec.Code)
	}
}

// ---- resume and close -----------------------------------------------------------------------------

func TestPlacementResume(t *testing.T) {
	h, store, a, _ := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	d.post("/_hx/placement/start", url.Values{})
	// Opened but unanswered: the intro, not a resume.
	if screenOf(getSignedIn(t, h, a.cookie, "/placement")) != views.PlaceIntro {
		t.Error("a run with no answers did not land on the intro")
	}
	d.answer(mixedScript())
	if progress := getSignedIn(t, h, a.cookie, "/"); !strings.Contains(progress, "Resume placement") {
		t.Error("Today does not offer to resume")
	}
	page := getSignedIn(t, h, a.cookie, "/placement")
	if screenOf(page) != views.PlaceResume || countMatches(primaryRE, page) != 1 || !strings.Contains(page, "Pick up where you left off") {
		t.Fatalf("an open run with answers is %q: %s", screenOf(page), page)
	}
	if strings.Contains(page, "<audio") {
		t.Error("the resume screen plays a clip before the learner asks")
	}
	runID := hidden(t, page, "run_id")
	if runID != d.runID() {
		t.Errorf("resume names run %s, the open run is %s", runID, d.runID())
	}
	back := a.post(t, h, "/_hx/placement/next", url.Values{"run_id": {runID}, "_csrf": {a.csrf}}, "", true)
	if back.Code != http.StatusOK || screenOf(back.Body.String()) != views.PlaceLetters || phaseOf(back.Body.String()) != views.PlaceAsk {
		t.Errorf("Continue did not return the open item: %d %q", back.Code, screenOf(back.Body.String()))
	}
	// Starting again returns the same run. It never opens a second.
	again := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", true)
	if hidden(t, again.Body.String(), "run_id") != runID {
		t.Error("start opened a second run")
	}
	// The close control is a link home, and the dialog says the answers are saved.
	for _, want := range []string{`data-pl-close`, `<dialog class="pl-confirm"`, "Your answers are saved", `data-pl-keep`, "Leave for now"} {
		if !strings.Contains(back.Body.String(), want) {
			t.Errorf("close does not offer %q", want)
		}
	}
}

// ---- ac-6: the review lists every miss ---------------------------------------------------------

func TestPlacementReviewListsEveryMiss(t *testing.T) {
	h, store, a, b := newPlacementRouter(t)
	d := newDriver(t, h, store, a)
	results := d.run(mixedScript())
	_ = results
	misses := dbMisses(t, store, a.id)
	run, _ := store.LatestRun(context.Background(), a.id)
	if !strings.Contains(results, `href="/placement/review?run=`+strconv.FormatInt(run.ID, 10)+`"`) {
		t.Fatalf("results do not link to the review: %s", results)
	}
	page := getSignedIn(t, h, a.cookie, "/placement/review?run="+strconv.FormatInt(run.ID, 10))
	if screenOf(page) != views.PlaceReview {
		t.Fatalf("not the review screen: %s", screenOf(page))
	}
	rows := strings.Count(page, `<li class="pl-miss">`)
	if rows != len(misses) {
		t.Fatalf("the review has %d rows for %d misses", rows, len(misses))
	}
	if !strings.Contains(page, "The "+strconv.Itoa(len(misses))+" words you missed") {
		t.Error("the review does not count its misses")
	}
	var typedMisses int
	for _, m := range misses {
		w, _, _ := placement.Lookup(m.ItemID)
		if !strings.Contains(page, `<bdi lang="he" dir="rtl">`+html.EscapeString(w.TextNFC)+`</bdi>`) {
			t.Errorf("the review is missing the pointed form of %s", m.ItemID)
		}
		if !strings.Contains(page, `<b lang="en">`+html.EscapeString(w.GlossEN)+`</b>`) {
			t.Errorf("the review is missing the gloss of %s", m.ItemID)
		}
		if m.Part == "typing" {
			typedMisses++
		}
	}
	// Each row has a clip that plays and a button for it.
	if got := strings.Count(page, `<audio id="pl-miss-`); got != len(misses) {
		t.Errorf("%d clips for %d misses", got, len(misses))
	}
	if got := strings.Count(page, `data-pl-listen="pl-miss-`); got != len(misses) {
		t.Errorf("%d play buttons for %d misses", got, len(misses))
	}
	if !regexp.MustCompile(`<audio id="pl-miss-1" src="/audio/[0-9a-f]{64}\.mp3" preload="none">`).MatchString(page) {
		t.Error("a review clip is not a /audio/<id>.mp3 source")
	}
	for _, want := range []string{"Listening", "Reading", "Typing", "You said “I don’t know”", "You picked “", "You typed", "Start my first lesson", `class="pl__back" href="/">`} {
		if !strings.Contains(page, want) {
			t.Errorf("the review is missing %q", want)
		}
	}
	if typedMisses == 0 {
		t.Error("the script made no typing miss")
	}
	// What the learner typed is escaped, and is theirs alone.
	if !strings.Contains(page, "&lt;i&gt;nope&lt;/i&gt;") || strings.Contains(page, "<i>nope</i>") {
		t.Error("typed text is not escaped")
	}
	if countMatches(primaryRE, page) != 1 {
		t.Errorf("the review has %d primary buttons", countMatches(primaryRE, page))
	}
	// Boosted from the results, the review is a whole page too, or the swap would be empty.
	if body := boostedGet(t, h, a.cookie, "/placement/review?run="+strconv.FormatInt(run.ID, 10)); !strings.Contains(body, `<div id="app">`) || !strings.Contains(body, `<li class="pl-miss">`) {
		t.Error("a boosted GET of the review is not a whole page")
	}
	// Latest is the default run.
	if latest := getSignedIn(t, h, a.cookie, "/placement/review"); strings.Count(latest, `<li class="pl-miss">`) != len(misses) {
		t.Error("the review with no run is not the latest run")
	}
	// Another learner's run is not found, and an unknown one is too.
	for _, path := range []string{"/placement/review?run=" + strconv.FormatInt(run.ID, 10), "/placement/review?run=99999"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(b.cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("another learner's %s = %d", path, rec.Code)
		}
	}
	// An unfinished run goes back to the placement page.
	e := newDriver(t, h, store, b)
	e.post("/_hx/placement/start", url.Values{})
	req := httptest.NewRequest(http.MethodGet, "/placement/review?run="+e.runID(), nil)
	req.AddCookie(b.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/placement" {
		t.Errorf("an unfinished run's review = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

// ---- Today ------------------------------------------------------------------------------------------

func TestTodayPlacementCards(t *testing.T) {
	h, store, a, b := newPlacementRouter(t)
	en := views.PlacementFor("en")

	none := getSignedIn(t, h, a.cookie, "/")
	if !strings.Contains(none, `class="today-place today-place--open"`) || !strings.Contains(none, en.TakeTitle) {
		t.Fatal("empty placement should offer the check")
	}
	if !strings.Contains(none, `href="/placement"`) || strings.Contains(none, `href="/placement?lang=`) {
		t.Fatal("english placement link should be bare")
	}
	if strings.Contains(none, "Resume placement") {
		t.Fatal("a new learner is offered resume")
	}

	heReq := httptest.NewRequest(http.MethodGet, "/", nil)
	heReq.AddCookie(b.cookie)
	heReq.AddCookie(&http.Cookie{Name: langCookieName, Value: "he"})
	heRec := httptest.NewRecorder()
	h.ServeHTTP(heRec, heReq)
	he := heRec.Body.String()
	if heRec.Code != http.StatusOK || !strings.Contains(he, views.PlacementFor("he").TakeTitle) || !strings.Contains(he, `href="/placement?lang=he"`) {
		t.Fatal("hebrew shell did not translate the placement card")
	}

	d := newDriver(t, h, store, a)
	d.post("/_hx/placement/start", url.Values{})
	d.answer(mixedScript())
	progress := getSignedIn(t, h, a.cookie, "/")
	if !strings.Contains(progress, `class="today-place today-place--resume"`) || !strings.Contains(progress, "Resume placement") || strings.Contains(progress, en.TakeTitle) {
		t.Fatal("an open run should offer resume")
	}

	done := newDriver(t, h, store, b)
	done.run(mixedScript())
	if got := getSignedIn(t, h, b.cookie, "/"); strings.Contains(got, "today-place") || strings.Contains(got, "Resume placement") {
		t.Fatal("finished placement should leave Today alone")
	}
}

// ---- the stylesheet and the script ---------------------------------------------------------------

func TestPlacementCSS(t *testing.T) {
	css := readStatic(t, "static/css/placement.css")
	tokens := readStatic(t, "static/css/tokens.css")
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`--([a-z0-9-]+)\s*:`).FindAllStringSubmatch(tokens, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\(--([a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		if !defined[m[1]] {
			t.Errorf("placement.css uses undefined token --%s", m[1])
		}
	}
	// Placement gives no verdict, so it never paints one.
	for _, bad := range []string{"var(--go", "var(--stop", `data-state="correct"`, `data-state="wrong"`, "feedback"} {
		if strings.Contains(css, bad) {
			t.Errorf("placement.css uses %s", bad)
		}
	}
	for _, want := range []string{".pl .c-answer[data-dim]", `.pl .c-answer[data-state="selected"]`, ".pl-chip", ".pl-confirm", "prefers-reduced-motion: no-preference"} {
		if !strings.Contains(css, want) {
			t.Errorf("placement.css is missing %s", want)
		}
	}
	// The layout loads the stylesheet, and the page loads the script, once each.
	h, _, a, _ := newPlacementRouter(t)
	body := getSignedIn(t, h, a.cookie, "/placement")
	if strings.Count(body, "/static/css/placement.css") != 1 || strings.Count(body, "/static/js/placement.js") != 1 {
		t.Error("the page does not load the placement stylesheet and script once each")
	}
}
