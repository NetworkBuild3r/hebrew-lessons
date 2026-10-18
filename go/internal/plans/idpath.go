package plans

import (
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/jsstr"
)

// buckets are the block lists read_part can address by id.
var buckets = map[string]struct{}{
	"words": {}, "paragraphs": {}, "lines": {}, "questions": {},
}

// toIDPath turns an RFC 6901 pointer into the deepest id path read_part
// accepts and the remaining field pointer. It mirrors server/plans/id-path.js.
func toIDPath(plan Value, ptr string) (path, field string) {
	segs := segmentsOf(ptr)
	var parts []string
	used := 0
	if plan.IsObject() && len(segs) >= 2 && segs[0] == "units" {
		units, _ := plan.Get("units")
		u, uOK := jsIndex(unescape(segs[1]))
		if uOK && addressable(units, u) {
			id, _ := units.A[u].Get("id")
			parts = append(parts, "units", id.S)
			used = 2
			unit := units.A[u]
			if len(segs) >= 4 && segs[2] == "lessons" {
				l, lOK := jsIndex(unescape(segs[3]))
				lessons, _ := unit.Get("lessons")
				if lOK && addressable(lessons, l) {
					lid, _ := lessons.A[l].Get("id")
					parts = append(parts, "lessons", lid.S)
					used = 4
					lesson := lessons.A[l]
					if len(segs) >= 6 && segs[4] == "blocks" {
						b, bOK := jsIndex(unescape(segs[5]))
						blocks, _ := lesson.Get("blocks")
						if bOK && addressable(blocks, b) {
							bid, _ := blocks.A[b].Get("id")
							parts = append(parts, "blocks", bid.S)
							used = 6
							block := blocks.A[b]
							bucket := ""
							if len(segs) >= 8 {
								bucket = unescape(segs[6])
							}
							m, mOK := 0, false
							if bucket != "" && len(segs) >= 8 {
								m, mOK = jsIndex(unescape(segs[7]))
							}
							if _, ok := buckets[bucket]; ok && mOK {
								col, _ := block.Get(bucket)
								if addressable(col, m) {
									mid, _ := col.A[m].Get("id")
									parts = append(parts, bucket, mid.S)
									used = 8
								}
							}
						}
					}
				}
			}
		}
	}
	return strings.Join(parts, "/"), strings.Join(segs[used:], "/")
}

func actionFor(field string) string {
	if field != "" && !strings.Contains(field, "/") {
		return "set_field"
	}
	return "replace_part"
}

func segmentsOf(pointer string) []string {
	if pointer == "" {
		return nil
	}
	parts := strings.Split(pointer, "/")
	if len(parts) == 0 {
		return nil
	}
	return parts[1:]
}

func unescape(segment string) string {
	s := strings.ReplaceAll(segment, "~1", "/")
	return strings.ReplaceAll(s, "~0", "~")
}

// jsIndex is Number(segment) when that number is an integer array index.
// Number("") is 0, matching JavaScript.
func jsIndex(s string) (int, bool) {
	s = jsstr.Trim(s)
	if s == "" {
		return 0, true
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func addressable(items Value, index int) bool {
	if items.K != KindArr || index < 0 || index >= len(items.A) {
		return false
	}
	item := items.A[index]
	if !item.IsObject() {
		return false
	}
	id, ok := item.Get("id")
	if !ok || id.K != KindStr || id.S == "" {
		return false
	}
	for i, other := range items.A {
		if !other.IsObject() {
			continue
		}
		oid, ok := other.Get("id")
		if ok && oid.K == KindStr && oid.S == id.S {
			return i == index
		}
	}
	return false
}
