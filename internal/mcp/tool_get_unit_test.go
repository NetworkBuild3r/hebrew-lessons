package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPutUnitOmitsEmptyTarget(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	doc := helloDoc()
	want, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(want, []byte(`"target"`)) {
		t.Fatalf("marshal added target: %s", want)
	}
	first := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: doc})
	if first.UnitID == 0 || first.Revision != 1 {
		t.Fatalf("put %+v", first)
	}
	if got := storedCanonical(mem, first.UnitID); !bytes.Equal(got, want) {
		t.Fatalf("stored %s\nwant %s", got, want)
	}
	second := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &first.UnitID, Position: 0, Doc: doc})
	if !second.Unchanged || second.Revision != 1 {
		t.Fatalf("second put %+v", second)
	}
	if got := storedCanonical(mem, first.UnitID); !bytes.Equal(got, want) {
		t.Fatalf("after second put %s", got)
	}
}

func TestTargetOptionalInToolSchema(t *testing.T) {
	session := connectMCP(t, Handler(Options{AuthToken: testToken}), testToken)
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"put_unit", "validate_unit"} {
		raw, err := json.Marshal(tools.Tools[indexOf(tools, name)].InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		props, required, ok := schemaWithTarget(schema)
		if !ok {
			t.Fatalf("%s schema has no target: %s", name, raw)
		}
		for _, item := range required {
			if item == "target" {
				t.Fatalf("%s marks target required: %s", name, raw)
			}
		}
		field, _ := props["target"].(map[string]any)
		desc, _ := field["description"].(string)
		if !strings.Contains(desc, "more than one token") || !strings.Contains(desc, "exact pointed") {
			t.Fatalf("%s target description %q", name, desc)
		}
		tr, _ := props["translation"].(map[string]any)
		trDesc, _ := tr["description"].(string)
		if !strings.Contains(trDesc, "two or more Hebrew words") || !strings.Contains(trDesc, "120") {
			t.Fatalf("%s translation description %q", name, trDesc)
		}
		for _, item := range required {
			if item == "translation" {
				t.Fatalf("%s marks translation required: %s", name, raw)
			}
		}
	}
	schemaText := readResource(t, session, resourceUnitSchema)
	if !strings.Contains(schemaText, `"target"`) {
		t.Fatalf("unit schema has no target: %s", schemaText)
	}
	props, required, ok := schemaWithTarget(mustJSONValue(t, schemaText))
	if !ok {
		t.Fatal("unit schema target was not a property")
	}
	for _, item := range required {
		if item == "target" {
			t.Fatal("unit schema marks target required")
		}
	}
	tr, _ := props["translation"].(map[string]any)
	trDesc, _ := tr["description"].(string)
	if !strings.Contains(trDesc, "two or more Hebrew words") || !strings.Contains(schemaText, `"translation"`) {
		t.Fatalf("unit schema translation %q", trDesc)
	}
	for _, item := range required {
		if item == "translation" {
			t.Fatal("unit schema marks translation required")
		}
	}
	var lines map[string]any
	if items, ok := props["lines"].(map[string]any); ok {
		if itemSchema, ok := items["items"].(map[string]any); ok {
			if lineProps, ok := itemSchema["properties"].(map[string]any); ok {
				lines = lineProps
			}
		}
	}
	lineTr, _ := lines["translation"].(map[string]any)
	lineDesc, _ := lineTr["description"].(string)
	if !strings.Contains(lineDesc, "sentence-choice distractor") {
		t.Fatalf("line translation description %q", lineDesc)
	}

	catalogText := readResource(t, session, resourceActivities)
	if strings.Contains(catalogText, "no separate") {
		t.Fatalf("catalog still says there is no separate field: %s", catalogText)
	}
	var catalog struct {
		Activities []struct {
			Type   string `json:"type"`
			Fields []struct {
				Name        string `json:"name"`
				Required    bool   `json:"required"`
				Description string `json:"description"`
			} `json:"fields"`
		} `json:"activities"`
	}
	if err := json.Unmarshal([]byte(catalogText), &catalog); err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, info := range catalog.Activities {
		if info.Type != "listen_pick" {
			continue
		}
		for _, field := range info.Fields {
			if field.Name != "translation" {
				continue
			}
			saw = true
			if field.Required {
				t.Fatal("catalog marks listen_pick translation required")
			}
			if !strings.Contains(field.Description, "two or more lexicon tokens") {
				t.Fatalf("catalog translation description %q", field.Description)
			}
		}
	}
	if !saw {
		t.Fatal("catalog listen_pick has no translation field")
	}
}

func TestGetUnit(t *testing.T) {
	mem, srv := roleServer(t)
	doc := helloDoc()
	shared, err := mem.PutUnit(context.Background(), nil, &doc, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	ownDoc := doc
	ownDoc.Title = "Mine"
	own, err := mem.PutUnit(context.Background(), nil, &ownDoc, "learner:7")
	if err != nil {
		t.Fatal(err)
	}
	otherDoc := doc
	otherDoc.Title = "Theirs"
	otherDoc.Lessons[0].Activities[0].Text = coffee
	other, err := mem.PutUnit(context.Background(), nil, &otherDoc, "learner:9")
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(shared.UnitID, "published")
	mem.SetStatus(own.UnitID, "published")
	mem.SetStatus(other.UnitID, "published")

	draft, err := mem.PutUnit(context.Background(), nil, &doc, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}

	admin := postTool(t, srv.URL, "admin", "get_unit", getUnitInput{UnitID: shared.UnitID}, nil)
	body := readBody(t, admin)
	if admin.StatusCode != 200 || strings.Contains(body, `"isError":true`) || !strings.Contains(body, `"status":"published"`) || !strings.Contains(body, hello) || !strings.Contains(body, `"revision":1`) {
		t.Fatalf("admin shared %s", body)
	}
	adminOther := postTool(t, srv.URL, "admin", "get_unit", getUnitInput{UnitID: other.UnitID}, nil)
	body = readBody(t, adminOther)
	if adminOther.StatusCode != 200 || strings.Contains(body, `"isError":true`) || !strings.Contains(body, "Theirs") || !strings.Contains(body, `"scope":"learner:9"`) {
		t.Fatalf("admin other %s", body)
	}

	self := postTool(t, srv.URL, "learner", "get_unit", getUnitInput{UnitID: own.UnitID}, nil)
	body = readBody(t, self)
	if self.StatusCode != 200 || strings.Contains(body, `"isError":true`) || !strings.Contains(body, "Mine") || !strings.Contains(body, `"scope":"learner:7"`) {
		t.Fatalf("own %s", body)
	}
	sharedLearner := postTool(t, srv.URL, "learner", "get_unit", getUnitInput{UnitID: shared.UnitID}, nil)
	body = readBody(t, sharedLearner)
	if sharedLearner.StatusCode != 200 || strings.Contains(body, `"isError":true`) || !strings.Contains(body, hello) {
		t.Fatalf("learner shared %s", body)
	}

	denied := postTool(t, srv.URL, "learner", "get_unit", getUnitInput{UnitID: other.UnitID}, nil)
	body = assertDenied(t, denied, msgOtherLearnerUnit)
	if strings.Contains(body, "Theirs") || strings.Contains(body, coffee) {
		t.Fatalf("other learner leaked %s", body)
	}

	unpublished := postTool(t, srv.URL, "admin", "get_unit", getUnitInput{UnitID: draft.UnitID}, nil)
	body = assertDenied(t, unpublished, msgUnitNotPublished)
	if strings.Contains(body, `"doc"`) && strings.Contains(body, hello) {
		t.Fatalf("draft leaked %s", body)
	}

	missing := postTool(t, srv.URL, "admin", "get_unit", getUnitInput{UnitID: 99}, nil)
	assertDenied(t, missing, msgUnitNotFound)

	bad := postTool(t, srv.URL, "admin", "get_unit", getUnitInput{UnitID: 0}, nil)
	assertDenied(t, bad, "unitId must be a positive id")
}

func TestGetUnitAfterPublishThenEdit(t *testing.T) {
	mem, srv := roleServer(t)
	doc := helloDoc()
	saved, err := mem.PutUnit(context.Background(), nil, &doc, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mem.PublishUnit(context.Background(), saved.UnitID, nil); err != nil {
		t.Fatal(err)
	}
	live := postTool(t, srv.URL, "admin", "get_unit", getUnitInput{UnitID: saved.UnitID}, nil)
	body := readBody(t, live)
	if live.StatusCode != 200 || strings.Contains(body, `"isError":true`) || !strings.Contains(body, `"status":"published"`) || !strings.Contains(body, "Hello") {
		t.Fatalf("published read %s", body)
	}

	edited := doc
	edited.Title = "DraftOnlyTitle"
	edit := postTool(t, srv.URL, "admin", "put_unit", putInput{
		UnitID: &saved.UnitID, Scope: ScopeShared, Position: 0, Doc: edited,
	}, nil)
	body = readBody(t, edit)
	if edit.StatusCode != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatalf("edit %s", body)
	}
	if mem.Status(saved.UnitID) != "draft" {
		t.Fatalf("status after edit %q", mem.Status(saved.UnitID))
	}
	got := postTool(t, srv.URL, "admin", "get_unit", getUnitInput{UnitID: saved.UnitID}, nil)
	body = assertDenied(t, got, msgUnitNotPublished)
	if strings.Contains(body, "DraftOnlyTitle") {
		t.Fatalf("draft leaked %s", body)
	}
}

func TestReadOnlyTokenCanGetUnit(t *testing.T) {
	mem, srv := readOnlyHandler(t, false)
	doc := helloDoc()
	saved, err := mem.PutUnit(context.Background(), nil, &doc, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(saved.UnitID, "published")
	own, err := mem.PutUnit(context.Background(), nil, &doc, "learner:7")
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(own.UnitID, "published")
	other, err := mem.PutUnit(context.Background(), nil, &doc, "learner:9")
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(other.UnitID, "published")

	got := postTool(t, srv.URL, readOnlySmokeToken, "get_unit", getUnitInput{UnitID: saved.UnitID}, nil)
	body := readBody(t, got)
	if got.StatusCode != 200 || strings.Contains(body, `"isError":true`) || !strings.Contains(body, hello) || !strings.Contains(body, `"status":"published"`) {
		t.Fatalf("readonly shared %d %s", got.StatusCode, body)
	}
	ownRes := postTool(t, srv.URL, readOnlySmokeToken, "get_unit", getUnitInput{UnitID: own.UnitID}, nil)
	body = readBody(t, ownRes)
	if ownRes.StatusCode != 200 || strings.Contains(body, `"isError":true`) || !strings.Contains(body, `"scope":"learner:7"`) {
		t.Fatalf("readonly own %d %s", ownRes.StatusCode, body)
	}
	denied := postTool(t, srv.URL, readOnlySmokeToken, "get_unit", getUnitInput{UnitID: other.UnitID}, nil)
	assertDenied(t, denied, msgOtherLearnerUnit)
}

func storedCanonical(m *Memory, id int64) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID(id)
	if u == nil {
		return nil
	}
	return append([]byte(nil), u.canonical...)
}

func schemaWithTarget(v any) (props map[string]any, required []string, ok bool) {
	switch n := v.(type) {
	case map[string]any:
		if p, isMap := n["properties"].(map[string]any); isMap {
			if _, has := p["target"]; has {
				var req []string
				switch raw := n["required"].(type) {
				case []any:
					for _, item := range raw {
						if s, ok := item.(string); ok {
							req = append(req, s)
						}
					}
				}
				return p, req, true
			}
		}
		for _, child := range n {
			if p, req, found := schemaWithTarget(child); found {
				return p, req, true
			}
		}
	case []any:
		for _, child := range n {
			if p, req, found := schemaWithTarget(child); found {
				return p, req, true
			}
		}
	}
	return nil, nil, false
}

func mustJSONValue(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
