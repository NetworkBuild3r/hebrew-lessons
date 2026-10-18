# hebrew-lessons

Greenfield rebuild (ADR-R1). This branch does not carry the previous Node app or the `go/` port. Migration `001` is a clean start: nothing is imported from the old database.

One static binary, `cmd/hebrew`, listens for the learner app on `:8080` (`HTTP_ADDR`) and for MCP on `:8787` (`MCP_ADDR`). Both serve `GET /healthz` until the features land. `SIGTERM` shuts them down. SQLite is `DATA_DIR/hebrew.sqlite` (`DATA_DIR` defaults to `/data`) in WAL mode, with one writer connection. Audio files will live in `DATA_DIR/audio/<sha256>.mp3`.

The image name is `ghcr.io/networkbuild3r/hebrew`. It must not be `ghcr.io/networkbuild3r/hebrew-lessons`: the production auto-updater watches that name. `scripts/check-image-name.sh` refuses any reference containing `hebrew-lessons`.

## Layout

| Path | Owner | What |
|---|---|---|
| `cmd/hebrew` | Backend | Process: environment, both listeners, `SIGTERM` |
| `internal/app` | Backend | `Config`, `New`, `Web`, `MCP`, `Close` |
| `internal/clock` | Backend | Injectable clock. Services take this and do not call `time.Now` |
| `internal/db` | Backend | SQLite open, migration runner, store |
| `migrations` | Backend | `001_init.sql`, embedded |
| `internal/auth` | Backend | Google sign-in, sessions, CSRF. `oauth*.go` is reserved for MCP Builder |
| `internal/lexicon` | Backend | Lexemes |
| `internal/audio` | Backend | Clip store and ElevenLabs TTS |
| `internal/units` | Backend | Unit document and `Validate`. `validate_*.go` and `rules.go` are reserved for MCP Builder |
| `internal/fsrs` | Backend | go-fsrs wrapper, cards, review log |
| `internal/mcp` | MCP Builder | Authoring tools |
| `internal/web` | Frontend | `net/http` ServeMux, templ views, embedded static assets |
| `internal/web/views` | Frontend | templ sources |
| `internal/web/static` | Frontend | css, icons, vendored htmx |

`//go:embed` cannot name a path that contains `..`, so `internal/db` cannot embed `migrations/` itself. The small root package `migrations` embeds `*.sql`, and the runner reads that filesystem. The SQL stays at the repository root.

## Test seams

`internal/app.Config` is the composition root.

- `DataDir`, `HTTPAddr`, `MCPAddr`, `PublicOrigin`, `TrustedProxies`, `CookieSecure`, and `LogLevel` come from `DATA_DIR`, `HTTP_ADDR`, `MCP_ADDR`, `PUBLIC_ORIGIN`, `TRUSTED_PROXIES`, `COOKIE_SECURE`, and `LOG_LEVEL`.
- `GoogleAuthURL`, `GoogleTokenURL`, `GoogleUserinfoURL`, `GoogleJWKSURL`, and `GoogleIssuer` come from `GOOGLE_AUTH_URL`, `GOOGLE_TOKEN_URL`, `GOOGLE_USERINFO_URL`, `GOOGLE_JWKS_URL`, and `GOOGLE_ISSUER`. Unset, they point at Google, so a test can aim them at a fake server.
- `Clock` implements `internal/clock.Clock` (`Now() time.Time`). Nil uses the system clock. Migration timestamps use this clock.
- `TTS` implements `internal/audio.TTS` (`Synthesize(ctx, textNFC, voice, model string) ([]byte, error)`). A non-nil value replaces the client. Nil with `ELEVENLABS_API_KEY` set selects the ElevenLabs stub. Nil with an empty key leaves synthesis off.
- `New` opens `DataDir`, runs migration 001, and returns handlers from `Web()` and `MCP()`. `Close` releases the database; `/healthz` then returns 503.

`cmd/hebrew` only parses the environment into `Config` and calls `New`.

## Checks

```sh
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test ./...
scripts/check-image-name.sh ghcr.io/networkbuild3r/hebrew:sha-abc
```

The image is `gcr.io/distroless/static-debian12:nonroot` (uid 65532) and exposes 8080 and 8787. A data volume has to be writable by that uid.
