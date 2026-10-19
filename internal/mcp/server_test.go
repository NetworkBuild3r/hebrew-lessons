package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testToken = "secret"
	hello     = "שָׁלוֹם"
	helloBare = "שלום"
	coffee    = "קָפֶה"
	book      = "סֵפֶר"
)

func TestAuthRejectAndAccept(t *testing.T) {
	h := Handler(Options{AuthToken: testToken})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	missing := postInitialize(t, srv.URL, "")
	if missing.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", missing.StatusCode)
	}
	if !strings.Contains(missing.Header.Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("challenge %q", missing.Header.Get("WWW-Authenticate"))
	}
	missing.Body.Close()

	wrong := postInitialize(t, srv.URL, "nope")
	if wrong.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d", wrong.StatusCode)
	}
	wrong.Body.Close()

	// A different length must also fail. The compare hashes both sides.
	short := postInitialize(t, srv.URL, "x")
	if short.StatusCode != http.StatusUnauthorized {
		t.Fatalf("short token status = %d", short.StatusCode)
	}
	short.Body.Close()

	session := connectMCP(t, h, testToken)
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 6 {
		t.Fatalf("tools = %d", len(tools.Tools))
	}
}

func TestEmptyConfiguredTokenRejects(t *testing.T) {
	h := Handler(Options{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	res := postInitialize(t, srv.URL, testToken)
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestAcceptHeaderReachesSDK(t *testing.T) {
	h := Handler(Options{AuthToken: testToken})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(initializeBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d body %s", res.StatusCode, body)
	}
	if !strings.Contains(string(body), "Accept") {
		t.Fatalf("body %s", body)
	}
}

func TestChainAcceptsStaticToken(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	h := Handler(Options{
		Validator: Chain{rejectToken{}, StaticToken{Token: testToken, Actor: "author-7"}},
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	})
	session := connectMCP(t, h, testToken)
	out := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: helloDoc()})
	if mem.Actor(out.UnitID) != "author-7" {
		t.Fatalf("actor = %q", mem.Actor(out.UnitID))
	}
}

func TestToolsResourcesAndIdempotency(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{
		TextNFC:  coffee,
		Bare:     "קפה",
		Gloss:    "coffee",
		POS:      "noun",
		Gender:   "m",
		Translit: "kafe",
	})
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	gotNames := map[string]bool{}
	for _, tool := range tools.Tools {
		gotNames[tool.Name] = true
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"type":"object"`) && !strings.Contains(string(raw), `"type": "object"`) {
			t.Fatalf("%s schema %s", tool.Name, raw)
		}
		if !strings.Contains(string(raw), "additionalProperties") {
			t.Fatalf("%s schema is not closed: %s", tool.Name, raw)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint && tool.Name != "put_unit" && tool.Name != "generate_audio" {
			t.Fatalf("%s annotations %#v", tool.Name, tool.Annotations)
		}
	}
	for _, name := range []string{"get_context", "lookup", "validate_unit", "put_unit", "generate_audio", "get_learner_words"} {
		if !gotNames[name] {
			t.Fatalf("missing %s in %#v", name, gotNames)
		}
	}
	contextTool := tools.Tools[indexOf(tools, "get_context")]
	if !strings.Contains(contextTool.Description, "cardsTruncated") || !strings.Contains(contextTool.Description, "get_learner_words") || !strings.Contains(contextTool.Description, "mixTarget") || !strings.Contains(contextTool.Description, "lapses") {
		t.Fatalf("get_context description %s", contextTool.Description)
	}
	wordsTool := tools.Tools[indexOf(tools, "get_learner_words")]
	if !strings.Contains(wordsTool.Description, "nextCursor") || !strings.Contains(wordsTool.Description, "due") || !strings.Contains(wordsTool.Description, "lapses") {
		t.Fatalf("get_learner_words description %s", wordsTool.Description)
	}
	if tools.Tools[indexOf(tools, "put_unit")].Annotations.ReadOnlyHint {
		t.Fatal("put_unit is marked read-only")
	}
	if !tools.Tools[indexOf(tools, "put_unit")].Annotations.IdempotentHint {
		t.Fatal("put_unit is not marked idempotent")
	}

	resources, err := session.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources.Resources) != 3 {
		t.Fatalf("resources = %d", len(resources.Resources))
	}
	schemaText := readResource(t, session, resourceUnitSchema)
	if !strings.Contains(schemaText, units.ActivityListenPick) || !strings.Contains(schemaText, "additionalProperties") {
		t.Fatalf("unit schema %s", schemaText)
	}
	catalogText := readResource(t, session, resourceActivities)
	for _, typ := range []string{
		units.ActivityListenPick, units.ActivityRecall, units.ActivityProduce,
		units.ActivityCloze, units.ActivityPattern, units.ActivityStory, units.ActivityShadow,
	} {
		if !strings.Contains(catalogText, typ) {
			t.Fatalf("catalog missing %s", typ)
		}
	}
	rulesText := readResource(t, session, resourceRules)
	if !strings.Contains(rulesText, units.RuleAudioMissing) || !strings.Contains(rulesText, `"id":"N1"`) {
		t.Fatalf("rules %s", rulesText)
	}
	if !strings.Contains(rulesText, `"maxNewWordsPerUnit":8`) {
		t.Fatalf("budgets %s", rulesText)
	}
	if !strings.Contains(rulesText, `"enforced":false`) || !strings.Contains(rulesText, "Not an enforced rule") || !strings.Contains(rulesText, "struggled words first") || !strings.Contains(rulesText, "minimal-pair") || !strings.Contains(rulesText, "assumes no learner data") {
		t.Fatalf("guidance %s", rulesText)
	}

	ctx := getContext(t, session, 0)
	if ctx.Placement.Taken {
		t.Fatal("placement was taken")
	}
	untaken := callRaw(t, session, "get_context", getContextInput{Position: 0})
	if !strings.Contains(untaken, `"placement":{"taken":false}`) || strings.Contains(untaken, "adjustedScore") || strings.Contains(untaken, `"reliable"`) {
		t.Fatalf("untaken placement %s", untaken)
	}
	if !strings.Contains(untaken, `"struggles":`) || !strings.Contains(untaken, `"lexemes":[]`) || !strings.Contains(untaken, `"confusions":[]`) {
		t.Fatalf("untaken struggles %s", untaken)
	}
	if ctx.Budgets != currentBudgets() {
		t.Fatalf("budgets %+v", ctx.Budgets)
	}
	if len(ctx.Outline) != 0 || len(ctx.KnownLexemes) != 0 {
		t.Fatalf("context %+v", ctx)
	}

	hit := call[lookupOutput](t, session, "lookup", lookupInput{Query: hello})
	if len(hit.Hits) != 1 || hit.Hits[0].Gloss != "hello; peace" || hit.Hits[0].Translit != "shalom" || !hit.Hits[0].AudioPresent {
		t.Fatalf("pointed hit %+v", hit)
	}
	bareHit := call[lookupOutput](t, session, "lookup", lookupInput{Query: helloBare})
	if len(bareHit.Hits) != 1 || bareHit.Hits[0].TextNFC != hello {
		t.Fatalf("bare hit %+v", bareHit)
	}
	glossHit := call[lookupOutput](t, session, "lookup", lookupInput{Query: "coffee"})
	if len(glossHit.Hits) != 1 || glossHit.Hits[0].POS != "noun" || glossHit.Hits[0].Gender != "m" {
		t.Fatalf("gloss hit %+v", glossHit)
	}
	miss := call[lookupOutput](t, session, "lookup", lookupInput{Query: "coff"})
	if len(miss.Hits) != 0 || len(miss.Suggestions) != 1 || miss.Suggestions[0].TextNFC != coffee {
		t.Fatalf("suggestions %+v", miss)
	}
	if msg := callErr(t, session, "lookup", lookupInput{}); !strings.Contains(msg, "Query is empty") {
		t.Fatalf("empty query %q", msg)
	}

	doc := helloDoc()
	want := units.Validate(t.Context(), &doc, mem)
	dry := call[validateOutput](t, session, "validate_unit", validateInput{Position: 0, Doc: doc})
	if !sameIssues(dry.Issues, want) {
		t.Fatalf("validate issues %#v\nwant %#v", dry.Issues, want)
	}
	blocks, warns := countIssues(want)
	if dry.OK != (blocks == 0) || dry.Blocks != blocks || dry.Warns != warns {
		t.Fatalf("summary %+v blocks %d warns %d", dry, blocks, warns)
	}
	if len(mustList(t, mem)) != 0 {
		t.Fatal("validate_unit wrote a unit")
	}

	first := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: doc})
	if first.UnitID == 0 || first.Revision != 1 || first.Unchanged {
		t.Fatalf("first put %+v", first)
	}
	if !sameIssues(first.Issues, want) {
		t.Fatalf("put issues %#v", first.Issues)
	}
	if mem.Actor(first.UnitID) != defaultActor {
		t.Fatalf("actor %q", mem.Actor(first.UnitID))
	}
	second := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &first.UnitID, Position: 0, Doc: doc})
	if second.Revision != 1 || !second.Unchanged || second.UnitID != first.UnitID {
		t.Fatalf("idempotent put %+v", second)
	}
	moved := doc
	moved.Title = "Hello again"
	third := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &first.UnitID, Position: 0, Doc: moved})
	if third.Unchanged || third.Revision != 2 {
		t.Fatalf("changed put %+v", third)
	}
	shifted := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &first.UnitID, Position: 2, Doc: moved})
	if shifted.Unchanged || shifted.Revision != 3 {
		t.Fatalf("position change %+v", shifted)
	}

	bad := doc
	bad.Title = ""
	saved := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 1, Doc: bad})
	if saved.Unchanged || saved.Revision != 1 {
		t.Fatalf("blocked draft %+v", saved)
	}
	if !hasRule(saved.Issues, units.RuleSchemaRequired) {
		t.Fatalf("blocked draft issues %#v", saved.Issues)
	}

	missing := int64(99)
	if msg := callErr(t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &missing, Position: 0, Doc: doc}); !strings.Contains(msg, "No unit has id 99") {
		t.Fatalf("missing unit %q", msg)
	}
	before := len(mustList(t, mem))
	if msg := callErr(t, session, "put_unit", map[string]any{"scope": ScopeShared, "position": 0, "doc": "not-a-unit"}); !strings.Contains(msg, `want "object"`) {
		t.Fatalf("malformed %q", msg)
	}
	if msg := callErr(t, session, "put_unit", map[string]any{"scope": ScopeShared, "position": 0, "doc": map[string]any{"nope": true}}); !strings.Contains(msg, "nope") {
		t.Fatalf("unknown field %q", msg)
	}
	if msg := callErr(t, session, "put_unit", map[string]any{
		"scope":    ScopeShared,
		"position": 0,
		"doc":      map[string]any{"title": 1, "situation": "x", "concept": "y", "newWords": []any{}, "lessons": []any{}},
	}); !strings.Contains(msg, "title") {
		t.Fatalf("wrong title type %q", msg)
	}
	if len(mustList(t, mem)) != before {
		t.Fatal("malformed put wrote a unit")
	}
	if msg := callErr(t, session, "validate_unit", validateInput{Position: -1, Doc: doc}); !strings.Contains(msg, "Position must be 0 or greater") {
		t.Fatalf("negative %q", msg)
	}

	mem.SetPlacement(9, Placement{
		TakenAt:                time.Date(2026, 10, 18, 11, 0, 0, 0, time.UTC),
		Finished:               true,
		Level:                  2,
		CardsRated:             12,
		StopReason:             "band_stop",
		EstimatedKnownWords:    10,
		SuggestedStartPosition: 1,
	}, []string{coffee}, nil)
	learnerZero := int64(0)
	other := call[getContextOutput](t, session, "get_context", getContextInput{Position: 3, LearnerID: &learnerZero})
	if other.Placement.Taken {
		t.Fatal("user 0 saw user 9's placement")
	}
	takenAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	mem.SetPlacement(0, Placement{
		TakenAt:    takenAt,
		Finished:   true,
		Sittings:   1,
		Level:      0,
		CardsRated: 4,
		StopReason: "cap_cards",
		BandScores: []BandScore{{
			Band: 0, Rated: 4, Again: 1, Hard: 1, Good: 1, Easy: 1, DontKnow: 0, GoodPlusRate: 0.5,
		}},
		EstimatedKnownWords:    40,
		SuggestedStartPosition: 0,
	}, []string{hello, coffee}, []string{book, helloBare})
	again := call[getContextOutput](t, session, "get_context", getContextInput{Position: 3, LearnerID: &learnerZero})
	if !again.Placement.Taken || !again.Placement.TakenAt.Equal(takenAt) || !again.Placement.Finished || again.Placement.InProgress || again.Placement.Sittings != 1 || again.Placement.StopReason != "cap_cards" || again.Placement.CardsRated != 4 {
		t.Fatalf("placement %+v", again.Placement)
	}
	if again.Placement.Level != 0 || again.Placement.SuggestedStartPosition != 0 || again.Placement.EstimatedKnownWords != 40 {
		t.Fatalf("placement zeros %+v", again.Placement)
	}
	band := again.Placement.Bands
	if len(band) != 1 || band[0].Band != 0 || band[0].Rated != 4 || band[0].Again != 1 || band[0].Hard != 1 || band[0].Good != 1 || band[0].Easy != 1 || band[0].DontKnow != 0 || band[0].GoodPlusRate != 0.5 {
		t.Fatalf("bands %+v", band)
	}
	if len(again.Placement.KnownLexemes) != 2 || again.Placement.KnownLexemes[0] != coffee || again.Placement.KnownLexemes[1] != hello {
		t.Fatalf("known lexemes %+v", again.Placement.KnownLexemes)
	}
	if len(again.Placement.LearningLexemes) != 2 || again.Placement.LearningLexemes[0] != book || again.Placement.LearningLexemes[1] != helloBare {
		t.Fatalf("learning lexemes %+v", again.Placement.LearningLexemes)
	}
	takenText := callRaw(t, session, "get_context", getContextInput{Position: 3, LearnerID: &learnerZero})
	for _, want := range []string{
		`"taken":true`, `"finished":true`, `"sittings":1`, `"inProgress":false`, `"level":0`, `"cardsRated":4`, `"stopReason":"cap_cards"`,
		`"suggestedStartPosition":0`, `"bands":`, `"rated":4`, `"again":1`, `"hard":1`,
		`"good":1`, `"easy":1`, `"dontKnow":0`, `"goodPlusRate":0.5`, `"learningLexemes":`, hello, book,
	} {
		if !strings.Contains(takenText, want) {
			t.Fatalf("taken placement missing %s in %s", want, takenText)
		}
	}
	for _, gone := range []string{"adjustedScore", `"h":`, `"passed"`, `"reliable"`, `"falseAlarmRate"`, `"bandScores"`, `"shown"`, `"dontknow"`, `"struggleRate"`, `"cap_time"`} {
		if strings.Contains(takenText, gone) {
			t.Fatalf("old placement field %s in %s", gone, takenText)
		}
	}
	mem.SetPlacement(0, Placement{
		TakenAt:    takenAt,
		Finished:   false,
		Sittings:   2,
		CardsRated: 20,
	}, nil, nil)
	progress := call[getContextOutput](t, session, "get_context", getContextInput{Position: 3, LearnerID: &learnerZero})
	if !progress.Placement.Taken || progress.Placement.Finished || !progress.Placement.InProgress || progress.Placement.Sittings != 2 || progress.Placement.StopReason != "" || progress.Placement.CardsRated != 20 {
		t.Fatalf("in progress %+v", progress.Placement)
	}
	found := false
	for _, word := range again.KnownLexemes {
		if word == hello {
			found = true
		}
	}
	if !found {
		t.Fatalf("known %+v", again.KnownLexemes)
	}
	var sawHello bool
	for _, row := range again.Outline {
		if row.Title == "Hello again" && row.Position == 2 && row.Status == "draft" {
			sawHello = true
		}
	}
	if !sawHello {
		t.Fatalf("outline %+v", again.Outline)
	}
}

func TestScopeStruggleAndGuidance(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	doc := helloDoc()
	shared := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: doc})
	same := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &shared.UnitID, Position: 0, Doc: doc})
	if !same.Unchanged || same.Revision != 1 {
		t.Fatalf("same scope %+v", same)
	}
	mine := doc
	mine.Title = "For learner 7"
	custom := call[putOutput](t, session, "put_unit", putInput{Scope: " learner:7 ", Position: 1, Doc: mine})
	if custom.Unchanged || custom.Revision != 1 {
		t.Fatalf("custom create %+v", custom)
	}
	retargeted := call[putOutput](t, session, "put_unit", putInput{Scope: "learner:9", UnitID: &custom.UnitID, Position: 1, Doc: mine})
	if retargeted.Unchanged || retargeted.Revision != 2 {
		t.Fatalf("scope change %+v", retargeted)
	}
	back := call[putOutput](t, session, "put_unit", putInput{Scope: "learner:7", UnitID: &custom.UnitID, Position: 1, Doc: mine})
	if back.Unchanged || back.Revision != 3 {
		t.Fatalf("scope change back %+v", back)
	}
	held := call[putOutput](t, session, "put_unit", putInput{Scope: "learner:7", UnitID: &custom.UnitID, Position: 1, Doc: mine})
	if !held.Unchanged || held.Revision != 3 {
		t.Fatalf("same custom scope %+v", held)
	}
	theirs := doc
	theirs.Title = "For learner 8"
	call[putOutput](t, session, "put_unit", putInput{Scope: "learner:8", Position: 1, Doc: theirs})

	before := len(mustList(t, mem))
	for _, scope := range []string{"", "nope", "learner:", "learner:01", "learner:-3", "Shared"} {
		msg := callErr(t, session, "put_unit", putInput{Scope: scope, Position: 0, Doc: doc})
		if !strings.Contains(msg, "Scope must be") {
			t.Fatalf("scope %q: %s", scope, msg)
		}
	}
	if len(mustList(t, mem)) != before {
		t.Fatal("bad scope wrote a unit")
	}

	onlyShared := getContext(t, session, 2)
	if len(onlyShared.Outline) != 1 || onlyShared.Outline[0].Scope != ScopeShared || onlyShared.Outline[0].Title != doc.Title {
		t.Fatalf("shared outline %+v", onlyShared.Outline)
	}
	if onlyShared.Placement.Taken || len(onlyShared.Struggles.Lexemes) != 0 || len(onlyShared.Struggles.Confusions) != 0 || len(onlyShared.CompletedSharedUnits) != 0 {
		t.Fatalf("shared view leaked learner data %+v", onlyShared)
	}
	if len(onlyShared.KnownLexemes) != 1 || onlyShared.KnownLexemes[0] != hello {
		t.Fatalf("shared known %+v", onlyShared.KnownLexemes)
	}

	recent := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	mem.SetStruggle(7, []StruggleLexeme{
		{LexemeID: 11, TextNFC: "א", Gloss: "alef", Reviews: 2, Lapses: 9, AgainRate: 1, Misses7d: 0, LastSeen: recent},
		{LexemeID: 12, TextNFC: "ג", Gloss: "gimel", Reviews: 9, Lapses: 3, AgainRate: 0.1, Misses7d: 2, LastSeen: recent, ConfusionKeys: []string{"ג/א"}},
		{LexemeID: 13, TextNFC: "ב", Gloss: "bet", Reviews: 4, Lapses: 0, AgainRate: 0.2, Misses7d: 2, LastSeen: recent},
		{LexemeID: 14, TextNFC: "ד", Gloss: "dalet", Reviews: 5, Lapses: 0, AgainRate: 0.9, Misses7d: 1, LastSeen: recent},
	}, []StruggleConfusion{
		{Kind: "agreement", A: "א", Count: 2, LastAt: recent},
		{Kind: "pair", A: "ב", B: "א", Count: 4, LastAt: recent},
	})
	early := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	mem.SetCompletedSharedUnits(7, []UnitCompletion{
		{UnitID: 9, Position: 2, CompletedAt: later},
		{UnitID: 3, Position: 0, CompletedAt: early},
	})
	mem.SetCompletedSharedUnits(8, []UnitCompletion{
		{UnitID: 4, Position: 0, CompletedAt: early},
	})

	learner7 := int64(7)
	top := call[getContextOutput](t, session, "get_context", getContextInput{Position: 2, LearnerID: &learner7, TopN: 1})
	if len(top.Outline) != 2 {
		t.Fatalf("learner outline %+v", top.Outline)
	}
	var sawShared, sawMine bool
	for _, row := range top.Outline {
		switch row.Scope {
		case ScopeShared:
			sawShared = row.Title == doc.Title
		case "learner:7":
			sawMine = row.Title == "For learner 7"
		default:
			t.Fatalf("unexpected scope %+v", row)
		}
	}
	if !sawShared || !sawMine {
		t.Fatalf("learner outline %+v", top.Outline)
	}
	stats, err := mem.StruggleStats(t.Context(), 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Lexemes) != 4 || stats.Lexemes[0].TextNFC != "א" || stats.Lexemes[1].TextNFC != "ב" || stats.Lexemes[2].TextNFC != "ג" || stats.Lexemes[3].TextNFC != "ד" {
		t.Fatalf("raw lexemes sorted by text %+v", stats.Lexemes)
	}
	if len(stats.Top) != 1 || stats.Top[0] != "ג" || len(stats.Confusions) != 2 {
		t.Fatalf("topN stats %+v", stats)
	}
	if len(top.Struggles.Lexemes) != 1 || top.Struggles.Lexemes[0].TextNFC != stats.Top[0] || top.Struggles.Lexemes[0].Gloss != "gimel" || top.Struggles.Lexemes[0].Reviews != 9 || top.Struggles.Lexemes[0].Lapses != 3 || top.Struggles.Lexemes[0].Misses7d != 2 || len(top.Struggles.Lexemes[0].ConfusionKeys) != 1 || top.Struggles.Lexemes[0].ConfusionKeys[0] != "ג/א" || !top.Struggles.Lexemes[0].LastSeen.Equal(recent) {
		t.Fatalf("top row %+v", top.Struggles.Lexemes)
	}
	if len(top.Struggles.Confusions) != 2 || top.Struggles.Confusions[0].Kind != "agreement" || top.Struggles.Confusions[0].A != "א" || top.Struggles.Confusions[0].B != "" || top.Struggles.Confusions[1].Kind != "pair" || top.Struggles.Confusions[1].A != "ב" || top.Struggles.Confusions[1].B != "א" || top.Struggles.Confusions[1].Count != 4 {
		t.Fatalf("confusions %+v", top.Struggles.Confusions)
	}
	topText := callRaw(t, session, "get_context", getContextInput{Position: 2, LearnerID: &learner7, TopN: 1})
	if strings.Contains(topText, `"lexemeId"`) || strings.Contains(topText, `"top"`) || strings.Contains(topText, `"lastReviewAt"`) || strings.Contains(topText, `"struggleRate"`) {
		t.Fatalf("struggle leaked store-only fields %s", topText)
	}
	if !strings.Contains(topText, `"lastSeen"`) || !strings.Contains(topText, `"confusionKeys"`) || !strings.Contains(topText, `"agreement"`) {
		t.Fatalf("struggle shape %s", topText)
	}

	mem.SetPlacement(7, Placement{TakenAt: early, Finished: true, StopReason: "exhausted"}, []string{coffee}, nil)
	mem.SetCompletedUnitLexemes(7, []string{book, coffee})
	all := call[getContextOutput](t, session, "get_context", getContextInput{Position: 2, LearnerID: &learner7})
	full, err := mem.StruggleStats(t.Context(), 7, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Struggles.Lexemes) != 4 || len(full.Top) != 4 {
		t.Fatalf("default top %+v store %+v", all.Struggles.Lexemes, full.Top)
	}
	for i, text := range full.Top {
		if all.Struggles.Lexemes[i].TextNFC != text {
			t.Fatalf("tool order %d = %s, top %v", i, all.Struggles.Lexemes[i].TextNFC, full.Top)
		}
	}
	if full.Top[0] != "ג" || full.Top[1] != "ב" || full.Top[2] != "ד" || full.Top[3] != "א" {
		t.Fatalf("rank %+v", full.Top)
	}
	if all.Struggles.Lexemes[3].Misses7d != 0 || !all.Struggles.Lexemes[3].LastSeen.Equal(recent) {
		t.Fatalf("row fields %+v", all.Struggles.Lexemes[3])
	}
	if len(all.KnownLexemes) != 2 || all.KnownLexemes[0] != book || all.KnownLexemes[1] != coffee {
		t.Fatalf("learner known %+v", all.KnownLexemes)
	}
	if len(all.CompletedSharedUnits) != 2 || all.CompletedSharedUnits[0].UnitID != 3 || !all.CompletedSharedUnits[0].CompletedAt.Equal(early) || all.CompletedSharedUnits[1].Position != 2 {
		t.Fatalf("completed %+v", all.CompletedSharedUnits)
	}

	learner8 := int64(8)
	eight := call[getContextOutput](t, session, "get_context", getContextInput{Position: 2, LearnerID: &learner8})
	for _, row := range eight.Outline {
		if row.Scope == "learner:7" {
			t.Fatalf("learner 8 saw %+v", eight.Outline)
		}
	}
	if len(eight.CompletedSharedUnits) != 1 || eight.CompletedSharedUnits[0].UnitID != 4 {
		t.Fatalf("learner 8 completed %+v", eight.CompletedSharedUnits)
	}
	if len(eight.Struggles.Lexemes) != 0 || len(eight.Struggles.Confusions) != 0 {
		t.Fatalf("learner 8 struggle %+v", eight.Struggles)
	}
	if len(eight.KnownLexemes) != 0 {
		t.Fatalf("learner 8 known %+v", eight.KnownLexemes)
	}

	neg := int64(-1)
	if msg := callErr(t, session, "get_context", getContextInput{LearnerID: &neg}); !strings.Contains(msg, "learnerId must be 0 or greater") {
		t.Fatalf("negative learner %q", msg)
	}
	if msg := callErr(t, session, "get_context", getContextInput{TopN: -1}); !strings.Contains(msg, "topN must be 0 or greater") {
		t.Fatalf("negative top %q", msg)
	}
}

func TestNilStoresAreToolErrors(t *testing.T) {
	session := connectMCP(t, Handler(Options{AuthToken: testToken}), testToken)
	if msg := callErr(t, session, "get_context", getContextInput{}); !strings.Contains(msg, "not connected") {
		t.Fatalf("get_context %q", msg)
	}
	if msg := callErr(t, session, "put_unit", putInput{Scope: ScopeShared, Doc: helloDoc()}); !strings.Contains(msg, "not saved") {
		t.Fatalf("put_unit %q", msg)
	}
}

func TestMemoryPutRace(t *testing.T) {
	mem := NewMemory()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doc := helloDoc()
			doc.Title = doc.Title + string(rune('a'+i))
			doc.Position = i
			if _, err := mem.PutUnit(context.Background(), nil, &doc, ScopeShared); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	rows, err := mem.ListUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatalf("units = %d", len(rows))
	}
}

type rejectToken struct{}

func (rejectToken) Validate(context.Context, string, *http.Request) (Principal, error) {
	return Principal{}, ErrUnauthorized
}

func helloLexeme() Lexeme {
	return Lexeme{
		TextNFC:      hello,
		Bare:         helloBare,
		Gloss:        "hello; peace",
		POS:          "interjection",
		Translit:     "shalom",
		AudioPresent: true,
	}
}

func helloDoc() units.Unit {
	return units.Unit{
		Title:     "Hello",
		Situation: "Meeting a friend",
		Concept:   "Greeting",
		NewWords:  []string{hello},
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{
				{Type: units.ActivityListenPick, Text: hello},
				{Type: units.ActivityRecall, Text: hello},
			},
		}},
	}
}

func TestPlacementJSONKeepsZeros(t *testing.T) {
	raw, err := json.Marshal(placementView{})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"taken":false}` {
		t.Fatalf("untaken %s", raw)
	}
	raw, err = json.Marshal(placementView{
		Taken:                  true,
		Level:                  -1,
		SuggestedStartPosition: 0,
		Finished:               false,
		StopReason:             "",
		Bands:                  []BandScore{{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		`"taken":true`,
		`"finished":false`,
		`"sittings":0`,
		`"inProgress":false`,
		`"level":-1`,
		`"cardsRated":0`,
		`"stopReason":""`,
		`"suggestedStartPosition":0`,
		`"dontKnow":0`,
		`"goodPlusRate":0`,
		`"knownLexemes":[]`,
		`"learningLexemes":[]`,
		`"estimatedKnownWords":0`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
	for _, gone := range []string{"adjustedScore", `"reliable"`, `"falseAlarmRate"`, `"bandScores"`, `"dontknow"`, `"struggleRate"`, `"cap_time"`} {
		if strings.Contains(text, gone) {
			t.Fatalf("old field %s in %s", gone, text)
		}
	}
	raw, err = json.Marshal(placementView{Taken: true, Finished: false, Sittings: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"sittings":2`) || !strings.Contains(string(raw), `"inProgress":true`) || !strings.Contains(string(raw), `"finished":false`) {
		t.Fatalf("in progress %s", raw)
	}
}

func getContext(t *testing.T, session *mcp.ClientSession, position int) getContextOutput {
	t.Helper()
	return call[getContextOutput](t, session, "get_context", getContextInput{Position: position})
}

func mustList(t *testing.T, mem *Memory) []UnitOutline {
	t.Helper()
	rows, err := mem.ListUnits(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func indexOf(tools *mcp.ListToolsResult, name string) int {
	for i, tool := range tools.Tools {
		if tool.Name == name {
			return i
		}
	}
	return 0
}

func sameIssues(got, want []units.Issue) bool {
	if len(want) == 0 {
		return len(got) == 0
	}
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func countIssues(issues []units.Issue) (blocks, warns int) {
	for _, issue := range issues {
		switch issue.Severity {
		case units.SeverityBlock:
			blocks++
		case units.SeverityWarn:
			warns++
		}
	}
	return blocks, warns
}

func hasRule(issues []units.Issue, rule string) bool {
	for _, issue := range issues {
		if issue.Rule == rule {
			return true
		}
	}
	return false
}

func readResource(t *testing.T, session *mcp.ClientSession, uri string) string {
	t.Helper()
	res, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("%s contents %d", uri, len(res.Contents))
	}
	return res.Contents[0].Text
}

func call[T any](t *testing.T, session *mcp.ClientSession, name string, args any) T {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return decode[T](t, res)
}

func callRaw(t *testing.T, session *mcp.ClientSession, name string, args any) string {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	return resultText(res)
}

func callErr(t *testing.T, session *mcp.ClientSession, name string, args any) string {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s protocol error: %v", name, err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("%s: want isError", name)
	}
	return resultText(res)
}

func decode[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func resultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func connectMCP(t *testing.T, h http.Handler, token string) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "author-test", Version: "0"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
		HTTPClient:           &http.Client{Transport: bearerTrip{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

type bearerTrip struct{ token string }

func (b bearerTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if b.token != "" {
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

const initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

func postInitialize(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(initializeBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
