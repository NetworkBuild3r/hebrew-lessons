package web

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// listenClip is one word in the review fixture. speak is what generate-audio
// is given. Empty speak stores no clip. text is what upsert_lexemes is given.
type listenClip struct {
	name     string
	text     string
	speak    string
	gloss    string
	translit string
	weak     string
	due      time.Duration
}

// TestReviewListeningDrillsFollowAudioAssets is issue #120.
// Clips are written by audio.Generator. lexemes.audio_id stays null.
func TestReviewListeningDrillsFollowAudioAssets(t *testing.T) {
	const (
		shalom = "שָׁלוֹם"
		boker  = "בֹּקֶר"
		erev   = "עֶרֶב"
		mayim  = "מַיִם"
		kise   = "כִּסֵּא"
		lechem = "לֶחֶם"
	)
	messy := "\u200e " + norm.NFD.String(shalom) + " \u200f "
	if messy == shalom || units.Normalize(messy) != shalom {
		t.Fatalf("fixture %q normalizes to %q, want %q", messy, units.Normalize(messy), shalom)
	}
	plainLechem := units.Plain(lechem)
	if plainLechem == lechem || units.Normalize(plainLechem) == units.Normalize(lechem) {
		t.Fatalf("plain %q and pointed %q are not distinct NFC keys", plainLechem, lechem)
	}

	words := []listenClip{
		{name: "shalom", text: messy, speak: messy, gloss: "hello", translit: "shalom", weak: fsrs.SkillListen, due: -6 * time.Hour},
		{name: "boker", text: boker, speak: boker, gloss: "morning", translit: "boker", weak: fsrs.SkillListen, due: -5 * time.Hour},
		{name: "erev", text: erev, speak: erev, gloss: "evening", translit: "erev", weak: fsrs.SkillListen, due: -4 * time.Hour},
		{name: "mayim", text: mayim, speak: mayim, gloss: "water", translit: "mayim", weak: fsrs.SkillSpell, due: -3 * time.Hour},
		{name: "kise", text: kise, speak: "", gloss: "chair", translit: "kise", weak: fsrs.SkillListen, due: -2 * time.Hour},
		{name: "lechem", text: lechem, speak: plainLechem, gloss: "bread", translit: "lechem", weak: fsrs.SkillListen, due: -time.Hour},
	}

	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dir := t.TempDir()
	store, err := db.Open(ctx, filepath.Join(dir, "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	audioDir := filepath.Join(dir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var spoken []string
	for _, w := range words {
		if w.speak != "" {
			spoken = append(spoken, w.speak)
		}
	}
	gen := &audio.Generator{
		TTS:   listenTTS{},
		Store: store,
		Dir:   audioDir,
		Voice: audio.DefaultVoiceID,
		Model: audio.DefaultModel,
	}
	generated, err := gen.Generate(ctx, spoken)
	if err != nil {
		t.Fatal(err)
	}
	clipByText := map[string]string{}
	for _, res := range generated {
		if res.Status != audio.StatusGenerated || res.AudioID == "" || res.TextNFC == "" {
			t.Fatalf("generate %+v", res)
		}
		if _, err := os.Stat(filepath.Join(audioDir, res.AudioID+".mp3")); err != nil {
			t.Fatalf("clip file %s: %v", res.AudioID, err)
		}
		clipByText[res.TextNFC] = res.AudioID
	}

	pos := "noun"
	entries := make([]lexicon.Input, len(words))
	for i, w := range words {
		tr := w.translit
		entries[i] = lexicon.Input{TextNFC: w.text, Gloss: w.gloss, Translit: &tr, POS: &pos}
	}
	saved, _, err := store.UpsertLexemes(ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	idByText := map[string]int64{}
	for _, row := range saved {
		idByText[row.TextNFC] = row.ID
	}
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var audioSet int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM lexemes WHERE audio_id IS NOT NULL`).Scan(&audioSet); err != nil {
		t.Fatal(err)
	}
	if audioSet != 0 {
		t.Fatalf("lexemes.audio_id is set on %d rows; the fixture must leave it null", audioSet)
	}

	authSvc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	learner := addLearner(t, store, "a@example.com", "sub-listen", now)
	reviewed := now.Add(-48 * time.Hour)
	cardByLex := map[int64]int64{}
	for _, w := range words {
		text := units.Normalize(w.text)
		id := idByText[text]
		if id == 0 {
			t.Fatalf("no lexeme for %s (%q)", w.name, text)
		}
		cardByLex[id] = addCard(t, store, learner.user.ID, id, fsrs.SkillRead, now.Add(w.due), 1, int(fsrs.Good), reviewed)
		// A skill with only right attempts and no card is "seen", which the mix
		// ranks as weaker than "struggling". A future Again card makes it
		// learning, and it is not due, so it does not take a queue slot.
		// The weak skill stays struggling, with no card of its own.
		for _, skill := range []string{fsrs.SkillListen, fsrs.SkillRecall, fsrs.SkillSpell} {
			if skill == w.weak {
				continue
			}
			addCard(t, store, learner.user.ID, id, skill, now.Add(24*time.Hour), 1, int(fsrs.Again), reviewed)
		}
		listenOutcomes(t, store, fixedClock(now), learner.user.ID, id, w.weak)
	}

	h := NewOptions(nil, authSvc, Options{Store: store, Runs: store, SessionSecret: "listen-audio-test"})
	srv := newServer(store)

	sess, err := srv.study.BuildSession(ctx, learner.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	byLex := map[int64]session.CardRef{}
	var order []session.CardRef
	for {
		ref, ok := sess.Next()
		if !ok {
			break
		}
		if ref.Kind == session.KindLetter || ref.Kind == session.KindPattern {
			continue
		}
		order = append(order, ref)
		byLex[ref.LexemeID] = ref
	}

	wantKind := map[string]activity.Kind{
		shalom: activity.ListenPick,
		boker:  activity.ListenPick,
		erev:   activity.AudioMatch,
		mayim:  activity.Dictation,
	}
	for text, kind := range wantKind {
		ref, ok := byLex[idByText[text]]
		if !ok || ref.Activity != kind {
			t.Errorf("%s activity = %q, want %s; queue=%v", text, ref.Activity, kind, activitiesOf(order))
		}
	}
	for _, text := range []string{kise, lechem} {
		ref := byLex[idByText[text]]
		if isListening(ref.Activity) {
			t.Errorf("%s activity = %q, want no listening drill; queue=%v", text, ref.Activity, activitiesOf(order))
		}
	}
	// The third listening visit is audio_match only when the three listen-weak
	// words with a clip are the first three study cards.
	if len(order) < 3 || order[0].LexemeID != idByText[shalom] || order[1].LexemeID != idByText[boker] || order[2].LexemeID != idByText[erev] {
		t.Fatalf("queue order = %v", activitiesOf(order))
	}

	for _, text := range []string{shalom, boker, erev, mayim} {
		kind := wantKind[text]
		item, err := srv.engine.Build(ctx, learner.user.ID, idByText[text], kind, activity.WithSource(evidence.SourceReview))
		if err != nil {
			t.Fatalf("build %s %s: %v", text, kind, err)
		}
		got := clipIDs(item)
		want := clipByText[text]
		if !got[want] {
			t.Errorf("%s %s clips = %v, want %s", text, kind, got, want)
		}
		for id := range got {
			if id == "" {
				t.Errorf("%s %s has an empty clip id", text, kind)
			}
		}
	}

	// Pointed lechem must not play the unpointed clip. Exact NFC.
	if _, err := srv.engine.Build(ctx, learner.user.ID, idByText[lechem], activity.ListenPick, activity.WithSource(evidence.SourceReview)); !errors.Is(err, activity.ErrNoAudio) {
		t.Fatalf("pointed lechem listen_pick err = %v, want no audio", err)
	}
	if _, err := srv.engine.Build(ctx, learner.user.ID, idByText[kise], activity.Dictation, activity.WithSource(evidence.SourceReview)); !errors.Is(err, activity.ErrNoAudio) {
		t.Fatalf("kise dictation err = %v, want no audio", err)
	}

	// cardVM and the drill use one id. The non-NFC input stored the NFC clip.
	vm, err := srv.cardVM(ctx, &studyQueue{
		has:   true,
		cur:   session.CardRef{CardID: cardByLex[idByText[shalom]], LexemeID: idByText[shalom], Skill: fsrs.SkillRead, Kind: fsrs.SkillRead},
		index: 1,
		total: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "/audio/" + clipByText[shalom] + ".mp3"
	if vm.AudioURL != wantURL {
		t.Fatalf("cardVM audio = %q, want %q", vm.AudioURL, wantURL)
	}
	silent, err := srv.cardVM(ctx, &studyQueue{
		has:   true,
		cur:   session.CardRef{CardID: cardByLex[idByText[kise]], LexemeID: idByText[kise], Skill: fsrs.SkillRead, Kind: fsrs.SkillRead},
		index: 1,
		total: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if silent.AudioURL != "" {
		t.Fatalf("cardVM audio for a word with no clip = %q", silent.AudioURL)
	}
	pointed, err := srv.cardVM(ctx, &studyQueue{
		has: true,
		cur: session.CardRef{CardID: cardByLex[idByText[lechem]], LexemeID: idByText[lechem], Skill: fsrs.SkillRead, Kind: fsrs.SkillRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pointed.AudioURL != "" {
		t.Fatalf("cardVM played the unpointed clip: %q", pointed.AudioURL)
	}

	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-activity") != string(activity.ListenPick) {
		t.Fatalf("first review card is %q, want listen_pick", attr(t, page, "data-activity"))
	}
	m := promptRe.FindStringSubmatch(page)
	if m == nil || "/audio/"+m[1]+".mp3" != wantURL {
		t.Fatalf("review prompt clip = %v, want %s", m, wantURL)
	}

	// The helper and cardVM agree for the same lexeme. AudioID is empty:
	// the column was never written.
	assetID, ok, err := store.ResolveLexemeAudio(ctx, db.LexemeAudio{Text: messy, AudioID: ""})
	if err != nil || !ok || "/audio/"+assetID+".mp3" != vm.AudioURL {
		t.Fatalf("ResolveLexemeAudio = %q ok %v err %v, cardVM %q", assetID, ok, err, vm.AudioURL)
	}
	var nullAudio sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT audio_id FROM lexemes WHERE id = ?`, idByText[shalom]).Scan(&nullAudio); err != nil {
		t.Fatal(err)
	}
	if nullAudio.Valid {
		t.Fatalf("shalom audio_id = %q", nullAudio.String)
	}
}

func listenOutcomes(t *testing.T, store *db.Store, clk fixedClock, userID, lexemeID int64, weak string) {
	t.Helper()
	rec := evidence.New(store, nil, clk)
	for _, skill := range []string{fsrs.SkillListen, fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillSpell} {
		right, wrong := 6, 0
		if skill == weak {
			right, wrong = 1, 5
		}
		for i := 0; i < right+wrong; i++ {
			if _, err := rec.Attempt(context.Background(), evidence.Attempt{
				UserID: userID, LexemeID: lexemeID, Skill: skill, Activity: "story_question",
				Source: evidence.SourceLesson, Prompt: "p", Response: "r", Correct: i < right, SkipSchedule: true,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func isListening(k activity.Kind) bool {
	return k == activity.ListenPick || k == activity.AudioMatch || k == activity.Dictation
}

func activitiesOf(refs []session.CardRef) []string {
	out := make([]string, len(refs))
	for i, ref := range refs {
		out[i] = string(ref.Activity)
	}
	return out
}

func clipIDs(item activity.Item) map[string]bool {
	got := map[string]bool{}
	if item.Prompt.AudioID != "" || item.Kind == activity.ListenPick || item.Kind == activity.Dictation {
		got[item.Prompt.AudioID] = true
	}
	for _, c := range item.Left {
		got[c.AudioID] = true
	}
	return got
}

type listenTTS struct{}

func (listenTTS) Synthesize(_ context.Context, textNFC, voice, model string) ([]byte, error) {
	return []byte("fake-mp3:" + voice + ":" + model + ":" + textNFC), nil
}
