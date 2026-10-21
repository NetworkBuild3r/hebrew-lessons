# SPEC-022 completion summary: One authoring skill for every AI

- **Status:** complete. Merged into local `v3` as `344d27b`. Not pushed. No code path changes: docs, one skill file, tests.
- **Worker:** general-purpose sub-agent in worktree `Hebrew-v3-s022`. No personas, timer or trace scripts exist in this harness, so none were used.

## What shipped

- `internal/mcp/resources/authoring.md`, served as `hebrew://authoring` (313 lines): the eight steps in order, the unit JSON shape, every rule id with how to fix it, every activity type, and two worked examples (a struggle-heavy brief and a new-words brief).
- `.cursor/skills/hebrew-lesson-author/SKILL.md` on `v3`: the same text, with front matter.
- `internal/mcp/resources_test.go`: eight tests.

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | `TestResourcesAuthoringNamesEveryTool` (order checked), `TestResourcesAuthoringNamesEveryRule`, `TestResourcesAuthoringNamesEveryActivityType` | pass |
| ac-2 | `TestResourcesAuthoringSkillMirror` compares the body after the front matter | pass |
| ac-3 | `TestResourcesAuthoringDryRun` follows only the text against a test server and publishes a unit that passes validation; `TestResourcesAuthoringExamplesValidate` and `TestResourcesAuthoringExamplesFailWhenBroken` (the examples fail when broken, so the check is not vacuous) | pass |

Security: `TestResourcesAuthoringHoldsNoSecret`.

The dry-run client is a scripted test client, not a real model in Cursor (the spec's aud-1 said an agent in Cursor). It proves the text is sufficient and consistent. It does not prove that a real model follows it well.

## Verified by the orchestrator

Full `go test ./...` on merged `v3` passes, and `gofmt`, `go vet` and the tokens check are clean. I did not repeat the worker's mutations.

## Contradictions the worker found (reported, not changed)

- The brief's `activities` names are not unit types, so `schema.activity` blocks them if copied.
- The north-star says the validator needs a gloss and a transliteration. The code checks only the gloss for words outside the brief, and glosses live in the lexicon.
- `brief.new_intro_before_test` and the descriptions of `brief.coverage` and `brief.text_too_long` leave out dialogue.
- Prefix handling differs between the brief and the lexeme rules.
- Step 6 also needs `learnerId` for admin callers.

Treat these as follow-ups for the SPEC-020 owner. SPEC-023 should read the contradictions before it writes the coach prompt.

## For other specs

SPEC-023's weekly coach prompt points at `hebrew://authoring`. Any change to the resource must be mirrored into the skill file, or the mirror test fails.