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
	texts, err := a.texts(ctx, req)
	if err != nil {
		return nil, err
	}
	got, err := a.gen.Generate(ctx, texts)
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
			Status:  item.Status,
			AudioID: item.AudioID,
			Error:   item.Detail,
		})
	}
	return out, nil
}

func (a audioGen) texts(ctx context.Context, req mcp.GenerateAudioRequest) ([]string, error) {
	texts := append([]string{}, req.Texts...)
	seen := map[string]bool{}
	for _, text := range texts {
		if key := audio.NormalizeText(text); key != "" {
			seen[key] = true
		}
	}
	if req.UnitID != nil {
		more, err := a.unitTexts(ctx, *req.UnitID)
		if err != nil {
			return nil, err
		}
		for _, text := range more {
			key := audio.NormalizeText(text)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			texts = append(texts, text)
		}
	}
	if len(texts) == 0 {
		return nil, errors.New("Pass texts, or a unitId whose activities have Hebrew text.")
	}
	if len(texts) > audio.MaxTexts {
		return nil, fmt.Errorf("At most %d texts per call.", audio.MaxTexts)
	}
	return texts, nil
}

// unitTexts is the Hebrew activity text validate_unit checks for a clip,
// in document order, one row per normalized spelling.
func (a audioGen) unitTexts(ctx context.Context, unitID int64) ([]string, error) {
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
	var out []string
	for _, lesson := range unit.Lessons {
		for _, act := range lesson.Activities {
			if strings.TrimSpace(act.Text) == "" || !hebrewText(act.Text) {
				continue
			}
			key := audio.NormalizeText(act.Text)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, act.Text)
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
