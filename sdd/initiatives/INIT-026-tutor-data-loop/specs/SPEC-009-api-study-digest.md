# SPEC-009 · api: `get_study_digest` and the `hebrew://tutor` resource

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-026
title: "get_study_digest and the hebrew://tutor resource"
domain: api
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-004, SPEC-008]
blocks: []
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:** To plan a conversation today, an LLM calls `get_learner_model`, `get_progress`, and `get_learner_words`, then does its own arithmetic. `get_lesson_brief` is admin only. The numbers exist already:
- snapshots per skill and day
- retention against 85–90%
- `activeMinutes` (`internal/jobs/weekly.go:530`)
- struggle words with cause
- corrections, taught words, coach notes, and the weekly payload, after SPEC-005, SPEC-007, and SPEC-008

**Scope (in):**
- `get_study_digest {learnerId?}`, read only, in `learnerTools` and `learnerOwnDataTools`. It returns:
  - `zone`, `zoneSource`, `today`, and `nextCutoff`
  - `skills`: per skill, the known plus solid count today and 7 days ago, and the struggling count today, from snapshots
  - `retention`: per skill, the last 7 days against the 85–90% band
  - `minutesInApp`: the last 7 learner days, from attempts with the weekly job's idle-gap rule. Export the rule from `internal/jobs` so there is one copy.
  - `struggle`: up to 10 words with `text`, `gloss`, `cause`, `confusedWith`, and `lastSeen`
  - `corrections`: up to 20 from the last 30 learner days, by `last_on`. Each has `wrong`, `right`, `note`, `on`, `times`, the word when linked, and `status`.
    - `status` is `right_since` with a count once there is an in-app correct recall or spell attempt on the linked word after the end of `last_on` in the learner's zone. Otherwise it is `open`. A correction with no linked word is `open`.
    - Plus `closedLoop {since, total, rightSince}` over the same rows.
  - `taught`: up to 30 from the last 14 learner days, with `text`, `on`, each card's state, and reviews since
  - `due`: due now, and due by `nextCutoff`
  - `coachNotes`: the newest 5, with `authorRole`
  - `lastWeek`: the newest weekly payload
- Every number comes from app code. The description says: do not recompute these numbers.
- `hebrew://tutor` resource (Markdown), served like `hebrew://rules`. It tells a tutor LLM:
  1. Read the digest before a conversation.
  2. Keep its Hebrew to words the learner knows plus a few from `struggle` and `taught`. The 95–98% known-word figure in RSCH-003 was measured on reading, so treat it as a target, not a proof.
  3. For an `open` correction, ask the learner to try again, and do not just say the right form (RSCH-003, Moderate).
  4. After a session, call `record_taught_words` and `record_mistakes` with `on`.
  5. Never send transcript text, recordings, or other people's names.

**Scope (out):** Any page. Session records, outside minutes, and transcripts.

## Acceptance Criteria

- [ ] ac-1: On a fixture with one Denver learner, the digest returns every section above, and each count matches a direct store query in the test. The fixture holds snapshots on 2026-10-24 and 2026-10-31, three struggle words (one with a pair), two corrections (one answered right in review after its day), and two taught words.
- [ ] ac-2: The correction answered right in review after its `last_on` is `right_since` with count 1. A right answer earlier on that same learner day does not count. `closedLoop` is `{total: 2, rightSince: 1}`.
- [ ] ac-3: Learner B's words, notes, corrections, and payload never appear in learner A's digest (an isolation test in the style of `learner_tools_sql_test.go:369`). Learner A naming B gets `learnerId must be your own id.`
- [ ] ac-4: With every list at its cap, the JSON is at most 16 KB.
- [ ] ac-5: `minutesInApp` equals the weekly job's minutes for the same attempts and days, using one shared function.
- [ ] ac-6: `hebrew://tutor` is listed by the resource test. It names `get_study_digest`, `record_taught_words`, `record_mistakes`, and `remove_taught_words`, and holds no secret (`TestResourcesAuthoringHoldsNoSecret` style).

## Deliverables

- [ ] `internal/mcp/tool_study_digest.go` and test
- [ ] `internal/mcpstore/digest.go` and test
- [ ] `internal/jobs/weekly.go` (export the minutes rule)
- [ ] `internal/mcp/policy.go`, `tools.go`, `server.go` (resource), `resources/tutor.md` and test

## Verification Strategy

- **Claim:** one call gives a tutor LLM the learner's current state with app-computed numbers, for that learner only, within a fixed size.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 -run 'TestStudyDigest|TestTutorResource|TestToolsResources|TestResourcesAuthoringNamesEveryTool' ./internal/mcp ./internal/mcpstore` and `/tmp/go/bin/go test -count=1 -run 'TestActiveMinutes|TestWeekly' ./internal/jobs`. SQLite-backed fixtures, the pattern in `learner_tools_sql_test.go`.
- **Pass condition:** ac-1 to ac-6 pass. Counting a right answer from earlier on the correction's own day fails ac-2. A second copy of the minutes rule fails ac-5, because the test calls both paths.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-009`
- After deploy, a live smoke over OAuth as learner 1: call `set_time_zone`, then `get_study_digest`, and record the zone and section counts in the completion summary. This is read-only apart from setting the zone.
