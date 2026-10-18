package mcp

import (
	"strconv"
	"strings"
)

// schema is one node of a Zod-generated JSON Schema. Validation reproduces
// zod 4's accept/reject decisions and the English issue text the Node SDK
// puts in "Input validation error".
type schema struct {
	typ      string
	props    []namedSchema
	required map[string]bool
	minLen   *int
	maxLen   *int
	min      *float64
	max      *float64
	enum     []string
	hasConst bool
	constNum float64
	anyOf    []schema
	items    *schema
	minItems *int
	maxItems *int
}

type namedSchema struct {
	name string
	sch  schema
}

type issue struct {
	path    string
	message string
	abort   bool
}

func parseSchema(v Value) schema {
	var s schema
	if t, ok := v.Get("type"); ok && t.Kind == KindString {
		s.typ = t.Str
	}
	if props, ok := v.Get("properties"); ok && props.Kind == KindObject {
		for _, f := range props.Obj {
			s.props = append(s.props, namedSchema{name: f.Key, sch: parseSchema(f.Val)})
		}
	}
	if req, ok := v.Get("required"); ok && req.Kind == KindArray {
		s.required = map[string]bool{}
		for _, item := range req.Arr {
			if item.Kind == KindString {
				s.required[item.Str] = true
			}
		}
	}
	if n, ok := intField(v, "minLength"); ok {
		s.minLen = &n
	}
	if n, ok := intField(v, "maxLength"); ok {
		s.maxLen = &n
	}
	if n, ok := floatField(v, "minimum"); ok {
		s.min = &n
	}
	if n, ok := floatField(v, "maximum"); ok {
		s.max = &n
	}
	if en, ok := v.Get("enum"); ok && en.Kind == KindArray {
		for _, item := range en.Arr {
			if item.Kind == KindString {
				s.enum = append(s.enum, item.Str)
			}
		}
	}
	if c, ok := v.Get("const"); ok && c.Kind == KindNumber {
		if f, ok := c.Float(); ok {
			s.hasConst = true
			s.constNum = f
		}
	}
	if alts, ok := v.Get("anyOf"); ok && alts.Kind == KindArray {
		for _, item := range alts.Arr {
			s.anyOf = append(s.anyOf, parseSchema(item))
		}
	}
	if items, ok := v.Get("items"); ok && items.Kind == KindObject {
		inner := parseSchema(items)
		s.items = &inner
	}
	if n, ok := intField(v, "minItems"); ok {
		s.minItems = &n
	}
	if n, ok := intField(v, "maxItems"); ok {
		s.maxItems = &n
	}
	return s
}

func intField(v Value, key string) (int, bool) {
	f, ok := v.Get(key)
	if !ok || f.Kind != KindNumber {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(f.Num))
	if err != nil {
		fl, ok := f.Float()
		if !ok {
			return 0, false
		}
		return int(fl), true
	}
	return n, true
}

func floatField(v Value, key string) (float64, bool) {
	f, ok := v.Get(key)
	if !ok {
		return 0, false
	}
	return f.Float()
}

// validateArguments checks a tool's input object. Unknown keys are ignored,
// matching zod's default strip.
func validateArguments(s schema, args Value) []issue {
	if args.Kind != KindObject {
		return []issue{{message: "Invalid input: expected object, received " + args.TypeName(), abort: true}}
	}
	return validateSchema(s, args, "")
}

func validateSchema(s schema, v Value, path string) []issue {
	if len(s.anyOf) > 0 && s.typ == "" && len(s.props) == 0 {
		return validateUnion(s.anyOf, v, path)
	}
	var issues []issue
	if s.typ != "" {
		if iss, ok := checkType(s, v, path); !ok {
			return iss
		}
	}
	switch s.typ {
	case "string":
		issues = append(issues, checkString(s, v, path)...)
	case "integer", "number":
		issues = append(issues, checkNumber(s, v, path)...)
	case "object":
		issues = append(issues, checkObject(s, v, path)...)
	case "array":
		issues = append(issues, checkArray(s, v, path)...)
	}
	if len(s.enum) > 0 {
		if iss := checkEnum(s, v, path); iss != nil {
			issues = append(issues, iss...)
		}
	}
	if s.hasConst {
		f, _ := v.Float()
		if f != s.constNum {
			issues = append(issues, issue{
				path:    path,
				message: "Invalid input: expected " + trimFloat(s.constNum),
				abort:   true,
			})
		}
	}
	if len(s.anyOf) > 0 {
		issues = append(issues, validateUnion(s.anyOf, v, path)...)
	}
	return issues
}

func checkType(s schema, v Value, path string) ([]issue, bool) {
	expected := s.typ
	switch s.typ {
	case "integer":
		expected = "int"
		if v.Kind == KindNumber && v.IsInt() {
			return nil, true
		}
		if v.Kind == KindNumber {
			return []issue{{path: path, message: "Invalid input: expected int, received number", abort: true}}, false
		}
	case "number":
		if v.Kind == KindNumber {
			return nil, true
		}
	case "string":
		if v.Kind == KindString {
			return nil, true
		}
	case "boolean":
		if v.Kind == KindBool {
			return nil, true
		}
	case "object":
		if v.Kind == KindObject {
			return nil, true
		}
	case "array":
		if v.Kind == KindArray {
			return nil, true
		}
	default:
		return nil, true
	}
	return []issue{{path: path, message: "Invalid input: expected " + expected + ", received " + v.TypeName(), abort: true}}, false
}

func checkString(s schema, v Value, path string) []issue {
	n := UTF16Len(v.Str)
	var out []issue
	if s.minLen != nil && n < *s.minLen {
		out = append(out, issue{
			path:    path,
			message: "Too small: expected string to have >=" + strconv.Itoa(*s.minLen) + " characters",
		})
	}
	if s.maxLen != nil && n > *s.maxLen {
		out = append(out, issue{
			path:    path,
			message: "Too big: expected string to have <=" + strconv.Itoa(*s.maxLen) + " characters",
		})
	}
	return out
}

func checkNumber(s schema, v Value, path string) []issue {
	f, ok := v.Float()
	if !ok {
		return []issue{{path: path, message: "Invalid input: expected number, received " + v.TypeName(), abort: true}}
	}
	var out []issue
	if s.min != nil && f < *s.min {
		out = append(out, issue{
			path:    path,
			message: "Too small: expected number to be >=" + trimFloat(*s.min),
		})
	}
	if s.max != nil && f > *s.max {
		out = append(out, issue{
			path:    path,
			message: "Too big: expected number to be <=" + trimFloat(*s.max),
		})
	}
	return out
}

func checkObject(s schema, v Value, path string) []issue {
	var out []issue
	for _, prop := range s.props {
		child, ok := v.Get(prop.name)
		childPath := joinPath(path, prop.name)
		if !ok {
			if s.required[prop.name] {
				// An absent field is JavaScript undefined, not JSON null.
				out = append(out, validateSchema(prop.sch, Value{Kind: KindUndefined}, childPath)...)
			}
			continue
		}
		out = append(out, validateSchema(prop.sch, child, childPath)...)
	}
	return out
}

func checkArray(s schema, v Value, path string) []issue {
	var out []issue
	n := len(v.Arr)
	if s.minItems != nil && n < *s.minItems {
		out = append(out, issue{
			path:    path,
			message: "Too small: expected array to have >=" + strconv.Itoa(*s.minItems) + " items",
		})
	}
	if s.maxItems != nil && n > *s.maxItems {
		out = append(out, issue{
			path:    path,
			message: "Too big: expected array to have <=" + strconv.Itoa(*s.maxItems) + " items",
		})
	}
	if s.items != nil {
		for i, item := range v.Arr {
			out = append(out, validateSchema(*s.items, item, joinIndex(path, i))...)
		}
	}
	return out
}

func checkEnum(s schema, v Value, path string) []issue {
	if v.Kind != KindString {
		return []issue{{path: path, message: "Invalid input: expected string, received " + v.TypeName(), abort: true}}
	}
	for _, item := range s.enum {
		if v.Str == item {
			return nil
		}
	}
	if len(s.enum) == 1 {
		return []issue{{path: path, message: "Invalid input: expected " + quote(s.enum[0]), abort: true}}
	}
	parts := make([]string, len(s.enum))
	for i, item := range s.enum {
		parts[i] = quote(item)
	}
	return []issue{{
		path:    path,
		message: "Invalid option: expected one of " + strings.Join(parts, "|"),
		abort:   true,
	}}
}

// validateUnion applies zod's union rule: a branch that fails with an
// aborting issue (invalid_type, invalid_value) is discarded. When exactly
// one branch failed without aborting, that branch's issues are the result.
// Otherwise the union reports "Invalid input".
func validateUnion(alts []schema, v Value, path string) []issue {
	var soft [][]issue
	for _, alt := range alts {
		iss := validateSchema(alt, v, path)
		if len(iss) == 0 {
			return nil
		}
		aborted := false
		for _, item := range iss {
			if item.abort {
				aborted = true
				break
			}
		}
		if !aborted {
			soft = append(soft, iss)
		}
	}
	if len(soft) == 1 {
		return soft[0]
	}
	return []issue{{path: path, message: "Invalid input", abort: true}}
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func joinIndex(parent string, i int) string {
	return parent + "[" + strconv.Itoa(i) + "]"
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func formatIssues(tool string, issues []issue) string {
	parts := make([]string, len(issues))
	for i, item := range issues {
		if item.path == "" {
			parts[i] = item.message
			continue
		}
		parts[i] = item.message + " at " + item.path
	}
	return "MCP error -32602: Input validation error: Invalid arguments for tool " + tool + ": " + strings.Join(parts, "\n")
}
