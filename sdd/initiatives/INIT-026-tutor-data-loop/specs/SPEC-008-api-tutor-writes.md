# SPEC-008 · api: Tutor writes, taught words, dated corrections, your own coach notes

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-026
title: "Tutor writes: taught words, dated corrections, your own coach notes"
domain: api
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-005, SPEC-007]
blocks: [SPEC-009]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- `record_mistakes` is learner-callable on the caller's own id. It has no day (`internal/mcp/tool_mistakes.go:105`).
- `add_coach_note` is admin only, because a note is read into the next lesson's prompt (`internal/mcp/policy.go:20`, `internal/mcp/tool_coach_note.go:13`). The brief carries at most 5 notes (`internal/brief/build.go:75`).
- `LexemeByText` resolves pointed NFC text to a lexeme (`internal/mcpstore/lookup.go:86`).

**Scope (in):**
- `record_taught_words {learnerId?, on?, words: [{lexemeId?} | {he?}]}`:
  - Takes 1 to 50 words.
  - `he` resolves through `LexemeByText` after NFC normalization. A word that matches no lexeme comes back in `unknown`, and nothing is created for it. Creating lexemes stays admin authoring.
  - `on` defaults to the learner's today in their zone.
  - Calls `Recorder.Taught` and returns per word `created`, `hadCard`, and `unknown`, plus `due` (RFC3339) and `zone`.
- `remove_taught_words {learnerId?, on, lexemeIds}` calls `Recorder.RemoveTaught` and returns `removed` and `kept`.
- `record_mistakes` gains optional `on`, defaulting to the learner's today, passed to `MistakeInput.On`.
- `add_coach_note` becomes learner-callable on the caller's own id:
  - A learner's note is stored with `author_role = learner`. An admin's note is stored with `admin`.
  - A learner gets at most 20 notes in 24 hours. The admin cap stays 100.
  - The brief marks each note with `authorRole`. It renders a learner's note as a quoted line that starts "The learner wrote:". It never reads one learner's note into another learner's brief, which is already true and gets a test.
  - Update the `policy.go` comment to cite ADR-0022 in place of the SPEC-021 sentence.
- Policy: all four tools go in `learnerTools`, `learnerWriteTools`, and `learnerOwnDataTools`.
- Each description ends with: send extracted words and corrections only; never send transcript text, recordings, or other people's names.

**Scope (out):** The digest (SPEC-009). Removing corrections. Creating lexemes.

## Acceptance Criteria

- [ ] ac-1: A learner sends `record_taught_words` with `on` omitted and three words: two by `he` and one unknown string. The two known words get `new` read and recall cards due at the next 4:00 in the learner's zone. The unknown string is listed in `unknown`, and nothing is written for it.
- [ ] ac-2: Security:
  - A learner naming another `learnerId` on any of the four tools gets `learnerId must be your own id.`, and nothing is written.
  - `MCP_READONLY_TOKEN` gets `forbidden: admin only` on all four.
- [ ] ac-3: These are tool errors with a sentence, and nothing is written: 51 words, `on` tomorrow in the learner's zone, or `on` 31 days back.
- [ ] ac-4: `remove_taught_words` after one card was reviewed reports it in `kept` and removes the rest.
- [ ] ac-5: `record_mistakes` with `on: "2026-10-28"` stores `last_on = 2026-10-28`. Without `on`, it stores the learner's today.
- [ ] ac-6: Coach notes:
  - A learner's `add_coach_note` stores `author_role = learner`.
  - The 21st learner note in 24 hours is refused.
  - That learner's brief shows the note as quoted learner text with `authorRole: learner`.
  - Learner B's brief does not contain it.
- [ ] ac-7: A note body of `<script>alert(1)</script>` is stored as text, and a page that renders it shows it escaped.
- [ ] ac-8: `TestToolsResourcesAndIdempotency` and `TestResourcesAuthoringNamesEveryTool` include the two new tools.

## Deliverables

- [ ] `internal/mcp/tool_taught_words.go` and test
- [ ] `internal/mcp/tool_mistakes.go`, `tool_coach_note.go`, `policy.go`, `tools.go`, and tests
- [ ] `internal/mcpstore` taught-word and note-role calls
- [ ] `internal/db/progress.go` (note role and learner cap)
- [ ] `internal/brief/build.go` (note role in the brief) and test
- [ ] `internal/mcp/resources` authoring text

## Verification Strategy

- **Claim:** the tutor can write taught words, dated corrections, and the learner's own notes, for that learner only. Learner-written text reaches prompts only as labelled, quoted text in that learner's own brief.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 -run 'TestTaught|TestMistake|TestCoachNote|TestToolsResources|TestResourcesAuthoringNamesEveryTool' ./internal/mcp ./internal/mcpstore` and `/tmp/go/bin/go test -count=1 -run 'TestBrief|TestAC4' ./internal/brief`.
- **Pass condition:** ac-1 to ac-8 pass. Moving `add_coach_note` out of `learnerOwnDataTools` fails ac-2. Dropping the learner cap fails ac-6.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-008`
- Implements ADR-0022 decisions 2 and 4 (SPEC-001).
- Admin tokens keep every current behavior. The static `MCP_AUTH_TOKEN` stays admin, and the audit actor stays `mcp`.
