package web

import (
	"time"
	_ "time/tzdata" // America/Denver on images that ship no zoneinfo
)

// TODO: switch to #103 DueCutoff

// studyRolloverHour is the local hour when the study day rolls, the same
// boundary as studyday.DueCutoff. America/Denver is the learner zone until
// that helper lands.
const studyRolloverHour = 4

// studyDayCutoff is the next 4:00 in America/Denver. A time before 4:00
// uses 4:00 this morning. A time at or after 4:00 uses 4:00 tomorrow.
// That is the DueCutoff rule: one instant per study day.
func studyDayCutoff(now time.Time) time.Time {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		panic("web: America/Denver: " + err.Error())
	}
	local := now.In(loc)
	y, m, d := local.Date()
	start := time.Date(y, m, d, studyRolloverHour, 0, 0, 0, loc)
	if !local.Before(start) {
		start = time.Date(y, m, d+1, studyRolloverHour, 0, 0, 0, loc)
	}
	return start
}
