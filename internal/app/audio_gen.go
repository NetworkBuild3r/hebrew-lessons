package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// audioGen is the generate_audio adapter. The tool stays in internal/mcp.
// Resolving a unit id and writing clips stays here so internal/mcp does
// not import the database and internal/db does not import internal/mcp.
type audioGen struct {
	gen   *audio.Generator
	store *db.Store
}

func (a audioGen) GenerateAudio(ctx context.Context, req mcp.GenerateAudioRequest) ([]mcp.AudioClip, error) {
	items, err := a.items(ctx, req)
	if err != nil {
		return nil, err
	}
	got, err := a.gen.GenerateItems(ctx, items)
	if err != nil {
		slog.Error("audio generate", "err", err)
		if strings.Contains(err.Error(), "at most") {
			return nil, fmt.Errorf("At most %d texts per call.", audio.MaxTexts)
		}
		return nil, errors.New("Audio could not be generated.")
	}
	out := make([]mcp.AudioClip, 0, len(got))
	for _, item := range got {
		out = append(out, mcp.AudioClip{
			Text:    item.Text,
			TextNFC: item.TextNFC,
			Speaker: item.Speaker,
			Status:  item.Status,
			AudioID: item.AudioID,
			Error:   item.Detail,
		})
	}
	return out, nil
}

// itemKey is what makes two items one clip: the normalized text and the speaker.
func itemKey(text, speaker string) string {
	return speaker + "\x00" + audio.NormalizeText(text)
}

// items is the request as a list of clips to make, in order: texts, then lines,
// then what the unit adds. An item that repeats an earlier one is dropped, so a
// text the caller passed is not asked for twice when the unit has it too. The
// speaker of a line is checked against a and b by the tool and again by the
// generator, and nothing here maps a speaker to a voice.
func (a audioGen) items(ctx context.Context, req mcp.GenerateAudioRequest) ([]audio.Item, error) {
	var items []audio.Item
	seen := map[string]bool{}
	// What the caller passed is kept as sent, one result per entry, even a
	// repeat or a blank: the result row says skipped or failed. Only what the
	// unit adds is dropped when it repeats something already asked for.
	for _, text := range req.Texts {
		seen[itemKey(text, "")] = true
		items = append(items, audio.Item{Text: text})
	}
	for _, l := range req.Lines {
		seen[itemKey(l.Text, l.Speaker)] = true
		items = append(items, audio.Item{Text: l.Text, Speaker: l.Speaker})
	}
	if req.UnitID != nil {
		more, err := a.unitItems(ctx, *req.UnitID)
		if err != nil {
			return nil, err
		}
		for _, it := range more {
			key := itemKey(it.Text, it.Speaker)
			if seen[key] {
				continue
			}
			seen[key] = true
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return nil, errors.New("Pass texts, lines, or a unitId whose activities have Hebrew text.")
	}
	if len(items) > audio.MaxTexts {
		return nil, fmt.Errorf("At most %d texts per call.", audio.MaxTexts)
	}
	return items, nil
}

// unitItems is the Hebrew activity text validate_unit checks for a clip, in
// document order, one row per normalized spelling and speaker. A dialogue
// contributes one row per line with that line's speaker.
func (a audioGen) unitItems(ctx context.Context, unitID int64) ([]audio.Item, error) {
	conn, err := a.store.DB()
	if err != nil {
		slog.Error("audio unit", "err", err)
		return nil, errors.New("The unit could not be read.")
	}
	var doc string
	err = conn.QueryRowContext(ctx, `SELECT doc FROM units WHERE id = ?`, unitID).Scan(&doc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("No unit has id %d.", unitID)
	}
	if err != nil {
		slog.Error("audio unit", "err", err)
		return nil, errors.New("The unit could not be read.")
	}
	var unit units.Unit
	if err := json.Unmarshal([]byte(doc), &unit); err != nil {
		return nil, errors.New("The unit document could not be read.")
	}
	seen := map[string]bool{}
	var out []audio.Item
	take := func(text, speaker string) {
		if strings.TrimSpace(text) == "" || !hebrewText(text) {
			return
		}
		key := itemKey(text, speaker)
		if audio.NormalizeText(text) == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, audio.Item{Text: text, Speaker: speaker})
	}
	for _, lesson := range unit.Lessons {
		for _, act := range lesson.Activities {
			if act.Type == units.ActivityDialogue {
				for _, line := range act.Lines {
					take(line.Text, line.Speaker)
				}
				continue
			}
			take(act.Text, "")
		}
	}
	return out, nil
}

func hebrewText(s string) bool {
	for _, r := range s {
		if r >= 0x05D0 && r <= 0x05EA {
			return true
		}
	}
	return false
}
