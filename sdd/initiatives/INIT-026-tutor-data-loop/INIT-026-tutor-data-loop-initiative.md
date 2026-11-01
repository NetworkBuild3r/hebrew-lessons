# INIT-026: Each learner's day, and data and tools for an outside tutor

**Initiative ID:** INIT-026 · **Status:** complete · **Created:** 2026-11-01 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `tutor-data-loop` (already created)
**Source:** Brian's 2026-11-01 questions on progress reporting and on a Grok tutor. His answers that day were: one zone per learner, the same rule for everyone; his own account may write for himself; the app knows nothing about recordings, transcripts, or worksheets; scope is data and MCP. Evidence from [RSCH-003](../../research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md): spacing (High), retry-prompt feedback (Moderate), and coverage-graded input (Moderate, measured on reading). Write-and-restore shape from [RSCH-009](../../research/RSCH-009-worksheet-builder-design/RSCH-009-worksheet-builder-design-report.md), as revised by D1 to D6.

Cited lines are `origin/main` at `490eef015cf2a3eed16ca515d273a6e27c38dab7`. This worktree's `HEAD` is that commit. INIT-025 sits on branch `one-recall-card` and is not on `main`. That is why this initiative is INIT-026, although `main` stops at INIT-024. The two touch different files.

`.claude/agents/` is absent, so no spec has a persona, and nothing was manufactured. Specs execute inline, as INIT-023 and INIT-025 did. The repo has no `scripts/dev/next_adr_number.py`. ADR-0022 was checked free on every local and `origin` branch on 2026-11-01. The `software` profile has no `mcp` domain, so MCP tool specs use `api`. INIT-023 used `mcp` for the same layer.

## What we found

**Three different days.** Nothing uses the learner's own zone.

| Where | Day it uses | Line |
| --- | --- | --- |
| Daily snapshots, weekly reports | `TZ`, default `America/Chicago`. The live deploy sets no `TZ`. | `cmd/hebrew/jobs.go:25`, `internal/jobs/daily.go:134` |
| Review study day | `America/Denver`, 4:00 rollover, hard-coded | `internal/web/studyday.go:19` |
| `get_progress` | UTC. The description says so. | `internal/mcp/tool_progress.go:16`, `:83` |
| Progress page, Lessons slip list | UTC | `internal/web/progress.go:188`, `:479`, `internal/web/shell.go:554` |
| Lesson brief | UTC | `internal/brief/build.go:364` |

On an evening in Denver, `get_progress` and the Progress page call it tomorrow. The snapshot for that evening is filed under Chicago's day.

**The LLM sees less than the app knows.**
- `get_progress` returns each weekly report's summary only (`internal/mcpstore/learner_model.go:163`). The stored payload has gained and lost words with causes, cleared words, retention, minutes, and band slips. A test pins the payload as hidden (`internal/mcp/learner_tools_sql_test.go:311`).
- `get_learner_model` gives a word's cause, such as `confusion`, but not which pair. `learner.DominantPair` (`internal/learner/diagnose.go:120`) already computes it.
- There is no single call that answers "what should we work on today".

**Outside teaching has no way in.**
- A word a tutor taught gets no card until a lesson teaches it.
- `record_mistakes` stores a correction, but not the day it happened.
- `add_coach_note` is admin only (`internal/mcp/policy.go:23`).

## What you get

- Every day the app computes is your day, in your zone. Every MCP answer that names a day names the zone too.
- Grok can read one digest. It holds:
  - skill counts now and a week ago
  - retention against the 85–90% band
  - minutes in the app
  - struggle words with cause and confusion pair
  - corrections, each with whether you have got it right in review since
  - words taught in the last two weeks and their cards
  - coach notes and last week's report
- After a session, Grok sends the words taught and the corrections, each with the day. Taught words are in your review deck the next study day. Grok can undo a wrong batch of taught words.
- You or Grok can leave yourself a coach note.
- The app stores no recordings, transcripts, worksheet files, file links, tutor names, or session records. Grok keeps all of that.

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-docs-learner-day-and-outside-teaching.md) | docs | ADR-0022: each learner's day, and outside teaching that schedules but never grades | — | 1h | complete |
| [SPEC-002](specs/SPEC-002-data-learner-zone-and-taught-words.md) | data | Migration 020: learner zone, taught words, dated corrections, note author role | — | 2h | complete |
| [SPEC-003](specs/SPEC-003-service-learner-day.md) | service | One learner-day rule, used by the lesson brief | SPEC-002 | 3h | complete |
| [SPEC-004](specs/SPEC-004-scheduled-jobs-on-learner-day.md) | scheduled | Snapshots and weekly reports on each learner's day | SPEC-003 | 4h | complete |
| [SPEC-005](specs/SPEC-005-service-taught-words-and-dated-corrections.md) | service | Taught words schedule cards; corrections carry their day | SPEC-001, SPEC-003 | 4h | complete |
| [SPEC-006](specs/SPEC-006-frontend-pages-on-learner-day.md) | frontend | Progress, Lessons, and review compute the learner's day | SPEC-003 | 2h | complete |
| [SPEC-007](specs/SPEC-007-api-zone-payload-confusion.md) | api | `set_time_zone`, zone and weekly payload in `get_progress`, confusion pair in `get_learner_model` | SPEC-003 | 3h | complete |
| [SPEC-008](specs/SPEC-008-api-tutor-writes.md) | api | Tutor writes: taught words, dated corrections, your own coach notes | SPEC-005, SPEC-007 | 4h | complete |
| [SPEC-009](specs/SPEC-009-api-study-digest.md) | api | `get_study_digest` and the `hebrew://tutor` resource | SPEC-004, SPEC-008 | 4h | complete |

**Total:** 27h. **Critical path:** SPEC-002 → SPEC-003 → SPEC-005 → SPEC-008 → SPEC-009 = 17h. Realistic ×1.3 ≈ 22h, about 4 working days.

**Order.**
1. SPEC-001 and SPEC-002 can start together.
2. After SPEC-003, four specs can run side by side: SPEC-004 (`internal/jobs`), SPEC-005 (`internal/evidence`, `internal/db`), SPEC-006 (`internal/web`), and SPEC-007 (`internal/mcp`, `internal/mcpstore`).
3. SPEC-007, SPEC-008, and SPEC-009 share `internal/mcp/policy.go` and `learner_data.go`, so they run in that order.

## Decisions to confirm

- **A learner with no zone uses the app zone.** That is `TZ`, today `America/Chicago`. Every output says which was used (`zoneSource: learner | app`). It is never UTC unless someone sets UTC. The alternative is to skip that learner's snapshots until a zone is set.
- **The zone is set over MCP only.** `set_time_zone` takes an IANA name. The web app does not read the browser zone, which keeps this initiative to data and MCP. Your zone needs one call after deploy, for example `America/Denver`.
- **Two day boundaries, both in your zone.** Snapshots, weekly reports, `on` dates, and `get_progress` use calendar days that start at midnight. The review study day keeps its 4:00 rollover.
- **Days already stored stay as written.** Old snapshots carry Chicago days. When your zone changes, one day can be skipped or repeated at the seam. Nothing is rewritten.
- **Taught words schedule but never grade.**
  - A taught word gets `new` read and recall cards, plus listen when it has a clip. They are due at the next 4:00 in your zone.
  - A card that already exists is untouched.
  - Nothing outside the app moves a card's stability, difficulty, or due date. That keeps North Star §4.2, "every answer is an event", to in-app answers.
- **Learner coach notes revise an INIT-015 rule.**
  - INIT-015/SPEC-021 kept notes admin only, because a note is read into the next lesson's prompt (`policy.go:20`).
  - Here a learner may write notes about themselves.
  - Those notes are stored with `author_role = learner`, shown as quoted learner text in the brief, and read only into that learner's own brief. A learner gets at most 20 notes in 24 hours.
- **SPEC-006 touches pages only in which day they compute.** No layout or wording changes. Drop it if the pages should stay untouched in this initiative. The page's UTC day would then stay wrong.
- **Corrections are not undone.** A correction row merges repeats of the same pair (`times`), so it cannot be cleanly removed. Taught words can be removed. A wrong correction stays until an admin removes it.

## Threat model

- **Trust boundary.** The MCP session, OAuth or bearer, sits in front of every new tool.
  - A learner's id comes from the principal, never from the request body (`learnerOwnDataTools`).
  - `MCP_READONLY_TOKEN` may call no write tool (`learnerWriteTools`).
  - An admin may name any learner.
- **Data.** Only one learner's own learning data: words, cards, corrections, and notes. The app stores no third-party speech.
  - Free-text note fields could still hold a name or a quote. Tool descriptions say not to send them.
  - Length caps apply: a note is at most 500 characters and a mistake note at most 300.
- **Prompt injection.** Coach notes and mistake notes flow into AI prompts through the brief and the digest. Learner-written text is labelled, quoted, and never read into another learner's brief.
- **Abuse and integrity.**
  - Batches hold at most 50 items.
  - An `on` date may not be after the learner's today or more than 30 days before it.
  - A zone must load as IANA and be at most 64 characters.
  - Taught words create only `new` cards. Undo removes only cards that the same taught row created and that have no review.
- **Output.** Text read back over MCP is JSON. Anything a page renders stays escaped by templ.

## Depends on / unblocks

- Builds on the evidence package (INIT-015/SPEC-005), the weekly report (INIT-015/SPEC-023), and the brief's `fixMistakes` (`internal/brief/fix_test.go`).
- Unblocks Grok building each conversation from the digest and drilling open corrections with a retry prompt.
- Out of scope:
  - session records, minutes outside the app, recordings, transcripts, worksheet files and links
  - per-pattern grammar mastery
  - a text-coverage goal
  - any page feature beyond the day fix
  - in-app retry questions for corrections
  - outside answers grading a card

## Done when

- With your zone set to `America/Denver`, at 23:30 MDT on 2026-10-31, these all call the day 2026-10-31: `get_progress`, the Progress page, the Lessons slip list, the lesson brief, and the next snapshot. Each MCP answer says `zone: America/Denver`, `zoneSource: learner`.
- `get_progress` returns each of your weekly reports with its payload. Another learner's never appears.
- `get_learner_model` returns `confusedWith` for a word whose cause is a confusion pair.
- After `record_taught_words` for three words on today's date, each has `new` read and recall cards due at the next 4:00 in your zone. `remove_taught_words` takes them back while they have no review.
- `get_study_digest` returns every section above for you alone, under 16 KB, and counts a correction as right since once you answer that word right in review after its day.

## Plan quality

Completeness 8, accuracy 8, clarity 8, feasibility 8, guardrails 9. Confidence 0.82. Single-domain specs, acyclic graph, every spec has a Claim, a Check with its executor, and a Pass condition. Lowest score: feasibility. SPEC-004 changes the scheduler's wake rule, which carries the most existing tests.
