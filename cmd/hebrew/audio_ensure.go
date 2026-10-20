package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// maxFailureReasons is how many distinct failure reasons audio-ensure prints.
const maxFailureReasons = 3

// ensureCounts is what audio-ensure reports. Only counts and short reasons are
// printed: no key, no voice id, no clip text.
type ensureCounts struct {
	Total     int // distinct texts in the manifest
	Generated int
	Skipped   int
	Failed    int
	Reasons   []string
}

// runAudioEnsure is `hebrew audio-ensure [--dry-run]`. It makes sure every clip of
// the placement deck exists in the default voice, so the 206 clips reach the
// volume of the pod it runs in. The image is distroless, so this is a subcommand of
// the binary and the manifest is embedded in it.
func runAudioEnsure(args []string) error {
	return audioEnsure(args, os.Stdout, os.Stderr, nil)
}

// audioEnsure is runAudioEnsure with its output and provider passed in. A non-nil
// tts replaces the provider chosen from the environment, which is how the tests
// run it with a fake and never reach ElevenLabs.
func audioEnsure(args []string, stdout, stderr io.Writer, tts audio.TTS) error {
	fs := flag.NewFlagSet("audio-ensure", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew audio-ensure [--dry-run]\n")
		fmt.Fprintf(fs.Output(), "  Makes every placement clip that has no clip in the default voice.\n")
		fmt.Fprintf(fs.Output(), "  It prints counts only. It is safe to run again: stored clips are skipped.\n")
		fs.PrintDefaults()
	}
	dry := fs.Bool("dry-run", false, "count the clips that would be made, and call no provider")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	cfg, err := configFromEnv()
	if err != nil {
		return err
	}
	setLogLevel(cfg.LogLevel)

	manifest, err := placement.EmbeddedManifest()
	if err != nil {
		return err
	}
	texts := manifestTexts(manifest)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := db.Open(ctx, filepath.Join(cfg.DataDir, "hebrew.sqlite"), cfg.Clock)
	if err != nil {
		return err
	}
	defer store.Close()

	voices := cfg.Voices()
	defaultVoice, err := voices.ForSpeaker(units.SpeakerA)
	if err != nil {
		return errors.New("the default voice (ELEVENLABS_VOICE_ID) is not usable")
	}
	missing, err := missingInVoice(ctx, store, texts, defaultVoice)
	if err != nil {
		return err
	}
	if *dry {
		fmt.Fprintf(stdout, "audio-ensure dry run: would_generate=%d skipped=%d total=%d\n", len(missing), len(texts)-len(missing), len(texts))
		return nil
	}

	counts := ensureCounts{Total: len(texts), Skipped: len(texts) - len(missing)}
	if len(missing) > 0 {
		if tts == nil {
			tts = providerFrom(cfg.TTS, cfg.ElevenLabsAPIKey, defaultVoice, cfg.ElevenLabsModel)
		}
		if tts == nil {
			return errors.New("ELEVENLABS_API_KEY is not set, and " + fmt.Sprint(len(missing)) + " clips need it")
		}
		audioDir := filepath.Join(cfg.DataDir, "audio")
		if err := os.MkdirAll(audioDir, 0o755); err != nil {
			return err
		}
		gen := &audio.Generator{
			TTS:       tts,
			Store:     store,
			Dir:       audioDir,
			Voice:     cfg.ElevenLabsVoice,
			VoiceMale: cfg.ElevenLabsVoiceMale,
			Model:     cfg.ElevenLabsModel,
		}
		if err := ensureMissing(ctx, stderr, gen, missing, &counts, cfg.ElevenLabsAPIKey); err != nil {
			return err
		}
	}
	printEnsure(stdout, counts)
	if counts.Failed > 0 {
		return fmt.Errorf("%d clips failed; run audio-ensure again to retry them", counts.Failed)
	}
	return nil
}

// manifestTexts is the distinct clip texts of the manifest, in manifest order.
func manifestTexts(m placement.ClipManifest) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range m.Clips {
		key := audio.NormalizeText(c.Text)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

// missingInVoice is the texts that have no clip in the given voice.
func missingInVoice(ctx context.Context, store *db.Store, texts []string, voice string) ([]string, error) {
	var out []string
	for _, t := range texts {
		_, ok, err := store.AudioByTextVoice(ctx, t, voice)
		if err != nil {
			return nil, err
		}
		if !ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// ensureMissing makes the missing clips with the same generator generate_audio
// uses, audio.BatchSize at a time, as speaker a (the default, female voice).
func ensureMissing(ctx context.Context, progress io.Writer, gen *audio.Generator, missing []string, counts *ensureCounts, apiKey string) error {
	reasons := map[string]bool{}
	batches := (len(missing) + audio.BatchSize - 1) / audio.BatchSize
	for b := 0; b < batches; b++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lo := b * audio.BatchSize
		hi := min(lo+audio.BatchSize, len(missing))
		items := make([]audio.Item, 0, hi-lo)
		for _, t := range missing[lo:hi] {
			items = append(items, audio.Item{Text: t, Speaker: units.SpeakerA})
		}
		got, err := gen.GenerateItems(ctx, items)
		if err != nil {
			return errors.New("audio could not be generated")
		}
		for _, r := range got {
			switch r.Status {
			case audio.StatusGenerated:
				counts.Generated++
			case audio.StatusSkipped:
				counts.Skipped++
			default:
				counts.Failed++
				if d := scrub(r.Detail, apiKey); d != "" {
					reasons[d] = true
				}
			}
		}
		fmt.Fprintf(progress, "audio-ensure: batch %d of %d\n", b+1, batches)
	}
	for r := range reasons {
		counts.Reasons = append(counts.Reasons, r)
	}
	sort.Strings(counts.Reasons)
	return nil
}

func printEnsure(w io.Writer, c ensureCounts) {
	fmt.Fprintf(w, "audio-ensure: generated=%d skipped=%d failed=%d total=%d\n", c.Generated, c.Skipped, c.Failed, c.Total)
	for i, r := range c.Reasons {
		if i == maxFailureReasons {
			break
		}
		fmt.Fprintf(w, "audio-ensure: failure: %s\n", r)
	}
}

// scrub removes the API key from a message. The provider client already redacts it;
// this is the second line, because this output goes to a terminal and a log.
func scrub(msg, key string) string {
	msg = strings.TrimSpace(msg)
	if key != "" {
		msg = strings.ReplaceAll(msg, key, "[redacted]")
	}
	return msg
}

// providerFrom is the TTS client for the command: the one configured, or ElevenLabs
// when a key is set, or nil.
func providerFrom(configured audio.TTS, apiKey, voice, model string) audio.TTS {
	if configured != nil {
		return configured
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	client := audio.NewElevenLabs(apiKey)
	client.Voice = voice
	client.Model = model
	return client
}
