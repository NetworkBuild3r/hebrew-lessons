package normalize

import (
	"net/http"
	"testing"
)

func TestMaskVolatileValues(t *testing.T) {
	in := map[string]any{
		"id":                  "8ee73fd9-c223-4e01-8dcb-5f0a784eed97",
		"client_id":           "11111111-2222-4333-8444-555555555555",
		"updatedAt":           "2026-10-18T15:04:05.123Z",
		"client_id_issued_at": float64(1760800000),
		"access_token":        "abcdef0123456789abcdef0123456789",
		"refresh_token":       "short",
		"code":                "abcdefghijklmnopqrstuvwxyz0123456789ABCDE",
		"error":               "revision_conflict",
		"description":         "Keep the example 8ee73fd9-c223-4e01-8dcb-5f0a784eed97 in prose.",
		"title":               "List plans",
		"nested":              map[string]any{"at": "2026-01-02T03:04:05+00:00"},
		"plain":               "hebrew",
	}
	got := Mask(in).(map[string]any)
	if got["id"] != maskUUID || got["client_id"] != maskUUID {
		t.Fatalf("ids: %#v %#v", got["id"], got["client_id"])
	}
	if got["updatedAt"] != maskTimestamp {
		t.Fatalf("timestamp: %#v", got["updatedAt"])
	}
	if got["client_id_issued_at"] != maskEpoch {
		t.Fatalf("epoch: %#v", got["client_id_issued_at"])
	}
	if got["access_token"] != maskToken || got["refresh_token"] != maskToken || got["code"] != maskToken {
		t.Fatalf("tokens: %#v %#v %#v", got["access_token"], got["refresh_token"], got["code"])
	}
	if got["error"] != "revision_conflict" {
		t.Fatalf("domain code masked: %#v", got["error"])
	}
	if got["description"] != in["description"] || got["title"] != in["title"] {
		t.Fatal("prose was rewritten")
	}
	nested := got["nested"].(map[string]any)
	if nested["at"] != maskTimestamp {
		t.Fatalf("nested timestamp: %#v", nested["at"])
	}
	if got["plain"] != "hebrew" {
		t.Fatalf("plain text: %#v", got["plain"])
	}
}

func TestMaskStringInsideSentence(t *testing.T) {
	in := `Unknown plan 8ee73fd9-c223-4e01-8dcb-5f0a784eed97 at 2026-10-18T15:04:05Z.`
	got := MaskString(in)
	want := "Unknown plan " + maskUUID + " at " + maskTimestamp + "."
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestMaskCookieKeepsAttributes(t *testing.T) {
	h := http.Header{}
	h.Add("Set-Cookie", "session=abc.def; Path=/; HttpOnly; SameSite=Lax")
	h.Add("Date", "Sun, 18 Oct 2026 15:00:00 GMT")
	h.Add("Cf-Ray", "abc")
	h.Add("Content-Type", "application/json")
	got := MaskHeaders(h)
	if _, ok := got["Date"]; ok {
		t.Fatal("kept Date")
	}
	if _, ok := got["Cf-Ray"]; ok {
		t.Fatal("kept cf-ray")
	}
	if got["Content-Type"] != "application/json" {
		t.Fatalf("content-type: %#v", got["Content-Type"])
	}
	if got["Set-Cookie"] != "session="+maskToken+"; Path=/; HttpOnly; SameSite=Lax" {
		t.Fatalf("cookie: %s", got["Set-Cookie"])
	}
}

func TestMaskIsIdempotent(t *testing.T) {
	once := Mask(map[string]any{"id": "8ee73fd9-c223-4e01-8dcb-5f0a784eed97"})
	twice := Mask(once)
	if marshal(t, once) != marshal(t, twice) {
		t.Fatalf("once %s twice %s", marshal(t, once), marshal(t, twice))
	}
}
