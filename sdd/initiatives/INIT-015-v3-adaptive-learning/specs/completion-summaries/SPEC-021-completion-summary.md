# SPEC-021 completion summary: MCP tools for the learner model and briefs

- **Status:** complete. Merged into local `v3` as `fc7f7e6` (worker commit `751575e`, 22 files, no migration). Not pushed.
- **Worker:** general-purpose sub-agent in worktree `Hebrew-v3-s021`. No personas, timer or trace scripts exist in this harness, so none were used.
- **Security review:** `security_review: required`. **No security-review agent has run yet.** It is scheduled for the whole line after the remaining specs land, together with SPECs 015, 019, 020 and 026.

## What shipped

Five new tools and a resource, plus two extended ones. 13 tools and 4 resources in all.

| Tool | Caller |
| --- | --- |
| `get_learner_model` (per-word states, paged, limit clamped to 200) | learner (own data), admin |
| `get_progress` (snapshots, net change, regressions, retention, placements, weekly reports; range up to 366 days) | learner (own), admin |
| `get_lesson_brief` | admin / static token only |
| `validate_unit` (adds `briefId`, `learnerId`) | learner, admin |
| `add_coach_note` (1 to 500 characters, 100 a day per learner) | admin / static token only |
| `unpublish_unit` (learner-scoped units only) | learner (own), admin |
| `get_context` (adds `coachNotes`, `model`) | unchanged |

`hebrew://authoring` is a stub resource that SPEC-022 replaces.

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | `TestAuthzRowsForTheLearnerTools` (six tools by learner own, learner other, admin, static token, no token, wrong token), `TestLearnerForRules`, `TestEveryLearnerIDToolIsGuarded`, `TestLearnerIDSpellingsDoNotCrossLearners` | pass |
| ac-2 | `TestLearnerModelPagesAndClampsTheLimit`, `TestLearnerModelPagesOverSQLite` (454 real words, no repeats or gaps) | pass |
| ac-3 | `TestE2EBriefToToday`: brief, validate (blocks on missing audio), generate_audio with a fake TTS, validate (passes), put_unit publish, add_coach_note; the unit is in the learner's scope and absent from another learner's, and the brief is spent | pass |
| ac-4 | `TestUnpublishOverSQLite` (one audit row with actor, scope and revision; repeat writes nothing; others refused) | pass |
| ac-5 | `TestAuthoringResource` | pass |

The worker made 33 mutations (authorization, a missing `user_id` filter in each progress and brief query, unpublish scope, input bounds, error leakage, rate and note limits); all were caught and the source was restored. A single-layer break of the learner-id check is invisible over HTTP by design (the role middleware and `learnerFor` back each other up); structural and unit tests catch it.

## Verified by the orchestrator

- The worktree was clean before the merge (no mutation left behind). The merge was conflict-free.
- On merged `v3`: `gofmt` empty, `go vet` clean, tokens check exit 0, `templ generate` leaves the host tree clean, the full `go test ./... -count=1` passes in every package, and `-race` passes on mcp, mcpstore, app, brief and units.
- Read `learnerFor`: default-deny. A learner may only name their own id, an admin must name an existing positive id, any other role is refused.
- I did not repeat the worker's mutations.

## Deviations from the spec

- `get_lesson_brief` and `add_coach_note` are admin/static-token only. A brief is only usable through `put_unit` (already admin-only), and a coach note is read into the next lesson's prompt, so a learner must not write one.
- A learner naming another id is refused with an error (as `get_context` already does) and not silently redirected.
- `get_learner_words` keeps its shape and still reads FSRS cards. The model lives in `get_learner_model` and `get_context`.
- `get_progress`: "gained and lost" is the net change in known words per skill between the first and last snapshot (no per-word history table exists). Retention is the newest snapshot's mean per-skill retention against a fixed 0.85 target. Weekly reports return their summary only.
- Coach notes have no `publish_audit` action (adding one needs a migration). The `coach_notes` row (author, learner, time) is the record.
- A limiter of 60 calls a minute per caller was added (none existed). It is per process.
- The memory store does not implement auto-publish, because an existing test requires a store without it to keep a learner unit as a draft.
- No `contract/` golden files exist on `v3` (that folder is on `main`). Three assertions in `server_test.go` changed: tool count 8 to 13, the write-tool allow-list, and resource count 3 to 4.

## Risks and open items

- Each `get_learner_model` page and `get_context` summary evaluates the learner's whole evidence (about 90 ms at 50k attempts). The limiter bounds it; caching is a later option.
- Coach notes are free text written by one AI and read by another. They are admin-write only and stripped of control and bidi characters, but the content is not filtered.
- Reported-unit hiding on the web Today is still open; it belongs to SPEC-017.
- No tool writes `weekly_reports`. SPEC-023 must add a writer.

## For SPEC-022 and SPEC-023

- **SPEC-022:** replace `internal/mcp/resources/authoring.md`. Flow: brief, write the unit, `validate_unit` with `briefId` and `learnerId`, `generate_audio` for every Hebrew text and new word (dialogue lines carry a speaker), `put_unit` with `publish:true` and the same `briefId`, then `add_coach_note`. A brief builds one unit and expires in 7 days. Brief fields are camelCase. A missing or foreign brief returns the same `brief.not_found`.
- **SPEC-023:** use the admin or static token. Limits are 60 calls a minute and 100 notes a day per learner. `get_progress` reads the previous day's snapshots.
