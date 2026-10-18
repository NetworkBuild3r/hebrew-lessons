# Hebrew worksheets

Practice pages for דרך עברית. Each lesson is a folder of content. One shared player walks an ordered path of activities (words, story, drills, speaking). Sign-in is required; progress saves on the server for the signed-in learner.

Run the container and open `http://127.0.0.1:8080` (or your deployed origin). Opening lesson files via `file://` no longer works. Operator setup (first admin, backups, lesson sharing): [docs/operations/accounts-and-admin.md](docs/operations/accounts-and-admin.md).

**חזרה היום (Review today)** (`review/index.html`, linked from the home shelf) mixes due words from every lesson the learner can see for spaced recall.

## Lesson path

A lesson can list `activities` in any order. If it omits that list, the player builds a default from the sections that exist. Each lesson has a `level` (`basic`, `intermediate`, `advanced`) that sets choice count, word-bank openness, and quiz timing.

On a phone you get one step per screen, a progress bar, and a bottom Continue bar. On a desktop the same screens are centered with more room. Add the site to your home screen from the browser menu so Safari is less likely to clear saved answers.

## Repeat a lesson

**עוד סיבוב (Practice again)** clears the answers and starts the next round.

- A day between rounds helps memory. The lesson and the shelf show when the next round is due.
- Round 1 shows meanings. Later rounds hide the English until you ask, then ask *knew it / missed it*.
- Vowels stay on by default. A button hides them.
- Story read-aloud has Pause, Resume, Stop, and optional follow-along highlighting.
- Example sentences and speaking questions have a microphone. Recordings stay in the page until you reload.

## Add a lesson

Prefer the MCP tools (`list_activity_types`, `list_words`, `upsert_lesson`). Or:

1. Copy `worksheets/_template` to `worksheets/<new-slug>`.
2. Edit only `worksheet.js`. Set `id`, `level`, and `activities`.
3. Add one object to `worksheets/catalog.js`.

Do not edit a lesson's `index.html`. Behavior lives in `shared/player/` and `shared/activities/`. Appearance lives in `shared/worksheet.css`.

## Run in a container

Sign-in is required. Create the first admin before anyone can use the site (password from a TTY prompt or `--password-file`; never `--password=` on the command line):

```powershell
$env:DATA_DIR = "C:\path\to\app-data"
npm run user -- create <name> --admin
```

Then:

```
docker compose up --build
```

Open `http://127.0.0.1:8080`. Agents speak to `http://127.0.0.1:8787/mcp`. Full account, backup, and audio steps: [docs/operations/accounts-and-admin.md](docs/operations/accounts-and-admin.md).

For a deploy that keeps MCP-written lessons across image rebuilds, set `LESSONS_DIR` to a mounted volume. See [deploy/k8s-lessons-volume.md](deploy/k8s-lessons-volume.md). Database volume and k8s env: [deploy/k8s-database.md](deploy/k8s-database.md). If a lesson exists only on a live pod, pull it first with `node scripts/rescue-lesson.js <slug> <mcpUrl>`.

The login throttle keys on the socket address unless `TRUSTED_PROXIES` lists the proxy that opened the connection. The same list decides whether `X-Forwarded-Proto` may set the scheme of the CSRF host origin. A trusted peer uses the rightmost `http` or `https` value, which is the scheme Traefik appends. Any other peer uses the socket address for the throttle and the socket's TLS state for the scheme. The value is a comma-separated list of IPv4 or IPv6 addresses or CIDRs. Leave it empty when the process is reached directly. Note for the team lead (Hebrew Dev) and CI/CD: behind Cloudflare and Traefik, set it to Traefik's pod CIDR before or with deploy, and confirm Traefik passes `CF-Connecting-IP` through untouched. Until that value is set, every learner shares the Traefik peer's throttle bucket. Details: [deploy/k8s-database.md](deploy/k8s-database.md).

## Agents

Cursor loads `.cursor/mcp.json` (`node mcp/stdio.js`):

- `list_lessons`
- `get_lesson`
- `list_words`
- `list_activity_types`
- `upsert_lesson` (generates ElevenLabs audio after a successful write; pass `audio:false` to skip)
- `generate_lesson_audio` (backfill or force-regenerate sentence clips for an existing lesson)

## What a lesson can include

| Key | Role |
| --- | --- |
| `level` | `basic` / `intermediate` / `advanced` defaults for drills |
| `activities` | Ordered path of activity types |
| `vocab.items` | Word cards and feed for match / quiz / listen / type |
| `fill` | Word-bank sentences |
| `grammar` | One pattern + multiple choice |
| `reading` | Pointed story, `{ word }` marks, optional `retell` and questions |
| `askFriend` | Speaking questions with `pointed` |

Activity types include `words`, `match`, `story`, `fill`, `pattern`, `retell`, `ask-friend`, `speed-quiz`, `build-sentence`, `listen-pick`, and `type-it`.

Answers live under `hebrew-worksheet:<id>` in this browser, including the current step (`#step-N`). There is no JSON save/load.

Speech prefers ElevenLabs clips for every listen action (words, listen-pick, story read-aloud, pattern, build-sentence, speaking questions). Browser `speechSynthesis` is only the fallback when a clip is missing.

`upsert_lesson` generates audio automatically. Agents should omit `audio` / `sentenceAudio` / `reading.audio` fields — the server fills them in. Word clips are permanent under `worksheets/_audio/words/` (or `LESSONS_DIR/_audio/words/` on the volume) and are reused across lessons by exact pointed text (never overwritten). The player also loads `shared/word-audio-catalog.js` so a word like קָם plays the same ElevenLabs clip in every lesson even if that lesson’s worksheet omitted `audio`. Lesson clips live under `<lesson>/audio/` and regenerate only when their text changes.

API key lives in `.env` locally (`11_labs_api_key` or `ELEVENLABS_API_KEY`) and as a k8s secret in deploy — never commit it, never bake it into the image.

```
npm run audio -- my-morning
npm run audio -- my-morning --force-sentences   # regenerate sentences/story/grammar only; words are never recreated
npm run audio -- my-morning --words-only --dry-run
```
