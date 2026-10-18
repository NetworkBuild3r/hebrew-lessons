package library

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

// cursorVersion is the only cursor shape this package writes.
//
// A cursor is base64url (no padding) of compact JSON:
//
//	{"v":1,"ord":["<word>"],"key":"<word>"}
//
// ord is the ordering tuple. v1 sorts by the word alone, using JavaScript
// string order, so the tuple has one element and key repeats that word.
// key is the row identity used to break ties. A row is strictly after the
// cursor when its (ord..., key) is greater than the cursor's. Inserts of
// other words do not move a cursor, so a page that has already been returned
// is not repeated and is not shifted by a concurrent insert.
const cursorVersion = 1

type cursorBody struct {
	V   int      `json:"v"`
	Ord []string `json:"ord"`
	Key string   `json:"key"`
}

func encodeCursor(word string) (string, error) {
	raw, err := json.Marshal(cursorBody{V: cursorVersion, Ord: []string{word}, Key: word})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeCursor accepts "" as the start of the list. Any other value must be
// the canonical encoding of a v1 cursor.
func decodeCursor(s string) (cursorBody, error) {
	if s == "" {
		return cursorBody{}, nil
	}
	if len(s) > 512 {
		return cursorBody{}, errCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return cursorBody{}, errCursor
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var body cursorBody
	if err := dec.Decode(&body); err != nil {
		return cursorBody{}, errCursor
	}
	if dec.More() {
		return cursorBody{}, errCursor
	}
	if body.V != cursorVersion || body.Key == "" || len(body.Ord) != 1 || body.Ord[0] == "" {
		return cursorBody{}, errCursor
	}
	again, err := json.Marshal(body)
	if err != nil {
		return cursorBody{}, errCursor
	}
	if base64.RawURLEncoding.EncodeToString(again) != s {
		return cursorBody{}, errCursor
	}
	return body, nil
}

func rowAfter(word string, c cursorBody) bool {
	if c.Key == "" && len(c.Ord) == 0 {
		return true
	}
	ord := ""
	if len(c.Ord) > 0 {
		ord = c.Ord[0]
	}
	cmp := jsCompare(word, ord)
	if cmp > 0 {
		return true
	}
	if cmp < 0 {
		return false
	}
	return jsCompare(word, c.Key) > 0
}
