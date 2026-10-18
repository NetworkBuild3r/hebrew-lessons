// Package normalize freezes MCP JSON into a form two servers can share.
// Schema rules drop SDK and zod artifacts. Masking hides volatile values.
// Error records are the shape later cases will compare; PR1 uses them for
// the validation and transport failures it actually calls.
package normalize

import "sort"

// jsMaxSafe is Number.MAX_SAFE_INTEGER. The MCP SDK stamps it onto unbounded
// z.number().int() fields. It is not a domain constraint.
const (
	jsMaxSafe int64 = 9007199254740991
	jsMinSafe int64 = -9007199254740991
)

// Canonical rewrites a JSON Schema so equivalent SDK shapes compare equal.
// Object key order is not significant; callers compare the marshaled form.
//
// Enum value order is kept. A reorder of ["working","published"] stays
// visible. Type-name unions are sorted, because that order carries no meaning.
// required is sorted. An empty properties object and a missing properties
// key become the same schema. An empty required array is dropped, matching
// Node, which omits it.
func Canonical(schema any) any {
	switch n := schema.(type) {
	case map[string]any:
		return canonSchema(n)
	default:
		return schema
	}
}

func canonSchema(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, child := range in {
		out[k] = canonChild(k, child)
	}
	delete(out, "$schema")
	if ap, ok := out["additionalProperties"].(bool); ok && !ap {
		delete(out, "additionalProperties")
	}
	for i := 0; i < 8; i++ {
		if !collapseSingle(out) && !collapseNullUnion(out) && !collapseConstUnion(out) && !collapseTypeUnion(out) {
			break
		}
	}
	foldNullable(out)
	foldInteger(out)
	dropSafeBounds(out)
	dropEmpty(out)
	sortRequired(out)
	sortTypeNames(out)
	markAny(out)
	return out
}

func canonChild(key string, child any) any {
	switch key {
	case "properties":
		props, ok := child.(map[string]any)
		if !ok {
			return child
		}
		next := make(map[string]any, len(props))
		for name, schema := range props {
			next[name] = Canonical(schema)
		}
		return next
	case "items", "additionalProperties", "additionalItems", "contains", "not", "if", "then", "else", "propertyNames", "unevaluatedItems", "unevaluatedProperties":
		switch n := child.(type) {
		case map[string]any:
			return canonSchema(n)
		case []any:
			next := make([]any, len(n))
			for i, el := range n {
				next[i] = Canonical(el)
			}
			return next
		default:
			return child
		}
	case "anyOf", "oneOf", "allOf", "prefixItems":
		arr, ok := child.([]any)
		if !ok {
			return child
		}
		next := make([]any, len(arr))
		for i, el := range arr {
			next[i] = Canonical(el)
		}
		return next
	default:
		return child
	}
}

func collapseSingle(m map[string]any) bool {
	changed := false
	for _, key := range []string{"anyOf", "oneOf"} {
		arr, ok := m[key].([]any)
		if !ok || len(arr) != 1 {
			continue
		}
		member, ok := arr[0].(map[string]any)
		if !ok {
			continue
		}
		delete(m, key)
		mergeSchema(m, member)
		changed = true
	}
	return changed
}

func collapseNullUnion(m map[string]any) bool {
	for _, key := range []string{"anyOf", "oneOf"} {
		arr, ok := m[key].([]any)
		if !ok || len(arr) != 2 {
			continue
		}
		a, aok := arr[0].(map[string]any)
		b, bok := arr[1].(map[string]any)
		if !aok || !bok {
			continue
		}
		var non map[string]any
		switch {
		case isNullSchema(a) && !isNullSchema(b):
			non = b
		case isNullSchema(b) && !isNullSchema(a):
			non = a
		default:
			continue
		}
		typ := non["type"]
		delete(m, key)
		mergeSchema(m, non)
		m["type"] = addNull(typ)
		return true
	}
	return false
}

func collapseConstUnion(m map[string]any) bool {
	for _, key := range []string{"anyOf", "oneOf"} {
		arr, ok := m[key].([]any)
		if !ok || len(arr) < 1 {
			continue
		}
		members := make([]map[string]any, 0, len(arr))
		for _, el := range arr {
			mem, ok := el.(map[string]any)
			if !ok || !isConstMember(mem) {
				members = nil
				break
			}
			members = append(members, mem)
		}
		if len(members) != len(arr) {
			continue
		}
		enums := make([]any, 0, len(members))
		var types []string
		seen := map[string]bool{}
		for _, mem := range members {
			enums = append(enums, mem["const"])
			if t, ok := mem["type"].(string); ok && !seen[t] {
				seen[t] = true
				types = append(types, t)
			}
		}
		delete(m, key)
		m["enum"] = enums
		switch len(types) {
		case 0:
		case 1:
			m["type"] = types[0]
		default:
			sort.Strings(types)
			m["type"] = stringsToAny(types)
		}
		return true
	}
	return false
}

func collapseTypeUnion(m map[string]any) bool {
	for _, key := range []string{"anyOf", "oneOf"} {
		arr, ok := m[key].([]any)
		if !ok || len(arr) < 2 {
			continue
		}
		var types []string
		for _, el := range arr {
			mem, ok := el.(map[string]any)
			if !ok || !isPureType(mem) {
				types = nil
				break
			}
			types = append(types, mem["type"].(string))
		}
		if len(types) != len(arr) {
			continue
		}
		delete(m, key)
		if len(types) == 1 {
			m["type"] = types[0]
		} else {
			m["type"] = sortedTypeAny(types)
		}
		return true
	}
	return false
}

func isNullSchema(m map[string]any) bool {
	t, _ := m["type"].(string)
	if t != "null" {
		return false
	}
	for k := range m {
		switch k {
		case "type", "description", "title":
		default:
			return false
		}
	}
	return true
}

func isConstMember(m map[string]any) bool {
	if _, ok := m["const"]; !ok {
		return false
	}
	for k, v := range m {
		switch k {
		case "const", "description", "title":
		case "type":
			if _, ok := v.(string); !ok {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func isPureType(m map[string]any) bool {
	if len(m) != 1 {
		return false
	}
	_, ok := m["type"].(string)
	return ok
}

func mergeSchema(dst, src map[string]any) {
	for k, v := range src {
		if k == "description" || k == "title" {
			if existing, ok := dst[k]; ok && !isEmptyString(existing) {
				continue
			}
		}
		dst[k] = v
	}
}

func foldNullable(m map[string]any) {
	n, ok := m["nullable"].(bool)
	if !ok {
		return
	}
	delete(m, "nullable")
	if !n {
		return
	}
	m["type"] = addNull(m["type"])
}

func foldInteger(m map[string]any) {
	t, _ := m["type"].(string)
	if t != "number" && t != "integer" {
		return
	}
	if !isOne(m["multipleOf"]) {
		return
	}
	delete(m, "multipleOf")
	if t == "number" {
		m["type"] = "integer"
	}
}

func dropSafeBounds(m map[string]any) {
	if numEq(m["minimum"], jsMinSafe) {
		delete(m, "minimum")
	}
	if numEq(m["maximum"], jsMaxSafe) {
		delete(m, "maximum")
	}
}

func dropEmpty(m map[string]any) {
	if props, ok := m["properties"].(map[string]any); ok && len(props) == 0 {
		delete(m, "properties")
	}
	if req, ok := m["required"].([]any); ok && len(req) == 0 {
		delete(m, "required")
	}
}

func sortRequired(m map[string]any) {
	req, ok := m["required"].([]any)
	if !ok {
		return
	}
	strs := make([]string, len(req))
	for i, el := range req {
		s, ok := el.(string)
		if !ok {
			return
		}
		strs[i] = s
	}
	sort.Strings(strs)
	m["required"] = stringsToAny(strs)
}

func sortTypeNames(m map[string]any) {
	arr, ok := m["type"].([]any)
	if !ok {
		return
	}
	strs := make([]string, len(arr))
	for i, el := range arr {
		s, ok := el.(string)
		if !ok {
			return
		}
		strs[i] = s
	}
	sort.Strings(strs)
	m["type"] = stringsToAny(strs)
}

func markAny(m map[string]any) {
	for k := range m {
		switch k {
		case "description", "title":
		default:
			return
		}
	}
	if desc, ok := m["description"]; ok && isEmptyString(desc) {
		delete(m, "description")
	}
	m["type"] = "any"
}

func addNull(t any) any {
	var types []string
	switch n := t.(type) {
	case string:
		if n == "" || n == "null" {
			types = []string{"null"}
		} else {
			types = []string{n, "null"}
		}
	case []any:
		for _, el := range n {
			if s, ok := el.(string); ok {
				types = append(types, s)
			}
		}
		types = append(types, "null")
	case nil:
		types = []string{"any", "null"}
	default:
		return t
	}
	return sortedTypeAny(types)
}

func sortedTypeAny(in []string) []any {
	seen := map[string]bool{}
	var ss []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		ss = append(ss, s)
	}
	sort.Strings(ss)
	return stringsToAny(ss)
}

func stringsToAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func isEmptyString(v any) bool {
	s, ok := v.(string)
	return ok && s == ""
}
