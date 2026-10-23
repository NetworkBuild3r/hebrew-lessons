// Package richtext parses the restricted markdown stored on an explainer.
// The same node tree is what the Learn page renders. A body that uses
// anything outside the grammar is refused, and nothing is written.
package richtext

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Kind is one node in the allow-listed tree.
type Kind int

const (
	KindH2 Kind = iota
	KindH3
	KindP
	KindUL
	KindLI
	KindText
	KindStrong
	KindEm
	KindHe
	KindLink
)

// Node is one piece of an explainer. Text is plain. Children hold inline
// nodes for a heading, paragraph, or list item, and list items for a list.
// Href is set only on a link, and only after it has passed AllowHref.
// Translit is the reading under a Hebrew span, empty when the author
// wrote {he:...} with no bar.
type Node struct {
	Kind     Kind
	Text     string
	Translit string
	Href     string
	Children []Node
}

// ParseError is a refusal. Pointer is a JSON pointer into the body the
// tool was given, so the author can find the line.
type ParseError struct {
	Pointer string
	Msg     string
}

func (e *ParseError) Error() string {
	if e == nil {
		return "richtext: refused"
	}
	return e.Pointer + ": " + e.Msg
}

// MaxBody is the longest explainer body, in bytes, matching the column.
const MaxBody = 20000

// Parse turns body into a node tree. A refusal returns a *ParseError and
// no nodes. The tree contains only the allowed kinds.
func Parse(body string) ([]Node, error) {
	if !utf8.ValidString(body) {
		return nil, &ParseError{Pointer: "/body", Msg: "body is not UTF-8"}
	}
	if len(body) == 0 || len(strings.TrimSpace(body)) == 0 {
		return nil, &ParseError{Pointer: "/body", Msg: "body is empty"}
	}
	if len(body) > MaxBody {
		return nil, &ParseError{Pointer: "/body", Msg: "body is longer than 20000 bytes"}
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	var blocks []Node
	var para []string
	var paraAt int
	var list []Node
	flushPara := func() error {
		if len(para) == 0 {
			return nil
		}
		inline, err := parseInline(strings.Join(para, " "), fmt.Sprintf("/body/lines/%d", paraAt))
		para = nil
		if err != nil {
			return err
		}
		blocks = append(blocks, Node{Kind: KindP, Children: inline})
		return nil
	}
	flushList := func() {
		if len(list) == 0 {
			return
		}
		blocks = append(blocks, Node{Kind: KindUL, Children: list})
		list = nil
	}
	for i, line := range lines {
		pointer := fmt.Sprintf("/body/lines/%d", i)
		if strings.TrimSpace(line) == "" {
			if err := flushPara(); err != nil {
				return nil, err
			}
			flushList()
			continue
		}
		if refused, msg := lineRefused(line); refused {
			return nil, &ParseError{Pointer: pointer, Msg: msg}
		}
		switch {
		case strings.HasPrefix(line, "### "):
			if err := flushPara(); err != nil {
				return nil, err
			}
			flushList()
			inline, err := parseInline(strings.TrimPrefix(line, "### "), pointer)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, Node{Kind: KindH3, Children: inline})
		case strings.HasPrefix(line, "## "):
			if err := flushPara(); err != nil {
				return nil, err
			}
			flushList()
			inline, err := parseInline(strings.TrimPrefix(line, "## "), pointer)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, Node{Kind: KindH2, Children: inline})
		case strings.HasPrefix(line, "- "):
			if err := flushPara(); err != nil {
				return nil, err
			}
			inline, err := parseInline(strings.TrimPrefix(line, "- "), pointer)
			if err != nil {
				return nil, err
			}
			list = append(list, Node{Kind: KindLI, Children: inline})
		default:
			flushList()
			if len(para) == 0 {
				paraAt = i
			}
			para = append(para, strings.TrimSpace(line))
		}
	}
	if err := flushPara(); err != nil {
		return nil, err
	}
	flushList()
	if len(blocks) == 0 {
		return nil, &ParseError{Pointer: "/body", Msg: "body is empty"}
	}
	return blocks, nil
}

func lineRefused(line string) (bool, string) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(line, "<") || strings.Contains(line, ">"):
		return true, "raw HTML is not allowed"
	case strings.Contains(lower, "javascript:"):
		return true, "raw HTML is not allowed"
	case strings.Contains(line, "!["):
		return true, "images are not allowed"
	case strings.Contains(line, "`"):
		return true, "code spans are not allowed"
	case strings.HasPrefix(line, "# ") || line == "#" || strings.HasPrefix(line, "####"):
		return true, "only h2 and h3 headings are allowed"
	default:
		return false, ""
	}
}

func parseInline(s, pointer string) ([]Node, error) {
	var out []Node
	var text strings.Builder
	flush := func() {
		if text.Len() == 0 {
			return
		}
		out = append(out, Node{Kind: KindText, Text: text.String()})
		text.Reset()
	}
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "**"):
			end := strings.Index(s[i+2:], "**")
			if end < 0 {
				return nil, &ParseError{Pointer: pointer, Msg: "bold is not closed"}
			}
			inner := s[i+2 : i+2+end]
			if strings.ContainsAny(inner, "*{[") {
				return nil, &ParseError{Pointer: pointer, Msg: "bold holds plain text only"}
			}
			flush()
			out = append(out, Node{Kind: KindStrong, Children: []Node{{Kind: KindText, Text: inner}}})
			i += 2 + end + 2
		case s[i] == '*':
			end := strings.IndexByte(s[i+1:], '*')
			if end < 0 {
				return nil, &ParseError{Pointer: pointer, Msg: "italic is not closed"}
			}
			inner := s[i+1 : i+1+end]
			if strings.ContainsAny(inner, "*{[") {
				return nil, &ParseError{Pointer: pointer, Msg: "italic holds plain text only"}
			}
			flush()
			out = append(out, Node{Kind: KindEm, Children: []Node{{Kind: KindText, Text: inner}}})
			i += 1 + end + 1
		case strings.HasPrefix(s[i:], "{he:"):
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return nil, &ParseError{Pointer: pointer, Msg: "Hebrew span is not closed"}
			}
			raw := s[i+4 : i+end]
			he, tr, ok := splitHe(raw)
			if !ok {
				return nil, &ParseError{Pointer: pointer, Msg: "Hebrew span is empty or malformed"}
			}
			flush()
			out = append(out, Node{Kind: KindHe, Text: he, Translit: tr})
			i += end + 1
		case s[i] == '[':
			close := strings.Index(s[i:], "](")
			if close < 0 {
				return nil, &ParseError{Pointer: pointer, Msg: "link is not closed"}
			}
			label := s[i+1 : i+close]
			rest := s[i+close+2:]
			end := strings.IndexByte(rest, ')')
			if end < 0 {
				return nil, &ParseError{Pointer: pointer, Msg: "link is not closed"}
			}
			href := rest[:end]
			if !AllowHref(href) {
				return nil, &ParseError{Pointer: pointer, Msg: "link is not allowed"}
			}
			if strings.ContainsAny(label, "[]()") {
				return nil, &ParseError{Pointer: pointer, Msg: "link label is plain text"}
			}
			flush()
			out = append(out, Node{Kind: KindLink, Href: href, Children: []Node{{Kind: KindText, Text: label}}})
			i += close + 2 + end + 1
		default:
			text.WriteByte(s[i])
			i++
		}
	}
	flush()
	return out, nil
}

func splitHe(raw string) (he, tr string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "{") || strings.Contains(raw, "}") {
		return "", "", false
	}
	he, tr, found := strings.Cut(raw, "|")
	he = strings.TrimSpace(he)
	tr = strings.TrimSpace(tr)
	if he == "" || (found && tr == "") || strings.Contains(tr, "|") {
		return "", "", false
	}
	return he, tr, true
}

// AllowHref reports whether href may be stored and rendered.
// Only same-site Learn, Roots, and lesson paths pass.
func AllowHref(href string) bool {
	if href == "" || strings.Contains(href, "://") || strings.Contains(href, "\\") {
		return false
	}
	if strings.ContainsAny(href, "?# ") || strings.Contains(href, "..") || strings.Contains(href, "//") {
		return false
	}
	switch {
	case href == "/learn" || strings.HasPrefix(href, "/learn/"):
		return href == "/learn" || len(href) > len("/learn/")
	case href == "/roots" || strings.HasPrefix(href, "/roots/"):
		return href == "/roots" || len(href) > len("/roots/")
	case strings.HasPrefix(href, "/lesson/"):
		return len(href) > len("/lesson/")
	default:
		return false
	}
}

// Sanitize walks a tree and drops any link that fails AllowHref, leaving
// its text. The renderer calls this again so a stored body cannot grow a
// link the grammar would refuse.
func Sanitize(nodes []Node) []Node {
	out := make([]Node, len(nodes))
	for i, n := range nodes {
		out[i] = sanitize(n)
	}
	return out
}

func sanitize(n Node) Node {
	if n.Kind == KindLink && !AllowHref(n.Href) {
		n.Kind = KindText
		if len(n.Children) == 1 && n.Children[0].Kind == KindText {
			n.Text = n.Children[0].Text
		}
		n.Href = ""
		n.Children = nil
		return n
	}
	if len(n.Children) == 0 {
		return n
	}
	kids := make([]Node, len(n.Children))
	for i, c := range n.Children {
		kids[i] = sanitize(c)
	}
	n.Children = kids
	return n
}
