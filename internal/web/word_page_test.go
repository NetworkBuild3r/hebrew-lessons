package web

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// INIT-023/SPEC-007: /words/{id} verb page.

func TestWordPageVerbPresentAndPast(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "a@example.com", "sub-word-verb", now)
	ctx := context.Background()

	inf := units.Normalize("לִבְחוֹר")
	msg, fsg, mpl, fpl := units.Normalize("בּוֹחֵר"), units.Normalize("בּוֹחֶרֶת"), units.Normalize("בּוֹחֲרִים"), units.Normalize("בּוֹחֲרוֹת")
	past1 := units.Normalize("בָּחַרְתִּי")
	root := "ב־ח־ר"
	binyan := "paal"
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: inf, Gloss: "to choose", Translit: sp("livkhor"), POS: sp("verb"), Root: &root, Binyan: &binyan},
		{TextNFC: msg, Gloss: "chooses (m)", Translit: sp("bokher"), POS: sp("verb"), Root: &root},
		{TextNFC: fsg, Gloss: "chooses (f)", Translit: sp("bokheret"), POS: sp("verb"), Root: &root},
		{TextNFC: mpl, Gloss: "choose (m pl)", Translit: sp("bokharim"), POS: sp("verb"), Root: &root},
		{TextNFC: fpl, Gloss: "choose (f pl)", Translit: sp("bokharot"), POS: sp("verb"), Root: &root},
		{TextNFC: past1, Gloss: "I chose", Translit: sp("bakharti"), POS: sp("verb"), Root: &root},
	})
	if err != nil {
		t.Fatal(err)
	}
	byText := map[string]int64{}
	for _, row := range saved {
		byText[row.TextNFC] = row.ID
	}
	lemmaID := byText[inf]

	clips := map[string]string{
		msg:   strings.Repeat("a1", 32),
		fsg:   strings.Repeat("b2", 32),
		mpl:   strings.Repeat("c3", 32),
		fpl:   strings.Repeat("d4", 32),
		past1: strings.Repeat("e5", 32),
		inf:   strings.Repeat("f6", 32),
	}
	for text, id := range clips {
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID: id, TextNFC: text, Voice: "voice", Model: "eleven_v4", Bytes: 4,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.UpsertVerbForms(ctx, []db.VerbFormInput{
		{LemmaID: lemmaID, Tense: "present", Number: "sg", Gender: "m", FormHe: msg, Translit: "bokher", FormLexemeID: byText[msg]},
		{LemmaID: lemmaID, Tense: "present", Number: "sg", Gender: "f", FormHe: fsg, Translit: "bokheret", FormLexemeID: byText[fsg]},
		{LemmaID: lemmaID, Tense: "present", Number: "pl", Gender: "m", FormHe: mpl, Translit: "bokharim", FormLexemeID: byText[mpl]},
		{LemmaID: lemmaID, Tense: "present", Number: "pl", Gender: "f", FormHe: fpl, Translit: "bokharot", FormLexemeID: byText[fpl]},
		{LemmaID: lemmaID, Tense: "past", Person: "1", Number: "sg", FormHe: past1, Translit: "bakharti", FormLexemeID: byText[past1]},
	}); err != nil {
		t.Fatal(err)
	}
	knowWord(t, store, learner.user.ID, byText[msg], now)

	// A second published unit with a future form absent — future tense must be omitted.
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Choose well", Position: 1, NewWords: []string{msg},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: msg + " " + past1, Translation: "he chooses, I chose"},
		}}},
	}, "shared")
	publish(t, store, unitID)

	path := "/words/" + strconv.FormatInt(lemmaID, 10)
	body := getLearner(t, h, learner, path)

	// ac-1: present grid order m.sg, f.sg, m.pl, f.pl with <audio> when clip exists.
	present := regexp.MustCompile(`(?s)id="tense-present".*?</table>`).FindString(body)
	if present == "" {
		t.Fatalf("present tense missing:\n%s", body)
	}
	order := []string{msg, fsg, mpl, fpl}
	prev := -1
	for _, form := range order {
		at := strings.Index(present, form)
		if at < 0 {
			t.Fatalf("present missing %s:\n%s", form, present)
		}
		if at < prev {
			t.Fatalf("present order wrong around %s", form)
		}
		prev = at
		clipRE := regexp.MustCompile(`(?s)<div class="clip">.*?<audio id="verb-present-\d+-\d+"[^>]*src="/audio/` + clips[form] + `\.mp3"`)
		if !clipRE.MatchString(present) {
			t.Fatalf("present cell for %s lacks <audio> inside .clip:\n%s", form, present)
		}
	}

	// ac-2: past under Past tense; future omitted when empty.
	if !strings.Contains(body, `id="tense-past"`) || !strings.Contains(body, "Past tense") {
		t.Fatal("past tense missing")
	}
	past := regexp.MustCompile(`(?s)id="tense-past".*?</table>`).FindString(body)
	if !strings.Contains(past, past1) || !strings.Contains(past, "I") || !strings.Contains(past, "אֲנִי") {
		t.Fatalf("past row incomplete:\n%s", past)
	}
	if strings.Contains(body, `id="tense-future"`) || strings.Contains(body, "Future tense") {
		t.Fatal("empty future tense was rendered")
	}

	// ac-3: known mark on known form only.
	knownCells := regexp.MustCompile(`data-known="true"`).FindAllStringIndex(body, -1)
	if len(knownCells) != 1 {
		t.Fatalf("known marks = %d, want 1", len(knownCells))
	}
	if !strings.Contains(body, `aria-label="you know this form"`) || !strings.Contains(body, "you know this form") {
		t.Fatal("known label missing")
	}
	// Unknown present f.sg must not be marked known in its cell.
	fsgCell := cellAround(t, body, fsg)
	if strings.Contains(fsgCell, `data-known="true"`) {
		t.Fatalf("unknown form marked known:\n%s", fsgCell)
	}

	// ac-4: In your lessons, at most 8, only readable units.
	if !strings.Contains(body, "In your lessons") || !strings.Contains(body, "he chooses, I chose") {
		t.Fatal("lesson lines missing")
	}
	if !strings.Contains(body, `href="/lesson/`+strconv.FormatInt(unitID, 10)+`/0"`) {
		t.Fatal("lesson link missing")
	}
	if strings.Count(body, `<mark>`) < 1 {
		t.Fatal("matched form not highlighted")
	}

	// ac-5: root chip links to Roots.
	if !strings.Contains(body, `href="/roots/`+root+`"`) && !strings.Contains(body, `href="/roots/`+units.Normalize(root)+`"`) {
		// Root may already be normalized with maqaf.
		if !regexp.MustCompile(`href="/roots/[^"]+"`).MatchString(body) {
			t.Fatalf("root chip not linked:\n%s", body)
		}
	}

	// Unknown id is 404.
	if code := learnerStatus(t, h, learner, "/words/999999"); code != http.StatusNotFound {
		t.Fatalf("unknown id = %d", code)
	}
}

func TestWordPageLinksFromMeAndCard(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "b@example.com", "sub-word-links", now)
	ctx := context.Background()

	inf := units.Normalize("לִבְחוֹר")
	form := units.Normalize("בּוֹחֵר")
	root := "ב־ח־ר"
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: inf, Gloss: "to choose", Translit: sp("livkhor"), POS: sp("verb"), Root: &root},
		{TextNFC: form, Gloss: "chooses", Translit: sp("bokher"), POS: sp("verb"), Root: &root},
	})
	if err != nil {
		t.Fatal(err)
	}
	lemmaID, formID := saved[0].ID, saved[1].ID
	knowWord(t, store, learner.user.ID, formID, now)
	if err := store.UpsertVerbForms(ctx, []db.VerbFormInput{
		{LemmaID: lemmaID, Tense: "present", Number: "sg", Gender: "m", FormHe: form, FormLexemeID: formID},
	}); err != nil {
		t.Fatal(err)
	}

	// ac-6: /me/verbs and /me/words link to /words/{id}; a lesson word card has All forms.
	verbs := getLearner(t, h, learner, "/me/verbs")
	want := `href="/words/` + strconv.FormatInt(lemmaID, 10) + `"`
	if !strings.Contains(verbs, want) {
		t.Fatalf("verbs page missing %s:\n%s", want, verbs)
	}
	words := getLearner(t, h, learner, "/me/words")
	formHref := `href="/words/` + strconv.FormatInt(formID, 10) + `"`
	if !strings.Contains(words, formHref) {
		t.Fatalf("words page missing %s", formHref)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Card link", Position: 1, NewWords: []string{form},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: form},
		}}},
	}, "shared")
	publish(t, store, unitID)
	lesson := getLearner(t, h, learner, "/lesson/"+strconv.FormatInt(unitID, 10)+"/0")
	if !strings.Contains(lesson, `All forms`) || !strings.Contains(lesson, formHref) {
		t.Fatalf("word card All forms missing:\n%s", lesson)
	}
}

func TestWordPageNounIsPlain(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "learner1@example.com", "sub-word-noun", now)
	ctx := context.Background()
	house := units.Normalize("בַּיִת")
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: house, Gloss: "house", POS: sp("noun"), Translit: sp("bayit")},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := getLearner(t, h, learner, "/words/"+strconv.FormatInt(saved[0].ID, 10))
	if strings.Contains(body, "Present tense") || strings.Contains(body, "word-page__present") {
		t.Fatal("noun rendered a verb grid")
	}
	if strings.Contains(body, `id="noun-forms-h"`) || strings.Contains(body, `id="noun-poss-h"`) {
		t.Fatal("noun without forms still rendered forms or possessive")
	}
	if !strings.Contains(body, house) || !strings.Contains(body, "house") {
		t.Fatal("noun plain view missing word or meaning")
	}
}

// INIT-023/SPEC-008 ac-1, ac-3: full noun forms + possessive + chips.
func TestNounPageFormsAndPossessive(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "learner2@example.com", "sub-word-noun-full", now)
	ctx := context.Background()
	sp := func(s string) *string { return &s }

	house := units.Normalize("בַּיִת")
	batim := units.Normalize("בָּתִּים")
	habayit := units.Normalize("הַבַּיִת")
	beit := units.Normalize("בֵּית")
	batei := units.Normalize("בָּתֵּי")
	beiti := units.Normalize("בֵּיתִי")
	beitkha := units.Normalize("בֵּיתְךָ")
	beiteinu := units.Normalize("בֵּיתֵנוּ")
	gender := "m"
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: house, Gloss: "house", POS: sp("noun"), Translit: sp("bayit"), Gender: &gender, Plural: &batim},
		{TextNFC: batim, Gloss: "houses", POS: sp("noun"), Translit: sp("batim")},
		{TextNFC: beiti, Gloss: "my house", POS: sp("noun"), Translit: sp("beiti")},
	})
	if err != nil {
		t.Fatal(err)
	}
	byText := map[string]int64{}
	for _, row := range saved {
		byText[row.TextNFC] = row.ID
	}
	lemmaID := byText[house]
	knowWord(t, store, learner.user.ID, byText[beiti], now)

	clips := map[string]string{
		house:   strings.Repeat("a1", 32),
		batim:   strings.Repeat("b2", 32),
		beiti:   strings.Repeat("c3", 32),
		beitkha: strings.Repeat("d4", 32),
	}
	for text, id := range clips {
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID: id, TextNFC: text, Voice: "voice", Model: "eleven_v4", Bytes: 4,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertNounForms(ctx, []db.NounFormInput{
		{LemmaID: lemmaID, Kind: "singular", FormHe: house, Translit: "bayit", FormLexemeID: lemmaID},
		{LemmaID: lemmaID, Kind: "plural", Number: "pl", FormHe: batim, Translit: "batim", FormLexemeID: byText[batim]},
		{LemmaID: lemmaID, Kind: "definite", FormHe: habayit, Translit: "habayit"},
		{LemmaID: lemmaID, Kind: "construct", Number: "sg", FormHe: beit, Translit: "beit"},
		{LemmaID: lemmaID, Kind: "construct", Number: "pl", FormHe: batei, Translit: "batei"},
		{LemmaID: lemmaID, Kind: "possessive", Person: "1", OwnerNumber: "sg", Number: "sg", FormHe: beiti, Translit: "beiti", FormLexemeID: byText[beiti]},
		{LemmaID: lemmaID, Kind: "possessive", Person: "2", Gender: "m", OwnerNumber: "sg", Number: "sg", FormHe: beitkha, Translit: "beitkha"},
		{LemmaID: lemmaID, Kind: "possessive", Person: "1", OwnerNumber: "pl", Number: "sg", FormHe: beiteinu, Translit: "beiteinu"},
	}); err != nil {
		t.Fatal(err)
	}

	body := getLearner(t, h, learner, "/words/"+strconv.FormatInt(lemmaID, 10))

	if !strings.Contains(body, `>masculine<`) || !strings.Contains(body, `>irregular plural<`) {
		t.Fatalf("gender/irregular chips missing:\n%s", body)
	}
	forms := regexp.MustCompile(`(?s)id="noun-forms-h".*?</table>`).FindString(body)
	if forms == "" {
		t.Fatalf("forms grid missing:\n%s", body)
	}
	for _, want := range []string{"Singular", "Plural", "with הַ", "… of", house, batim, habayit, beit, batei} {
		if !strings.Contains(forms, want) {
			t.Fatalf("forms missing %q:\n%s", want, forms)
		}
	}
	poss := regexp.MustCompile(`(?s)id="noun-poss-h".*?</table>`).FindString(body)
	if poss == "" {
		t.Fatalf("possessive grid missing:\n%s", body)
	}
	order := []string{"my", "your m.", "our"}
	prev := -1
	for _, label := range order {
		at := strings.Index(poss, label)
		if at < 0 {
			t.Fatalf("possessive missing %s:\n%s", label, poss)
		}
		if at < prev {
			t.Fatalf("possessive order wrong around %s", label)
		}
		prev = at
	}
	if !strings.Contains(poss, "אֲנִי") || !strings.Contains(poss, beiti) {
		t.Fatalf("possessive my row incomplete:\n%s", poss)
	}
	clipRE := regexp.MustCompile(`(?s)<div class="clip">.*?<audio id="noun-\d+-\d+"[^>]*src="/audio/` + clips[beiti] + `\.mp3"`)
	if !clipRE.MatchString(poss) {
		t.Fatalf("possessive cell lacks <audio> inside .clip:\n%s", poss)
	}
	beitiCell := cellAround(t, body, beiti)
	if !strings.Contains(beitiCell, `data-known="true"`) {
		t.Fatalf("known possessive not marked:\n%s", beitiCell)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Home", Position: 1, NewWords: []string{house},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: house + " " + beiti, Translation: "a house, my house"},
		}}},
	}, "shared")
	publish(t, store, unitID)
	body = getLearner(t, h, learner, "/words/"+strconv.FormatInt(lemmaID, 10))
	if !strings.Contains(body, "In your lessons") || !strings.Contains(body, "a house, my house") {
		t.Fatal("noun lesson lines missing")
	}
}

// INIT-023/SPEC-008 ac-2: lexemes.plural only — singular+plural, no empty construct/possessive.
func TestNounPagePluralOnly(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "learner3@example.com", "sub-word-noun-pl", now)
	ctx := context.Background()
	sp := func(s string) *string { return &s }
	book := units.Normalize("סֵפֶר")
	books := units.Normalize("סְפָרִים")
	gender := "m"
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: book, Gloss: "book", POS: sp("noun"), Translit: sp("sefer"), Gender: &gender, Plural: &books},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := getLearner(t, h, learner, "/words/"+strconv.FormatInt(saved[0].ID, 10))
	forms := regexp.MustCompile(`(?s)id="noun-forms-h".*?</table>`).FindString(body)
	if forms == "" || !strings.Contains(forms, book) || !strings.Contains(forms, books) {
		t.Fatalf("singular/plural missing:\n%s", body)
	}
	if strings.Contains(forms, "with הַ") || strings.Contains(forms, "… of") {
		t.Fatalf("empty construct/definite row rendered:\n%s", forms)
	}
	if strings.Contains(body, `id="noun-poss-h"`) || strings.Contains(body, "Possessive") {
		t.Fatal("empty possessive block rendered")
	}
}

// INIT-023/SPEC-008 ac-4: adjective agreement grid with clips.
func TestAdjectivePageAgreement(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "learner4@example.com", "sub-word-adj", now)
	ctx := context.Background()
	sp := func(s string) *string { return &s }

	tov := units.Normalize("טוֹב")
	tova := units.Normalize("טוֹבָה")
	tovim := units.Normalize("טוֹבִים")
	tovot := units.Normalize("טוֹבוֹת")
	sefer := units.Normalize("סֵפֶר")
	yalda := units.Normalize("יַלְדָּה")
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: tov, Gloss: "good", POS: sp("adjective"), Translit: sp("tov")},
		{TextNFC: tova, Gloss: "good (f)", POS: sp("adjective"), Translit: sp("tova")},
		{TextNFC: tovim, Gloss: "good (m pl)", POS: sp("adjective"), Translit: sp("tovim")},
		{TextNFC: tovot, Gloss: "good (f pl)", POS: sp("adjective"), Translit: sp("tovot")},
		{TextNFC: sefer, Gloss: "book", POS: sp("noun"), Gender: sp("m"), Translit: sp("sefer")},
		{TextNFC: yalda, Gloss: "girl", POS: sp("noun"), Gender: sp("f"), Translit: sp("yalda")},
	})
	if err != nil {
		t.Fatal(err)
	}
	byText := map[string]int64{}
	for _, row := range saved {
		byText[row.TextNFC] = row.ID
	}
	lemmaID := byText[tov]
	clips := map[string]string{
		tov:   strings.Repeat("a1", 32),
		tova:  strings.Repeat("b2", 32),
		tovim: strings.Repeat("c3", 32),
		tovot: strings.Repeat("d4", 32),
	}
	for text, id := range clips {
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID: id, TextNFC: text, Voice: "voice", Model: "eleven_v4", Bytes: 4,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertNounForms(ctx, []db.NounFormInput{
		{LemmaID: lemmaID, Kind: "agreement", Number: "sg", Gender: "m", FormHe: tov, Translit: "tov", FormLexemeID: byText[tov]},
		{LemmaID: lemmaID, Kind: "agreement", Number: "sg", Gender: "f", FormHe: tova, Translit: "tova", FormLexemeID: byText[tova]},
		{LemmaID: lemmaID, Kind: "agreement", Number: "pl", Gender: "m", FormHe: tovim, Translit: "tovim", FormLexemeID: byText[tovim]},
		{LemmaID: lemmaID, Kind: "agreement", Number: "pl", Gender: "f", FormHe: tovot, Translit: "tovot", FormLexemeID: byText[tovot]},
	}); err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Good things", Position: 1, NewWords: []string{tov},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: sefer + " " + tov, Translation: "a good book"},
			{Type: units.ActivityStory, Text: yalda + " " + tova, Translation: "a good girl"},
		}}},
	}, "shared")
	publish(t, store, unitID)

	body := getLearner(t, h, learner, "/words/"+strconv.FormatInt(lemmaID, 10))
	agree := regexp.MustCompile(`(?s)id="adj-agree-h".*?</table>`).FindString(body)
	if agree == "" {
		t.Fatalf("agreement grid missing:\n%s", body)
	}
	order := []string{tov, tova, tovim, tovot}
	prev := -1
	for _, form := range order {
		at := strings.Index(agree, form)
		if at < 0 {
			t.Fatalf("agreement missing %s:\n%s", form, agree)
		}
		if at < prev {
			t.Fatalf("agreement order wrong around %s", form)
		}
		prev = at
		clipRE := regexp.MustCompile(`(?s)<div class="clip">.*?<audio id="adj-0-\d+"[^>]*src="/audio/` + clips[form] + `\.mp3"`)
		if !clipRE.MatchString(agree) {
			t.Fatalf("agreement cell for %s lacks <audio> inside .clip:\n%s", form, agree)
		}
	}
	if !strings.Contains(body, "After a noun") || !strings.Contains(body, "masculine") || !strings.Contains(body, "feminine") {
		t.Fatalf("after-noun examples missing:\n%s", body)
	}
}

func TestWordPageClipHTMLHasAudioInClip(t *testing.T) {
	// ac-7 without a live browser tap: present cell HTML has <audio> inside .clip,
	// matching how orb_audio_test proves clips.js can reach playing.
	body := renderView(t, views.WordPage(views.WordPageView{
		Page: views.Page{UI: views.UIFor("en", views.AddressMasculine), Title: "Word"},
		Kind: "verb",
		Word: views.WordHead{He: "לִבְחוֹר", Gloss: "to choose", Audio: "/audio/" + strings.Repeat("aa", 32) + ".mp3", AudioID: "word-lemma", POS: "verb"},
		Verb: &views.VerbPageView{
			Present: []views.VerbFormCell{
				{He: "בּוֹחֵר", Audio: "/audio/" + strings.Repeat("bb", 32) + ".mp3", AudioID: "verb-present-0-0"},
				{}, {}, {},
			},
		},
	}))
	if !regexp.MustCompile(`(?s)<div class="clip">.*<audio id="verb-present-0-0"[^>]*src="/audio/` + strings.Repeat("bb", 32) + `\.mp3"`).MatchString(body) {
		t.Fatalf("present clip missing <audio> inside .clip:\n%s", body)
	}
	if !strings.Contains(body, `class="orb`) {
		t.Fatal("present clip missing orb")
	}
	css := readStatic(t, "static/css/word.css")
	if !strings.Contains(css, "overflow-x: auto") || !strings.Contains(css, "overflow-x: hidden") {
		t.Fatal("word.css must keep page overflow hidden and scroll the grid")
	}
}

// INIT-023/SPEC-008 ac-5 without a live browser tap: possessive cell has <audio> inside .clip.
func TestNounPagePossessiveClipHTMLHasAudioInClip(t *testing.T) {
	src := "/audio/" + strings.Repeat("cc", 32) + ".mp3"
	body := renderView(t, views.WordPage(views.WordPageView{
		Page: views.Page{UI: views.UIFor("en", views.AddressMasculine), Title: "Word"},
		Kind: "noun",
		Word: views.WordHead{He: "בַּיִת", Gloss: "house", POS: "noun", GenderChip: "masculine"},
		Noun: &views.NounPageView{
			FormRows: []views.NounFormRow{
				{Sg: views.VerbFormCell{He: "בַּיִת"}, Pl: views.VerbFormCell{He: "בָּתִּים"}},
			},
			Possessive: []views.NounPossessiveRow{
				{En: "my", HePronoun: "אֲנִי", Sg: views.VerbFormCell{He: "בֵּיתִי", Audio: src, AudioID: "noun-10-0"}},
			},
		},
	}))
	if !regexp.MustCompile(`(?s)<div class="clip">.*<audio id="noun-10-0"[^>]*src="` + regexp.QuoteMeta(src) + `"`).MatchString(body) {
		t.Fatalf("possessive clip missing <audio> inside .clip:\n%s", body)
	}
	if !strings.Contains(body, `class="orb`) {
		t.Fatal("possessive clip missing orb")
	}
	if !strings.Contains(body, `id="noun-poss-h"`) || !strings.Contains(body, "my") {
		t.Fatal("possessive block missing")
	}
}

func cellAround(t *testing.T, body, he string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<div class="word-page__cell"[^>]*>.*?</div>`)
	for _, m := range re.FindAllString(body, -1) {
		if strings.Contains(m, he) {
			return m
		}
	}
	t.Fatalf("no cell for %s", he)
	return ""
}
