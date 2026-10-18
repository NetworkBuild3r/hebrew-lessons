package path

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type planDoc struct {
	ID       string
	Title    string
	Status   string
	Audience string
	Assignee string
	Units    []unitDoc
}

type unitDoc struct {
	ID             string
	Title          string
	Theme          string
	Order          int
	HasOrder       bool
	Index          int
	Prerequisites  []string
	Introduces     []string
	HasIntroduces  bool
	Source         string
	TutorClassDate string
	Lessons        []lessonDoc
}

type lessonDoc struct {
	ID            string
	Title         string
	Visibility    string
	LegacyKey     string
	Checkpoint    bool
	NewWords      []string
	Introduces    []string
	HasIntroduces bool
	Recycles      []Recycle
}

func parseSnapshot(planID, text string) (planDoc, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return planDoc{}, fmt.Errorf("learner: plan %s snapshot: %w", planID, err)
	}
	doc := planDoc{ID: planID}
	var err error
	if doc.Title, err = jsonString(raw["title"]); err != nil {
		return planDoc{}, fmt.Errorf("learner: plan %s title: %w", planID, err)
	}
	if doc.Status, err = jsonString(raw["status"]); err != nil {
		return planDoc{}, fmt.Errorf("learner: plan %s status: %w", planID, err)
	}
	if doc.Audience, err = jsonString(raw["audience"]); err != nil {
		return planDoc{}, fmt.Errorf("learner: plan %s audience: %w", planID, err)
	}
	if doc.Assignee, err = jsonNullableString(raw["assigneeId"]); err != nil {
		return planDoc{}, fmt.Errorf("learner: plan %s assigneeId: %w", planID, err)
	}
	units, err := parseUnits(raw["units"])
	if err != nil {
		return planDoc{}, fmt.Errorf("learner: plan %s units: %w", planID, err)
	}
	doc.Units = units
	return doc, nil
}

func parseUnits(raw json.RawMessage) ([]unitDoc, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []unitDoc{}, nil
	}
	var list []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	out := make([]unitDoc, 0, len(list))
	for i, item := range list {
		unit := unitDoc{Index: i}
		var err error
		if unit.ID, err = jsonString(item["id"]); err != nil {
			return nil, fmt.Errorf("unit %d id: %w", i, err)
		}
		if unit.Title, err = jsonString(item["title"]); err != nil {
			return nil, fmt.Errorf("unit %s title: %w", unit.ID, err)
		}
		if unit.Theme, err = jsonString(item["theme"]); err != nil {
			return nil, fmt.Errorf("unit %s theme: %w", unit.ID, err)
		}
		if unit.Source, err = jsonString(item["source"]); err != nil {
			return nil, fmt.Errorf("unit %s source: %w", unit.ID, err)
		}
		if unit.TutorClassDate, err = jsonString(item["tutor_class_date"]); err != nil {
			return nil, fmt.Errorf("unit %s tutor_class_date: %w", unit.ID, err)
		}
		unit.Order, unit.HasOrder, err = jsonInt(item["order"])
		if err != nil {
			return nil, fmt.Errorf("unit %s order: %w", unit.ID, err)
		}
		unit.Prerequisites, err = jsonStringList(item["prerequisites"])
		if err != nil {
			return nil, fmt.Errorf("unit %s prerequisites: %w", unit.ID, err)
		}
		unit.Introduces, unit.HasIntroduces, err = jsonOptionalStringList(item["introduces_words"])
		if err != nil {
			return nil, fmt.Errorf("unit %s introduces_words: %w", unit.ID, err)
		}
		unit.Lessons, err = parseLessons(unit.ID, item["lessons"])
		if err != nil {
			return nil, err
		}
		out = append(out, unit)
	}
	return out, nil
}

func parseLessons(unitID string, raw json.RawMessage) ([]lessonDoc, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []lessonDoc{}, nil
	}
	var list []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("unit %s lessons: %w", unitID, err)
	}
	out := make([]lessonDoc, 0, len(list))
	for i, item := range list {
		lesson := lessonDoc{}
		var err error
		if lesson.ID, err = jsonString(item["id"]); err != nil {
			return nil, fmt.Errorf("unit %s lesson %d id: %w", unitID, i, err)
		}
		if lesson.Title, err = jsonString(item["title"]); err != nil {
			return nil, fmt.Errorf("lesson %s title: %w", lesson.ID, err)
		}
		if lesson.Visibility, err = jsonString(item["visibility"]); err != nil {
			return nil, fmt.Errorf("lesson %s visibility: %w", lesson.ID, err)
		}
		if lesson.LegacyKey, err = jsonString(item["legacyStorageKey"]); err != nil {
			return nil, fmt.Errorf("lesson %s legacyStorageKey: %w", lesson.ID, err)
		}
		kind, err := jsonString(item["kind"])
		if err != nil {
			return nil, fmt.Errorf("lesson %s kind: %w", lesson.ID, err)
		}
		lesson.NewWords, err = jsonStringList(item["newWords"])
		if err != nil {
			return nil, fmt.Errorf("lesson %s newWords: %w", lesson.ID, err)
		}
		lesson.Introduces, lesson.HasIntroduces, err = jsonOptionalStringList(item["introduces_words"])
		if err != nil {
			return nil, fmt.Errorf("lesson %s introduces_words: %w", lesson.ID, err)
		}
		lesson.Recycles, err = parseRecycles(item["recycles"])
		if err != nil {
			return nil, fmt.Errorf("lesson %s recycles: %w", lesson.ID, err)
		}
		checkpoint, err := checkpointBlock(item["blocks"])
		if err != nil {
			return nil, fmt.Errorf("lesson %s blocks: %w", lesson.ID, err)
		}
		lesson.Checkpoint = checkpoint || kind == "checkpoint"
		out = append(out, lesson)
	}
	return out, nil
}

func parseRecycles(raw json.RawMessage) ([]Recycle, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []Recycle{}, nil
	}
	var list []struct {
		Word         string `json:"word"`
		FromLessonID string `json:"fromLessonId"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	out := make([]Recycle, 0, len(list))
	for _, item := range list {
		word := ExactKey(item.Word)
		if word == "" {
			continue
		}
		out = append(out, Recycle{Word: word, FromLessonID: item.FromLessonID})
	}
	return out, nil
}

func checkpointBlock(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, nil
	}
	var blocks []struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return false, err
	}
	for _, block := range blocks {
		if block.Type == "spaced-review-checkpoint" {
			return true, nil
		}
	}
	return false, nil
}

func jsonString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func jsonNullableString(raw json.RawMessage) (string, error) {
	return jsonString(raw)
}

func jsonStringList(raw json.RawMessage) ([]string, error) {
	list, _, err := jsonOptionalStringList(raw)
	return list, err
}

func jsonOptionalStringList(raw json.RawMessage) ([]string, bool, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []string{}, false, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, true, err
	}
	if list == nil {
		list = []string{}
	}
	return list, true, nil
}

// jsonInt accepts a JSON integer. A missing value is unset. A fraction is
// an error: ADR-0017 says order is an integer, and a fractional order is
// not a signal to fall back to array order for that unit alone.
func jsonInt(raw json.RawMessage) (int, bool, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, false, nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, false, err
	}
	i, err := n.Int64()
	if err != nil {
		return 0, false, err
	}
	if int64(int(i)) != i {
		return 0, false, fmt.Errorf("integer %s does not fit", n)
	}
	return int(i), true, nil
}

// positiveNumber is completedRounds > 0 for a JSON number.
// 0.5 counts. null, a JSON string, and a bool do not, including the
// string "2". Go 1.26's json.Number accepts that quoted form, and
// Node's Number("2") coerces it, so the first byte is checked here.
// See the path PR notes on that mismatch with model.js.
func positiveNumber(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] == '"' || bytes.Equal(trimmed, []byte("null")) {
		return false
	}
	var n json.Number
	if err := json.Unmarshal(trimmed, &n); err != nil {
		return false
	}
	f, err := n.Float64()
	return err == nil && f > 0
}
