# INIT-038/SPEC-001 completion summary: ADR-0033 and the pass and fail lines

**Status:** complete, 2026-11-09. **Executor:** inline agent; no commit was made.

## What changed

| File | Change |
| --- | --- |
| `docs/decisions/ADR-0033-stage-1-trust-and-measurement.md` | New. D1 to D13, and every exit, gate, success, guardrail, and abandon line. Accepted 2026-11-09. |
| `docs/operations/local-check.md` | New. The one-command local Docker check. |
| `sdd/complaints.md` | New. The weekly complaints list and its 24-hour rule. |
| `.cursor/skills/hebrew-worksheet/` | Removed. It told agents to call `create_plan`, which no code defines. |
| `.cursor/skills/hebrew-lesson-author/SKILL.md`, `.cursor/skills/hebrew-lesson-reviser/SKILL.md` | A paused notice is the first line of each front-matter description. |
| `docs/operations/accounts-and-admin.md` | The link to the removed skill's recipe now says the skill was removed, and why. |

## Brian's confirmation (2026-11-09)

- **The lines.** He confirmed them as written, including the ones set in this ADR.
- **The wider reading of D3.** Blanks, `order` steps, and multi-word `listen_pick` steps in AI-written units become ungraded.
- **The "Say it from English" row** is renamed **"Hebrew from English"**. Brian read the old name as a pronunciation check. The row is the `recall` skill (English shown, Hebrew typed or picked), and pronunciation is never checked. ADR-0033 D6, INIT-038, SPEC-010, and SPEC-014 use the new name. The BRAIN-001 records keep the old name, as written at the time.

## Local check, run once

The full suite was run on 2026-11-09 with the documented command and `PKGS` unset. It passed in 308s:
- `gofmt` listed nothing;
- `templ generate` (v0.3.960) changed no `*_templ.go`;
- `go vet` was clean;
- every package's tests passed, the longest being `internal/web` (211s), `internal/placement` (180s), `internal/db` (67s), and `internal/session` (65s).

After the skill edits, it was rerun with `PKGS='./internal/mcp/... ./internal/units/...'` and passed. `TestKeyboardNode` and `TestClipPlayAfterSwap` print `--- SKIP` in `golang:1.26`, as the doc says.

## Checks

- **Rule ids.** All ten D9 correctness rule ids, and the excluded ids it names, are constants in `internal/units/rules.go`.
- **Generated files.** `git status` shows no generated-file drift after the `templ` run.
- **Skill references.** No live doc or code references `.cursor/skills/hebrew-worksheet/`. ADR-0003 line 491 still names it; Accepted ADRs are not edited in place.

## Disclosures

- **ADR number.** `next_adr_number.py` is not in this kit, so ADR-0033 was taken by hand and there was no reservation to release.
- **Self-checked.** The mechanized checks above were run by the agent that wrote the spec's files. That is a self-check, labelled as such per ADR-0033 D13. Brian confirmed the lines himself.
