package mcp

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/google/jsonschema-go/jsonschema"
)

// Budgets is the numeric authoring thresholds from units.Rules.
// get_context and hebrew://rules both serve this object.
type Budgets struct {
	MaxNewWordsPerUnit   int `json:"maxNewWordsPerUnit"`
	MinNewWordsPerLesson int `json:"minNewWordsPerLesson"`
	MaxNewWordsPerLesson int `json:"maxNewWordsPerLesson"`
	RecycleMinCount      int `json:"recycleMinCount"`
	RecycleMinLessons    int `json:"recycleMinLessons"`
	CoverageBlockBelow   int `json:"coverageBlockBelow"`
	CoverageWarnBelow    int `json:"coverageWarnBelow"`
}

func currentBudgets() Budgets {
	return Budgets{
		MaxNewWordsPerUnit:   units.MaxNewWordsPerUnit,
		MinNewWordsPerLesson: units.MinNewWordsPerLesson,
		MaxNewWordsPerLesson: units.MaxNewWordsPerLesson,
		RecycleMinCount:      units.RecycleMinCount,
		RecycleMinLessons:    units.RecycleMinLessons,
		CoverageBlockBelow:   units.CoverageBlockBelow,
		CoverageWarnBelow:    units.CoverageWarnBelow,
	}
}

// currentMixTarget is the unit new-word budget plus the session mix.
// targetSuccess is sessionlimits.TargetSuccess, the same value
// session.BuildSession aims at. maxNewPerSession and maxNewShare are the
// coaching caps. BuildSession does not apply those two.
func currentMixTarget() MixTarget {
	return MixTarget{
		MaxNewWordsPerUnit:   units.MaxNewWordsPerUnit,
		MinNewWordsPerLesson: units.MinNewWordsPerLesson,
		MaxNewWordsPerLesson: units.MaxNewWordsPerLesson,
		TargetSuccess:        sessionlimits.TargetSuccess,
		MaxNewPerSession:     sessionlimits.MaxNewPerSession,
		MaxNewShare:          sessionlimits.MaxNewShare,
	}
}

// unitDocumentSchema is the JSON Schema for the unit document.
// It describes the shape an author should write. put_unit's tool schema
// is the Go type, which still unmarshals an empty string, so a draft
// with block issues can be saved. Enums live here, not on that tool schema.
func unitDocumentSchema() (*jsonschema.Schema, error) {
	s, err := jsonschema.For[units.Unit](nil)
	if err != nil {
		return nil, err
	}
	if err := annotateUnitSchema(s); err != nil {
		return nil, err
	}
	return s, nil
}

func annotateUnitSchema(s *jsonschema.Schema) error {
	s.Description = "One course unit. Position is not a field of this document; pass it next to the document. " +
		"put_unit still saves a draft when a string is empty or an activity type is unknown, and returns those as block issues."
	setDesc(s, "title", "Learner-facing title.")
	setDesc(s, "situation", "The scene the unit is set in.")
	setDesc(s, "concept", "The one idea the unit teaches.")
	setDesc(s, "newWords", "Pointed Hebrew in NFC, one entry per new word. At most 8. Each word appears in a lesson.")
	if words := prop(s, "newWords"); words != nil && words.Items != nil {
		words.Items.Description = "Pointed NFC Hebrew."
	}
	lessons := prop(s, "lessons")
	if lessons == nil || lessons.Items == nil {
		return fmt.Errorf("unit schema has no lessons items")
	}
	lessons.Description = "Lessons and checkpoints, in play order."
	lesson := lessons.Items
	kind := prop(lesson, "kind")
	if kind == nil {
		return fmt.Errorf("unit schema has no lesson kind")
	}
	kind.Enum = []any{units.KindLesson, units.KindCheckpoint}
	kind.Description = "lesson or checkpoint. A checkpoint introduces no new word."
	activities := prop(lesson, "activities")
	if activities == nil || activities.Items == nil {
		return fmt.Errorf("unit schema has no activity items")
	}
	activities.Description = "Screens, in play order. Read hebrew://catalog/activities for what each type uses text for."
	act := activities.Items
	typ := prop(act, "type")
	text := prop(act, "text")
	if typ == nil || text == nil {
		return fmt.Errorf("unit schema has no activity type or text")
	}
	typ.Enum = []any{
		units.ActivityListenPick,
		units.ActivityRecall,
		units.ActivityProduce,
		units.ActivityCloze,
		units.ActivityPattern,
		units.ActivityStory,
		units.ActivityShadow,
	}
	typ.Description = "One of the seven activity types."
	text.Description = "Pointed Hebrew this screen speaks or asks about. Audio is looked up from this exact text."
	if !slices.Contains(act.Required, "text") {
		act.Required = append(act.Required, "text")
	}
	return nil
}

func prop(s *jsonschema.Schema, name string) *jsonschema.Schema {
	if s == nil || s.Properties == nil {
		return nil
	}
	return s.Properties[name]
}

func setDesc(s *jsonschema.Schema, name, desc string) {
	if p := prop(s, name); p != nil {
		p.Description = desc
	}
}

type activityField struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type activityInfo struct {
	Type    string          `json:"type"`
	Purpose string          `json:"purpose"`
	Fields  []activityField `json:"fields"`
}

func activityCatalog() []activityInfo {
	text := activityField{
		Name:     "text",
		Required: true,
		Description: "Pointed Hebrew in NFC. There is no separate audio, gloss, or choices field. " +
			"Audio is the clip for this exact text. The English gloss comes from the lexicon.",
	}
	typ := func(description string) activityField {
		return activityField{Name: "type", Required: true, Description: description}
	}
	return []activityInfo{
		{
			Type:    units.ActivityListenPick,
			Purpose: "The learner hears the clip and picks the meaning. The spelling stays hidden until the answer.",
			Fields: []activityField{
				typ("listen_pick"),
				text,
			},
		},
		{
			Type:    units.ActivityRecall,
			Purpose: "Hebrew to English. text is the pointed prompt. The answer is the lexicon gloss.",
			Fields: []activityField{
				typ("recall"),
				text,
			},
		},
		{
			Type:    units.ActivityProduce,
			Purpose: "English to Hebrew. text is the pointed answer the learner should produce. The prompt is the gloss. Tiles are not stored on the activity.",
			Fields: []activityField{
				typ("produce"),
				text,
			},
		},
		{
			Type:    units.ActivityCloze,
			Purpose: "A pointed sentence with a gap. text is the whole sentence. The new word that appears in it is the gap. There is no blank marker yet.",
			Fields: []activityField{
				typ("cloze"),
				text,
			},
		},
		{
			Type:    units.ActivityPattern,
			Purpose: "One pattern plus a pointed example. text is the example. The unit concept is the rule. Root and binyan practice wait until the learner already knows enough family words.",
			Fields: []activityField{
				typ("pattern"),
				text,
			},
		},
		{
			Type:    units.ActivityStory,
			Purpose: "Comprehensible input. text is the whole pointed story. Coverage is counted on this text. Names and numbers are exempt. Every other Hebrew word needs niqqud and a lexicon entry.",
			Fields: []activityField{
				typ("story"),
				text,
			},
		},
		{
			Type:    units.ActivityShadow,
			Purpose: "The learner repeats the line. text is the pointed line. Use it only after the same text has a listen_pick or recall in that lesson.",
			Fields: []activityField{
				typ("shadow"),
				text,
			},
		},
	}
}

// scopeGuidance is authoring advice. Enforced is false: the validator
// does not check these notes.
type scopeGuidance struct {
	Kind     string   `json:"kind"`
	Enforced bool     `json:"enforced"`
	Summary  string   `json:"summary"`
	Notes    []string `json:"notes"`
}

func authoringGuidance() scopeGuidance {
	return scopeGuidance{
		Kind:     "guidance",
		Enforced: false,
		Summary:  "Guidance for shared and custom units. Not an enforced rule. The validator does not check these notes.",
		Notes: []string{
			"A custom unit, scope learner:<id>, recycles that learner's struggled words first, then learning lexemes from placement, then new words.",
			"Confusion pairs make good minimal-pair distractors.",
			"A shared unit assumes no learner data. Do not depend on struggle, placement, or which shared units a learner has finished.",
		},
	}
}

func rulesDocument() ([]byte, error) {
	doc := struct {
		Rules          []units.Rule     `json:"rules"`
		PlayerGuidance []units.Guidance `json:"playerGuidance"`
		Budgets        Budgets          `json:"budgets"`
		Guidance       scopeGuidance    `json:"guidance"`
	}{
		Rules:          units.Rules,
		PlayerGuidance: units.PlayerGuidance,
		Budgets:        currentBudgets(),
		Guidance:       authoringGuidance(),
	}
	return json.Marshal(doc)
}
