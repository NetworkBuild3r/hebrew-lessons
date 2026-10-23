package web

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// canSayView reads what the learner can say and shapes it for the page. The line is empty
// when nothing is known yet, and the panel then carries the empty state.
func (s *server) canSayView(ctx context.Context, userID int64) (*views.CanSayPanel, string, error) {
	cs, err := s.model().CanSay(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	texts := make([]string, 0, len(cs.Items)+len(cs.Next))
	for _, it := range append(cs.Items, cs.Next...) {
		texts = append(texts, it.Text)
	}
	audio, err := s.store.AudioIDs(ctx, texts)
	if err != nil {
		return nil, "", err
	}
	panel := &views.CanSayPanel{
		Empty:     cs.Total == 0,
		TotalText: views.CountText(cs.Total, "thing", "things"),
		AddedText: strconv.Itoa(cs.AddedThisWeek) + " this week",
		Items:     canSayWords(cs.Items, audio),
		Next:      canSayWords(cs.Next, audio),
	}
	return panel, views.CanSayLineText(cs.Total, cs.AddedThisWeek), nil
}

func canSayWords(items []learner.CanSayItem, audio map[string]string) []views.CanSayWord {
	out := make([]views.CanSayWord, 0, len(items))
	for _, it := range items {
		word := views.CanSayWord{He: it.Text, Gloss: it.Gloss}
		if id := audio[it.Text]; id != "" {
			word.Audio = "/audio/" + id + ".mp3"
		}
		out = append(out, word)
	}
	return out
}

// fillCanSay sets the Progress section. A read failure leaves it off and is logged, so the
// rest of the page still renders.
func (s *server) fillCanSay(ctx context.Context, userID int64, v *views.ProgressView) {
	panel, _, err := s.canSayView(ctx, userID)
	if err != nil {
		slog.Warn("progress: can say", "err", err)
		return
	}
	v.CanSay = panel
}
