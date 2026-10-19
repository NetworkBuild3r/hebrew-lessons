// Command gen writes internal/placement/items.json.
//
//	go run ./internal/placement/gen
//
// Run it from the module root. It downloads Hermit Dave's Hebrew frequency
// list from the commit pinned by freqURL and checks each frequency item's
// rank. The 50k list is not written into the repository.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

const (
	// freqCommit is the FrequencyWords commit that states CC-BY-SA 4.0
	// and still carries the 2018 Hebrew list unchanged.
	freqCommit = "34bb6447a96d4434dc2d4b24e0cf946c523090b3"
	freqURL    = "https://raw.githubusercontent.com/hermitdave/FrequencyWords/" + freqCommit + "/content/2018/he/he_50k.txt"
)

// ktivMale maps a pointed skeleton to the subtitle spelling when the list
// writes a mater the dictionary form leaves out.
var ktivMale = map[string]string{
	"לסים": "לסיים",
	"עיף":  "עייף",
	"שלחן": "שולחן",
}

type rawItem struct {
	ID       string
	Text     string
	Band     *int
	Gloss    string
	POS      string
	Translit string
	Gender   string
	Source   string
	Rank     int
	List     string
}

type outItem struct {
	ID       string `json:"id"`
	TextNFC  string `json:"text_nfc"`
	Band     int    `json:"band"`
	GlossEN  string `json:"gloss_en"`
	Translit string `json:"translit"`
	Gender   string `json:"gender"`
	POS      string `json:"pos"`
	Verified bool   `json:"verified"`
	Source   string `json:"source"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "placement gen: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "internal", "placement")
	notice, err := os.ReadFile(filepath.Join(dir, "NOTICE"))
	if err != nil {
		return err
	}
	if !bytes.Contains(notice, []byte(freqCommit)) {
		return fmt.Errorf("NOTICE does not cite %s", freqCommit)
	}

	list, err := downloadList()
	if err != nil {
		return err
	}
	if len(list) != 50000 {
		return fmt.Errorf("frequency list has %d lines, want 50000", len(list))
	}
	raw := curatedItems()
	out := make([]outItem, 0, len(raw))
	seenID := map[string]struct{}{}
	seenText := map[string]string{}
	seenBare := map[string]string{}
	counts := map[int]int{}

	for _, item := range raw {
		if item.Source == "tutor" && !norm.NFC.IsNormalString(item.Text) {
			return fmt.Errorf("%s: tutor pointing is not NFC", item.ID)
		}
		text := norm.NFC.String(item.Text)
		bare := unpointed(text)
		if _, ok := seenID[item.ID]; ok {
			return fmt.Errorf("duplicate id %s", item.ID)
		}
		seenID[item.ID] = struct{}{}
		if prev, ok := seenText[text]; ok {
			return fmt.Errorf("%s and %s share pointed text", item.ID, prev)
		}
		seenText[text] = item.ID
		switch item.Gender {
		case "", "m", "f", "both":
		default:
			return fmt.Errorf("%s: gender %q", item.ID, item.Gender)
		}
		if item.Translit == "" || item.Gloss == "" || item.POS == "" {
			return fmt.Errorf("%s: gloss, translit, and pos are required", item.ID)
		}
		if item.Band == nil {
			return fmt.Errorf("%s: band is required", item.ID)
		}

		switch item.Source {
		case "tutor":
			if *item.Band != 0 {
				return fmt.Errorf("%s: tutor items are band 0", item.ID)
			}
			counts[0]++
		case "freq":
			if item.Rank < 1 || item.Rank > len(list) {
				return fmt.Errorf("%s: frequency item is incomplete", item.ID)
			}
			if list[item.Rank-1] != item.List {
				return fmt.Errorf("%s: rank %d is %s, want %s", item.ID, item.Rank, list[item.Rank-1], item.List)
			}
			if got := bandForRank(item.Rank); got != *item.Band {
				return fmt.Errorf("%s: rank %d is band %d, item says %d", item.ID, item.Rank, got, *item.Band)
			}
			if !sameWord(bare, item.List) {
				return fmt.Errorf("%s: pointed form %s does not match list token %s", item.ID, bare, item.List)
			}
			counts[*item.Band]++
		default:
			return fmt.Errorf("%s: source %s", item.ID, item.Source)
		}
		if prev, ok := seenBare[bare]; ok {
			return fmt.Errorf("%s and %s share unpointed form %s", item.ID, prev, bare)
		}
		seenBare[bare] = item.ID
		out = append(out, outItem{
			ID:       item.ID,
			TextNFC:  text,
			Band:     *item.Band,
			GlossEN:  item.Gloss,
			Translit: item.Translit,
			Gender:   item.Gender,
			POS:      item.POS,
			Verified: false,
			Source:   item.Source,
		})
	}

	for band := 0; band <= 4; band++ {
		if counts[band] < placement.BandSize {
			return fmt.Errorf("band %d has %d items, want at least %d", band, counts[band], placement.BandSize)
		}
	}
	if len(out) < 5*placement.BandSize {
		return fmt.Errorf("items = %d, want at least %d", len(out), 5*placement.BandSize)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	path := filepath.Join(dir, "items.json")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d items)\n", path, len(out))
	return nil
}

func sameWord(bare, list string) bool {
	if bare == list {
		return true
	}
	return ktivMale[bare] == list
}

func bandForRank(rank int) int {
	switch {
	case rank >= 1 && rank <= 500:
		return 1
	case rank <= 1000:
		return 2
	case rank <= 2000:
		return 3
	case rank <= 4000:
		return 4
	default:
		return -1
	}
}

func unpointed(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x0591 && r <= 0x05C7 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func downloadList() ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, freqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "hebrew-lessons-placement-gen")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", freqURL, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var words []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("bad frequency line %q", line)
		}
		words = append(words, fields[0])
	}
	return words, nil
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
