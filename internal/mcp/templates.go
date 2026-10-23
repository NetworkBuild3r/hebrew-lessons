package mcp

import (
	"embed"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed resources/templates/*.md
var templateFS embed.FS

// WorksheetTemplate is one class worksheet shape and the resource that describes it.
type WorksheetTemplate struct {
	Type  string `json:"type"`
	URI   string `json:"uri"`
	Title string `json:"title"`
}

func worksheetTemplates() []WorksheetTemplate {
	return []WorksheetTemplate{
		{Type: "topic-conversation", URI: "hebrew://templates/topic-conversation", Title: "Topic conversation"},
		{Type: "review-sheet", URI: "hebrew://templates/review-sheet", Title: "Review sheet"},
		{Type: "verb-workshop", URI: "hebrew://templates/verb-workshop", Title: "Verb workshop"},
		{Type: "short-story", URI: "hebrew://templates/short-story", Title: "Short story with true or false"},
		{Type: "foundations-reading", URI: "hebrew://templates/foundations-reading", Title: "Foundations reading"},
	}
}

func registerTemplateResources(server *mcp.Server) error {
	for _, t := range worksheetTemplates() {
		name := t.Type + ".md"
		body, err := templateFS.ReadFile("resources/templates/" + name)
		if err != nil {
			return err
		}
		addTextResource(server, &mcp.Resource{
			URI:         t.URI,
			Name:        t.Type,
			Title:       t.Title,
			Description: "Worksheet shape. Original guidance. It names no person.",
			MIMEType:    "text/markdown",
		}, string(body))
	}
	return nil
}
