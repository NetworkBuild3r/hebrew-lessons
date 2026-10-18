package pages

import (
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
)

// coverNames are the six decorative shelf illustrations. The pick is a stable
// hash of the plan id, so a guide keeps its cover across requests.
var coverNames = [...]string{
	"cover-arches",
	"cover-olive",
	"cover-sea",
	"cover-hills",
	"cover-pomegranate",
	"cover-night",
}

var topicSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// CoverFor picks one of the six covers. key is planID + "/" + lessonID, never
// a path, so two lessons on one plan do not share a picture.
func CoverFor(key string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return coverNames[h.Sum32()%uint32(len(coverNames))]
}

// NextCover is the illustration after name. An unknown name starts at the first.
func NextCover(name string) string {
	for i, n := range coverNames {
		if n == name {
			return coverNames[(i+1)%len(coverNames)]
		}
	}
	return coverNames[0]
}

// CoverLetter is one unpointed Hebrew base letter. It tries the title, then
// the topic, then a Hebrew example. A Latin title with no Hebrew example
// returns empty so the card does not paint a Latin glyph.
func CoverLetter(title, topic, example string) string {
	if letter := hebrewBase(title); letter != "" {
		return letter
	}
	if letter := hebrewBase(topic); letter != "" {
		return letter
	}
	return hebrewBase(example)
}

func hebrewBase(s string) string {
	letter := firstHebrewCluster(s)
	if letter == "" {
		return ""
	}
	return stripPoints(letter)
}

// stripPoints drops Hebrew points. Punctuation in the Hebrew block stays.
// The string is not normalized.
func stripPoints(s string) string {
	return strings.Map(func(r rune) rune {
		if isPoint(r) {
			return -1
		}
		return r
	}, s)
}

// cardSpeak is the Hebrew line a card button can voice. A separate Hebrew
// title is voiced from that line. Otherwise a Hebrew card title is voiced,
// and the button stays outside the link.
func cardSpeak(card GuideCard) string {
	if card.HeTitle != "" {
		return ""
	}
	if HasHebrew(card.Title) {
		return card.Title
	}
	return ""
}

// LangOf is the lang half of LangDir, for a template attribute.
func LangOf(s string) string {
	lang, _ := LangDir(s)
	return lang
}

// DirOf is the dir half of LangDir, for a template attribute.
func DirOf(s string) string {
	_, dir := LangDir(s)
	return dir
}

// HeroLettering is the guide title when it is Hebrew. An English title has
// no decorative Hebrew line.
func HeroLettering(title string) string {
	if HasHebrew(title) {
		return title
	}
	return ""
}

// TopicSlug is a fragment id. A Latin topic is its own slug. Anything else
// gets a stable tN id so the fragment is still a slug.
func TopicSlug(topic string, index int) string {
	lower := strings.ToLower(topic)
	if topicSlugPattern.MatchString(lower) {
		return lower
	}
	var b strings.Builder
	dash := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if topicSlugPattern.MatchString(out) {
		return out
	}
	return "t" + strconv.Itoa(index+1)
}

// AssignSlugs fills Slug and keeps two topics from sharing a fragment.
func AssignSlugs(groups []TopicGroup) {
	seen := map[string]int{}
	for i := range groups {
		base := TopicSlug(groups[i].Topic, i)
		n := seen[base]
		seen[base] = n + 1
		if n > 0 {
			base = base + "-" + strconv.Itoa(n+1)
		}
		groups[i].Slug = base
	}
}

func firstHebrewCluster(s string) string {
	rs := []rune(s)
	for i, r := range rs {
		if !isBase(r) {
			continue
		}
		j := i + 1
		for j < len(rs) && isPoint(rs[j]) {
			j++
		}
		return string(rs[i:j])
	}
	return ""
}

// Clusters splits s into base letters with the marks that follow them.
// The string is not normalized.
func Clusters(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) == 0 {
			return
		}
		out = append(out, string(cur))
		cur = nil
	}
	for _, r := range s {
		if isPoint(r) {
			cur = append(cur, r)
			continue
		}
		flush()
		cur = append(cur, r)
	}
	flush()
	return out
}

// SplitEnding takes the last n grapheme clusters as the marked ending.
func SplitEnding(s string, n int) (stem, ending string) {
	parts := Clusters(s)
	if n <= 0 || n >= len(parts) {
		return s, ""
	}
	return strings.Join(parts[:len(parts)-n], ""), strings.Join(parts[len(parts)-n:], "")
}
