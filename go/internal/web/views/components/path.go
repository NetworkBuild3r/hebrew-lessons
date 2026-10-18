package components

import (
	"fmt"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/path"
)

// RailGoal is the sidebar block. An empty UnitID hides it.
type RailGoal struct {
	UnitID  string
	UnitN   int
	Learned int
	Total   int
	WordsEn string
	WordsHe string
}

// GateDialog is the soft-gate sheet. Next and Checkpoint are local paths.
// TriggerID is the link showModal restores focus to.
type GateDialog struct {
	UnitID     string
	Known      bool
	TitleEn    string
	TitleHe    string
	Next       string
	Checkpoint string
	TriggerID  string
}

// RingDash is the stroke for a pathLength="100" circle, from the same
// counts as the "5/8" text. Integer math, so 5 of 8 is 62.5.
func RingDash(value, max int) string {
	if max <= 0 || value <= 0 {
		return "0 100"
	}
	if value > max {
		value = max
	}
	times10 := value * 1000 / max
	return fmt.Sprintf("%d.%d 100", times10/10, times10%10)
}

// RingText is the label inside the ring, "5/8".
func RingText(value, max int) string {
	return strconv.Itoa(value) + "/" + strconv.Itoa(max)
}

func strengthEn(bucket int) string {
	en, _ := path.StrengthName(bucket)
	return "Strength: " + en
}

func strengthHe(bucket int) string {
	_, he := path.StrengthName(bucket)
	return "חוזק: " + he
}

func gateURL(id string) string {
	return "/_hx/units/" + id + "/gate-ack"
}

func unitLabelEn(n int) string {
	return fmt.Sprintf("Unit %d", n)
}

func unitLabelHe(n int) string {
	return fmt.Sprintf("יחידה %d", n)
}
