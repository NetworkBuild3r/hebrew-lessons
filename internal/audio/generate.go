package audio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const (
	// MaxTexts is how many clips one Generate call will accept.
	// Unit 1 is 20 texts. A larger batch waits for another call so one
	// request cannot hold the provider open without a bound.
	MaxTexts = 40

	// BatchSize is how many clips go to the provider before the next batch starts.
	// Ten clips in one HTTP call is the size SPEC-006 found safe inside the 30 s
	// server write timeout. A caller with no time limit of its own (audio-ensure)
	// works through a long list in batches of this size.
	BatchSize = 10

	// StatusGenerated means this call stored a new clip.
	StatusGenerated = "generated"
	// StatusSkipped means a clip for this text was already stored.
	// Synthesize was not called.
	StatusSkipped = "skipped"
	// StatusFailed means this text was not stored. Earlier texts in the
	// same call stay stored.
	StatusFailed = "failed"

	// storeTimeout bounds saving a clip that has already been made.
	storeTimeout = 5 * time.Second

	detailBudget = "not attempted: the time budget of one call was used up; call again, because clips already stored are skipped"
)

// CallBudget is how long one generate_audio call may spend with the provider.
// The server's write timeout is 30 s, and a response written after it is lost
// and the clips the caller waited for look like they were never made. 22 s
// leaves room to store the last clip and write the reply. It is a variable only
// so that a test can shorten it; production does not change it. A test whose
// provider returns immediately sets Generator.Budget above this default,
// instead of raising it, so a busy CPU cannot trip the deadline.
var CallBudget = 22 * time.Second

// BudgetClock measures a Generator's Budget. A nil clock uses the wall clock
// through context.WithTimeout, which is the production path. A test clock
// moves only when the test advances it, so the deadline does not follow CPU load.
type BudgetClock interface {
	Now() time.Time
	// AfterFunc runs f after d on this clock. stop reports whether f has not
	// run. f runs on the goroutine that advances the clock.
	AfterFunc(d time.Duration, f func()) (stop func() bool)
}

// Item is one clip to make: the text, and who says it. Speaker is "" for
// narration or a single word (the default voice), or a or b for one line of a
// dialogue (INIT-015/SPEC-026).
type Item struct {
	Text    string
	Speaker string
}

// Result is one text from Generate, in request order.
// Text is the caller's string. TextNFC is the audio_assets key, the same
// string units.Normalize produces and validate passes to HasAudio.
// Speaker is the item's speaker. AudioID is set when a clip is stored. Detail
// is set when Status is failed.
type Result struct {
	Text    string
	TextNFC string
	Speaker string
	Status  string
	AudioID string
	Detail  string
}

// Generator synthesizes missing clips and stores them.
// A text that already has a row in audio_assets is skipped.
// TTS nil fails those texts at call time. It does not stop the process
// from starting, and it does not skip the lookup of clips already stored.
//
// Voice is ELEVENLABS_VOICE_ID: the female voice, speaker a, and the default
// for every item with no speaker. VoiceMale is ELEVENLABS_VOICE_ID_MALE, speaker b.
type Generator struct {
	TTS   TTS
	Store *db.Store
	// Dir is the directory of <sha256>.mp3 files, <DataDir>/audio.
	Dir       string
	Voice     string
	VoiceMale string
	Model     string
	// Budget, when it is above zero, limits how long one call spends. Texts the
	// call does not reach come back failed with a "call again" detail. Zero means
	// no limit, which is what the audio-ensure command uses.
	Budget time.Duration
	// BudgetClock measures Budget. Nil uses the wall clock.
	BudgetClock BudgetClock
}

// Voices is the speaker table this generator uses.
func (g *Generator) Voices() Voices { return Voices{Female: g.Voice, Male: g.VoiceMale} }

// Generate stores one clip per text, all in the default voice. It is
// GenerateItems with no speakers.
func (g *Generator) Generate(ctx context.Context, texts []string) ([]Result, error) {
	items := make([]Item, len(texts))
	for i, t := range texts {
		items[i] = Item{Text: t}
	}
	return g.GenerateItems(ctx, items)
}

// GenerateItems stores one clip per item. Items that normalize to the same
// text and speaker share one clip: the first stores it, and a later copy is
// skipped. The same text with speaker a and speaker b is two clips, in two voices.
//
// An item with no speaker keeps the rule of every clip made before dialogue: it
// is skipped when its text has a clip in any voice, and a new clip uses the
// default voice. An item with speaker a or b is skipped only when its text has
// a clip in that speaker's voice. A speaker b item with no male voice
// configured fails with ErrMaleVoiceUnset and the provider is not called, and
// no clip is ever stored in the other speaker's voice.
//
// A provider or database failure is that item's StatusFailed. Items that
// already succeeded stay stored. There is no batch transaction.
func (g *Generator) GenerateItems(ctx context.Context, items []Item) ([]Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g == nil || g.Store == nil {
		return nil, errors.New("audio: generator is not configured")
	}
	if g.Dir == "" {
		return nil, errors.New("audio: audio directory is empty")
	}
	if len(items) == 0 {
		return nil, errors.New("audio: at least one text is required")
	}
	if len(items) > MaxTexts {
		return nil, fmt.Errorf("audio: at most %d texts", MaxTexts)
	}
	model := firstNonEmpty(g.Model, DefaultModel)
	voices := g.Voices()
	// The budget bounds the whole call. The parent's own cancellation is not a
	// budget stop, so the two are told apart below.
	callCtx, cancel := g.callContext(ctx)
	defer cancel()
	out := make([]Result, 0, len(items))
	for _, it := range items {
		if err := callCtx.Err(); err != nil {
			out = append(out, failRemaining(items[len(out):], stopDetail(ctx, err))...)
			return out, nil
		}
		res := g.one(callCtx, it, voices, model)
		if res.Status == StatusFailed && ctx.Err() == nil && callCtx.Err() != nil {
			// The budget ran out inside this item. Its provider call was cut.
			res.Detail = detailBudget
		}
		out = append(out, res)
	}
	return out, nil
}

// callContext is the context one call may spend. A nil BudgetClock is the
// production path: context.WithTimeout for Budget of wall time. A test clock
// cancels the context when that clock, not the wall clock, passes Budget.
func (g *Generator) callContext(parent context.Context) (context.Context, context.CancelFunc) {
	if g == nil || g.Budget <= 0 {
		return parent, func() {}
	}
	if g.BudgetClock == nil {
		return context.WithTimeout(parent, g.Budget)
	}
	ctx, cancel := context.WithCancel(parent)
	stop := g.BudgetClock.AfterFunc(g.Budget, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}

// stopDetail is the failed detail for an item that was never tried: a budget
// stop gets the "call again" text, and a cancelled caller gets the cause.
func stopDetail(parent context.Context, err error) string {
	if parent.Err() == nil {
		return detailBudget
	}
	return err.Error()
}

// lookup finds a stored clip for the item: any voice for an item with no speaker,
// otherwise only the speaker's voice.
func (g *Generator) lookup(ctx context.Context, key, speaker, voice string) (db.Asset, bool, error) {
	if speaker == "" {
		return g.Store.AudioByText(ctx, key)
	}
	return g.Store.AudioByTextVoice(ctx, key, voice)
}

func (g *Generator) one(ctx context.Context, it Item, voices Voices, model string) Result {
	key := NormalizeText(it.Text)
	item := Result{Text: it.Text, TextNFC: key, Speaker: it.Speaker}
	if key == "" {
		item.Status = StatusFailed
		item.Detail = "text is empty"
		return item
	}
	voice := voices.Default()
	if it.Speaker != "" {
		v, err := voices.ForSpeaker(it.Speaker)
		if err != nil {
			item.Status = StatusFailed
			item.Detail = err.Error()
			return item
		}
		voice = v
	}
	existing, ok, err := g.lookup(ctx, key, it.Speaker, voice)
	if err != nil {
		slog.Error("audio lookup", "err", err)
		item.Status = StatusFailed
		item.Detail = "The clip could not be read."
		return item
	}
	if ok {
		item.Status = StatusSkipped
		item.AudioID = existing.ID
		return item
	}
	if g.TTS == nil {
		item.Status = StatusFailed
		item.Detail = "audio: elevenlabs api key is not set"
		return item
	}
	body, err := g.TTS.Synthesize(ctx, key, voice, model)
	if err != nil {
		item.Status = StatusFailed
		item.Detail = err.Error()
		return item
	}
	if len(body) == 0 {
		item.Status = StatusFailed
		item.Detail = "audio: elevenlabs returned no audio"
		return item
	}
	sum := sha256.Sum256(body)
	id := hex.EncodeToString(sum[:])
	if err := writeAudioFile(g.Dir, id, body); err != nil {
		slog.Error("audio write", "err", err)
		item.Status = StatusFailed
		item.Detail = "The clip could not be stored."
		return item
	}
	// The clip is made, so it is stored even if the call's budget ran out a moment
	// ago: the budget bounds the provider, and storing is quick. Without this a clip
	// could be saved and still be reported as failed.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
	defer cancel()
	inserted, err := g.Store.InsertAudio(ctx, db.Asset{
		ID:      id,
		TextNFC: key,
		Voice:   voice,
		Model:   model,
		Bytes:   int64(len(body)),
	})
	if err != nil {
		slog.Error("audio insert", "err", err)
		item.Status = StatusFailed
		item.Detail = "The clip could not be stored."
		return item
	}
	if !inserted {
		stored, found, err := g.lookup(ctx, key, it.Speaker, voice)
		if err != nil {
			slog.Error("audio lookup after insert", "err", err)
			item.Status = StatusFailed
			item.Detail = "The clip could not be stored."
			return item
		}
		if !found || stored.ID == "" {
			item.Status = StatusFailed
			item.Detail = "The clip bytes are already stored for different text."
			return item
		}
		id = stored.ID
	}
	item.Status = StatusGenerated
	item.AudioID = id
	return item
}

func failRemaining(items []Item, detail string) []Result {
	out := make([]Result, 0, len(items))
	for _, it := range items {
		out = append(out, Result{
			Text:    it.Text,
			TextNFC: NormalizeText(it.Text),
			Speaker: it.Speaker,
			Status:  StatusFailed,
			Detail:  detail,
		})
	}
	return out
}
