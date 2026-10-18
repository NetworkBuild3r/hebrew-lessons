package units

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type lookup struct{}

func (lookup) KnownLexemes(context.Context, int) (map[string]bool, error) { return nil, nil }
func (lookup) HasAudio(context.Context, string) (bool, error)             { return false, nil }
func (lookup) LexemeByText(context.Context, string) (int64, bool, error) {
	return 0, false, nil
}
func (lookup) Facts(context.Context, string) (LexemeFacts, bool, error) {
	return LexemeFacts{}, false, nil
}

func TestValidateStub(t *testing.T) {
	doc := &Unit{
		Title:     "Hello",
		Situation: "street",
		Concept:   "greetings",
		NewWords:  []string{"שָׁלוֹם"},
		Lessons: []Lesson{{
			Kind: KindLesson,
			Activities: []Activity{{
				Type: ActivityListenPick,
				Text: "שָׁלוֹם",
			}},
		}},
	}
	if got := Validate(context.Background(), doc, lookup{}); len(got) == 0 {
		t.Fatal("incomplete unit should not validate")
	}
	doc.Position = 4
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "position") || strings.Contains(string(raw), "Position") {
		t.Fatalf("position leaked into the document: %s", raw)
	}
	var back Unit
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.NewWords[0] != "שָׁלוֹם" || back.Lessons[0].Activities[0].Type != ActivityListenPick {
		t.Fatalf("round trip: %+v", back)
	}
	if back.Position != 0 {
		t.Fatalf("position round trip = %d", back.Position)
	}
}
