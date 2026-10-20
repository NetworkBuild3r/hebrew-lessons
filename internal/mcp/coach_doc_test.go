package mcp

// INIT-015/SPEC-023 ac-3: docs/ops/weekly-coach.md is the definition of the weekly coach
// automation. These tests read that page and do not trust it:
//
//   - every tool and resource it names is in the real registry, read from a running handler;
//   - the roles it states for each tool match the server's own role check, and a call by a
//     learner session to each tool marked no is refused;
//   - it never tells the operator to use MCP_AUTH_TOKEN as the automation's credential;
//   - the limits it quotes are the server's constants;
//   - the prompt follows the steps and the section of the real hebrew://authoring text.
//
// coachDocProblems is the whole check, a function of the page text. TestCoachDocChecksCanFail
// feeds it broken copies of the page, so a check that stops checking is caught here.

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const coachDocPath = "../../docs/ops/weekly-coach.md"

// coachDocNotTools are snake_case words in backticks that the page uses and that are not tools.
// They are names of activity labels in a brief, quoted to say they are not unit types.
var coachDocNotTools = []string{"read_pick", "type_it"}

var (
	coachBackticked = regexp.MustCompile("`([^`\n]+)`")
	coachSnake      = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)+$`)
	coachPlainSnake = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b`)
	coachResource   = regexp.MustCompile(`hebrew://[a-z][a-z/]*`)
	coachNegation   = regexp.MustCompile(`(?i)\b(never|not|no)\b|n't`)
	coachSecret     = regexp.MustCompile(`(?i)(bearer\s+[A-Za-z0-9._~+/=-]{16,}|sk-[A-Za-z0-9]{16,}|-----BEGIN|api[_-]?key\s*[:=]\s*\S{8,})`)
)

func coachLF(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// coachSection is the text under a "## heading" up to the next "## ", or "" when it is missing.
func coachSection(doc, heading string) string {
	lines := strings.Split(coachLF(doc), "\n")
	var out []string
	in := false
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			if in {
				break
			}
			in = strings.TrimPrefix(l, "## ") == heading
			continue
		}
		if in {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// coachPrompt is the body of the fenced block that opens with "```text prompt".
func coachPrompt(doc string) string {
	lines := strings.Split(coachLF(doc), "\n")
	var out []string
	in := false
	for _, l := range lines {
		switch {
		case !in && strings.HasPrefix(l, "```text prompt"):
			in = true
		case in && strings.HasPrefix(l, "```"):
			return strings.Join(out, "\n")
		case in:
			out = append(out, l)
		}
	}
	return ""
}

// coachFenced is the text inside every fenced block.
func coachFenced(doc string) string {
	var out []string
	in := false
	for _, l := range strings.Split(coachLF(doc), "\n") {
		if strings.HasPrefix(l, "```") {
			in = !in
			continue
		}
		if in {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

type coachRow struct {
	name, learner, admin string
}

// coachRoleRows reads the rows of the tools table: the backticked name in the second column
// and the first word of the learner and admin columns.
func coachRoleRows(doc string) []coachRow {
	var rows []coachRow
	for _, l := range strings.Split(coachSection(doc, "Tools this automation uses"), "\n") {
		if !strings.HasPrefix(l, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(l, "| "), "|")
		if len(cells) != 4 {
			continue
		}
		m := coachBackticked.FindStringSubmatch(cells[1])
		if m == nil {
			continue
		}
		first := func(s string) string {
			f := strings.Fields(s)
			if len(f) == 0 {
				return ""
			}
			return f[0]
		}
		rows = append(rows, coachRow{name: m[1], learner: first(cells[2]), admin: first(cells[3])})
	}
	return rows
}

type coachFacts struct {
	tools, resources []string
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// coachDocProblems is every way the page is wrong, or none.
func coachDocProblems(doc string, f coachFacts) []string {
	var bad []string
	add := func(format string, args ...any) { bad = append(bad, fmt.Sprintf(format, args...)) }

	// Names. Every backticked snake_case word is a tool, every hebrew:// address is a resource.
	seen := map[string]bool{}
	for _, m := range coachBackticked.FindAllStringSubmatch(doc, -1) {
		w := m[1]
		if coachSnake.MatchString(w) && !slices.Contains(coachDocNotTools, w) {
			seen[w] = true
			if !slices.Contains(f.tools, w) {
				add("the page names a tool that does not exist: %s", w)
			}
		}
	}
	for _, r := range coachResource.FindAllString(doc, -1) {
		if !slices.Contains(f.resources, r) {
			add("the page names a resource that does not exist: %s", r)
		}
	}
	for _, must := range []string{"get_progress", "get_lesson_brief", "add_coach_note", "put_unit", "generate_audio", "validate_unit"} {
		if !seen[must] {
			add("the page does not name %s", must)
		}
	}
	if !strings.Contains(doc, "hebrew://authoring") {
		add("the page does not name hebrew://authoring")
	}

	// The prompt names the three steps in order.
	prompt := coachPrompt(doc)
	if prompt == "" {
		add("there is no prompt block")
	} else {
		at := func(s string) int { return strings.Index(prompt, s) }
		if a, b, c := at("get_progress"), at("hebrew://authoring"), at("add_coach_note"); a < 0 || b < 0 || c < 0 || !(a < b && b < c) {
			add("the prompt does not name get_progress, then hebrew://authoring, then add_coach_note, in that order")
		}
		for _, want := range []string{"<LEARNER_ID>", "get_lesson_brief", "put_unit", "are not unit types", "Reading a brief", "7 days"} {
			if !strings.Contains(prompt, want) {
				add("the prompt does not say %q", want)
			}
		}
		// The prompt is plain text in a code block, so its names are not in backticks. Every
		// snake_case word in it is a tool, and each must exist.
		for _, w := range coachPlainSnake.FindAllString(prompt, -1) {
			if !slices.Contains(f.tools, w) {
				add("the prompt names a tool that does not exist: %s", w)
			}
		}
		for _, banned := range []string{"MCP_AUTH_TOKEN", "Bearer"} {
			if strings.Contains(prompt, banned) {
				add("the prompt holds %q", banned)
			}
		}
	}

	// Roles.
	rows := coachRoleRows(doc)
	if len(rows) < 8 {
		add("the tools table has %d rows, want at least 8", len(rows))
	}
	inTable := map[string]bool{}
	for _, r := range rows {
		inTable[r.name] = true
		if strings.HasPrefix(r.name, "hebrew://") {
			if r.learner != "yes" || r.admin != "yes" {
				add("%s: a resource is not role-gated, the table says learner %q admin %q", r.name, r.learner, r.admin)
			}
			continue
		}
		learner := Principal{Role: RoleLearner}.allowed(r.name)
		admin := Principal{Role: RoleAdmin}.allowed(r.name)
		if r.learner != yesNo(learner) {
			add("%s: the table says a learner session is %q, the server says %q", r.name, r.learner, yesNo(learner))
		}
		if r.admin != yesNo(admin) {
			add("%s: the table says an admin session is %q, the server says %q", r.name, r.admin, yesNo(admin))
		}
	}
	for _, must := range []string{"get_progress", "get_lesson_brief", "add_coach_note", "put_unit", "hebrew://authoring"} {
		if !inTable[must] {
			add("the tools table has no row for %s", must)
		}
	}

	// The static token is never the credential.
	fenced := coachFenced(doc)
	if strings.Contains(fenced, "MCP_AUTH_TOKEN") {
		add("MCP_AUTH_TOKEN is inside a code block")
	}
	rule := false
	for _, l := range strings.Split(coachLF(doc), "\n") {
		if !strings.Contains(l, "MCP_AUTH_TOKEN") && !strings.Contains(strings.ToLower(l), "static token") {
			continue
		}
		if !coachNegation.MatchString(l) {
			add("a line names the static token without refusing it: %q", strings.TrimSpace(l))
		}
		if strings.Contains(l, "MCP_AUTH_TOKEN") && regexp.MustCompile(`(?i)\bnever\b`).MatchString(l) {
			rule = true
		}
	}
	if !rule {
		add("the page never says, in one line, that the automation never uses MCP_AUTH_TOKEN")
	}
	if !strings.Contains(coachSection(doc, "Auth"), "OAuth session") {
		add("the Auth section does not say the automation uses an OAuth session")
	}
	if coachSecret.MatchString(doc) {
		add("the page holds something that looks like a secret")
	}

	// Sections.
	for _, h := range []string{"The definition", "Tools this automation uses", "Auth", "Open decision", "Turn it on (operator, SPEC-025 step 6)", "Turn it off", "Limits", "If it fails"} {
		if strings.TrimSpace(coachSection(doc, h)) == "" {
			add("the page has no section %q", h)
		}
	}
	od := coachSection(doc, "Open decision")
	for _, want := range []string{"forbidden: admin only", "get_lesson_brief", "add_coach_note", "admin-role"} {
		if !strings.Contains(od, want) {
			add("the Open decision section does not say %q", want)
		}
	}

	// Limits and the definition fields, from the server's constants.
	limits := coachSection(doc, "Limits")
	for _, want := range []string{
		fmt.Sprintf("%d calls a minute", learnerToolsPerMinute),
		fmt.Sprintf("%d coach notes a day", maxCoachNotesPerDay),
	} {
		if !strings.Contains(limits, want) {
			add("the Limits section does not say %q", want)
		}
	}
	for _, want := range []string{"`0 7 * * 1`", "`" + serverName + "`", "Monday 07:00", "Memory | Off"} {
		if !strings.Contains(doc, want) {
			add("the definition does not hold %q", want)
		}
	}
	return bad
}

func coachDoc(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(coachDocPath)
	if err != nil {
		t.Fatalf("the coach automation page is missing: %v", err)
	}
	return coachLF(string(b))
}

// coachRegistry reads the tool and resource names from a running handler, as a client does.
func coachRegistry(t *testing.T, e *ltEnv) coachFacts {
	t.Helper()
	s := e.session("admin")
	tools, err := s.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var f coachFacts
	for _, tool := range tools.Tools {
		f.tools = append(f.tools, tool.Name)
	}
	for _, r := range res.Resources {
		f.resources = append(f.resources, r.URI)
	}
	if len(f.tools) < 14 || len(f.resources) < 4 {
		t.Fatalf("the registry has %d tools and %d resources, want at least 14 and 4", len(f.tools), len(f.resources))
	}
	return f
}

// ac-3: every tool and resource the page names exists, and the rest of the page agrees with the server.
func TestCoachDocNamesAndClaimsMatchTheServer(t *testing.T) {
	e := newLT(t)
	doc := coachDoc(t)
	if bad := coachDocProblems(doc, coachRegistry(t, e)); len(bad) > 0 {
		t.Fatalf("docs/ops/weekly-coach.md has %d problems:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
}

// The roles the page states are what a learner session meets: a call to a tool marked no is
// refused with the admin-only text, and the tools marked yes answer. A resource reads for a learner.
func TestCoachDocRolesMatchTheServer(t *testing.T) {
	e := newLT(t)
	rows := coachRoleRows(coachDoc(t))
	if len(rows) < 8 {
		t.Fatalf("%d rows", len(rows))
	}
	noRows, yesRows := 0, 0
	for _, r := range rows {
		switch {
		case strings.HasPrefix(r.name, "hebrew://"):
			if text := readResource(t, e.session("learner-a"), r.name); len(text) < 100 {
				t.Errorf("%s: a learner read %d bytes", r.name, len(text))
			}
		case r.learner == "no":
			noRows++
			if msg := e.fail("learner-a", r.name, map[string]any{}); msg != msgAdminOnly {
				t.Errorf("%s: a learner got %q, want %q", r.name, msg, msgAdminOnly)
			}
		case r.name == "get_progress" || r.name == "get_learner_model":
			yesRows++
			e.ok("learner-a", r.name, map[string]any{})
			e.ok("admin", r.name, map[string]any{"learnerId": 7})
		}
	}
	if noRows < 5 || yesRows != 2 {
		t.Fatalf("checked %d no rows and %d yes rows, want at least 5 and 2", noRows, yesRows)
	}
}

// The prompt points at things in the real guide: step 7 is the publish, step 8 is the note,
// and the section it sends the model to for the activities names exists and says what the
// prompt says about it.
func TestCoachDocPromptFollowsTheRealGuide(t *testing.T) {
	e := newLT(t)
	guide := coachLF(readResource(t, e.session("admin"), resourceAuthoring))
	steps := coachSection(guide, "Steps")
	var seven, eight string
	for _, l := range strings.Split(steps, "\n") {
		switch {
		case strings.HasPrefix(l, "7. "):
			seven = l
		case strings.HasPrefix(l, "8. "):
			eight = l
		}
	}
	if !strings.Contains(seven, "`put_unit`") || !strings.Contains(seven, "publish") {
		t.Errorf("step 7 of the guide is no longer the publish: %q", seven)
	}
	if !strings.Contains(eight, "`add_coach_note`") {
		t.Errorf("step 8 of the guide is no longer the coach note: %q", eight)
	}
	if !strings.Contains(guide, "### Reading a brief") && !strings.Contains(guide, "## Reading a brief") {
		t.Error("the guide has no section named Reading a brief")
	}
	if !strings.Contains(guide, "are not unit types") {
		t.Error("the guide no longer says that some activities names are not unit types")
	}
	prompt := coachPrompt(coachDoc(t))
	for _, want := range []string{"step 7", "Step 8 of hebrew://authoring is add_coach_note", "are not unit types", "section Reading a brief"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not say %q", want)
		}
	}
	// The prompt never tells the model to use the brief's activities names as they are.
	for _, banned := range []string{"copy the activities", "use the activities as", "use the brief's activities"} {
		if strings.Contains(strings.ToLower(prompt), banned) {
			t.Errorf("the prompt says %q", banned)
		}
	}
}

// Each check can fail. Every case breaks one thing in a copy of the real page, and the checker
// must say so. A check that never fires is not a check.
func TestCoachDocChecksCanFail(t *testing.T) {
	e := newLT(t)
	facts := coachRegistry(t, e)
	doc := coachDoc(t)
	if bad := coachDocProblems(doc, facts); len(bad) > 0 {
		t.Fatalf("the real page already has problems: %v", bad)
	}
	cases := []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"a tool name is renamed in the prompt", func(s string) string {
			return strings.Replace(s, "call get_learner_model with", "call get_learners_model with", 1)
		}, "get_learners_model"},
		{"a tool name is renamed in a table row", func(s string) string {
			return strings.Replace(s, "| `get_lesson_brief` |", "| `get_lesson_briefs` |", 1)
		}, "get_lesson_briefs"},
		{"a resource is renamed", func(s string) string {
			return strings.Replace(s, "read the resource hebrew://authoring and", "read the resource hebrew://authoringx and", 1)
		}, "hebrew://authoringx"},
		{"the static token becomes the credential", func(s string) string {
			return strings.Replace(s, "never with `MCP_AUTH_TOKEN`.", "with `MCP_AUTH_TOKEN`.", 1)
		}, "MCP_AUTH_TOKEN"},
		{"an instruction to use the static token is added", func(s string) string {
			return s + "\nSet the credential of the automation to the value of MCP_AUTH_TOKEN.\n"
		}, "without refusing"},
		{"the static token goes in the prompt", func(s string) string {
			return strings.Replace(s, "Use only the hebrew-lessons MCP server.", "Use only the hebrew-lessons MCP server, with MCP_AUTH_TOKEN.", 1)
		}, "MCP_AUTH_TOKEN"},
		{"a learner is said to be able to fetch the brief", func(s string) string {
			return strings.Replace(s, "| `get_lesson_brief` | no |", "| `get_lesson_brief` | yes |", 1)
		}, "get_lesson_brief"},
		{"an admin-only tool is said to be open to a learner", func(s string) string {
			return strings.Replace(s, "| `add_coach_note` | no |", "| `add_coach_note` | yes |", 1)
		}, "add_coach_note"},
		{"a learner tool is said to be closed", func(s string) string {
			return strings.Replace(s, "| `get_progress` | yes (own data only) |", "| `get_progress` | no |", 1)
		}, "get_progress"},
		{"the rate limit is misquoted", func(s string) string {
			return strings.Replace(s, "60 calls a minute", "600 calls a minute", 1)
		}, "calls a minute"},
		{"the note limit is misquoted", func(s string) string {
			return strings.Replace(s, "100 coach notes a day", "10 coach notes a day", 1)
		}, "coach notes a day"},
		{"the open decision is dropped", func(s string) string {
			return strings.Replace(s, "## Open decision", "## Something else", 1)
		}, "Open decision"},
		{"the turn-off section is dropped", func(s string) string {
			return strings.Replace(s, "## Turn it off", "## Stop", 1)
		}, "Turn it off"},
		{"the prompt loses the activities warning", func(s string) string {
			return strings.Replace(s, "The activities names in the brief are not unit types, so do not copy them into the unit.", "", 1)
		}, "unit types"},
		{"the prompt loses the order", func(s string) string {
			return strings.Replace(s, "3. Leave one coach note. Step 8 of hebrew://authoring is add_coach_note.", "3. Leave one coach note.", 1)
		}, "in that order"},
		{"the schedule changes", func(s string) string { return strings.Replace(s, "`0 7 * * 1`", "`0 4 * * 1`", 1) }, "0 7 * * 1"},
		{"a key is pasted", func(s string) string {
			return s + "\nAuthorization: Bearer abcdefghijklmnopqrstuvwxyz0123456789\n"
		}, "secret"},
	}
	for _, c := range cases {
		mutated := c.mutate(doc)
		if mutated == doc {
			t.Errorf("%s: the mutation changed nothing, so it proves nothing", c.name)
			continue
		}
		bad := coachDocProblems(mutated, facts)
		if len(bad) == 0 {
			t.Errorf("%s: the checker found no problem", c.name)
			continue
		}
		if !strings.Contains(strings.Join(bad, "\n"), c.want) {
			t.Errorf("%s: the problems %v do not mention %q", c.name, bad, c.want)
		}
	}
}
