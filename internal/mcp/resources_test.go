package mcp_test

// INIT-015/SPEC-022: the authoring guide, served as hebrew://authoring and mirrored into the repo
// skill. These tests read the text a client reads, and they do not trust it:
//
//   - the steps name the real tools in the documented order (ac-1);
//   - every validator rule id has a row in the rule table, with the right severity (ac-1);
//   - the repo skill carries the same body as the resource (ac-2);
//   - both worked examples are parsed out of the text and validated with units.ValidateWithBrief
//     against the brief printed next to them, so an example cannot drift from the validator;
//   - a client follows only the steps and the example from the text, against a real handler and a
//     real store, with a fake text-to-speech, and ends with a published unit (ac-3).
//
// No test here builds a real provider client. The fake provider never leaves the process.

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	resAuthoringURI = "hebrew://authoring"
	resSkillPath    = "../../.cursor/skills/hebrew-lesson-author/SKILL.md"
	resGuidePath    = "resources/authoring.md"
	resCaller       = "resources-test-caller"
)

// resStepTools is the order of the tools in the steps of the guide.
var resStepTools = []string{
	"get_context", "get_lesson_brief", "lookup", "upsert_lexemes",
	"generate_audio", "validate_unit", "put_unit", "add_coach_note",
}

// ---- reading the guide ---------------------------------------------------------------

func resConnect(t *testing.T) *sdkmcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(mcp.Handler(mcp.Options{AuthToken: resCaller}))
	t.Cleanup(srv.Close)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "resources-test", Version: "0"}, nil)
	session, err := client.Connect(t.Context(), &sdkmcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
		HTTPClient:           &http.Client{Transport: apBearer{token: resCaller}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func resRead(t *testing.T, s *sdkmcp.ClientSession, uri string) string {
	t.Helper()
	res, err := s.ReadResource(t.Context(), &sdkmcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("read %s: %v", uri, err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("read %s: %d contents", uri, len(res.Contents))
	}
	return res.Contents[0].Text
}

func resToolNames(t *testing.T, s *sdkmcp.ClientSession) []string {
	t.Helper()
	res, err := s.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// resGuide is the text a client reads from the server.
func resGuide(t *testing.T) string {
	t.Helper()
	return resRead(t, resConnect(t), resAuthoringURI)
}

func resLF(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

var (
	resBackticked = regexp.MustCompile("`([a-z][a-z_]*)`")
	resStepLine   = regexp.MustCompile(`^(\d+)\. `)
	resFence      = regexp.MustCompile("^```(?:json|text) (example-[12]-[a-z]+)$")
)

// resSteps reads the numbered steps of the "Steps" section. It returns how many numbered steps
// there are and the real tools they name, in order of first mention in each step.
func resSteps(text string, tools []string) (numbered int, order []string) {
	in := false
	for _, line := range strings.Split(resLF(text), "\n") {
		if strings.HasPrefix(line, "## ") {
			in = line == "## Steps"
			continue
		}
		if !in || !resStepLine.MatchString(line) {
			continue
		}
		numbered++
		for _, m := range resBackticked.FindAllStringSubmatch(line, -1) {
			if !slices.Contains(tools, m[1]) {
				continue
			}
			if len(order) > 0 && order[len(order)-1] == m[1] {
				continue
			}
			order = append(order, m[1])
		}
	}
	return numbered, order
}

// resBlocks returns the fenced blocks of the guide that carry an example name, such as
// "json example-1-unit", keyed by that name.
func resBlocks(text string) map[string]string {
	out := map[string]string{}
	name := ""
	var b strings.Builder
	for _, line := range strings.Split(resLF(text), "\n") {
		switch {
		case name == "":
			if m := resFence.FindStringSubmatch(line); m != nil {
				name = m[1]
				b.Reset()
			}
		case line == "```":
			out[name] = b.String()
			name = ""
		default:
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return out
}

// resRuleRows returns the rule table of the guide: rule id to severity.
func resRuleRows(text string) map[string]string {
	row := regexp.MustCompile("(?m)^\\| `([a-z]+\\.[a-z_.]+)` \\| (block|warn) \\|")
	out := map[string]string{}
	for _, m := range row.FindAllStringSubmatch(resLF(text), -1) {
		out[m[1]] = m[2]
	}
	return out
}

// ---- ac-1: the tools, the rules, the activity types ----------------------------------

func TestResourcesAuthoringNamesEveryTool(t *testing.T) {
	s := resConnect(t)
	text := resRead(t, s, resAuthoringURI)
	tools := resToolNames(t, s)
	for _, name := range resStepTools {
		if !slices.Contains(tools, name) {
			t.Fatalf("%s is not a tool of the server", name)
		}
		if !strings.Contains(text, "`"+name+"`") {
			t.Errorf("the guide never names %s", name)
		}
	}
	for _, name := range []string{"record_taught_words", "remove_taught_words"} {
		if !slices.Contains(tools, name) {
			t.Fatalf("%s is not a tool of the server", name)
		}
		if !strings.Contains(text, "`"+name+"`") {
			t.Errorf("the guide never names %s", name)
		}
	}
	numbered, order := resSteps(text, tools)
	if numbered != 8 {
		t.Errorf("the guide has %d numbered steps, want 8", numbered) // INIT-030/SPEC-006: steps 0–7
	}
	if !slices.Equal(order, resStepTools) {
		t.Errorf("the steps name the tools in this order:\n  %v\nwant:\n  %v", order, resStepTools)
	}
}

func TestResourcesAuthoringNamesEveryRule(t *testing.T) {
	s := resConnect(t)
	text := resRead(t, s, resAuthoringURI)
	rows := resRuleRows(text)

	for _, r := range units.Rules {
		got, ok := rows[r.ID]
		switch {
		case !ok:
			t.Errorf("the rule table has no row for %s", r.ID)
		case got != r.Severity:
			t.Errorf("the rule table says %s is %s, and the validator says %s", r.ID, got, r.Severity)
		}
	}
	// brief.revise_started is an MCP put_unit block, not a units.Rules entry. INIT-030/SPEC-006.
	mcpOnly := map[string]bool{mcp.RuleBriefReviseStarted: true}
	for id := range rows {
		if mcpOnly[id] {
			continue
		}
		if !slices.ContainsFunc(units.Rules, func(r units.Rule) bool { return r.ID == id }) {
			t.Errorf("the rule table has a row for %s, which is not a rule", id)
		}
	}
	if _, ok := rows[mcp.RuleBriefReviseStarted]; !ok {
		t.Errorf("the rule table has no row for %s", mcp.RuleBriefReviseStarted)
	}

	// The same ids are in the catalog the server serves at hebrew://rules.
	var served struct {
		Rules []struct {
			ID string `json:"id"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(resRead(t, s, "hebrew://rules")), &served); err != nil {
		t.Fatal(err)
	}
	for _, r := range served.Rules {
		if _, ok := rows[r.ID]; !ok {
			t.Errorf("hebrew://rules serves %s and the guide does not explain it", r.ID)
		}
	}
	if len(served.Rules) != len(units.Rules) || len(rows) != len(units.Rules)+len(mcpOnly) {
		t.Errorf("rules: %d served, %d in the validator, %d in the guide (want validator+%d mcp-only)", len(served.Rules), len(units.Rules), len(rows), len(mcpOnly))
	}
}

func TestResourcesAuthoringNamesEveryActivityType(t *testing.T) {
	text := resLF(resGuide(t))
	types := []string{
		units.ActivityListenPick, units.ActivityRecall, units.ActivityProduce, units.ActivityCloze,
		units.ActivityPattern, units.ActivityStory, units.ActivityShadow, units.ActivityDialogue, units.ActivitySay,
		units.ActivityOrder, // INIT-029/SPEC-008
	}
	for _, typ := range types {
		if !strings.Contains(text, "| `"+typ+"` |") {
			t.Errorf("the activity table has no row for %s", typ)
		}
	}
	// The unit shape is shown, and a conversation is a dialogue with two speakers.
	for _, want := range []string{`"speaker": "a"`, `"speaker": "b"`, "female voice", "male voice", "never a `story`", "`tier`", "maxTextTokens", "minKnownShare"} {
		if !strings.Contains(text, want) {
			t.Errorf("the guide does not say %q", want)
		}
	}
	// The tiers are the figures of the brief builder.
	for _, want := range []string{"| beginner | under 100 words | 0.60 | 20 words |", "| developing | 100 to 299 words | 0.90 | 60 words |", "| established | 300 or more | 0.95 | no limit |"} {
		if !strings.Contains(text, want) {
			t.Errorf("the coverage table has no row %q", want)
		}
	}
}

// ---- ac-2: the repo skill is the same text -------------------------------------------

func TestResourcesAuthoringSkillMirror(t *testing.T) {
	served := resLF(resGuide(t))
	file, err := os.ReadFile(resGuidePath)
	if err != nil {
		t.Fatal(err)
	}
	if resLF(string(file)) != served {
		t.Fatal("the served resource is not the text of resources/authoring.md")
	}

	raw, err := os.ReadFile(resSkillPath)
	if err != nil {
		t.Fatalf("the repo skill is missing: %v", err)
	}
	skill := resLF(string(raw))
	if !strings.HasPrefix(skill, "---\n") {
		t.Fatal("SKILL.md has no front matter")
	}
	end := strings.Index(skill[4:], "\n---\n")
	if end < 0 {
		t.Fatal("SKILL.md front matter is not closed")
	}
	front := skill[4 : 4+end]
	rest := skill[4+end+len("\n---\n"):]
	if !strings.Contains("\n"+front+"\n", "\nname: hebrew-lesson-author\n") {
		t.Errorf("SKILL.md front matter has no name: hebrew-lesson-author\n%s", front)
	}
	_, desc, ok := strings.Cut(front, "\ndescription:")
	if !ok || len(strings.TrimSpace(desc)) < 40 {
		t.Errorf("SKILL.md front matter has no description\n%s", front)
	}
	if !strings.HasPrefix(rest, "\n") {
		t.Fatal("SKILL.md has no blank line after the front matter")
	}
	if body := rest[1:]; body != served {
		t.Errorf("the SKILL.md body is not the resource text (%d bytes against %d); mirror internal/mcp/resources/authoring.md after the front matter", len(body), len(served))
	}
}

// TestResourcesReviserSkill covers the hebrew-lesson-reviser skill. INIT-030/SPEC-006.
func TestResourcesReviserSkill(t *testing.T) {
	const path = "../../.cursor/skills/hebrew-lesson-reviser/SKILL.md"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the reviser skill is missing: %v", err)
	}
	skill := resLF(string(raw))
	if !strings.HasPrefix(skill, "---\n") {
		t.Fatal("reviser SKILL.md has no front matter")
	}
	end := strings.Index(skill[4:], "\n---\n")
	if end < 0 {
		t.Fatal("reviser SKILL.md front matter is not closed")
	}
	front := skill[4 : 4+end]
	body := skill[4+end+len("\n---\n"):]
	if !strings.Contains("\n"+front+"\n", "\nname: hebrew-lesson-reviser\n") {
		t.Errorf("front matter has no name: hebrew-lesson-reviser\n%s", front)
	}
	_, desc, ok := strings.Cut(front, "\ndescription:")
	if !ok || len(strings.TrimSpace(desc)) < 40 {
		t.Errorf("front matter has no description\n%s", front)
	}
	order := []string{"get_context", "get_unit", "get_lesson_brief", "validate_unit", "generate_audio", "put_unit", "add_coach_note"}
	var found []string
	for _, line := range strings.Split(body, "\n") {
		if !resStepLine.MatchString(line) {
			continue
		}
		for _, m := range resBackticked.FindAllStringSubmatch(line, -1) {
			if !slices.Contains(order, m[1]) {
				continue
			}
			if len(found) > 0 && found[len(found)-1] == m[1] {
				continue
			}
			found = append(found, m[1])
		}
	}
	if !slices.Equal(found, order) {
		t.Errorf("reviser steps name tools %v, want %v", found, order)
	}
	for _, want := range []string{"started", "finished", "not revised", "untouched"} {
		if !strings.Contains(strings.ToLower(body), want) {
			t.Errorf("reviser skill does not say %q", want)
		}
	}
}

// ---- security: no secret, host, address, or credential in the text -------------------

func TestResourcesAuthoringHoldsNoSecret(t *testing.T) {
	text := resGuide(t)
	lower := strings.ToLower(text)
	for _, banned := range []string{"bearer", "secret", "password", "@", "mcp_auth", "elevenlabs", "api key", "api_key", "apikey", "authorization", "sk-", "http://", "https://", "localhost", "cookie"} {
		if strings.Contains(lower, banned) {
			t.Errorf("the guide mentions %q", banned)
		}
	}
	// A long run of letters and digits with no space is a key, not prose.
	for _, run := range regexp.MustCompile(`[A-Za-z0-9_\-]{32,}`).FindAllString(text, -1) {
		t.Errorf("the guide holds a long unbroken run: %q", run)
	}
	// An example is lexicon words only. It has no learner id and no brief id.
	for name, block := range resBlocks(text) {
		if regexp.MustCompile(`learner:\d|learnerId|briefId`).MatchString(block) {
			t.Errorf("%s names a learner or a brief", name)
		}
	}
}

// ---- the examples, parsed out of the guide -------------------------------------------

type resWord struct{ text, gloss, translit, pos, gender string }

// resLexicon is the lexicon the two examples are written against. The guide holds the Hebrew; this
// table holds what the lexicon knows about each word. A word in an example that is missing here
// fails the test.
var resLexicon = []resWord{
	{"אוֹכֵל", "eats", "okhel", "verb", ""},
	{"שׁוֹתֶה", "drinks", "shote", "verb", ""},
	{"תֵּה", "tea", "te", "noun", "m"},
	{"מַיִם", "water", "mayim", "noun", "m"},
	{"לֶחֶם", "bread", "lekhem", "noun", "m"},
	{"קָפֶה", "coffee", "kafe", "noun", "m"},
	{"חָלָב", "milk", "khalav", "noun", "m"},
	{"אֲנִי", "I", "ani", "pron", ""},
	{"טוֹב", "good", "tov", "adj", ""},
	{"בֹּקֶר", "morning", "boker", "noun", "m"},
	{"אוֹ", "or", "o", "conj", ""},
	{"חַם", "hot", "kham", "adj", ""},
	{"שָׁלוֹם", "hello", "shalom", "noun", "m"},
	{"כֵּן", "yes", "ken", "particle", ""},
	{"תּוֹדָה", "thanks", "toda", "noun", "f"},
}

func resWordByText(text string) (resWord, bool) {
	n := units.Normalize(text)
	for _, w := range resLexicon {
		if units.Normalize(w.text) == n {
			return w, true
		}
	}
	return resWord{}, false
}

// resBriefDoc is the cut-down brief printed in the guide.
type resBriefDoc struct {
	Tier struct {
		Name          string  `json:"name"`
		KnownWords    int     `json:"knownWords"`
		MaxTextTokens int     `json:"maxTextTokens"`
		MinKnownShare float64 `json:"minKnownShare"`
	} `json:"tier"`
	MinStoryTokens int      `json:"minStoryTokens"` // INIT-030/SPEC-006
	NextPattern    string   `json:"nextPattern"`    // INIT-030/SPEC-006
	ReusePatterns  []string `json:"reusePatterns"`  // INIT-030/SPEC-006
	Pace           struct {
		NewWordCount int    `json:"newWordCount"`
		Consolidate  bool   `json:"consolidate"`
		Reason       string `json:"reason"`
	} `json:"pace"` // INIT-030/SPEC-006
	PreviousUnits []struct {
		Title         string   `json:"title"`
		Situation     string   `json:"situation"`
		Pattern       string   `json:"pattern"`
		NewWords      []string `json:"newWords"`
		ActivityTypes []string `json:"activityTypes"`
	} `json:"previousUnits"` // INIT-030/SPEC-006
	NewWords []struct {
		Text    string `json:"text"`
		Gloss   string `json:"gloss"`
		MinUses int    `json:"minUses"`
	} `json:"newWords"`
	StruggleWords []struct {
		Text          string `json:"text"`
		Gloss         string `json:"gloss"`
		Skill         string `json:"skill"`
		RequiredCount int    `json:"requiredCount"`
	} `json:"struggleWords"`
	ComfortableWords []struct {
		Text  string `json:"text"`
		Gloss string `json:"gloss"`
	} `json:"comfortableWords"`
	SkillFocus string   `json:"skillFocus"`
	Activities []string `json:"activities"`
}

type resExample struct {
	brief   resBriefDoc
	unit    units.Unit
	lexemes []map[string]any // entries the example adds with upsert_lexemes; nil when none
	note    string
}

func resParse(t *testing.T, text string, n int) resExample {
	t.Helper()
	blocks := resBlocks(text)
	key := func(kind string) string { return fmt.Sprintf("example-%d-%s", n, kind) }
	var ex resExample
	for _, kind := range []string{"brief", "unit", "note"} {
		if _, ok := blocks[key(kind)]; !ok {
			t.Fatalf("the guide has no %s block for example %d", kind, n)
		}
	}
	if err := json.Unmarshal([]byte(blocks[key("brief")]), &ex.brief); err != nil {
		t.Fatalf("example %d brief: %v", n, err)
	}
	if err := json.Unmarshal([]byte(blocks[key("unit")]), &ex.unit); err != nil {
		t.Fatalf("example %d unit: %v", n, err)
	}
	if raw, ok := blocks[key("lexemes")]; ok {
		var body struct {
			Entries []map[string]any `json:"entries"`
		}
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			t.Fatalf("example %d lexemes: %v", n, err)
		}
		ex.lexemes = body.Entries
	}
	ex.note = strings.TrimSpace(blocks[key("note")])
	return ex
}

// resLookup is a lexicon and an audio store in memory. Every word of resLexicon is in it. audio
// says whether every text has a clip, in either voice.
type resLookup struct {
	ids   map[string]int64
	facts map[string]units.LexemeFacts
	audio bool
}

func newResLookup(audioPresent bool) *resLookup {
	l := &resLookup{ids: map[string]int64{}, facts: map[string]units.LexemeFacts{}, audio: audioPresent}
	for i, w := range resLexicon {
		n := units.Normalize(w.text)
		l.ids[n] = int64(i + 1)
		l.facts[n] = units.LexemeFacts{POS: w.pos, Gender: w.gender, Bare: units.Plain(w.text), Gloss: w.gloss}
	}
	return l
}

func (l *resLookup) KnownLexemes(context.Context, int) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func (l *resLookup) HasAudio(context.Context, string) (bool, error) { return l.audio, nil }
func (l *resLookup) HasAudioForSpeaker(context.Context, string, string) (bool, error) {
	return l.audio, nil
}
func (l *resLookup) LexemeByText(_ context.Context, text string) (int64, bool, error) {
	id, ok := l.ids[text]
	return id, ok, nil
}
func (l *resLookup) Facts(_ context.Context, text string) (units.LexemeFacts, bool, error) {
	f, ok := l.facts[text]
	return f, ok, nil
}

// resFacts is what the server would build from this brief for this learner: the brief's words,
// and the learner's known words, which are its comfortable words.
func resFacts(t *testing.T, b resBriefDoc, lk *resLookup) units.BriefFacts {
	t.Helper()
	id := func(text string) int64 {
		got, ok := lk.ids[units.Normalize(text)]
		if !ok {
			t.Fatalf("the brief names %q, which is not in the fixture lexicon", text)
		}
		return got
	}
	f := units.BriefFacts{
		Tier: b.Tier.Name, MinKnownShare: b.Tier.MinKnownShare, MaxTextTokens: b.Tier.MaxTextTokens,
		MinStoryTokens: b.MinStoryTokens, // INIT-030/SPEC-006
		NextPattern:    b.NextPattern,    // INIT-030/SPEC-006
		ReusePatterns:  slices.Clone(b.ReusePatterns),
		Consolidate:    b.Pace.Consolidate,
		BriefLexemeIDs: map[int64]bool{}, KnownLexemeIDs: map[int64]bool{},
	}
	for _, w := range b.NewWords {
		f.NewWords = append(f.NewWords, units.BriefNewWord{LexemeID: id(w.Text), Text: w.Text, MinUses: w.MinUses})
		f.BriefLexemeIDs[id(w.Text)] = true
	}
	for _, w := range b.StruggleWords {
		f.StruggleWords = append(f.StruggleWords, units.BriefStruggleWord{LexemeID: id(w.Text), Text: w.Text, RequiredCount: w.RequiredCount, Skill: w.Skill})
		f.BriefLexemeIDs[id(w.Text)] = true
	}
	for _, w := range b.ComfortableWords {
		f.BriefLexemeIDs[id(w.Text)] = true
		f.KnownLexemeIDs[id(w.Text)] = true
	}
	for _, p := range b.PreviousUnits {
		f.PreviousUnits = append(f.PreviousUnits, units.BriefPreviousUnit{
			NewWords:      slices.Clone(p.NewWords),
			ActivityTypes: slices.Clone(p.ActivityTypes),
		})
	}
	return f
}

// resApplyPattern sets the unit's pattern from the brief the way put_unit does. INIT-030/SPEC-006.
func resApplyPattern(u *units.Unit, b resBriefDoc) {
	if b.Pace.Consolidate {
		if len(b.ReusePatterns) > 0 {
			u.Pattern = b.ReusePatterns[0]
		}
		return
	}
	u.Pattern = b.NextPattern
}

func resRules(issues []units.Issue, severity string) []string {
	var out []string
	for _, is := range issues {
		if is.Severity == severity {
			out = append(out, is.Rule+" "+is.Pointer)
		}
	}
	return out
}

// resAllowedWarnings are the warnings an example may carry. Each is a warning because spacing is
// done by the review schedule, not by the unit.
var resAllowedWarnings = []string{units.RuleRecycleCount, units.RuleRecycleSpread}

func resCheckPasses(t *testing.T, issues []units.Issue) {
	t.Helper()
	if blocks := resRules(issues, units.SeverityBlock); len(blocks) > 0 {
		t.Fatalf("the unit does not pass: %v", blocks)
	}
	for _, is := range issues {
		if !slices.Contains(resAllowedWarnings, is.Rule) {
			t.Errorf("unexpected warning %s %s: %s", is.Rule, is.Pointer, is.Msg)
		}
	}
}

// Both worked examples pass units.ValidateWithBrief against the brief printed with them.
func TestResourcesAuthoringExamplesValidate(t *testing.T) {
	text := resGuide(t)
	for _, n := range []int{1, 2} {
		t.Run(fmt.Sprintf("example %d", n), func(t *testing.T) {
			ex := resParse(t, text, n)
			lk := newResLookup(true)
			facts := resFacts(t, ex.brief, lk)
			unit := ex.unit
			resApplyPattern(&unit, ex.brief) // INIT-030/SPEC-006
			issues := units.ValidateWithBrief(t.Context(), &unit, lk, &facts)
			resCheckPasses(t, issues)

			// The brief asks for what the example does: every new word is in newWords and the
			// other way round, and every struggle word is in an activity of its skill.
			var briefNew []string
			for _, w := range ex.brief.NewWords {
				briefNew = append(briefNew, units.Normalize(w.Text))
			}
			var unitNew []string
			for _, w := range ex.unit.NewWords {
				unitNew = append(unitNew, units.Normalize(w))
			}
			slices.Sort(briefNew)
			slices.Sort(unitNew)
			if !slices.Equal(briefNew, unitNew) {
				t.Errorf("newWords of the unit %v, new words of the brief %v", unitNew, briefNew)
			}
			if got := resWordCount(ex.unit); got > ex.brief.Tier.MaxTextTokens {
				t.Errorf("a passage has %d words, over the tier's %d", got, ex.brief.Tier.MaxTextTokens)
			}
			if ex.note == "" || strings.Count(ex.note, ". ") != 0 || !strings.HasSuffix(ex.note, ".") {
				t.Errorf("the coach note is not one sentence: %q", ex.note)
			}
		})
	}

	// INIT-030/SPEC-006: example 1 continues a previous unit; example 2 consolidates.
	one, two := resParse(t, text, 1), resParse(t, text, 2)
	if one.brief.Pace.Consolidate || len(one.brief.PreviousUnits) == 0 || one.brief.MinStoryTokens < 12 {
		t.Errorf("example 1 is not a continuing unit: consolidate=%v previous=%d minStory=%d",
			one.brief.Pace.Consolidate, len(one.brief.PreviousUnits), one.brief.MinStoryTokens)
	}
	if len(one.brief.StruggleWords) < 3 || len(one.brief.NewWords) == 0 {
		t.Errorf("example 1 should teach new words with struggles: %d struggle, %d new", len(one.brief.StruggleWords), len(one.brief.NewWords))
	}
	if !two.brief.Pace.Consolidate || len(two.brief.NewWords) != 0 || len(two.unit.NewWords) != 0 {
		t.Errorf("example 2 is not consolidation: pace.consolidate=%v brief new=%d unit new=%d",
			two.brief.Pace.Consolidate, len(two.brief.NewWords), len(two.unit.NewWords))
	}
	speakers := map[string]bool{}
	dialogues := 0
	for _, les := range two.unit.Lessons {
		for _, act := range les.Activities {
			if act.Type != units.ActivityDialogue {
				continue
			}
			dialogues++
			for _, line := range act.Lines {
				speakers[line.Speaker] = true
			}
			if act.Text != "" {
				t.Error("a dialogue in example 2 has text")
			}
		}
	}
	if dialogues == 0 || !speakers["a"] || !speakers["b"] || len(speakers) != 2 {
		t.Errorf("example 2 has %d dialogues, with speakers %v", dialogues, speakers)
	}
}

// resWordCount is the most Hebrew words in one passage of the unit, a dialogue line being a passage.
func resWordCount(u units.Unit) int {
	most := 0
	count := func(text string) {
		n := 0
		for _, tok := range units.Tokenize(text) {
			if resHebrew(tok) {
				n++
			}
		}
		most = max(most, n)
	}
	for _, les := range u.Lessons {
		for _, act := range les.Activities {
			switch act.Type {
			case units.ActivityStory, units.ActivityListenPick:
				count(act.Text)
			case units.ActivityDialogue:
				for _, line := range act.Lines {
					count(line.Text)
				}
			}
		}
	}
	return most
}

func resHebrew(s string) bool {
	for _, r := range s {
		if r >= 0x05D0 && r <= 0x05EA {
			return true
		}
	}
	return false
}

// The examples are real: breaking one makes the same check fail, with the rule the guide names.
func TestResourcesAuthoringExamplesFailWhenBroken(t *testing.T) {
	text := resGuide(t)
	type breakage struct {
		name  string
		n     int
		audio bool
		edit  func(u *units.Unit)
		want  string
	}
	dropWhere := func(u *units.Unit, drop func(units.Activity) bool) {
		for i := range u.Lessons {
			var keep []units.Activity
			for _, a := range u.Lessons[i].Activities {
				if !drop(a) {
					keep = append(keep, a)
				}
			}
			u.Lessons[i].Activities = keep
		}
	}
	for _, c := range []breakage{
		{"no clips", 1, false, func(*units.Unit) {}, units.RuleAudioMissing},
		{"the spelling activity of the milk word is gone", 1, true, func(u *units.Unit) {
			dropWhere(u, func(a units.Activity) bool { return a.Type == units.ActivityProduce })
		}, units.RuleBriefStruggle},
		{"every story is gone", 1, true, func(u *units.Unit) {
			dropWhere(u, func(a units.Activity) bool { return a.Type == units.ActivityStory })
		}, units.RuleBriefIntro},
		{"a markup character", 1, true, func(u *units.Unit) { u.Title += "<b>" }, units.RuleUnitNoMarkup},
		{"consolidation lists a new word", 2, true, func(u *units.Unit) {
			u.NewWords = []string{"תֵּה"}
		}, units.RuleBriefConsolidation}, // INIT-030/SPEC-006
		{"speaker c", 2, true, func(u *units.Unit) { u.Lessons[0].Activities[0].Lines[1].Speaker = "c" }, units.RuleUnitDialogueSpeakers},
		{"only one speaker speaks", 2, true, func(u *units.Unit) {
			for i := range u.Lessons[0].Activities[0].Lines {
				u.Lessons[0].Activities[0].Lines[i].Speaker = "a"
			}
		}, units.RuleUnitDialogueSpeakers},
		{"a dialogue with a text", 2, true, func(u *units.Unit) { u.Lessons[0].Activities[0].Text = "שָׁלוֹם" }, units.RuleUnitDialogueLines},
		{"a long line", 2, true, func(u *units.Unit) {
			line := &u.Lessons[0].Activities[0].Lines[0]
			line.Text = strings.Repeat("תֵּה ", 21)
		}, units.RuleBriefTextTooLong},
	} {
		t.Run(c.name, func(t *testing.T) {
			ex := resParse(t, text, c.n)
			lk := newResLookup(c.audio)
			facts := resFacts(t, ex.brief, lk)
			unit := ex.unit
			// Copy the slices the edit touches, so a case never changes another.
			raw, err := json.Marshal(unit)
			if err != nil {
				t.Fatal(err)
			}
			var copyOf units.Unit
			if err := json.Unmarshal(raw, &copyOf); err != nil {
				t.Fatal(err)
			}
			resApplyPattern(&copyOf, ex.brief) // INIT-030/SPEC-006
			c.edit(&copyOf)
			issues := units.ValidateWithBrief(t.Context(), &copyOf, lk, &facts)
			found := false
			for _, is := range issues {
				if is.Rule == c.want && is.Severity == units.SeverityBlock {
					found = true
				}
			}
			if !found {
				t.Errorf("want a block %s, got %v", c.want, resRules(issues, units.SeverityBlock))
			}
		})
	}
}

// ---- ac-3: a client follows only the guide and publishes a unit ---------------------

// resTTS is the fake provider. It records the voices it was asked for and makes bytes from the text.
type resTTS struct {
	mu     sync.Mutex
	voices map[string]int
	calls  int
}

func (f *resTTS) Synthesize(ctx context.Context, textNFC, voice, _ string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.voices == nil {
		f.voices = map[string]int{}
	}
	f.voices[voice]++
	f.calls++
	return []byte("fake-mp3:" + voice + ":" + textNFC), nil
}

// resKnow makes a word known to a learner in listen, as apEnv.know does, for a lexeme that exists.
func resKnow(e *apEnv, user int64, text string) {
	e.t.Helper()
	id := e.ids[text]
	for i := range 3 {
		at := apNow.Add(-2 * time.Hour).Add(-time.Duration(2-i) * time.Minute)
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, 'listen', 'test', 'review', '', '', 1, 0, 0, 0, ?)`, user, id, db.FormatTime(at))
	}
	last := apNow.AddDate(0, 0, -1)
	e.exec(`INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, 'listen', ?, ?, 10, 5, 3, 0, ?, 0)`, user, id, fsrs.StateReview, db.FormatTime(apNow), db.FormatTime(last))
	card := e.count(`SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, id)
	for i := range 3 {
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 1)`,
			card, db.FormatTime(last.Add(-time.Duration(2-i)*24*time.Hour)))
	}
}

// resEnv builds the learner the example's brief describes: the struggle words with a run of wrong
// answers in the skill the brief names, the comfortable words known, and the new words never seen.
// A word the example adds with upsert_lexemes is not in the lexicon yet. The male voice is set,
// and the text-to-speech is the fake.
func resEnv(t *testing.T, ex resExample) (*apEnv, *sdkmcp.ClientSession, *resTTS) {
	t.Helper()
	e := newAP(t)
	e.exec(`DELETE FROM lexemes`)
	e.ids = map[string]int64{}
	for _, entry := range ex.lexemes {
		text, _ := entry["text_nfc"].(string)
		if _, ok := resWordByText(text); !ok {
			t.Fatalf("the example adds %q, which is not in the fixture lexicon", text)
		}
		for _, w := range ex.brief.NewWords {
			if units.Normalize(w.Text) == units.Normalize(text) {
				t.Fatalf("the example adds %q, which is a word of its brief", text)
			}
		}
	}
	insert := func(text string) string {
		n := units.Normalize(text)
		w, ok := resWordByText(text)
		if !ok {
			t.Fatalf("%q is not in the fixture lexicon", text)
		}
		var gender any
		if w.gender != "" {
			gender = w.gender
		}
		e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos, gender, translit) VALUES (?, ?, ?, ?, ?, ?)`,
			n, units.Plain(n), w.gloss, w.pos, gender, w.translit)
		e.ids[n] = int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, n))
		return n
	}
	for _, w := range ex.brief.StruggleWords {
		n := insert(w.Text)
		e.attempts(e.a, n, w.Skill, "RRWWWWWW")
	}
	for _, w := range ex.brief.NewWords {
		insert(w.Text)
	}
	for _, w := range ex.brief.ComfortableWords {
		resKnow(e, e.a, insert(w.Text))
	}
	if ex.brief.Pace.Consolidate {
		// INIT-030/SPEC-006: Pace consolidates when review accuracy is under 75%.
		// Each comfortable word has 3 Good reviews; add enough Again ratings to drop below.
		goods := 3 * len(ex.brief.ComfortableWords)
		needAgain := goods // 50% < 75%, and reviews > 0
		conn, err := e.store.DB()
		if err != nil {
			t.Fatal(err)
		}
		rows, err := conn.QueryContext(t.Context(), `SELECT id FROM cards WHERE user_id = ? ORDER BY id`, e.a)
		if err != nil {
			t.Fatal(err)
		}
		var cardIDs []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			cardIDs = append(cardIDs, id)
		}
		rows.Close()
		if len(cardIDs) == 0 {
			t.Fatal("consolidation example needs known-word cards for Again reviews")
		}
		for i := 0; i < needAgain; i++ {
			card := cardIDs[i%len(cardIDs)]
			at := apNow.Add(-time.Duration(i+1) * time.Minute)
			e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 1, ?, 0, 1)`,
				card, db.FormatTime(at))
		}
	} else {
		// The newest attempts set the brief's load. This seen word at 80% right keeps it steady, so the
		// brief stays the one the guide prints.
		anchor := units.Normalize("עֹגֶן")
		e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, translit) VALUES (?, ?, 'anchor', 'ogen')`, anchor, units.Plain(anchor))
		e.ids[anchor] = int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, anchor))
		e.attempts(e.a, anchor, "listen", strings.Repeat("W", 8)+strings.Repeat("R", 32))
	}

	tts := &resTTS{}
	gen := &audio.Generator{
		TTS: tts, Store: e.store, Dir: t.TempDir(),
		Voice: audio.DefaultVoiceID, VoiceMale: "test-voice-b", Model: audio.DefaultModel,
	}
	e.adapter.WithVoices(gen.Voices())
	return e, e.as("static", e2eAudio{gen: gen}), tts
}

// resCompareBrief fails when the brief the server built is not the brief printed in the guide.
func resCompareBrief(t *testing.T, doc resBriefDoc, live mcp.LessonBrief) {
	t.Helper()
	if live.Tier.Name != doc.Tier.Name || live.Tier.KnownWords != doc.Tier.KnownWords ||
		live.Tier.MaxTextTokens != doc.Tier.MaxTextTokens || live.Tier.MinKnownShare != doc.Tier.MinKnownShare {
		t.Errorf("tier: the guide says %+v, the server built %+v", doc.Tier, live.Tier)
	}
	want, got := map[string]string{}, map[string]string{}
	for _, w := range doc.NewWords {
		want["new "+units.Normalize(w.Text)] = fmt.Sprintf("%s/%d", w.Gloss, w.MinUses)
	}
	for _, w := range live.NewWords {
		got["new "+units.Normalize(w.Text)] = fmt.Sprintf("%s/%d", w.Gloss, w.MinUses)
	}
	for _, w := range doc.StruggleWords {
		want["struggle "+units.Normalize(w.Text)] = fmt.Sprintf("%s/%s/%d", w.Gloss, w.Skill, w.RequiredCount)
	}
	for _, w := range live.StruggleWords {
		got["struggle "+units.Normalize(w.Text)] = fmt.Sprintf("%s/%s/%d", w.Gloss, w.Skill, w.RequiredCount)
	}
	for _, w := range doc.ComfortableWords {
		want["comfortable "+units.Normalize(w.Text)] = w.Gloss
	}
	for _, w := range live.ComfortableWords {
		got["comfortable "+units.Normalize(w.Text)] = w.Gloss
	}
	if !maps.Equal(want, got) {
		t.Errorf("the words of the brief in the guide differ from the server's:\n guide:  %v\n server: %v", want, got)
	}
	if live.SkillFocus != doc.SkillFocus {
		t.Errorf("skillFocus: the guide says %q, the server built %q", doc.SkillFocus, live.SkillFocus)
	}
	a, b := slices.Sorted(slices.Values(doc.Activities)), slices.Sorted(slices.Values(live.Activities))
	if !slices.Equal(a, b) {
		t.Errorf("activities: the guide says %v, the server built %v", a, b)
	}
	if live.Targets.NewWordMinUses != 3 {
		t.Errorf("targets %+v", live.Targets)
	}
}

type resAudioItem struct{ text, speaker string }

func TestResourcesAuthoringDryRun(t *testing.T) {
	for _, n := range []int{1, 2} {
		t.Run(fmt.Sprintf("example %d", n), func(t *testing.T) { resDryRun(t, n) })
	}
}

// resDryRun is one MCP client that has only the guide. It reads the steps, and for each tool it
// finds, in the order it finds them, it does what the text and the example say. It adds nothing
// of its own: the words, the unit, the lexicon entry, and the note are the guide's.
func resDryRun(t *testing.T, n int) {
	// The first read is on a plain server, to learn the example. The client then reads the text from
	// the server it builds, over the same protocol.
	ex := resParse(t, resGuide(t), n)
	e, client, tts := resEnv(t, ex)
	text := resRead(t, client, resAuthoringURI)
	if resParse(t, text, n).unit.Title != ex.unit.Title {
		t.Fatal("the two servers serve different guides")
	}
	tools := resToolNames(t, client)
	_, order := resSteps(text, tools)
	if !slices.Equal(order, resStepTools) {
		t.Fatalf("the steps name %v, want %v", order, resStepTools)
	}

	var (
		brief   mcp.LessonBrief
		missing []string
		ran     = map[string]bool{}
		unitID  int64
	)
	unit := ex.unit
	step := map[string]func(){
		"get_context": func() {
			out := mustOK(t, client, "get_context", map[string]any{"position": 0, "learnerId": e.a})
			if !strings.Contains(out, "coachNotes") {
				t.Errorf("get_context has no coachNotes: %s", out)
			}
		},
		"get_lesson_brief": func() {
			args := map[string]any{"learnerId": e.a}
			if n == 2 && !ex.brief.Pace.Consolidate {
				args["theme"] = "cafe"
			}
			brief = decodeInto[mcp.LessonBrief](t, mustOK(t, client, "get_lesson_brief", args))
			if brief.BriefID == "" {
				t.Fatal("no briefId")
			}
			resCompareBrief(t, ex.brief, brief)
			// Writing the unit is the client's own step. It writes the guide's example.
			if len(unit.Lessons) == 0 {
				t.Fatal("the example has no lessons")
			}
		},
		"lookup": func() {
			seen := map[string]bool{}
			for _, tok := range resUnitWords(unit) {
				if seen[units.Normalize(tok)] {
					continue
				}
				seen[units.Normalize(tok)] = true
				out := decodeInto[struct {
					Hits []mcp.Lexeme `json:"hits"`
				}](t, mustOK(t, client, "lookup", map[string]any{"query": tok}))
				found := false
				for _, h := range out.Hits {
					if units.Normalize(h.TextNFC) == units.Normalize(tok) {
						found = true
					}
				}
				if !found {
					missing = append(missing, tok)
				}
			}
		},
		"upsert_lexemes": func() {
			if len(missing) == 0 {
				if len(ex.lexemes) > 0 {
					t.Error("the example adds a lexicon entry, and lookup found nothing missing")
				}
				return
			}
			have := map[string]bool{}
			for _, entry := range ex.lexemes {
				text, _ := entry["text_nfc"].(string)
				have[units.Normalize(text)] = true
			}
			for _, tok := range missing {
				if !have[units.Normalize(tok)] {
					t.Errorf("lookup found no entry for %q, and the example gives none", tok)
				}
			}
			if len(missing) != len(ex.lexemes) {
				t.Errorf("lookup missed %d words, and the example adds %d entries", len(missing), len(ex.lexemes))
			}
			mustOK(t, client, "upsert_lexemes", map[string]any{"entries": ex.lexemes})
		},
		"generate_audio": func() {
			var items []resAudioItem
			seen := map[string]bool{}
			add := func(text, speaker string) {
				k := speaker + "\x00" + units.Normalize(text)
				if !seen[k] {
					seen[k] = true
					items = append(items, resAudioItem{text, speaker})
				}
			}
			for _, w := range unit.NewWords {
				add(w, "")
			}
			for _, les := range unit.Lessons {
				for _, act := range les.Activities {
					if act.Type == units.ActivityDialogue {
						for _, line := range act.Lines {
							add(line.Text, line.Speaker)
						}
						continue
					}
					add(act.Text, "")
				}
			}
			for chunk := range slices.Chunk(items, 10) {
				var texts []string
				var lines []map[string]string
				for _, it := range chunk {
					if it.speaker == "" {
						texts = append(texts, it.text)
					} else {
						lines = append(lines, map[string]string{"text": it.text, "speaker": it.speaker})
					}
				}
				args := map[string]any{}
				if len(texts) > 0 {
					args["texts"] = texts
				}
				if len(lines) > 0 {
					args["lines"] = lines
				}
				out := decodeInto[struct {
					Results []struct{ Status, Error string } `json:"results"`
				}](t, mustOK(t, client, "generate_audio", args))
				if len(out.Results) != len(chunk) {
					t.Fatalf("%d results for %d items", len(out.Results), len(chunk))
				}
				for i, r := range out.Results {
					if r.Status != "generated" && r.Status != "skipped" {
						t.Fatalf("clip for %q (%s): %+v", chunk[i].text, chunk[i].speaker, r)
					}
				}
			}
			if tts.calls == 0 {
				t.Error("the fake provider was never called")
			}
		},
		"validate_unit": func() {
			out := decodeInto[struct {
				OK     bool          `json:"ok"`
				Blocks int           `json:"blocks"`
				Issues []units.Issue `json:"issues"`
			}](t, mustOK(t, client, "validate_unit", map[string]any{"position": 0, "doc": unit, "briefId": brief.BriefID, "learnerId": e.a}))
			if !out.OK || out.Blocks != 0 {
				t.Fatalf("the unit does not pass: %v", resRules(out.Issues, units.SeverityBlock))
			}
			resCheckPasses(t, out.Issues)
		},
		"put_unit": func() {
			put := decodeInto[apPut](t, mustOK(t, client, "put_unit", map[string]any{
				"scope": db.LearnerScope(e.a), "position": 0, "doc": unit, "publish": true, "briefId": brief.BriefID,
			}))
			if !put.Published || put.PublishReason != "" || len(apBlocks(put.Issues)) != 0 {
				t.Fatalf("not published: %+v, blocks %v", put, apBlocks(put.Issues))
			}
			unitID = put.UnitID
		},
		"add_coach_note": func() {
			note := decodeInto[struct {
				NoteID int64 `json:"noteId"`
			}](t, mustOK(t, client, "add_coach_note", map[string]any{"learnerId": e.a, "body": ex.note}))
			if note.NoteID == 0 {
				t.Fatal("no note id")
			}
		},
	}
	for _, tool := range order {
		run, ok := step[tool]
		if !ok {
			t.Fatalf("the guide names %s and the client has nothing to do for it", tool)
		}
		run()
		ran[tool] = true
	}
	for _, tool := range resStepTools {
		if !ran[tool] {
			t.Errorf("the guide never reached %s", tool)
		}
	}

	// The unit is on this learner's Today, published once with one audit row, and it spent the brief.
	if unitID == 0 || e.status(unitID) != "published" {
		t.Fatalf("unit %d is %q", unitID, e.status(unitID))
	}
	if got := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND action = 'auto_publish' AND brief_id = ? AND scope = ?`,
		unitID, brief.BriefID, db.LearnerScope(e.a)); got != 1 || e.audits() != 1 {
		t.Errorf("audit rows: %d for the unit, %d in all", got, e.audits())
	}
	if int64(e.briefUsed(brief.BriefID)) != unitID {
		t.Error("the brief was not spent by the publish")
	}
	for _, r := range mustScope(t, e, e.b) {
		if r.ID == unitID {
			t.Error("the unit is in another learner's scope")
		}
	}
	// A dialogue was voiced twice: one voice for each speaker, and no other voice.
	hasDialogue := false
	for _, les := range unit.Lessons {
		for _, act := range les.Activities {
			hasDialogue = hasDialogue || act.Type == units.ActivityDialogue
		}
	}
	if hasDialogue && (tts.voices[audio.DefaultVoiceID] == 0 || tts.voices["test-voice-b"] == 0 || len(tts.voices) != 2) {
		t.Errorf("voices asked of the provider: %v", tts.voices)
	}
	// The note is what the next get_context for this learner returns.
	if out := mustOK(t, client, "get_context", map[string]any{"position": 0, "learnerId": e.a}); !strings.Contains(out, ex.note[:30]) {
		t.Errorf("the coach note is not in the next context: %s", out)
	}
}

// resUnitWords is every Hebrew word of every activity text and dialogue line, in order.
func resUnitWords(u units.Unit) []string {
	var out []string
	for _, les := range u.Lessons {
		for _, act := range les.Activities {
			for _, text := range act.Texts() {
				for _, tok := range units.Tokenize(text) {
					if resHebrew(tok) {
						out = append(out, tok)
					}
				}
			}
		}
	}
	return out
}
