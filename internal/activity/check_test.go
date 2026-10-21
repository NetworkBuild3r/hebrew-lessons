package activity

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// world is an engine over a small lexicon with one learner.
type world struct {
	e     *Engine
	store *db.Store
	clk   *fakeClock
	user  int64
	lex   lexicon
}

func newWorld(t *testing.T) *world {
	t.Helper()
	e, store, clk := openEngine(t)
	return &world{e: e, store: store, clk: clk, user: addUser(t, store, "learner"), lex: fillLexicon(t, store, 40, nil)}
}

func (w *world) anchor() Word { return w.lex.words[w.lex.ids[4]] }

func (w *world) attempts(t *testing.T, user, lexeme int64) []db.Attempt {
	t.Helper()
	got, err := w.store.ListAttempts(context.Background(), user, lexeme)
	mustf(t, err, "attempts")
	return got
}

// card returns the learner's card for a word and skill, or false.
func (w *world) card(t *testing.T, user, lexeme int64, skill string) (db.Card, bool) {
	t.Helper()
	conn, err := w.store.DB()
	mustf(t, err, "db")
	var id int64
	err = conn.QueryRow(`SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = ?`, user, lexeme, skill).Scan(&id)
	if err != nil {
		return db.Card{}, false
	}
	card, err := w.store.GetCard(context.Background(), id)
	mustf(t, err, "card")
	return card, true
}

func (w *world) ratings(t *testing.T, cardID int64) []int {
	t.Helper()
	logs, err := w.store.ReviewLogs(context.Background(), cardID)
	mustf(t, err, "review log")
	var out []int
	for _, l := range logs {
		out = append(out, l.Rating)
	}
	return out
}

func (w *world) confusions(t *testing.T) []string {
	t.Helper()
	conn, err := w.store.DB()
	mustf(t, err, "db")
	rows, err := conn.Query(`SELECT kind || ':' || key FROM confusion_misses WHERE learner_id = ? ORDER BY id`, w.user)
	mustf(t, err, "confusions")
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		mustf(t, rows.Scan(&s), "scan")
		out = append(out, s)
	}
	return out
}

func (w *world) build(t *testing.T, lexeme int64, kind Kind, opts ...BuildOption) built {
	t.Helper()
	b, err := w.e.build(context.Background(), w.user, lexeme, kind, opts...)
	mustf(t, err, "build %s", kind)
	return b
}

func (w *world) check(t *testing.T, b built, resp Response) Result {
	t.Helper()
	res, err := w.e.Check(context.Background(), w.user, b.item.Token, resp)
	mustf(t, err, "check")
	return res
}

func wrongChoice(b built) int {
	for i := range b.p.words {
		if i != b.p.correct {
			return i
		}
	}
	return -1
}

func TestItemsShowOnlyWhatTheSkillAllows(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	t.Run("listen_pick: audio only", func(t *testing.T) {
		it := w.build(t, a.ID, ListenPick).item
		if it.Skill != fsrs.SkillListen || it.Prompt.AudioID == "" || it.Prompt.Hebrew != "" || it.Prompt.Gloss != "" {
			t.Fatalf("item = %+v", it)
		}
		if len(it.Options) != OptionCount || it.Typed || len(it.Left) != 0 {
			t.Fatalf("options = %v", it.Options)
		}
	})
	t.Run("read_pick: pointed Hebrew, no audio", func(t *testing.T) {
		it := w.build(t, a.ID, ReadPick).item
		if it.Skill != fsrs.SkillRead || it.Prompt.Hebrew != a.Text || it.Prompt.AudioID != "" || it.Prompt.Gloss != "" {
			t.Fatalf("item = %+v", it)
		}
		if len(it.Options) != OptionCount {
			t.Fatalf("options = %v", it.Options)
		}
	})
	t.Run("type_it: the gloss, typed", func(t *testing.T) {
		it := w.build(t, a.ID, TypeIt).item
		if it.Skill != fsrs.SkillRecall || it.Prompt.Gloss != a.Gloss || it.Prompt.Hebrew != "" || it.Prompt.AudioID != "" || !it.Typed || len(it.Options) != 0 {
			t.Fatalf("item = %+v", it)
		}
	})
	t.Run("dictation: audio only, typed", func(t *testing.T) {
		it := w.build(t, a.ID, Dictation).item
		if it.Skill != fsrs.SkillSpell || it.Prompt.AudioID == "" || it.Prompt.Hebrew != "" || it.Prompt.Gloss != "" || !it.Typed {
			t.Fatalf("item = %+v", it)
		}
	})
	t.Run("word_match: five Hebrew cues and five glosses", func(t *testing.T) {
		it := w.build(t, a.ID, WordMatch).item
		if it.Skill != fsrs.SkillRead || len(it.Left) != WordMatchPairs || len(it.Right) != WordMatchPairs {
			t.Fatalf("item = %+v", it)
		}
		for _, c := range it.Left {
			if c.Hebrew == "" || c.AudioID != "" {
				t.Fatalf("cue = %+v", c)
			}
		}
	})
	t.Run("audio_match: four clips and four glosses", func(t *testing.T) {
		it := w.build(t, a.ID, AudioMatch).item
		if it.Skill != fsrs.SkillListen || len(it.Left) != AudioMatchClips || len(it.Right) != AudioMatchClips {
			t.Fatalf("item = %+v", it)
		}
		for _, c := range it.Left {
			if c.AudioID == "" || c.Hebrew != "" {
				t.Fatalf("cue = %+v", c)
			}
		}
	})
}

// The answer is kept on the server. Nothing in the item says which option is right, and the
// right option does not sit in a fixed place. The signed payload must not spell the gloss or
// the Hebrew. The signature is not part of that check: it is random base64 and can spell a
// short gloss by chance.
func TestItemDoesNotGiveTheAnswerAway(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	places := map[int]bool{}
	for i := 0; i < 80; i++ {
		b := w.build(t, a.ID, ReadPick)
		places[b.p.correct] = true
		body, err := json.Marshal(b.item)
		mustf(t, err, "marshal")
		for _, banned := range []string{"correct", "answer", "lexeme", "\"id\""} {
			if strings.Contains(string(body), banned) {
				t.Fatalf("item json mentions %q: %s", banned, body)
			}
		}
		if payloadSpellsAnswer(t, b.item.Token, a.Gloss, a.Text) {
			t.Fatalf("token payload spells the answer: %s", tokenPayload(t, b.item.Token))
		}
	}
	if len(places) < 3 {
		t.Fatalf("the right option sat in %d places over 80 items", len(places))
	}
}

// tokenPayload is the decoded JSON of a payload.signature token. The signature is ignored:
// a gloss can occur in that base64 by chance, as "lao" did in RlWpePDwyesSGNTlao3c3-...
func tokenPayload(t *testing.T, token string) string {
	t.Helper()
	body64, sig64, ok := strings.Cut(token, ".")
	if !ok || body64 == "" || sig64 == "" {
		t.Fatalf("token is not payload.signature: %s", token)
	}
	body, err := base64.RawURLEncoding.DecodeString(body64)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	return string(body)
}

// payloadSpellsAnswer reports whether the decoded payload contains the gloss or the Hebrew.
// It does not look at the signature.
func payloadSpellsAnswer(t *testing.T, token, gloss, text string) bool {
	t.Helper()
	payload := tokenPayload(t, token)
	return strings.Contains(payload, gloss) || strings.Contains(payload, text)
}

// TestItemTokenSignatureMayContainTheGloss signs one fixed claim with keys 0, 1, 2, …
// until the signature segment contains the gloss. The raw token then contains it, which
// is the flake, and the decoded payload does not, so the item check still passes.
func TestItemTokenSignatureMayContainTheGloss(t *testing.T) {
	const gloss = "lao"
	const text = "אבגדה"
	c := claims{
		ID:     "0123456789abcdef0123456789abcdef",
		User:   1,
		Lexeme: 4,
		Kind:   ReadPick,
		Exp:    1_700_000_000,
	}
	var token string
	var key []byte
	var n uint64
	for n = 0; n < 1<<20; n++ {
		key = make([]byte, 32)
		binary.LittleEndian.PutUint64(key, n)
		signed, err := signClaims(key, c)
		mustf(t, err, "sign")
		_, sig, ok := strings.Cut(signed, ".")
		if !ok {
			t.Fatalf("token is not payload.signature: %s", signed)
		}
		if strings.Contains(sig, gloss) {
			token = signed
			break
		}
	}
	if token == "" {
		t.Fatalf("no key in %d tries put %q in the signature", n, gloss)
	}
	t.Logf("key %d signature contains %q: %s", n, gloss, token)
	if !strings.Contains(token, gloss) {
		t.Fatalf("raw token lost %q: %s", gloss, token)
	}
	if payloadSpellsAnswer(t, token, gloss, text) {
		t.Fatalf("payload spells the answer, so this is not the signature flake: %s", tokenPayload(t, token))
	}
	got, err := verifyClaims(key, token)
	mustf(t, err, "verify")
	if got != c {
		t.Fatalf("claims = %+v, want %+v", got, c)
	}
}

func TestPickIsGradedAndRecorded(t *testing.T) {
	for _, tc := range []struct {
		kind  Kind
		skill string
		shown func(Word) string
	}{
		{ListenPick, fsrs.SkillListen, func(w Word) string { return w.Text }},
		{ReadPick, fsrs.SkillRead, func(w Word) string { return w.Text }},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			w := newWorld(t)
			a := w.anchor()

			b := w.build(t, a.ID, tc.kind)
			w.clk.Advance(4 * time.Second)
			res := w.check(t, b, Response{Choice: b.p.correct})
			if !res.Correct || res.Rating != fsrs.Good || !res.Feedback || res.Answer != a.Gloss || res.Text != a.Text || res.AudioID != a.AudioID || len(res.Confused) != 0 {
				t.Fatalf("right answer: %+v", res)
			}
			got := w.attempts(t, w.user, a.ID)
			if len(got) != 1 {
				t.Fatalf("attempts = %+v", got)
			}
			row := got[0]
			if row.Skill != tc.skill || row.Activity != string(tc.kind) || row.Source != evidence.SourcePractice ||
				!row.Correct || row.DontKnow || row.HintUsed || row.Prompt != tc.shown(a) || row.Response != a.Gloss ||
				row.LatencyMS != 4000 || !row.CreatedAt.Equal(w.clk.Now()) {
				t.Fatalf("attempt = %+v", row)
			}
			card, ok := w.card(t, w.user, a.ID, tc.skill)
			if !ok || card.Reps != 1 || len(w.ratings(t, card.ID)) != 1 {
				t.Fatalf("card = %+v, %v", card, ok)
			}

			// A wrong pick is an Again, a confusion with the word picked, and the card lapses.
			b = w.build(t, a.ID, tc.kind)
			wrong := wrongChoice(b)
			picked := b.p.words[wrong]
			res = w.check(t, b, Response{Choice: wrong})
			if res.Correct || res.Rating != fsrs.Again || res.Answer != a.Gloss || len(res.Confused) != 1 || res.Confused[0].With != picked.Text {
				t.Fatalf("wrong answer: %+v", res)
			}
			got = w.attempts(t, w.user, a.ID)
			if len(got) != 2 || got[1].Correct || got[1].Response != picked.Gloss {
				t.Fatalf("attempts = %+v", got)
			}
			if rs := w.ratings(t, card.ID); len(rs) != 2 || rs[1] != int(fsrs.Again) {
				t.Fatalf("ratings = %v", rs)
			}
			cons := w.confusions(t)
			if len(cons) != 1 || cons[0] != res.Confused[0].Kind+":"+res.Confused[0].Key {
				t.Fatalf("confusions = %v, result says %+v", cons, res.Confused)
			}
			logs, err := w.store.ReviewLogs(context.Background(), card.ID)
			mustf(t, err, "logs")
			if logs[1].Chosen != picked.Text {
				t.Fatalf("review_log chosen = %q, want %q", logs[1].Chosen, picked.Text)
			}

			// "I don't know" is stored as such and is an Again.
			b = w.build(t, a.ID, tc.kind)
			res = w.check(t, b, Response{DontKnow: true})
			if res.Correct || res.Rating != fsrs.Again || len(res.Confused) != 0 {
				t.Fatalf("dont know: %+v", res)
			}
			got = w.attempts(t, w.user, a.ID)
			if last := got[len(got)-1]; !last.DontKnow || last.Correct || last.Response != "" {
				t.Fatalf("attempt = %+v", last)
			}

			// Right after a hint or a replay is Hard.
			b = w.build(t, a.ID, tc.kind)
			res = w.check(t, b, Response{Choice: b.p.correct, Hinted: true})
			if !res.Correct || res.Rating != fsrs.Hard {
				t.Fatalf("hinted: %+v", res)
			}
			got = w.attempts(t, w.user, a.ID)
			if !got[len(got)-1].HintUsed {
				t.Fatalf("attempt = %+v", got[len(got)-1])
			}
		})
	}
}

// Right and faster than the learner's own median for the activity is Easy.
func TestFastRightAnswerIsEasy(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	for i := 0; i < evidence.MedianMinAttempts; i++ {
		b := w.build(t, a.ID, ReadPick)
		w.clk.Advance(5 * time.Second)
		if res := w.check(t, b, Response{Choice: b.p.correct}); res.Rating == fsrs.Easy {
			t.Fatalf("answer %d was Easy with no median yet", i)
		}
	}
	b := w.build(t, a.ID, ReadPick)
	w.clk.Advance(time.Second)
	if res := w.check(t, b, Response{Choice: b.p.correct}); res.Rating != fsrs.Easy {
		t.Fatalf("rating = %d, want Easy: 1s against a 5s median", res.Rating)
	}
	b = w.build(t, a.ID, ReadPick)
	w.clk.Advance(9 * time.Second)
	if res := w.check(t, b, Response{Choice: b.p.correct}); res.Rating != fsrs.Good {
		t.Fatalf("rating = %d, want Good: slower than the median", res.Rating)
	}
}

func TestTypedAnswersAreGradedAndRecorded(t *testing.T) {
	for _, tc := range []struct {
		kind  Kind
		skill string
		shown string
	}{
		{TypeIt, fsrs.SkillRecall, "window"},
		{Dictation, fsrs.SkillSpell, "חַלּוֹן"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			w := newWorld(t)
			window := addWord(t, w.store, "חַלּוֹן", "window", "noun", true)

			// ac-3: חלון for חַלּוֹן is Good. Niqqud is ignored.
			b := w.build(t, window, tc.kind)
			res := w.check(t, b, Response{Text: "חלון"})
			if !res.Correct || res.NearMiss || res.Rating != fsrs.Good || res.Answer != "חַלּוֹן" || len(res.Confused) != 0 {
				t.Fatalf("exact: %+v", res)
			}
			got := w.attempts(t, w.user, window)
			if len(got) != 1 || got[0].Skill != tc.skill || got[0].Activity != string(tc.kind) || !got[0].Correct ||
				got[0].Prompt != tc.shown || got[0].Response != "חלון" {
				t.Fatalf("attempts = %+v", got)
			}
			if len(w.confusions(t)) != 0 {
				t.Fatalf("confusions = %v after an exact answer", w.confusions(t))
			}
			card, ok := w.card(t, w.user, window, tc.skill)
			if !ok {
				t.Fatal("no card after the first attempt")
			}

			// ac-3: חלום is one letter off. It is rated Hard, and the letter pair is logged.
			b = w.build(t, window, tc.kind)
			res = w.check(t, b, Response{Text: "חלום"})
			if !res.Correct || !res.NearMiss || res.Rating != fsrs.Hard {
				t.Fatalf("near miss: %+v", res)
			}
			if cons := w.confusions(t); len(cons) != 1 || cons[0] != "letter_pair:ם/ן" {
				t.Fatalf("confusions = %v, want the pair (ן, ם)", cons)
			}
			if len(res.Confused) != 1 || res.Confused[0].Key != "ם/ן" || res.Confused[0].Kind != fsrs.MissLetterPair {
				t.Fatalf("result confused = %+v", res.Confused)
			}
			if rs := w.ratings(t, card.ID); len(rs) != 2 || rs[1] != int(fsrs.Hard) {
				t.Fatalf("ratings = %v", rs)
			}

			// Two letters off: wrong, and both pairs are logged.
			b = w.build(t, window, tc.kind)
			res = w.check(t, b, Response{Text: "כלום"})
			if res.Correct || res.NearMiss || res.Rating != fsrs.Again {
				t.Fatalf("two off: %+v", res)
			}
			cons := w.confusions(t)
			if len(cons) != 3 || cons[1] != "letter_pair:ח/כ" || cons[2] != "letter_pair:ם/ן" {
				t.Fatalf("confusions = %v", cons)
			}

			// A different word, far off: wrong, and no letter pair.
			b = w.build(t, window, tc.kind)
			res = w.check(t, b, Response{Text: "שלום עולם"})
			if res.Correct || res.Rating != fsrs.Again || len(res.Confused) != 0 || len(w.confusions(t)) != 3 {
				t.Fatalf("far off: %+v, confusions %v", res, w.confusions(t))
			}
			got = w.attempts(t, w.user, window)
			if last := got[len(got)-1]; last.Correct || last.Response != "שלום עולם" {
				t.Fatalf("attempt = %+v", last)
			}

			// Giving up.
			b = w.build(t, window, tc.kind)
			res = w.check(t, b, Response{DontKnow: true})
			if res.Correct || res.Rating != fsrs.Again {
				t.Fatalf("give up: %+v", res)
			}
			got = w.attempts(t, w.user, window)
			if last := got[len(got)-1]; !last.DontKnow {
				t.Fatalf("attempt = %+v", last)
			}
		})
	}
}

// A typed answer that is exactly another word is a mix-up of two words, written once by the
// recorder, not also as a letter pair.
func TestTypedAnotherWordIsAWordConfusion(t *testing.T) {
	w := newWorld(t)
	window := addWord(t, w.store, "חַלּוֹן", "window", "noun", true)
	other := addWord(t, w.store, "שׁוֹלְחָן", "table", "noun", true)
	b := w.build(t, window, TypeIt)
	res := w.check(t, b, Response{Text: "שולחן"})
	if res.Correct || res.Rating != fsrs.Again || len(res.Confused) != 1 || res.Confused[0].With != "שׁוֹלְחָן" {
		t.Fatalf("result = %+v", res)
	}
	cons := w.confusions(t)
	if len(cons) != 1 || !strings.HasPrefix(cons[0], "other:") {
		t.Fatalf("confusions = %v", cons)
	}
	card, ok := w.card(t, w.user, window, fsrs.SkillRecall)
	if !ok {
		t.Fatal("no card")
	}
	logs, err := w.store.ReviewLogs(context.Background(), card.ID)
	mustf(t, err, "logs")
	if logs[0].Chosen != "שׁוֹלְחָן" {
		t.Fatalf("chosen = %q", logs[0].Chosen)
	}
	_ = other

	// One letter away from another word: that word is also a partner for the next pick.
	near := addWord(t, w.store, "חלום", "dream", "noun", true)
	b = w.build(t, window, TypeIt)
	res = w.check(t, b, Response{Text: "חלום"})
	if !res.Correct || !res.NearMiss {
		t.Fatalf("a one-letter-off answer is a near miss even when it spells another word: %+v", res)
	}
	if cons := w.confusions(t); len(cons) != 2 || cons[1] != "letter_pair:ם/ן" {
		t.Fatalf("confusions = %v", cons)
	}
	_ = near
}

func TestTypedResponsesAreValidated(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	b := w.build(t, a.ID, TypeIt)
	for _, resp := range []Response{
		{Text: ""},
		{Text: "   "},
		{Text: strings.Repeat("א", MaxResponseBytes)},
	} {
		if _, err := w.e.Check(context.Background(), w.user, b.item.Token, resp); !errors.Is(err, ErrBadResponse) {
			t.Fatalf("Check(%q) err = %v, want ErrBadResponse", resp.Text, err)
		}
	}
	if n := len(w.attempts(t, w.user, a.ID)); n != 0 {
		t.Fatalf("%d attempts written for refused responses", n)
	}
	// The item is still pending after a refused response.
	if res := w.check(t, b, Response{Text: a.Text}); !res.Correct {
		t.Fatalf("result = %+v", res)
	}
}

func TestPickResponsesAreValidated(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	b := w.build(t, a.ID, ListenPick)
	for _, resp := range []Response{{Choice: -1}, {Choice: OptionCount}, {Choice: 99}} {
		if _, err := w.e.Check(context.Background(), w.user, b.item.Token, resp); !errors.Is(err, ErrBadResponse) {
			t.Fatalf("Check(%+v) err = %v, want ErrBadResponse", resp, err)
		}
	}
	if res := w.check(t, b, Response{Choice: b.p.correct}); !res.Correct {
		t.Fatalf("result = %+v", res)
	}
}

// ac-4: a word_match with one swapped pair writes two confusion rows and one Again per word in
// the pair. The other pairs are right.
func TestWordMatchWithOneSwappedPair(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	b := w.build(t, a.ID, WordMatch)
	pairs := append([]int(nil), b.p.rightOf...)
	// Swap the answers of the first two entries.
	pairs[0], pairs[1] = pairs[1], pairs[0]
	x, y := b.p.words[0], b.p.words[1]

	res := w.check(t, b, Response{Pairs: pairs})
	if res.Correct || len(res.Pairs) != WordMatchPairs || len(res.Confused) != 2 {
		t.Fatalf("result = %+v", res)
	}
	right, wrong := 0, 0
	for _, p := range res.Pairs {
		if p.Correct {
			right++
		} else {
			wrong++
		}
	}
	if right != 3 || wrong != 2 {
		t.Fatalf("pairs = %+v", res.Pairs)
	}

	// One confusion row per wrong word: two rows.
	cons := w.confusions(t)
	if len(cons) != 2 {
		t.Fatalf("confusions = %v, want two rows", cons)
	}
	// One Again per word in the pair, and a Good for each of the others, on read cards.
	for i, word := range b.p.words {
		got := w.attempts(t, w.user, word.ID)
		if len(got) != 1 || got[0].Skill != fsrs.SkillRead || got[0].Activity != string(WordMatch) {
			t.Fatalf("word %d attempts = %+v", i, got)
		}
		card, ok := w.card(t, w.user, word.ID, fsrs.SkillRead)
		if !ok {
			t.Fatalf("word %d has no read card", i)
		}
		want := int(fsrs.Good)
		if word.ID == x.ID || word.ID == y.ID {
			want = int(fsrs.Again)
			if got[0].Correct {
				t.Fatalf("word %d was swapped but is stored correct", i)
			}
		}
		if rs := w.ratings(t, card.ID); len(rs) != 1 || rs[0] != want {
			t.Fatalf("word %d ratings = %v, want [%d]", i, rs, want)
		}
	}
	// The result's rating is the asked word's.
	if want := fsrs.Good; x.ID != a.ID && y.ID != a.ID && res.Rating != want {
		t.Fatalf("rating = %d", res.Rating)
	}
	if (x.ID == a.ID || y.ID == a.ID) && res.Rating != fsrs.Again {
		t.Fatalf("rating = %d, want Again: the asked word was swapped", res.Rating)
	}
}

func TestMatchAllRightAndGivingUp(t *testing.T) {
	for _, tc := range []struct {
		kind  Kind
		skill string
		n     int
	}{
		{WordMatch, fsrs.SkillRead, WordMatchPairs},
		{AudioMatch, fsrs.SkillListen, AudioMatchClips},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			w := newWorld(t)
			a := w.anchor()
			b := w.build(t, a.ID, tc.kind)
			res := w.check(t, b, Response{Pairs: b.p.rightOf})
			if !res.Correct || res.Rating != fsrs.Good || len(res.Confused) != 0 {
				t.Fatalf("all right: %+v", res)
			}
			if n := countRows(t, w.store, `SELECT COUNT(*) FROM attempts WHERE activity = ? AND skill = ? AND correct = 1`, string(tc.kind), tc.skill); n != tc.n {
				t.Fatalf("%d right attempts, want %d", n, tc.n)
			}
			if len(w.confusions(t)) != 0 {
				t.Fatalf("confusions = %v", w.confusions(t))
			}

			b = w.build(t, a.ID, tc.kind)
			res = w.check(t, b, Response{DontKnow: true})
			if res.Correct || res.Rating != fsrs.Again {
				t.Fatalf("give up: %+v", res)
			}
			if n := countRows(t, w.store, `SELECT COUNT(*) FROM attempts WHERE activity = ? AND dont_know = 1`, string(tc.kind)); n != tc.n {
				t.Fatalf("%d dont-know attempts, want %d", n, tc.n)
			}
		})
	}
}

func TestMatchResponsesMustBeAPermutation(t *testing.T) {
	w := newWorld(t)
	b := w.build(t, w.anchor().ID, WordMatch)
	for _, pairs := range [][]int{
		nil,
		{0, 1, 2},
		{0, 1, 2, 3, 3},
		{0, 1, 2, 3, 5},
		{-1, 1, 2, 3, 4},
	} {
		if _, err := w.e.Check(context.Background(), w.user, b.item.Token, Response{Pairs: pairs}); !errors.Is(err, ErrBadResponse) {
			t.Fatalf("Check(%v) err = %v, want ErrBadResponse", pairs, err)
		}
	}
	if n := countRows(t, w.store, `SELECT COUNT(*) FROM attempts`); n != 0 {
		t.Fatalf("%d attempts written for refused responses", n)
	}
}

// A word with no listen card gets one the first time it is asked through listening. Asking
// again does not make another, and the word's other cards are left alone.
func TestFirstAskCreatesTheCard(t *testing.T) {
	for _, tc := range []struct {
		kind  Kind
		skill string
		resp  func(b built, a Word) Response
	}{
		{ListenPick, fsrs.SkillListen, func(b built, a Word) Response { return Response{Choice: b.p.correct} }},
		{Dictation, fsrs.SkillSpell, func(b built, a Word) Response { return Response{Text: a.Text} }},
	} {
		t.Run(tc.skill, func(t *testing.T) {
			w := newWorld(t)
			a := w.anchor()
			// The word has old read and recall cards, as every word from before v3 does.
			for _, skill := range []string{fsrs.SkillRead, fsrs.SkillRecall} {
				_, err := w.store.InsertCard(context.Background(), db.Card{
					UserID: w.user, LexemeID: a.ID, Skill: skill, State: fsrs.StateReview,
					Due: w.clk.Now().Add(time.Hour), Stability: 3, Difficulty: 5, Reps: 2, LastReview: w.clk.Now().Add(-time.Hour),
				})
				mustf(t, err, "card")
			}
			if _, ok := w.card(t, w.user, a.ID, tc.skill); ok {
				t.Fatalf("a %s card exists before the word is asked", tc.skill)
			}
			b := w.build(t, a.ID, tc.kind)
			if _, ok := w.card(t, w.user, a.ID, tc.skill); ok {
				t.Fatalf("Build made a %s card: it is made when the word is answered", tc.skill)
			}
			w.check(t, b, tc.resp(b, a))
			card, ok := w.card(t, w.user, a.ID, tc.skill)
			if !ok || card.Reps != 1 {
				t.Fatalf("card = %+v, %v", card, ok)
			}
			first := card.ID
			b = w.build(t, a.ID, tc.kind)
			w.check(t, b, tc.resp(b, a))
			card, _ = w.card(t, w.user, a.ID, tc.skill)
			if card.ID != first || card.Reps != 2 {
				t.Fatalf("card = %+v, want the same card reviewed twice", card)
			}
			if n := countRows(t, w.store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, w.user, a.ID); n != 3 {
				t.Fatalf("%d cards for the word, want read, recall and %s", n, tc.skill)
			}
			for _, skill := range []string{fsrs.SkillRead, fsrs.SkillRecall} {
				old, _ := w.card(t, w.user, a.ID, skill)
				if old.Reps != 2 {
					t.Fatalf("the %s card was reviewed by a %s attempt: %+v", skill, tc.skill, old)
				}
			}
		})
	}
}

func TestSourcesDecideWhatIsScheduled(t *testing.T) {
	t.Run("lesson stores the attempt and leaves cards alone", func(t *testing.T) {
		w := newWorld(t)
		a := w.anchor()
		unit, err := w.store.ExecContext(context.Background(),
			`INSERT INTO units (position, title, situation, concept, status, rev, doc) VALUES (1, 'u', 's', 'c', 'published', 1, '{}')`)
		mustf(t, err, "unit")
		unitID, _ := unit.LastInsertId()
		b := w.build(t, a.ID, ListenPick, WithSource(evidence.SourceLesson), WithUnit(unitID))
		res := w.check(t, b, Response{Choice: b.p.correct})
		if !res.Correct || !res.Feedback {
			t.Fatalf("result = %+v", res)
		}
		got := w.attempts(t, w.user, a.ID)
		if len(got) != 1 || got[0].Source != evidence.SourceLesson || got[0].UnitID != unitID {
			t.Fatalf("attempts = %+v", got)
		}
		if _, ok := w.card(t, w.user, a.ID, fsrs.SkillListen); ok {
			t.Fatal("a lesson check made a card")
		}
		if n := countRows(t, w.store, `SELECT COUNT(*) FROM review_log`); n != 0 {
			t.Fatalf("%d review_log rows from a lesson check", n)
		}
	})
	t.Run("review reschedules", func(t *testing.T) {
		w := newWorld(t)
		a := w.anchor()
		b := w.build(t, a.ID, ReadPick, WithSource(evidence.SourceReview))
		w.check(t, b, Response{Choice: b.p.correct})
		got := w.attempts(t, w.user, a.ID)
		if len(got) != 1 || got[0].Source != evidence.SourceReview {
			t.Fatalf("attempts = %+v", got)
		}
		if _, ok := w.card(t, w.user, a.ID, fsrs.SkillRead); !ok {
			t.Fatal("review made no card")
		}
	})
	t.Run("placement gives no feedback and seeds a card only for a right answer", func(t *testing.T) {
		w := newWorld(t)
		a := w.anchor()
		b := w.build(t, a.ID, ListenPick, WithSource(evidence.SourcePlacement))
		res := w.check(t, b, Response{Choice: wrongChoice(b)})
		if res.Correct || res.Feedback || res.Answer != "" || res.Text != "" || res.AudioID != "" || len(res.Confused) != 0 || len(res.Pairs) != 0 {
			t.Fatalf("placement result leaked feedback: %+v", res)
		}
		if _, ok := w.card(t, w.user, a.ID, fsrs.SkillListen); ok {
			t.Fatal("a wrong placement answer made a card")
		}
		b = w.build(t, a.ID, ListenPick, WithSource(evidence.SourcePlacement))
		res = w.check(t, b, Response{Choice: b.p.correct})
		if !res.Correct || res.Feedback || res.Answer != "" || res.Rating != fsrs.Good {
			t.Fatalf("placement result = %+v", res)
		}
		card, ok := w.card(t, w.user, a.ID, fsrs.SkillListen)
		if !ok || w.ratings(t, card.ID)[0] != int(fsrs.Good) {
			t.Fatalf("card = %+v, %v", card, ok)
		}
	})
	t.Run("an unknown source is refused", func(t *testing.T) {
		w := newWorld(t)
		if _, err := w.e.Build(context.Background(), w.user, w.anchor().ID, ReadPick, WithSource("exam")); !errors.Is(err, ErrBadSource) {
			t.Fatalf("err = %v", err)
		}
	})
}

// ac-5: Check with a token from another learner's session is refused.
func TestCheckRefusesAnotherLearnersToken(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	other := addUser(t, w.store, "other")
	b := w.build(t, a.ID, ReadPick)

	_, err := w.e.Check(context.Background(), other, b.item.Token, Response{Choice: b.p.correct})
	if !errors.Is(err, ErrTokenUser) {
		t.Fatalf("err = %v, want ErrTokenUser", err)
	}
	if n := countRows(t, w.store, `SELECT COUNT(*) FROM attempts`); n != 0 {
		t.Fatalf("%d attempts written for a refused token", n)
	}
	// The refusal does not use the item up: its owner can still answer it.
	if res := w.check(t, b, Response{Choice: b.p.correct}); !res.Correct {
		t.Fatalf("result = %+v", res)
	}
	if n := len(w.attempts(t, other, a.ID)); n != 0 {
		t.Fatalf("the other learner has %d attempts", n)
	}
	// Nor can learner 0 or a made-up learner answer it.
	b = w.build(t, a.ID, ReadPick)
	for _, id := range []int64{0, -1, w.user + 1000} {
		if _, err := w.e.Check(context.Background(), id, b.item.Token, Response{Choice: b.p.correct}); err == nil {
			t.Fatalf("learner %d answered another learner's item", id)
		}
	}
}

func TestTokenIsSingleUseSignedAndExpires(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	ctx := context.Background()

	t.Run("single use", func(t *testing.T) {
		b := w.build(t, a.ID, ReadPick)
		w.check(t, b, Response{Choice: b.p.correct})
		if _, err := w.e.Check(ctx, w.user, b.item.Token, Response{Choice: b.p.correct}); !errors.Is(err, ErrTokenUnknown) {
			t.Fatalf("replay err = %v, want ErrTokenUnknown", err)
		}
	})
	t.Run("a wrong answer uses the item up too", func(t *testing.T) {
		b := w.build(t, a.ID, ReadPick)
		w.check(t, b, Response{Choice: wrongChoice(b)})
		if _, err := w.e.Check(ctx, w.user, b.item.Token, Response{Choice: b.p.correct}); !errors.Is(err, ErrTokenUnknown) {
			t.Fatalf("second try err = %v, want ErrTokenUnknown", err)
		}
	})
	t.Run("expires after 30 minutes", func(t *testing.T) {
		b := w.build(t, a.ID, ReadPick)
		if !b.item.ExpiresAt.Equal(w.clk.Now().Add(30 * time.Minute)) {
			t.Fatalf("expires at %v", b.item.ExpiresAt)
		}
		w.clk.Advance(29 * time.Minute)
		if _, err := w.e.Check(ctx, w.user, b.item.Token, Response{Choice: -5}); !errors.Is(err, ErrBadResponse) {
			t.Fatalf("before expiry err = %v", err)
		}
		w.clk.Advance(2 * time.Minute)
		if _, err := w.e.Check(ctx, w.user, b.item.Token, Response{Choice: b.p.correct}); !errors.Is(err, ErrTokenExpired) {
			t.Fatalf("after expiry err = %v, want ErrTokenExpired", err)
		}
	})
	t.Run("tampered or foreign tokens are invalid", func(t *testing.T) {
		b := w.build(t, a.ID, ReadPick)
		body, sig, _ := strings.Cut(b.item.Token, ".")
		flipped := body[:len(body)-2] + "AA"
		if flipped == body {
			flipped = body[:len(body)-2] + "BB"
		}
		for name, tok := range map[string]string{
			"flipped body":  flipped + "." + sig,
			"no signature":  body,
			"empty":         "",
			"junk":          "not.a.token",
			"swapped parts": sig + "." + body,
		} {
			if _, err := w.e.Check(ctx, w.user, tok, Response{Choice: b.p.correct}); !errors.Is(err, ErrTokenInvalid) {
				t.Fatalf("%s: err = %v, want ErrTokenInvalid", name, err)
			}
		}
		// A token signed with another key is not accepted.
		otherKey, err := KeyFromSecret("another secret")
		mustf(t, err, "key")
		c, err := verifyClaims(w.e.key, b.item.Token)
		mustf(t, err, "claims")
		forged, err := signClaims(otherKey, c)
		mustf(t, err, "sign")
		if _, err := w.e.Check(ctx, w.user, forged, Response{Choice: b.p.correct}); !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("forged err = %v", err)
		}
		// The real token still works.
		if res := w.check(t, b, Response{Choice: b.p.correct}); !res.Correct {
			t.Fatalf("result = %+v", res)
		}
	})
	t.Run("the claims bind learner, word and activity", func(t *testing.T) {
		b := w.build(t, a.ID, ReadPick)
		c, err := verifyClaims(w.e.key, b.item.Token)
		mustf(t, err, "claims")
		if c.User != w.user || c.Lexeme != a.ID || c.Kind != ReadPick || c.Exp != b.item.ExpiresAt.Unix() {
			t.Fatalf("claims = %+v", c)
		}
		// Right key, right nonce, but another word: the claims do not match the pending item.
		c.Lexeme = w.lex.ids[7]
		forged, err := signClaims(w.e.key, c)
		mustf(t, err, "sign")
		if _, err := w.e.Check(ctx, w.user, forged, Response{Choice: b.p.correct}); !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("err = %v, want ErrTokenInvalid", err)
		}
		c.Lexeme, c.Kind = a.ID, TypeIt
		forged, _ = signClaims(w.e.key, c)
		if _, err := w.e.Check(ctx, w.user, forged, Response{Choice: b.p.correct}); !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("err = %v, want ErrTokenInvalid for another activity", err)
		}
		if res := w.check(t, b, Response{Choice: b.p.correct}); !res.Correct {
			t.Fatalf("the real token stopped working: %+v", res)
		}
	})
}

func TestTwoChecksOfOneItemWriteOnce(t *testing.T) {
	w := newWorld(t)
	a := w.anchor()
	b := w.build(t, a.ID, ReadPick)
	var wg sync.WaitGroup
	results := make([]error, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = w.e.Check(context.Background(), w.user, b.item.Token, Response{Choice: b.p.correct})
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("%d of 8 concurrent checks succeeded: %v", ok, results)
	}
	if n := len(w.attempts(t, w.user, a.ID)); n != 1 {
		t.Fatalf("%d attempts", n)
	}
}

func TestBuildRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("flashcards have no item", func(t *testing.T) {
		w := newWorld(t)
		if _, err := w.e.Build(ctx, w.user, w.anchor().ID, Flashcard); !errors.Is(err, ErrNotChecked) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unknown activity and word", func(t *testing.T) {
		w := newWorld(t)
		if _, err := w.e.Build(ctx, w.user, w.anchor().ID, "cloze"); !errors.Is(err, ErrUnknownActivity) {
			t.Fatalf("err = %v", err)
		}
		for _, id := range []int64{0, -3, 99999} {
			if _, err := w.e.Build(ctx, w.user, id, ReadPick); !errors.Is(err, ErrUnknownWord) {
				t.Fatalf("lexeme %d: err = %v", id, err)
			}
		}
		if _, err := w.e.Build(ctx, 0, w.anchor().ID, ReadPick); !errors.Is(err, ErrUnknownWord) {
			t.Fatalf("no learner: err = %v", err)
		}
	})
	t.Run("a word with no gloss or no clip", func(t *testing.T) {
		w := newWorld(t)
		bare := addWord(t, w.store, "עַכְבָּר", "", "noun", true)
		silent := addWord(t, w.store, "חָתוּל", "cat", "noun", false)
		for kind, want := range map[Kind]error{ReadPick: ErrNoGloss, TypeIt: ErrNoGloss, WordMatch: ErrNoGloss, ListenPick: ErrNoGloss} {
			if _, err := w.e.Build(ctx, w.user, bare, kind); !errors.Is(err, want) {
				t.Fatalf("bare %s: err = %v, want %v", kind, err, want)
			}
		}
		if _, err := w.e.Build(ctx, w.user, bare, Dictation); err != nil {
			t.Fatalf("dictation needs no gloss: %v", err)
		}
		for _, kind := range []Kind{ListenPick, AudioMatch, Dictation} {
			if _, err := w.e.Build(ctx, w.user, silent, kind); !errors.Is(err, ErrNoAudio) {
				t.Fatalf("silent %s: err = %v, want ErrNoAudio", kind, err)
			}
		}
		if _, err := w.e.Build(ctx, w.user, silent, ReadPick); err != nil {
			t.Fatalf("read_pick needs no clip: %v", err)
		}
	})
	t.Run("not enough fair distractors", func(t *testing.T) {
		e, store, _ := openEngine(t)
		user := addUser(t, store, "small")
		a := addWord(t, store, "בַּיִת", "house", "noun", true)
		addWord(t, store, "עֵץ", "tree", "noun", true)
		addWord(t, store, "בַּיְתָה", "house, home", "noun", true) // a synonym
		if _, err := e.Build(ctx, user, a, ReadPick); !errors.Is(err, ErrNotEnoughWords) {
			t.Fatalf("err = %v, want ErrNotEnoughWords", err)
		}
		if _, err := e.Build(ctx, user, a, TypeIt); err != nil {
			t.Fatalf("type_it needs no distractors: %v", err)
		}
	})
	t.Run("an engine with no store", func(t *testing.T) {
		var e *Engine
		if _, err := e.Build(ctx, 1, 1, ReadPick); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("err = %v", err)
		}
		if _, err := e.Check(ctx, 1, "x", Response{}); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestMatchPrefersWordsTheLearnerHasMet(t *testing.T) {
	w := newWorld(t)
	a := w.lex.words[w.lex.ids[2]] // a noun, as are the even ids
	var met []int64
	for _, i := range []int{4, 6, 8, 10, 12, 14} {
		id := w.lex.ids[i]
		met = append(met, id)
		_, err := w.store.InsertCard(context.Background(), db.Card{
			UserID: w.user, LexemeID: id, Skill: fsrs.SkillRead, State: fsrs.StateReview,
			Due: w.clk.Now(), Stability: 3, Difficulty: 5, Reps: 2, LastReview: w.clk.Now().Add(-time.Hour),
		})
		mustf(t, err, "card")
	}
	for n := 0; n < 30; n++ {
		b := w.build(t, a.ID, WordMatch)
		for _, word := range b.p.words {
			if word.ID == a.ID {
				continue
			}
			seen := false
			for _, id := range met {
				seen = seen || id == word.ID
			}
			if !seen {
				t.Fatalf("build %d paired an unmet word %+v while six met words of the group were free", n, word)
			}
		}
	}
}

func TestPendingSetIsBounded(t *testing.T) {
	s := newPendingSet()
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < MaxPending+500; i++ {
		s.add(string(rune('a'+i%26))+strings.Repeat("x", i/26), &pending{expires: now.Add(time.Hour)}, now)
	}
	if n := s.size(); n > MaxPending {
		t.Fatalf("%d pending items, cap %d", n, MaxPending)
	}
	// Expired items go as newer ones arrive.
	s = newPendingSet()
	for i := 0; i < 10; i++ {
		s.add("old"+string(rune('a'+i)), &pending{expires: now.Add(time.Minute)}, now)
	}
	later := now.Add(time.Hour)
	s.add("new", &pending{expires: later.Add(time.Hour)}, later)
	if n := s.size(); n != 1 {
		t.Fatalf("%d pending items after expiry, want 1", n)
	}
}

func TestKeyFromSecret(t *testing.T) {
	a, err := KeyFromSecret("secret")
	mustf(t, err, "key")
	b, _ := KeyFromSecret(" secret ")
	c, _ := KeyFromSecret("other")
	if len(a) != 32 || string(a) != string(b) || string(a) == string(c) {
		t.Fatalf("keys: %x %x %x", a, b, c)
	}
	r1, _ := KeyFromSecret("")
	r2, _ := KeyFromSecret("")
	if len(r1) != 32 || string(r1) == string(r2) {
		t.Fatal("an empty secret must give a fresh random key")
	}
	// Tokens made under one secret are not valid under another.
	e1, store, clk := openEngine(t, WithKey(a))
	user := addUser(t, store, "keys")
	lex := fillLexicon(t, store, 20, nil)
	it, err := e1.Build(context.Background(), user, lex.ids[0], ReadPick)
	mustf(t, err, "build")
	e2 := New(store, clk, nil, WithKey(c))
	if _, err := e2.Check(context.Background(), user, it.Token, Response{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestExposeRecordsALessonIntro(t *testing.T) {
	w := newWorld(t)
	a, b := w.lex.ids[0], w.lex.ids[1]
	mustf(t, w.e.Expose(context.Background(), w.user, []int64{a, b, a}, 0), "expose")
	if n := countRows(t, w.store, `SELECT COUNT(*) FROM exposures WHERE user_id = ? AND context = 'lesson_intro'`, w.user); n != 2 {
		t.Fatalf("%d exposures, want 2", n)
	}
}
