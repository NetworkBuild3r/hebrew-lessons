package mcp

import "strings"

// Field-level plan diff. Port of server/plans/diff.js. Pure: no I/O.

var diffBuckets = []string{"words", "paragraphs", "lines", "questions"}

var childLists = [][]string{
	{"units"},
	{"lessons"},
	{"blocks"},
	diffBuckets,
	{},
}

const (
	diffMaxEntries    = 500
	diffMaxBytes      = 64 * 1024
	diffMaxValueChars = 4000
	diffCut           = "…"
)

type diffNode struct {
	root      bool
	parent    string
	raw       Value
	fields    []Field
	lists     map[string][]string
	listOrder []string
}

func diffDocuments(a, b Value, scope string) (Value, bool) {
	scope = strings.Trim(scope, "/")
	wanted := allEntries(a, b)
	var kept []Value
	for _, entry := range wanted {
		path, _ := entry.Get("path")
		if inScope(path.Str, scope) {
			kept = append(kept, entry)
		}
	}
	var entries []Value
	bytes := 0
	truncated := false
	for _, entry := range kept {
		if len(entries) >= diffMaxEntries {
			truncated = true
			break
		}
		out, cut := shapeEntry(entry)
		if cut {
			out = putField(out, "valueTruncated", Bool(true))
		}
		size := UTF16Len(Stringify(out, 0))
		if bytes+size > diffMaxBytes && len(entries) > 0 {
			truncated = true
			break
		}
		bytes += size
		entries = append(entries, out)
	}
	if entries == nil {
		entries = []Value{}
	}
	return Array(entries...), truncated
}

func shapeEntry(entry Value) (Value, bool) {
	path, _ := entry.Get("path")
	field, _ := entry.Get("field")
	kind, _ := entry.Get("kind")
	out := Object(
		Field{Key: "path", Val: path},
		Field{Key: "field", Val: field},
		Field{Key: "kind", Val: kind},
	)
	cut := false
	for _, side := range []string{"before", "after"} {
		if !entry.Has(side) {
			continue
		}
		val, _ := entry.Get(side)
		shaped, did := shapeValue(val)
		out = putField(out, side, shaped)
		if did {
			cut = true
		}
	}
	return out, cut
}

func shapeValue(v Value) (Value, bool) {
	if v.Kind == KindString {
		if UTF16Len(v.Str) > diffMaxValueChars {
			runes := []rune(v.Str)
			// Cut by JS length, which for BMP equals runes.
			if len(runes) > diffMaxValueChars {
				runes = runes[:diffMaxValueChars]
			}
			return String(string(runes) + diffCut), true
		}
		return v, false
	}
	text := Stringify(v, 0)
	if UTF16Len(text) > diffMaxValueChars {
		runes := []rune(text)
		if len(runes) > diffMaxValueChars {
			runes = runes[:diffMaxValueChars]
		}
		return String(string(runes) + diffCut), true
	}
	return v, false
}

func inScope(path, scope string) bool {
	return scope == "" || path == scope || strings.HasPrefix(path, scope+"/")
}

func allEntries(a, b Value) []Value {
	left := collect(a)
	right := collect(b)
	var entries []Value
	for _, path := range right.order {
		nodeB := right.nodes[path]
		nodeA, ok := left.nodes[path]
		if !ok {
			if !nodeB.root && left.has(nodeB.parent) {
				entries = append(entries, Object(
					Field{Key: "path", Val: String(path)},
					Field{Key: "field", Val: String("")},
					Field{Key: "kind", Val: String("added")},
					Field{Key: "after", Val: nodeB.raw},
				))
			}
			continue
		}
		names := fieldNames(nodeB.fields)
		for _, f := range nodeA.fields {
			if !contains(names, f.Key) {
				names = append(names, f.Key)
			}
		}
		for _, name := range names {
			va, inA := fieldGet(nodeA.fields, name)
			vb, inB := fieldGet(nodeB.fields, name)
			if inA && inB && stable(va) == stable(vb) {
				continue
			}
			entry := Object(
				Field{Key: "path", Val: String(path)},
				Field{Key: "field", Val: String(name)},
				Field{Key: "kind", Val: String("changed")},
			)
			if inA {
				entry = putField(entry, "before", va)
			}
			if inB {
				entry = putField(entry, "after", vb)
			}
			entries = append(entries, entry)
		}
		listNames := append([]string{}, nodeB.listOrder...)
		for _, name := range nodeA.listOrder {
			if !contains(listNames, name) {
				listNames = append(listNames, name)
			}
		}
		for _, name := range listNames {
			idsA := nodeA.lists[name]
			idsB := nodeB.lists[name]
			inB := map[string]bool{}
			for _, id := range idsB {
				inB[id] = true
			}
			inA := map[string]bool{}
			for _, id := range idsA {
				inA[id] = true
			}
			var commonA, commonB []string
			for _, id := range idsA {
				if inB[id] {
					commonA = append(commonA, id)
				}
			}
			for _, id := range idsB {
				if inA[id] {
					commonB = append(commonB, id)
				}
			}
			if !sameStrings(commonA, commonB) {
				entries = append(entries, Object(
					Field{Key: "path", Val: String(path)},
					Field{Key: "field", Val: String(name)},
					Field{Key: "kind", Val: String("moved")},
					Field{Key: "before", Val: stringArray(commonA)},
					Field{Key: "after", Val: stringArray(commonB)},
				))
			}
		}
	}
	for _, path := range left.order {
		if right.has(path) {
			continue
		}
		nodeA := left.nodes[path]
		if !nodeA.root && right.has(nodeA.parent) {
			entries = append(entries, Object(
				Field{Key: "path", Val: String(path)},
				Field{Key: "field", Val: String("")},
				Field{Key: "kind", Val: String("removed")},
				Field{Key: "before", Val: nodeA.raw},
			))
		}
	}
	return entries
}

type collected struct {
	nodes map[string]diffNode
	order []string
}

func (c collected) has(path string) bool {
	_, ok := c.nodes[path]
	return ok
}

func collect(doc Value) collected {
	c := collected{nodes: map[string]diffNode{}}
	var visit func(path string, root bool, parent string, node Value, depth int)
	visit = func(path string, root bool, parent string, node Value, depth int) {
		listsAt := []string{}
		if depth < len(childLists) {
			listsAt = childLists[depth]
		}
		var fields []Field
		lists := map[string][]string{}
		var listOrder []string
		type child struct {
			key   string
			items []Value
		}
		var children []child
		for _, f := range node.Obj {
			if contains(listsAt, f.Key) && addressable(f.Val) {
				ids := make([]string, len(f.Val.Arr))
				for i, item := range f.Val.Arr {
					id, _ := item.Get("id")
					ids[i] = id.Str
				}
				lists[f.Key] = ids
				listOrder = append(listOrder, f.Key)
				children = append(children, child{f.Key, f.Val.Arr})
			} else if !(depth == 0 && f.Key == "revision") {
				fields = append(fields, f)
			}
		}
		c.nodes[path] = diffNode{root: root, parent: parent, raw: node, fields: fields, lists: lists, listOrder: listOrder}
		c.order = append(c.order, path)
		for _, ch := range children {
			for _, item := range ch.items {
				id, _ := item.Get("id")
				visit(joinPathID(path, ch.key, id.Str), false, path, item, depth+1)
			}
		}
	}
	if doc.Kind == KindObject {
		visit("", true, "", doc, 0)
	}
	return c
}

func joinPathID(parent, list, id string) string {
	if parent == "" {
		return list + "/" + id
	}
	return parent + "/" + list + "/" + id
}

func addressable(list Value) bool {
	if list.Kind != KindArray {
		return false
	}
	seen := map[string]bool{}
	for _, item := range list.Arr {
		if item.Kind != KindObject {
			return false
		}
		id, ok := item.Get("id")
		if !ok || id.Kind != KindString || id.Str == "" || strings.Contains(id.Str, "/") {
			return false
		}
		if seen[id.Str] {
			return false
		}
		seen[id.Str] = true
	}
	return true
}

func stable(v Value) string {
	switch v.Kind {
	case KindArray:
		parts := make([]string, len(v.Arr))
		for i, item := range v.Arr {
			parts[i] = stable(item)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case KindObject:
		keys := make([]string, len(v.Obj))
		for i, f := range v.Obj {
			keys[i] = f.Key
		}
		// insertion order is not sorted yet
		sorted := append([]Field{}, v.Obj...)
		for i := 1; i < len(sorted); i++ {
			j := i
			for j > 0 && sorted[j].Key < sorted[j-1].Key {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
				j--
			}
		}
		parts := make([]string, len(sorted))
		for i, f := range sorted {
			parts[i] = quote(f.Key) + ":" + stable(f.Val)
		}
		return "{" + strings.Join(parts, ",") + "}"
	default:
		return Stringify(v, 0)
	}
}

func fieldNames(fields []Field) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.Key
	}
	return out
}

func fieldGet(fields []Field, key string) (Value, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f.Val, true
		}
	}
	return Value{}, false
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringArray(ids []string) Value {
	items := make([]Value, len(ids))
	for i, id := range ids {
		items[i] = String(id)
	}
	return Array(items...)
}
