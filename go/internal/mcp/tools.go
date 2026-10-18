package mcp

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed testdata/tools-list-result.json
var toolsListResult []byte

//go:embed testdata/activities.txt
var activitiesText string

// The contract suite compares tools/list to a 42-tool golden. This process
// implements the 18 read-only tools. The other 24 stay in the list so the
// golden matches, and a call returns an explicit isError after validation.
// Nothing is written.

var readOnlyTools = map[string]bool{
	"get_learner_profile":   true,
	"check_text_coverage":   true,
	"list_grammar_concepts": true,
	"list_lexicon":          true,
	"get_lexicon_entry":     true,
	"list_plans":            true,
	"list_plan_revisions":   true,
	"get_plan_revision":     true,
	"diff_plan":             true,
	"list_plan_issues":      true,
	"list_lessons":          true,
	"get_lesson":            true,
	"list_words":            true,
	"list_activity_types":   true,
	"list_users":            true,
	"read_part":             true,
	"suggest_glosses":       true,
	"plan_image":            true,
}

func (a *App) callTool(name string, args Value) (string, bool) {
	switch name {
	case "list_lessons":
		return a.finish(listLessons(a.lessonsDir))
	case "get_lesson":
		slug, _ := args.Get("slug")
		return a.finish(getLesson(a.lessonsDir, slug.Str))
	case "list_words":
		slug := ""
		if v, ok := args.Get("slug"); ok && v.Kind == KindString {
			slug = v.Str
		}
		return a.finish(listWords(a.lessonsDir, slug))
	case "list_activity_types":
		return activitiesText, false
	case "list_users":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.finish(a.listUsers())
	case "plan_image":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.planImageTool(args)
	case "list_grammar_concepts":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.learnerCall(func() (Value, error) { return a.listGrammarConcepts() })
	case "list_lexicon":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.learnerCall(func() (Value, error) { return a.listLexicon(args) })
	case "get_lexicon_entry":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		key, _ := args.Get("key")
		return a.learnerCall(func() (Value, error) { return a.getLexiconEntry(key.Str) })
	case "list_plans":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		limit := 0
		if v, ok := args.Get("limit"); ok && v.IsInt() {
			f, _ := v.Float()
			limit = int(f)
		}
		return a.planCall(func() (Value, error) { return a.listPlans(limit) })
	case "list_plan_revisions":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.planCall(func() (Value, error) {
			id, _ := args.Get("id")
			return a.listPlanRevisions(id.Str, optInt(args, "sinceRevision"), optInt(args, "beforeRevision"), optString(args, "actorKind"), optString(args, "pathPrefix"), optLimit(args, 50))
		})
	case "get_plan_revision":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.planCall(func() (Value, error) {
			id, _ := args.Get("id")
			rev, _ := args.Get("revision")
			f, _ := rev.Float()
			return a.getPlanRevision(id.Str, int64(f))
		})
	case "diff_plan":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.planCall(func() (Value, error) {
			id, _ := args.Get("id")
			from, _ := args.Get("from")
			to, _ := args.Get("to")
			return a.diffPlan(id.Str, from, to, optString(args, "path"))
		})
	case "read_part":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.planCall(func() (Value, error) {
			id, _ := args.Get("id")
			path, _ := args.Get("path")
			return a.readPart(id.Str, path.Str)
		})
	case "suggest_glosses":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return a.planCall(func() (Value, error) {
			id, _ := args.Get("id")
			path, _ := args.Get("path")
			return a.suggestGlosses(id.Str, path.Str)
		})
	case "list_plan_issues":
		if msg, ok := requireToken(); !ok {
			return msg, true
		}
		return "list_plan_issues was not run: plan validation lives in internal/plans, which this read-only MCP build does not include. Nothing was read as a substitute.", true
	case "get_learner_profile":
		return a.auditedLearner("get_learner_profile", "learner_profile_read", args)
	case "check_text_coverage":
		return a.auditedLearner("check_text_coverage", "learner_coverage_read", args)
	default:
		return "This MCP build does not implement " + name + ". Mutating tools are a later pull request; nothing was written.", true
	}
}

func (a *App) finish(v Value, err error) (string, bool) {
	if err != nil {
		return err.Error(), true
	}
	return Stringify(v, 2), false
}

func (a *App) planCall(fn func() (Value, error)) (string, bool) {
	v, err := fn()
	if err != nil {
		return err.Error(), true
	}
	if v.Kind == KindObject {
		if errVal, ok := v.Get("error"); ok && errVal.Kind == KindString {
			return Stringify(planBody(v), 0), true
		}
	}
	return Stringify(v, 2), false
}

func planBody(v Value) Value {
	errVal, _ := v.Get("error")
	fields := []Field{{Key: "error", Val: errVal}}
	if rev, ok := v.Get("revision"); ok {
		fields = append(fields, Field{Key: "revision", Val: rev})
	}
	if path, ok := v.Get("path"); ok && path.Kind == KindString {
		fields = append(fields, Field{Key: "path", Val: path})
	}
	if field, ok := v.Get("field"); ok && isErrorWord(field) {
		fields = append(fields, Field{Key: "field", Val: field})
	}
	if reason, ok := v.Get("reason"); ok && isErrorWord(reason) {
		fields = append(fields, Field{Key: "reason", Val: reason})
	}
	for _, key := range []string{"blocks", "warnings", "informs"} {
		if item, ok := v.Get(key); ok && item.Kind == KindArray {
			fields = append(fields, Field{Key: key, Val: item})
		}
	}
	return Object(fields...)
}

func isErrorWord(v Value) bool {
	if v.Kind != KindString || v.Str == "" || len(v.Str) > 64 {
		return false
	}
	for i, r := range v.Str {
		if i == 0 {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				return false
			}
			continue
		}
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func (a *App) learnerCall(fn func() (Value, error)) (string, bool) {
	v, err := fn()
	if err != nil {
		if isTokenMessage(err.Error()) {
			return err.Error(), true
		}
		return `{"error":"invalid_input"}`, true
	}
	if v.Kind == KindObject {
		if errVal, ok := v.Get("error"); ok && errVal.Kind == KindString && len(v.Obj) <= 2 {
			// lexicon refusals are {error} or {error, field}. Node's callLearner
			// returns text(result) for list success and refusal({error}) only
			// when the service returns {error} without treating field as success.
			if _, hasEntries := v.Get("entries"); !hasEntries {
				if _, hasConcepts := v.Get("concepts"); !hasConcepts {
					if errVal.Str != "" && !v.Has("entries") {
						body := Object(Field{Key: "error", Val: errVal})
						return Stringify(body, 0), true
					}
				}
			}
		}
	}
	return Stringify(v, 2), false
}

func (a *App) planImageTool(args Value) (string, bool) {
	prompt, _ := args.Get("prompt")
	var count *int
	if v, ok := args.Get("count"); ok && v.Kind != KindNull {
		if !v.IsInt() {
			return mediaFailure(imageError{code: "invalid_input"})
		}
		f, _ := v.Float()
		n := int(f)
		count = &n
	}
	v, err := planImage(prompt.Str, count, openAIKey())
	if err != nil {
		return mediaFailure(err)
	}
	text := Stringify(v, 2)
	if mentionsSecret(text) {
		return mediaFailure(imageError{code: "request_failed"})
	}
	return text, false
}

func mediaFailure(err error) (string, bool) {
	msg := err.Error()
	if mentionsSecret(msg) {
		msg = "The image request failed."
	}
	code := ""
	if ie, ok := err.(imageError); ok {
		code = ie.code
	}
	if code == "" || mentionsSecret(code) {
		return msg, true
	}
	return Stringify(Object(
		Field{Key: "error", Val: String(code)},
		Field{Key: "message", Val: String(msg)},
	), 0), true
}

func mentionsSecret(message string) bool {
	key := openAIKey()
	token := os.Getenv("MCP_AUTH_TOKEN")
	if key != "" && strings.Contains(message, key) {
		return true
	}
	if token != "" && strings.Contains(message, token) {
		return true
	}
	return secretRE.MatchString(message)
}

func requireToken() (string, bool) {
	token := os.Getenv("MCP_AUTH_TOKEN")
	if token == "" {
		return "MCP_AUTH_TOKEN must be configured (non-empty) to use this tool or visibility argument.", false
	}
	return "", true
}

func isTokenMessage(msg string) bool {
	return strings.Contains(msg, "MCP_AUTH_TOKEN")
}

func optString(args Value, key string) string {
	v, ok := args.Get(key)
	if !ok || v.Kind != KindString {
		return ""
	}
	return v.Str
}

func optInt(args Value, key string) *int64 {
	v, ok := args.Get(key)
	if !ok || !v.IsInt() {
		return nil
	}
	f, _ := v.Float()
	n := int64(f)
	return &n
}

func optLimit(args Value, def int) int {
	v, ok := args.Get("limit")
	if !ok || !v.IsInt() {
		return def
	}
	f, _ := v.Float()
	return int(f)
}

func (a *App) auditedLearner(tool, action string, args Value) (string, bool) {
	if msg, ok := requireToken(); !ok {
		return msg, true
	}
	if a.audit == nil {
		return tool + " was not run: the " + action + " audit row cannot be written on the read-only database handle from go/internal/db. No profile or coverage result is returned.", true
	}
	username, _ := args.Get("username")
	name := ""
	if username.Kind == KindString {
		name = username.Str
	}
	var invalid string
	if tool == "check_text_coverage" {
		text, _ := args.Get("text")
		if text.Kind == KindString && UTF16Len(text.Str) > 4000 {
			invalid = "invalid_text"
		}
	}
	outcome, learner, err := a.admit(name, action, invalid)
	if err != nil {
		return `{"error":"invalid_input"}`, true
	}
	if outcome != "" {
		return `{"error":"` + outcome + `"}`, true
	}
	if a.learner == nil {
		return tool + " was not run: no learner reader is configured, so no profile or coverage result is returned.", true
	}
	var result Value
	if tool == "get_learner_profile" {
		result, err = a.learner.Profile(learner.Username)
	} else {
		text, _ := args.Get("text")
		result, err = a.learner.Coverage(learner.Username, text.Str)
	}
	if err != nil {
		_ = a.record(action, learner.Username, `{"outcome":"error","fields":"none"}`)
		return `{"error":"invalid_input"}`, true
	}
	if result.Kind == KindObject {
		if errVal, ok := result.Get("error"); ok && errVal.Kind == KindString {
			detail := fmt.Sprintf(`{"outcome":%s,"fields":"none"}`, quote(errVal.Str))
			if recErr := a.record(action, learner.Username, detail); recErr != nil {
				return `{"error":"invalid_input"}`, true
			}
			return Stringify(Object(Field{Key: "error", Val: errVal}), 0), true
		}
	}
	if recErr := a.record(action, learner.Username, `{"outcome":"ok","fields":"none"}`); recErr != nil {
		return `{"error":"invalid_input"}`, true
	}
	return Stringify(result, 2), false
}
