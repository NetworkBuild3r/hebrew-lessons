package testharness

import (
	"bytes"
	"context"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"

	_ "modernc.org/sqlite"
)

func TestHealthzBothServers(t *testing.T) {
	t.Parallel()
	env := Start(t)
	for _, base := range []string{env.WebURL, env.MCPURL} {
		status, body := getJSON(t, base+"/healthz")
		if status != http.StatusOK || !body.OK {
			t.Fatalf("GET %s/healthz = %d %+v", base, status, body)
		}
	}
	if env.App.TTS() != env.TTS {
		t.Fatal("app TTS is not the harness fake")
	}
	if _, ok := env.App.TTS().(*audio.ElevenLabs); ok {
		t.Fatal("harness constructed the ElevenLabs client")
	}
	if env.App.AudioDir() != filepath.Join(env.DataDir, "audio") {
		t.Fatalf("audio dir %s", env.App.AudioDir())
	}
	if _, err := os.Stat(filepath.Join(env.DataDir, "hebrew.sqlite")); err != nil {
		t.Fatal(err)
	}
}

func TestParallelHarnessesDoNotShareState(t *testing.T) {
	t.Parallel()
	left := Start(t)
	right := Start(t)
	if left.DataDir == right.DataDir || left.WebURL == right.WebURL || left.MCPURL == right.MCPURL || left.IDP.Issuer == right.IDP.Issuer {
		t.Fatalf("shared endpoints left=%+v right=%+v", left, right)
	}
	if _, err := os.Stat(filepath.Join(left.DataDir, "hebrew.sqlite")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(right.DataDir, "hebrew.sqlite")); err != nil {
		t.Fatal(err)
	}

	errc := make(chan error, 2)
	go func() { errc <- poke(left, time.Hour, "left@example.com") }()
	go func() { errc <- poke(right, 2*time.Hour, "right@example.com") }()
	for range 2 {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}

	if !left.Clock.Now().Equal(Epoch().Add(time.Hour)) || !right.Clock.Now().Equal(Epoch().Add(2*time.Hour)) {
		t.Fatalf("clocks left %s right %s", left.Clock.Now(), right.Clock.Now())
	}
	if left.TTS.Calls("ping") != 1 || right.TTS.Calls("ping") != 1 {
		t.Fatalf("tts calls left %d right %d", left.TTS.Calls("ping"), right.TTS.Calls("ping"))
	}
	if email := userinfoEmail(t, left); email != "left@example.com" {
		t.Fatalf("left userinfo %q", email)
	}
	if email := userinfoEmail(t, right); email != "right@example.com" {
		t.Fatalf("right userinfo %q", email)
	}

	tok, err := left.IDP.MintIDToken()
	if err != nil {
		t.Fatal(err)
	}
	leftKey := jwksKey(t, left.IDP.JWKSURL, mustKID(t, tok))
	rightKey := jwksKey(t, right.IDP.JWKSURL, "")
	if _, err := VerifyIDToken(tok, leftKey); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIDToken(tok, rightKey); err == nil {
		t.Fatal("left token verified with the right harness key")
	}
	if leftKey.N.Cmp(rightKey.N) == 0 {
		t.Fatal("harnesses share an RSA modulus")
	}
}

func TestFakeTTSAndClock(t *testing.T) {
	t.Parallel()
	env := Start(t)
	hello := lexemeByGloss(t, loadLexemes(t), "hello; peace")
	coffee := lexemeByGloss(t, loadLexemes(t), "coffee")
	ctx := context.Background()

	first, err := env.TTS.Synthesize(ctx, hello, "voice-a", "model-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.TTS.Synthesize(ctx, hello, "voice-b", "model-b")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || string(first) != "fake:"+hello {
		t.Fatalf("bytes %q and %q", first, second)
	}
	if env.TTS.Calls(hello) != 2 || env.TTS.Calls(coffee) != 0 {
		t.Fatalf("calls hello %d coffee %d", env.TTS.Calls(hello), env.TTS.Calls(coffee))
	}

	env.TTS.FailText(coffee, errors.New("tts down"))
	if _, err := env.TTS.Synthesize(ctx, coffee, "voice", "model"); err == nil || err.Error() != "tts down" {
		t.Fatalf("fail text: %v", err)
	}
	if env.TTS.Calls(coffee) != 1 {
		t.Fatalf("failed call was not counted: %d", env.TTS.Calls(coffee))
	}
	env.TTS.FailText(coffee, nil)
	got, err := env.TTS.Synthesize(ctx, coffee, "voice", "model")
	if err != nil || string(got) != "fake:"+coffee {
		t.Fatalf("cleared failure: %v %q", err, got)
	}

	env.TTS.FailNext(errors.New("next"))
	if _, err := env.TTS.Synthesize(ctx, hello, "voice", "model"); err == nil || err.Error() != "next" {
		t.Fatalf("fail next: %v", err)
	}
	if _, err := env.TTS.Synthesize(ctx, hello, "voice", "model"); err != nil {
		t.Fatal(err)
	}
	if env.TTS.Calls(hello) != 4 {
		t.Fatalf("hello calls %d", env.TTS.Calls(hello))
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := env.TTS.Synthesize(canceled, hello, "voice", "model"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
	if env.TTS.Calls(hello) != 4 {
		t.Fatal("canceled call was counted")
	}

	if !env.Clock.Now().Equal(Epoch()) {
		t.Fatalf("clock started at %s", env.Clock.Now())
	}
	env.Clock.Advance(90 * time.Minute)
	if !env.Clock.Now().Equal(Epoch().Add(90 * time.Minute)) {
		t.Fatalf("clock advanced to %s", env.Clock.Now())
	}
	if got := migrationTime(t, env.DataDir); !got.Equal(Epoch()) {
		t.Fatalf("schema_migrations.applied_at = %s, want the clock at open", got)
	}
}

func TestIDPTokenVerifiesAgainstJWKS(t *testing.T) {
	t.Parallel()
	env := Start(t)
	env.IDP.SetEmail("one@example.com")
	first, err := env.IDP.MintIDToken()
	if err != nil {
		t.Fatal(err)
	}
	pub := jwksKey(t, env.IDP.JWKSURL, mustKID(t, first))
	claims, err := VerifyIDToken(first, pub)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Email != "one@example.com" || claims.Iss != env.IDP.Issuer || !claims.EmailVerified {
		t.Fatalf("claims %+v", claims)
	}
	if claims.Iat != Epoch().Unix() || claims.Exp != claims.Iat+3600 {
		t.Fatalf("iat %d exp %d", claims.Iat, claims.Exp)
	}
	if !strings.HasPrefix(env.IDP.Issuer, "http://127.0.0.1:") {
		t.Fatalf("issuer %s", env.IDP.Issuer)
	}
	for _, raw := range []string{env.IDP.AuthURL, env.IDP.TokenURL, env.IDP.UserinfoURL, env.IDP.JWKSURL} {
		if !strings.HasPrefix(raw, env.IDP.Issuer+"/") {
			t.Fatalf("endpoint %s is not on the issuer", raw)
		}
	}

	env.IDP.SetEmail("two@example.com")
	second, err := env.IDP.MintIDToken()
	if err != nil {
		t.Fatal(err)
	}
	claims, err = VerifyIDToken(second, pub)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Email != "two@example.com" {
		t.Fatalf("email %q", claims.Email)
	}
	if _, err := VerifyIDToken(tamperSig(t, second), pub); err == nil {
		t.Fatal("tampered token verified")
	}

	if email := userinfoEmail(t, env); email != "two@example.com" {
		t.Fatalf("userinfo %q", email)
	}
	authResp, err := http.Get(env.IDP.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	authBody, _ := io.ReadAll(authResp.Body)
	authResp.Body.Close()
	if authResp.StatusCode != http.StatusOK {
		t.Fatalf("auth status %d %s", authResp.StatusCode, authBody)
	}

	resp, err := http.Post(env.IDP.TokenURL, "application/x-www-form-urlencoded", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token status %d %s", resp.StatusCode, raw)
	}
	var issued struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		IDToken     string `json:"id_token"`
	}
	if err := json.Unmarshal(raw, &issued); err != nil {
		t.Fatal(err)
	}
	if issued.TokenType != "Bearer" || issued.ExpiresIn != 3600 || issued.AccessToken == "" {
		t.Fatalf("token response %+v", issued)
	}
	claims, err = VerifyIDToken(issued.IDToken, pub)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Email != "two@example.com" || claims.Iss != env.IDP.Issuer {
		t.Fatalf("token endpoint claims %+v", claims)
	}
}

func TestSeedHelpers(t *testing.T) {
	t.Parallel()
	env := Start(t)
	t.Run("learner", func(t *testing.T) {
		env.SeedLearner(t, "learner@example.com")
	})
	t.Run("admin", func(t *testing.T) {
		env.SeedAdmin(t, "admin@example.com")
	})
}

func poke(env *Env, d time.Duration, email string) error {
	env.Clock.Advance(d)
	if _, err := env.TTS.Synthesize(context.Background(), "ping", "voice", "model"); err != nil {
		return err
	}
	env.IDP.SetEmail(email)
	for _, raw := range []string{env.WebURL + "/healthz", env.MCPURL + "/healthz"} {
		resp, err := http.Get(raw)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.New(raw + " " + resp.Status)
		}
		var doc struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(body, &doc); err != nil || !doc.OK {
			return errors.New(raw + " " + string(body))
		}
	}
	return nil
}

func userinfoEmail(t *testing.T, env *Env) string {
	t.Helper()
	resp, err := http.Get(env.IDP.UserinfoURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var doc struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !doc.EmailVerified {
		t.Fatalf("userinfo status %d %+v", resp.StatusCode, doc)
	}
	return doc.Email
}

func jwksKey(t *testing.T, rawURL, kid string) *rsa.PublicKey {
	t.Helper()
	resp, err := http.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("jwks %d %s", resp.StatusCode, body)
	}
	pub, err := RSAPublicKeyFromJWKS(body, kid)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func mustKID(t *testing.T, token string) string {
	t.Helper()
	kid, err := IDTokenKID(token)
	if err != nil {
		t.Fatal(err)
	}
	return kid
}

func tamperSig(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	sig[0] ^= 0xff
	parts[2] = base64.RawURLEncoding.EncodeToString(sig)
	return strings.Join(parts, ".")
}

func migrationTime(t *testing.T, dataDir string) time.Time {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(dataDir, "hebrew.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("_busy_timeout", "5000")
	u.RawQuery = q.Encode()
	conn, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var raw string
	if err := conn.QueryRowContext(ctx,
		`SELECT applied_at FROM schema_migrations WHERE version = 1`,
	).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	got, err := db.ParseTime(raw)
	if err != nil {
		t.Fatalf("applied_at %q: %v", raw, err)
	}
	return got
}

type healthBody struct {
	OK bool `json:"ok"`
}

func getJSON(t *testing.T, rawURL string) (int, healthBody) {
	t.Helper()
	resp, err := http.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body healthBody
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("%s: %v (%s)", rawURL, err, raw)
		}
	}
	return resp.StatusCode, body
}
