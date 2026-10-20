package web

import (
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The view models of the placement screens: what an engine item or result becomes on screen.

// placementScreen is the screen of an item's part.
func placementScreen(p placement.Part) string {
	switch p {
	case placement.PartLetters:
		return views.PlaceLetters
	case placement.PartReading:
		return views.PlaceReading
	case placement.PartTyping:
		return views.PlaceTyping
	}
	return views.PlaceListening
}

// placementAnswered counts the items the learner has answered.
func placementAnswered(items []db.PlacementItem) int {
	n := 0
	for _, it := range items {
		if it.Outcome != "" {
			n++
		}
	}
	return n
}

// placementProgress is the place of an item in its part, "4 / 12". The engine decides after each band
// whether a part goes on, so the part's length is not known ahead. The total is 12 (a band and its
// extra half) until the learner passes it, then the next step of 6. Letters are six. Typing asks the
// words that were right, up to eight, which is known exactly.
func placementProgress(items []db.PlacementItem, it placement.Item) (index, total int) {
	right := map[string]bool{}
	for _, x := range items {
		if x.Part == string(it.Part) && x.Seq <= it.Seq {
			index++
		}
		if (x.Part == string(placement.PartListening) || x.Part == string(placement.PartReading)) && x.Outcome == placement.OutcomeRight {
			right[x.ItemID] = true
		}
	}
	if index < 1 {
		index = 1
	}
	switch it.Part {
	case placement.PartLetters:
		total = placement.LettersAsked
	case placement.PartTyping:
		total = min(placement.TypingMax, len(right))
	default:
		total = placement.FirstSize + placement.ExtraSize
		if index > total {
			step := placement.FirstSize
			total = (index + step - 1) / step * step
		}
	}
	return index, max(total, index)
}

// resultsView is the results screen of a finished run.
func (h *placementHTTP) resultsView(w http.ResponseWriter, r *http.Request, res placement.RunResult) views.PlacementView {
	v := h.base(w, r, views.PlaceResults, res.RunID)
	v.Page.Title = "Your starting point · " + views.AppName
	v.Result = resultView(res)
	return v
}

// placementBands are the band names a level is described by. Level 0 is "none yet".
var placementBands = [placement.BandCount + 1]string{
	"Starting from the beginning",
	"First words",
	"Home and kitchen",
	"Out in town",
	"Daily life and work",
	"Stories and opinions",
}

func levelView(skill, icon string, s placement.SkillResult, tested bool) views.PlaceLevelView {
	l := views.PlaceLevelView{Skill: skill, Icon: icon, Tested: tested, Level: s.Level, Words: s.Estimate}
	switch {
	case !tested:
		l.Blurb = "Reading starts once the letters feel solid. Your first lessons begin with them."
	case s.Level <= 0:
		l.Blurb = "Not yet. We’ll begin with the very first words."
	default:
		l.Blurb = "Comfortable with “" + placementBands[min(s.Level, placement.BandCount)] + "” words."
	}
	return l
}

// resultView turns a run result into the results screen.
func resultView(res placement.RunResult) *views.PlaceResultView {
	out := &views.PlaceResultView{
		Listening: levelView("Listening", "sound", res.Listening, res.Listening.Tested),
		Reading:   levelView("Reading", "book", res.Reading, res.ReadingReady && res.Reading.Tested),
		Misses:    len(res.Misses),
		ReviewURL: "/placement/review?run=" + strconv.FormatInt(res.RunID, 10),
	}
	// The words figure is the same one Today and Progress read: reading when it was tested, listening otherwise.
	out.Words = res.Listening.Estimate
	if res.ReadingReady && res.Reading.Tested {
		out.Words = res.Reading.Estimate
	}
	l, rd := res.Listening.Level, res.Reading.Level
	switch {
	case !res.ReadingReady:
		out.Headline = "Letters first, then reading."
		out.Sub = "Reading comes once the letters feel solid, so your first lessons start with them."
	case l > rd:
		out.Headline = "You know more by ear than by eye."
	case rd > l:
		out.Headline = "You read better than you hear."
	case l == 0:
		out.Headline = "Let’s start from the very beginning."
	default:
		out.Headline = "You hear and read about the same."
	}
	seen := map[int64]bool{}
	for _, m := range res.Misses {
		if len(out.Work) == 3 {
			break
		}
		if seen[m.LexemeID] {
			continue
		}
		seen[m.LexemeID] = true
		out.Work = append(out.Work, views.PlaceWorkView{Pointed: m.Pointed, Gloss: m.RightGloss, Note: workNote(m.Skill)})
	}
	return out
}

func workNote(skill string) string {
	switch skill {
	case placement.SkillRead:
		return "You missed this one by eye."
	case placement.SkillRecall:
		return "You couldn’t type this one yet."
	}
	return "You missed this one by ear."
}

// placementReview groups every miss of the run under Listening, Reading and Typing, in the order asked.
func placementReview(res placement.RunResult) *views.PlaceReviewView {
	groups := []struct {
		skill, title string
	}{{placement.SkillListen, "Listening"}, {placement.SkillRead, "Reading"}, {placement.SkillRecall, "Typing"}}
	out := &views.PlaceReviewView{Title: "", Total: len(res.Misses)}
	n := 0
	for _, g := range groups {
		group := views.PlaceReviewGroup{Title: g.title}
		for _, m := range res.Misses {
			if m.Skill != g.skill {
				continue
			}
			n++
			row := views.PlaceReviewRow{ID: strconv.Itoa(n), Gloss: m.RightGloss, Pointed: m.Pointed, Picked: m.Picked, PickedKind: "picked"}
			switch {
			case m.Picked == "":
				row.PickedKind = "idk"
			case m.Skill == placement.SkillRecall:
				row.PickedKind = "typed"
			}
			if m.AudioID != "" {
				row.AudioURL = "/audio/" + m.AudioID + ".mp3"
			}
			group.Rows = append(group.Rows, row)
		}
		if len(group.Rows) > 0 {
			out.Groups = append(out.Groups, group)
		}
	}
	out.Title = placementReviewTitle(out.Total)
	return out
}

func placementReviewTitle(n int) string {
	switch n {
	case 0:
		return "You didn’t miss any"
	case 1:
		return "The word you missed"
	}
	return "The " + strconv.Itoa(n) + " words you missed"
}
