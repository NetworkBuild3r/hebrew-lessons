package richtext

import (
	"strings"
	"testing"
)

func TestParseAllowsTheGrammar(t *testing.T) {
	body := strings.Join([]string{
		"## How the verb agrees",
		"",
		"The present changes with **who** is speaking.",
		"",
		"- one form",
		"- {he:אֲנִי לוֹמֵד|ani lomed}",
		"",
		"See [the lesson](/lesson/3/0) and [the root](/roots/ל־מ־ד).",
	}, "\n")
	nodes, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 4 || nodes[0].Kind != KindH2 || nodes[1].Kind != KindP || nodes[2].Kind != KindUL || nodes[3].Kind != KindP {
		t.Fatalf("blocks = %#v", nodes)
	}
	he := nodes[2].Children[1].Children[0]
	if he.Kind != KindHe || he.Text != "אֲנִי לוֹמֵד" || he.Translit != "ani lomed" {
		t.Fatalf("hebrew = %#v", he)
	}
	if !hasLink(nodes, "/lesson/3/0") || !hasLink(nodes, "/roots/ל־מ־ד") {
		t.Fatalf("links missing: %#v", nodes)
	}
}

func TestParseRefusesOutsideTheGrammar(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		pointer string
	}{
		{"script", "Hello <script>alert(1)</script>", "/body/lines/0"},
		{"image", "Look ![x](/learn/a)", "/body/lines/0"},
		{"offsite", "See [docs](https://example.com).", "/body/lines/0"},
		{"h1", "# Title", "/body/lines/0"},
		{"empty", "   ", "/body"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nodes, err := Parse(c.body)
			if err == nil || len(nodes) != 0 {
				t.Fatalf("got nodes %v err %v", nodes, err)
			}
			pe, ok := err.(*ParseError)
			if !ok || pe.Pointer != c.pointer {
				t.Fatalf("err = %v, want pointer %s", err, c.pointer)
			}
		})
	}
}

func TestSanitizeDropsABadLink(t *testing.T) {
	in := []Node{{Kind: KindLink, Href: "https://example.com", Children: []Node{{Kind: KindText, Text: "docs"}}}}
	out := Sanitize(in)
	if out[0].Kind != KindText || out[0].Text != "docs" || out[0].Href != "" {
		t.Fatalf("got %#v", out[0])
	}
}

func hasLink(nodes []Node, href string) bool {
	for _, n := range nodes {
		if n.Kind == KindLink && n.Href == href {
			return true
		}
		if hasLink(n.Children, href) {
			return true
		}
	}
	return false
}
