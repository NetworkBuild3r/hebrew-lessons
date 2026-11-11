package lessons

import "fmt"

// Week is the counts the weekly note reads. It adds no table.
type Week struct {
	Offered       int
	Started       int
	Finished      int
	Flags         int
	Answers       int
	Right         int
	Voids         int
	Pool          int
	HeavyDays     int
	LaterDayRight int
	LaterDayN     int
}

// Readout is five lines. A small week says it is too early and does not retire a lesson type.
func Readout(w Week) []string {
	early := ""
	if w.Started < 3 {
		early = " Too early to judge a lesson type."
	}
	share := 0
	if w.Offered > 0 {
		share = w.Finished * 100 / w.Offered
	}
	flag := 0
	if w.Answers > 0 {
		flag = w.Flags * 100 / w.Answers
	}
	low, high := accuracyRange(w.Right, w.Answers, w.Voids)
	later := "no later-day answers yet"
	if w.LaterDayN > 0 {
		later = fmt.Sprintf("%d%% of %d later-day answers right", w.LaterDayRight*100/w.LaterDayN, w.LaterDayN)
	}
	return []string{
		fmt.Sprintf("Lessons: %d offered, %d started, %d%% finished.%s", w.Offered, w.Started, share, early),
		fmt.Sprintf("Flags: %d%% of answers. About 3%% is healthy. Over 10%% means one fix.", flag),
		fmt.Sprintf("Accuracy: %d%% if voids count as misses, %d%% if voids are dropped.", low, high),
		fmt.Sprintf("Ready words still untaught: %d. Heavy days: %d.", w.Pool, w.HeavyDays),
		"Later-day recall: " + later + "." + early,
	}
}

func accuracyRange(right, answers, voids int) (low, high int) {
	if answers <= 0 {
		return 0, 0
	}
	low = right * 100 / answers
	base := answers - voids
	if base < 1 {
		base = 1
	}
	high = right * 100 / base
	if high > 100 {
		high = 100
	}
	return low, high
}
