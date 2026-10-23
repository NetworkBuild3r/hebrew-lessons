package units

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The brief fixtures (INIT-015/SPEC-020, ac-1). Each brief_bad_*.json is the valid
// brief unit with one change, and it must produce exactly one issue: the rule id
// and JSON pointer in the file. brief_valid.json produces none. The fixtures are
// built here and written with WRITE_GOLDENS=1, the same way as the v2 goldens, so
// the JSON on disk cannot drift from the builder.

type briefGoldenFile struct {
	Case   string        `json:"case"`
	Lookup goldenLookup  `json:"lookup"`
	Brief  BriefFacts    `json:"brief"`
	Issues []storedIssue `json:"issues"`
	Unit   Unit          `json:"unit"`
}

type briefCase struct {
	file   string
	name   string
	unit   Unit
	lex    map[string]glex
	brief  BriefFacts
	issues []Issue
}

const briefStruggleID = 9

// briefBase is a unit that passes every rule, with its lexicon and brief.
// Eight new words in four lessons. Each lesson opens with a shadow step per new
// word (the exposure), then recall, listen_pick, and story. Three checkpoints
// recycle the words. The struggle word kos is a failing listening word, and the
// last checkpoint plays it in three listen_pick steps.
func briefBase() (Unit, map[string]glex, BriefFacts) {
	words := wordTexts(vocab8)
	u := assemble(words, pairs(4), evenRepeats(len(words), 3))
	for li := 0; li < 4; li++ {
		var shadows []Activity
		for _, wi := range pairs(4)[li] {
			shadows = append(shadows, Activity{Type: ActivityShadow, Text: words[wi]})
		}
		u.Lessons[li].Activities = append(shadows, u.Lessons[li].Activities...)
	}
	for k := 0; k < 3; k++ {
		addAct(&u, ActivityListenPick, kos)
	}
	lex := map[string]glex{}
	facts := BriefFacts{
		Tier:           "established",
		MinKnownShare:  0.95,
		BriefLexemeIDs: map[int64]bool{},
		KnownLexemeIDs: map[int64]bool{},
	}
	for i, w := range vocab8 {
		id := int64(i + 1)
		lex[normalize(w.text)] = glex{ID: id, POS: "noun", Gender: w.gender, Bare: stripNiqqud(w.text), Gloss: "gloss " + itoa(i+1)}
		facts.NewWords = append(facts.NewWords, BriefNewWord{LexemeID: id, Text: w.text, MinUses: 3})
		facts.BriefLexemeIDs[id] = true
	}
	lex[normalize(kos)] = glex{ID: briefStruggleID, POS: "noun", Gender: "f", Bare: stripNiqqud(kos), Gloss: "cup"}
	facts.StruggleWords = []BriefStruggleWord{{LexemeID: briefStruggleID, Text: kos, RequiredCount: 3, Skill: SkillListen}}
	facts.BriefLexemeIDs[briefStruggleID] = true
	lex[normalize(naal)] = glex{ID: 50, POS: "noun", Gender: "f", Bare: stripNiqqud(naal), Gloss: "sandal"}
	return u, lex, facts
}

func (c briefCase) lookup() goldenLookup {
	return goldenLookup{Lexemes: c.lex, Audio: audiosOf(c.unit)}
}

func briefCases() []briefCase {
	var out []briefCase
	add := func(file, name string, mut func(u *Unit, lex map[string]glex, f *BriefFacts) []Issue) {
		u, lex, f := briefBase()
		issues := mut(&u, lex, &f)
		out = append(out, briefCase{file: file, name: name, unit: u, lex: lex, brief: f, issues: issues})
	}
	add("brief_valid.json", "A unit that passes every v2 rule and every brief rule. It is the base of each brief_bad_* fixture.",
		func(*Unit, map[string]glex, *BriefFacts) []Issue { return nil })
	add("brief_bad_coverage.json", "brief.coverage: a listen_pick of 10 tokens has 9 words of the brief and one word the learner does not know. 90.0% is under the 0.95 figure.",
		func(u *Unit, _ map[string]glex, _ *BriefFacts) []Issue {
			li, ai := addAct(u, ActivityListenPick, repeatJoin(vocab8[0].text, 9)+" "+naal)
			return []Issue{iss(RuleBriefCoverage, SeverityBlock, at("lessons", li, "activities", ai))}
		})
	add("brief_bad_intro.json", "brief.new_intro_before_test: the first word of lesson 0 loses its shadow step, so its first appearance is a checked recall.",
		func(u *Unit, _ map[string]glex, _ *BriefFacts) []Issue {
			u.Lessons[0].Activities = u.Lessons[0].Activities[1:]
			return []Issue{iss(RuleBriefIntro, SeverityBlock, at("lessons", 0, "activities", 1, "text"))}
		})
	add("brief_bad_min_uses.json", "brief.new_min_uses: the brief asks for 9 uses of the first new word, and the unit uses it 8 times.",
		func(_ *Unit, _ map[string]glex, f *BriefFacts) []Issue {
			f.NewWords[0].MinUses = 9
			return []Issue{iss(RuleBriefNewMinUses, SeverityBlock, "/newWords/0")}
		})
	add("brief_bad_struggle.json", "brief.struggle_reuse: the brief requires 4 uses of the struggle word, and the unit uses it 3 times.",
		func(_ *Unit, _ map[string]glex, f *BriefFacts) []Issue {
			f.StruggleWords[0].RequiredCount = 4
			return []Issue{iss(RuleBriefStruggle, SeverityBlock, "/lessons")}
		})
	add("brief_bad_gloss.json", "brief.gloss_out_of_brief: a recall step uses a word outside the brief and the learner's known words, and its lexicon entry has no gloss.",
		func(u *Unit, lex map[string]glex, _ *BriefFacts) []Issue {
			e := lex[normalize(naal)]
			e.Gloss = ""
			lex[normalize(naal)] = e
			li, ai := addAct(u, ActivityRecall, naal)
			return []Issue{iss(RuleBriefGloss, SeverityBlock, at("lessons", li, "activities", ai, "text"))}
		})
	add("brief_bad_audio_all.json", "brief.audio_all: the title is Hebrew and no clip exists for it.",
		func(u *Unit, _ map[string]glex, _ *BriefFacts) []Issue {
			u.Title = "\u05D1\u05D5\u05E7\u05E8 \u05D8\u05D5\u05D1"
			return []Issue{iss(RuleBriefAudioAll, SeverityBlock, "/title")}
		})
	add("brief_bad_markup.json", "unit.no_markup: the title carries a script tag.",
		func(u *Unit, _ map[string]glex, _ *BriefFacts) []Issue {
			u.Title = "Cafe <script>alert(1)</script>"
			return []Issue{iss(RuleUnitNoMarkup, SeverityBlock, "/title")}
		})
	add("brief_bad_text_too_long.json", "brief.text_too_long: the tier allows 20 tokens and a story has 21. Every token is a word of the brief, so coverage holds.",
		func(u *Unit, _ map[string]glex, f *BriefFacts) []Issue {
			f.Tier = "beginner"
			f.MaxTextTokens = 20
			f.MinKnownShare = 0.6
			li, ai := addAct(u, ActivityStory, repeatJoin(vocab8[0].text, 21))
			return []Issue{iss(RuleBriefTextTooLong, SeverityBlock, at("lessons", li, "activities", ai))}
		})
	add("brief_bad_used.json", "brief.used: the brief already built a unit.",
		func(_ *Unit, _ map[string]glex, f *BriefFacts) []Issue {
			f.Used = true
			f.UsedByUnitID = 41
			return []Issue{iss(RuleBriefUsed, SeverityBlock, "/briefId")}
		})
	add("brief_bad_fix_contrast.json", "brief.fix_contrast: the brief asks for a right form the unit never uses.",
		func(_ *Unit, _ map[string]glex, f *BriefFacts) []Issue {
			f.FixMistakes = []BriefFix{{Right: "\u05D6\u05B4\u05D2\u05D5\u05BC\u05D2"}}
			return []Issue{iss(RuleBriefFixContrast, SeverityBlock, "/fixMistakes/0")}
		})
	return out
}

func marshalBriefGolden(c briefCase) []byte {
	file := briefGoldenFile{
		Case:   c.name,
		Lookup: c.lookup(),
		Brief:  c.brief,
		Issues: make([]storedIssue, 0, len(c.issues)),
		Unit:   c.unit,
	}
	for _, is := range c.issues {
		file.Issues = append(file.Issues, storedIssue{Rule: is.Rule, Severity: is.Severity, Pointer: is.Pointer})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(raw, '\n')
}

// TestBriefGoldens is ac-1. Every fixture is read back from disk and checked
// against ValidateWithBrief. A bad fixture yields exactly its one rule id at its
// one JSON pointer, with no other rule firing.
func TestBriefGoldens(t *testing.T) {
	catalog := map[string]bool{}
	for _, r := range Rules {
		catalog[r.ID] = true
	}
	for _, c := range briefCases() {
		t.Run(c.file, func(t *testing.T) {
			body := marshalBriefGolden(c)
			path := filepath.Join("testdata", c.file)
			if os.Getenv("WRITE_GOLDENS") == "1" {
				if err := os.WriteFile(path, body, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (WRITE_GOLDENS=1 writes it)", err)
			}
			if !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(body)) {
				t.Fatalf("golden %s drifted from the fixture builder; run WRITE_GOLDENS=1", c.file)
			}
			var file briefGoldenFile
			if err := json.Unmarshal(raw, &file); err != nil {
				t.Fatal(err)
			}
			got := ValidateWithBrief(context.Background(), &file.Unit, file.Lookup.fake(), &file.Brief)
			want := make([]Issue, 0, len(file.Issues))
			for _, is := range file.Issues {
				want = append(want, iss(is.Rule, is.Severity, is.Pointer))
			}
			if !sameIssues(got, want) {
				t.Fatalf("issues\n got %s\nwant %s", mustJSON(stripMsg(got)), mustJSON(want))
			}
			for _, is := range got {
				if is.Rule == "" || is.Pointer == "" || is.Msg == "" || is.Severity != SeverityBlock || !catalog[is.Rule] {
					t.Fatalf("bad issue %#v", is)
				}
			}
		})
	}
}

// TestBriefGoldenPerRule fails when a brief rule has no failing fixture.
func TestBriefGoldenPerRule(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range briefCases() {
		for _, is := range c.issues {
			covered[is.Rule] = true
		}
	}
	// brief.not_found and brief.expired come from the publish path, which cannot
	// build a BriefFacts for a brief it cannot read. Their tests are in internal/mcp.
	for _, rule := range []string{RuleBriefAudioAll, RuleBriefCoverage, RuleBriefFixContrast, RuleBriefGloss, RuleBriefIntro,
		RuleBriefNewMinUses, RuleBriefStruggle, RuleBriefTextTooLong, RuleBriefUsed, RuleUnitNoMarkup} {
		if !covered[rule] {
			t.Errorf("no failing fixture for %s", rule)
		}
	}
	if len(covered) != 10 {
		t.Errorf("fixtures cover %d rules: %v", len(covered), covered)
	}
}
