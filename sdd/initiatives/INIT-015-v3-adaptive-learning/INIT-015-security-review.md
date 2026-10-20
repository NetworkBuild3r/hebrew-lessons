# INIT-015 Initiative-wide Security Review (Phase 3.5, `INIT-015/SECURITY-REVIEW`)

**Date:** 2026-10-19 · **Reviewed tree:** local `v3` at `844f16d` (pushed to `origin/v3`) · **Baseline:** `origin/v3` at `9e58365` (the deployed line)

## Verdict

**No unresolved Critical or High finding. The review is partial, and that gap is recorded below, not accepted away.**

The `security-review` tool computes its diff against `origin/main`, reviews about 20 to 30 files per run, and cannot choose files. `v3` shares no history with `main`, so each pass ran in a throwaway worktree whose branch differed from `9e58365` only in the target paths. Five passes (a to e) plus the orchestrator's manual checks covered the line in pieces. No pass covered everything. The thorough, whole-diff review that the `complete-initiative` procedure describes was **not run**.

## Severity summary

| Severity | Found | Fixed | Open |
| --- | --- | --- | --- |
| Critical | 0 | 0 | 0 |
| High | 1 (conditional, see F-2) | 1 | 0 |
| Medium | 2 | 2 | 0 |
| Low | 1 | 1 | 0 |

## Findings and fixes

| ID | Severity | Where | Finding | Fix | Commit |
| --- | --- | --- | --- | --- | --- |
| F-1 | Low | `internal/mcp/tools.go` | `validate_unit` without a `briefId`, and `lookup`, sat outside the 60-per-minute per-caller limiter | Learner-role callers are counted on both; admin and the static token stay unlimited | `82cbf66` |
| F-2 | High if the read-only token had no stricter list; confirmed it did not | `internal/mcp/policy.go` | `MCP_READONLY_TOKEN` is a learner principal and shares the learner tool list, so it could call `unpublish_unit`, a write | `learnerWriteTools`; the `mcp-readonly` actor is denied them; `doc.go` corrected; test rows for `unpublish_unit`, `get_lesson_brief` and `add_coach_note` | `6037aa2` |
| F-3 | Medium | `internal/placement/service.go` | The three-swap cap on placement `Replace` lived only in the template. A client could post replace repeatedly to skip words until a familiar one appeared, then answer it right, which inflates the estimate | `Engine.Replace` refuses past `MaxReplaces` (3) per item; a web test keeps the screen constant equal | `6037aa2` |
| F-4 | Medium | migrations | `origin/v3` shipped `007_oauth_client_issued.sql` while INIT-015 also used 007. A duplicate version stops the app. Found by the orchestrator, not a review pass | Renumbered INIT-015's 007 to 010 as 008 to 011 (D9); `TestMigrateFromTheDeployedV3Database` proves the upgrade from the deployed schema; mutation confirmed | merge `2ac89b9` |

Each fix has a test that fails without it (mutation-checked).

## Checked, no finding

- **CSRF:** every non-safe method requires a CSRF token and an `Origin`; `csrfExempt` is empty; `Origin: null` is refused; `Referrer-Policy: same-origin` keeps native form posts carrying their `Origin`.
- **IDOR:** placement runs and items, activity tokens, review cards, briefs, units and reports are all scoped by the session user. A foreign or missing brief returns the same `brief.not_found`.
- **D1 (no verdict before the review):** step JSON carries no correctness field, `CorrectIdx` is stripped, a replaced word is never scored, and answers are judged from the stored item, not the request.
- **XSS and open redirect:** templ escaping, a CSP without `unsafe-inline` for scripts, audio served only for `^[0-9a-f]{64}\.mp3$`, and `Href()` appending only `?lang=he` to fixed paths.
- **Tokens:** activity tokens are HMAC-SHA256 under a key derived from `HEBREW_SESSION_SECRET`, bound to the user, single-use, 30-minute lifetime. With no secret the key is random per process, so tokens are not forgeable but not portable across replicas. OAuth refuses to start without the secret.
- **Evidence:** the recorder is service-only; `UserID` comes from the session, never a form.
- **MCP authorization:** unlisted tools are admin-only by default; learner tools force the learner's own id; `get_lesson_brief`, `add_coach_note`, `put_unit`, `publish_unit`, `generate_audio` and `upsert_lexemes` are admin-only.
- **Auto-publish (D5):** a learner publish needs a `briefId` and passes `ValidateWithBrief` before `AutoPublishLearner`; `publish:true` on a shared unit is ignored; `publish_unit` refuses learner-scoped units; the brief claim is a conditional update inside one transaction, so a double spend fails.
- **Secrets:** TTS keys are never written to files or logs; audio manifests print counts only.

## Not reviewed (the coverage gap)

- `internal/mcpstore/*` and `internal/units/*`, including the `unit.no_markup` source. The only evidence is the SPEC-020 completion summary (a reflection test over every string field with full-width and `javascript:` normalization).
- Most web handlers, templates and JS: no route-by-route CSRF matrix was run; `shell.js` `location.assign` and the full templ pass were not read.
- `internal/auth` cookie flags and the logout implementation (only the CSRF tests were read).
- Whether `get_context` ever lists another learner's `learner:<id>` unit metadata.

These are backed by tests, not by review. A follow-up pass over those paths is the way to close the gap.

## Known and left alone

- `get_context` and `get_learner_words` are not rate limited (own-data reads).
- The limiter is in-process, so it is not shared across replicas.
- A deleted unit frees its brief for reuse by the same learner (same tenant).
- Brief coverage matches by prefix, which is lenient (integrity, not authorization).
- A reported shared unit still shows in the MCP outline until the web filter lands.
- A pending-token set is global with a 4096 cap, so one learner could evict others' pending review tokens (denial of service only).
- A prompt-injection note from a trusted admin can steer the authoring agent. Admin-only write is the boundary, by design.

## Open decision

The weekly coach automation needs a credential. A learner-role session cannot work (the brief and publish tools are admin-only). The options are an admin-role OAuth session (works today, broader than the spec wanted) or a server authorization change (needs its own review). The automation must stay off until one is chosen.
