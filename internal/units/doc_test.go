package units

import (
	"context"
	"encoding/json"
	"testing"
)

type lookup struct{}

func (lookup) KnownLexemes(context.Context, int64) ([]string, error) { return nil, nil }
func (lookup) HasAudio(context.Context, string) (bool, error)        { return false, nil }
func (lookup) LexemeByText(context.Context, string) (int64, bool, error) {
	return 0, false, nil
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
	if got := Validate(context.Background(), doc, lookup{}); got != nil {
		t.Fatalf("Validate = %#v, want nil", got)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var back Unit
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.NewWords[0] != "שָׁלוֹם" || back.Lessons[0].Activities[0].Type != ActivityListenPick {
		t.Fatalf("round trip: %+v", back)
	}
}
