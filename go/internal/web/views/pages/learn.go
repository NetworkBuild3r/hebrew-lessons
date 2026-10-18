package pages

import (
	"encoding/json"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/path"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/components"
)

// LearnPage is Today or the path map.
type LearnPage struct {
	Snap   path.Snapshot
	CSRF   string
	Failed bool
}

func learnGoal(s path.Snapshot) components.RailGoal {
	return components.RailGoal{
		UnitID:  s.Rail.UnitID,
		UnitN:   s.Rail.UnitN,
		Learned: s.Rail.Learned,
		Total:   s.Rail.Total,
		WordsEn: s.Rail.WordsEn,
		WordsHe: s.Rail.WordsHe,
	}
}

func csrfHeader(token string) string {
	if token == "" {
		return ""
	}
	raw, err := json.Marshal(map[string]string{"X-CSRF-Token": token})
	if err != nil {
		return ""
	}
	return string(raw)
}
