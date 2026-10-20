# SPEC-019 Eval Report

**Spec:** SPEC-019
**Initiative:** INIT-006
**Verifier:** fresh sub-agent (`composer-2.5-fast`), read-only, did not author the files
**Date:** 2026-10-10
**Overall:** PASS

## Per-AC verdicts

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `mcp/tools.js` registers nine learner tools (`get_learner_profile`, `check_text_coverage`, `list_grammar_concepts`, `list_lexicon`, `get_lexicon_entry`, `upsert_lexicon_entries`, `add_library_words`, `set_lexicon_verified`, `set_ai_planning`). All nine appear in `.cursor/skills/hebrew-worksheet/SKILL.md`. The six-step loop is `SKILL.md` under "The six steps". |
| ac-2 | PASS | ADR-0009 §7 names six recipes. `.cursor/skills/hebrew-worksheet/recipes.md` has one `##` card for each, with a research-note column. `SKILL.md` repeats the block order in the recipe table. |
| ac-3 | PASS | `docs/operations/accounts-and-admin.md` points at the two ADR-0009 §1 tables and does not copy them. The disclosure sentence and the ADR-0009 §9 MCP rule match the ADR text. The unqualified sentence "The AI and MCP tools cannot read progress or answer history" is absent from the guide and from the skill. |
| ac-4 | PASS | Search of `SKILL.md`, `recipes.md`, and `accounts-and-admin.md` found no `_API_KEY=`, `MCP_AUTH_TOKEN=`, or `ADMIN_PASSWORD=`. Examples use `example-learner`. No class-sheet quotation was added. The pre-existing plan id `shai-date-indirect-speech` is a lesson slug, not a profile username. |

## Tests run

No spec test file. The verifier compared tool names, recipe headings, and the two ADR blockquotes, and searched for the forbidden assignment strings and the old privacy sentence.

## Remediation loops

0

## Not an AC

The verification strategy's human check (the operator reads the loop and accepts it) was not performed in this session. The gates section names the verification search as the blocking check. That search passed.
