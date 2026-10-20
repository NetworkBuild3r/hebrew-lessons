package audio

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestElevenLabsRequest(t *testing.T) {
	const key = "sk_test_secret_value"
	const voice = "6WRlPmU1fqPwYbSxeP16"
	const model = "eleven_v3"
	const text = "שָׁלוֹם"
	var gotMethod, gotPath, gotKey, gotAccept, gotType string
	var gotQuery string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotKey = r.Header.Get("xi-api-key")
		gotAccept = r.Header.Get("Accept")
		gotType = r.Header.Get("Content-Type")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("ID3fake-mp3"))
	}))
	t.Cleanup(srv.Close)

	client := NewElevenLabs(key)
	client.BaseURL = srv.URL
	client.HTTP = srv.Client()
	body, err := client.Synthesize(context.Background(), text, voice, model)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ID3fake-mp3" {
		t.Fatalf("body %q", body)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method %s", gotMethod)
	}
	if gotPath != "/v1/text-to-speech/"+voice {
		t.Fatalf("path %s", gotPath)
	}
	if gotQuery != "output_format=mp3_44100_128" {
		t.Fatalf("query %s", gotQuery)
	}
	if gotKey != key {
		t.Fatalf("xi-api-key %q", gotKey)
	}
	if gotAccept != "audio/mpeg" || gotType != "application/json" {
		t.Fatalf("headers accept %q type %q", gotAccept, gotType)
	}
	var payload struct {
		Text     string `json:"text"`
		ModelID  string `json:"model_id"`
		Language string `json:"language_code"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Text != text || payload.ModelID != model || payload.Language != "he" {
		t.Fatalf("payload %+v", payload)
	}
	if strings.Contains(string(gotBody), key) {
		t.Fatal("request body contained the api key")
	}
}

func TestElevenLabsDefaultsVoiceAndModel(t *testing.T) {
	var gotPath string
	var payload struct {
		ModelID string `json:"model_id"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode: %v", err)
		}
		_, _ = w.Write([]byte("mp3"))
	}))
	t.Cleanup(srv.Close)
	client := NewElevenLabs("key")
	client.BaseURL = srv.URL
	client.HTTP = srv.Client()
	if _, err := client.Synthesize(context.Background(), "שָׁלוֹם", "", ""); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/text-to-speech/"+DefaultVoiceID || payload.ModelID != DefaultModel {
		t.Fatalf("path %s model %s", gotPath, payload.ModelID)
	}
}

func TestElevenLabsNon2xxRedactsKey(t *testing.T) {
	const key = "sk_test_secret_value"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":{"message":"invalid ` + key + `","code":"unauthorized"}}`))
	}))
	t.Cleanup(srv.Close)
	client := NewElevenLabs(key)
	client.BaseURL = srv.URL
	client.HTTP = srv.Client()
	_, err := client.Synthesize(context.Background(), "שָׁלוֹם", DefaultVoiceID, DefaultModel)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("error %v", err)
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("api key leaked in error: %v", err)
	}
}

func TestElevenLabsPaidPlanErrorOmitsKey(t *testing.T) {
	const key = "sk_paid_plan_key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":{"code":"paid_plan_required","message":"upgrade `+key+`"}}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	client := NewElevenLabs(key)
	client.BaseURL = srv.URL
	client.HTTP = srv.Client()
	_, err := client.Synthesize(context.Background(), "שָׁלוֹם", "voice", "eleven_v3")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "paid ElevenLabs plan") {
		t.Fatalf("error %v", err)
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("api key leaked: %v", err)
	}
}

func TestElevenLabsMissingKeyDoesNotCall(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hit = true
	}))
	t.Cleanup(srv.Close)
	for _, client := range []*ElevenLabs{nil, NewElevenLabs(""), NewElevenLabs("   ")} {
		if client != nil {
			client.BaseURL = srv.URL
			client.HTTP = srv.Client()
		}
		_, err := client.Synthesize(context.Background(), "שָׁלוֹם", "voice", "eleven_v3")
		if err == nil || !strings.Contains(err.Error(), "api key") {
			t.Fatalf("client %#v error %v", client, err)
		}
	}
	if hit {
		t.Fatal("missing key called the provider")
	}
}

func TestElevenLabsContextCancel(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hit = true
	}))
	t.Cleanup(srv.Close)
	client := NewElevenLabs("sk_cancel_key")
	client.BaseURL = srv.URL
	client.HTTP = srv.Client()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Synthesize(ctx, "שָׁלוֹם", "voice", "model")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
	if strings.Contains(err.Error(), "sk_cancel_key") {
		t.Fatalf("api key leaked: %v", err)
	}
	if hit {
		t.Fatal("canceled context called the provider")
	}
}

func TestElevenLabsTimeout(t *testing.T) {
	const key = "sk_timeout_key"
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	client := NewElevenLabs(key)
	client.BaseURL = srv.URL
	client.HTTP = srv.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := client.Synthesize(ctx, "שָׁלוֹם", "voice", "model")
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("error %v", err)
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("api key leaked: %v", err)
	}
}
