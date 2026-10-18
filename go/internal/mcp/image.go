package mcp

import (
	"os"
	"regexp"
	"strings"
	"unicode"
)

// plan_image is the dry-run from server/media/openai-images.js. It never
// calls the network. NFKC and Default_Ignorable_Code_Point are approximated
// without golang.org/x/text: the module may only add the MCP SDK. Trim,
// Unicode space collapse, and Cf removal cover the prompts the service
// refuses; a compatibility-equivalent character that is not Cf can differ.

const (
	pinnedUnitMills = 11
	dollarMills     = 1000
	maxPlanCount    = 1000
	maxPromptLength = 4000
)

type imageError struct {
	code string
}

func (e imageError) Error() string {
	switch e.code {
	case "missing_key":
		return "Image generation is not configured."
	case "prompt_refused":
		return "That prompt is not allowed."
	default:
		return "The image request is not valid."
	}
}

func planImage(prompt string, count *int, apiKey string) (Value, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return Value{}, imageError{code: "missing_key"}
	}
	norm, err := readPrompt(prompt, key)
	if err != nil {
		return Value{}, err
	}
	_ = norm
	n := 1
	if count != nil {
		if *count < 1 || *count > maxPlanCount {
			return Value{}, imageError{code: "invalid_input"}
		}
		n = *count
	}
	return Object(
		Field{Key: "count", Val: Int(int64(n))},
		Field{Key: "estimatedDollars", Val: Number(dollarString(n))},
	), nil
}

func dollarString(count int) string {
	mills := count * pinnedUnitMills
	whole := mills / dollarMills
	frac := mills % dollarMills
	if frac == 0 {
		return itoa(whole)
	}
	s := itoa(frac)
	for len(s) < 3 {
		s = "0" + s
	}
	for len(s) > 1 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	return itoa(whole) + "." + s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func readPrompt(raw, apiKey string) (string, error) {
	if raw == "" || UTF16Len(raw) > maxPromptLength {
		return "", imageError{code: "invalid_input"}
	}
	for _, r := range raw {
		if r < 0x20 {
			return "", imageError{code: "invalid_input"}
		}
	}
	prompt := normalizePrompt(raw)
	if prompt == "" || UTF16Len(prompt) > maxPromptLength {
		return "", imageError{code: "invalid_input"}
	}
	if apiKey != "" && (strings.Contains(raw, apiKey) || strings.Contains(prompt, apiKey)) {
		return "", imageError{code: "prompt_refused"}
	}
	if promptRefused(prompt) {
		return "", imageError{code: "prompt_refused"}
	}
	return prompt, nil
}

func normalizePrompt(text string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.TrimSpace(text) {
		if unicode.Is(unicode.Cf, r) {
			continue
		}
		if unicode.IsSpace(r) {
			if prevSpace {
				continue
			}
			b.WriteByte(' ')
			prevSpace = true
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

type promptTerm struct {
	phrase string
	re     *regexp.Regexp
}

func phraseTerm(value string) promptTerm {
	body := regexp.QuoteMeta(value)
	body = strings.ReplaceAll(body, `\ `, `\s+`)
	re := regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])` + body + `(?:$|[^\p{L}\p{N}])`)
	return promptTerm{phrase: value, re: re}
}

var promptTerms = []promptTerm{
	phraseTerm("sexual act"),
	phraseTerm("sexual"),
	phraseTerm("sex"),
	phraseTerm("intercourse"),
	phraseTerm("pornography"),
	phraseTerm("porn"),
	phraseTerm("nude"),
	phraseTerm("naked"),
	phraseTerm("erotic"),
	phraseTerm("anal"),
	phraseTerm("minor"),
	phraseTerm("minors"),
	phraseTerm("child"),
	phraseTerm("children"),
	phraseTerm("kid"),
	phraseTerm("kids"),
	phraseTerm("toddler"),
	phraseTerm("infant"),
	phraseTerm("baby"),
	phraseTerm("underage"),
	phraseTerm("teenager"),
	phraseTerm("teen"),
	phraseTerm("teens"),
	{re: regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])under\s*(?:18|eighteen)(?:$|[^\p{L}\p{N}])`)},
	{re: regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])(?:[0-9]|1[0-7])\s*(?:years?|yrs?)\s*old(?:$|[^\p{L}\p{N}])`)},
	phraseTerm("likeness"),
	phraseTerm("celebrity"),
	phraseTerm("celebrities"),
	phraseTerm("real person"),
	phraseTerm("real person's"),
	phraseTerm("person's likeness"),
}

func promptRefused(prompt string) bool {
	for _, term := range promptTerms {
		if term.re.MatchString(prompt) {
			return true
		}
	}
	return false
}

func openAIKey() string {
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
}
