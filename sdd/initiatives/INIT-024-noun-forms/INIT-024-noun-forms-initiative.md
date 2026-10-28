# INIT-024: Noun and adjective forms on the live server

**Initiative ID:** INIT-024 · **Status:** on_track · **Created:** 2026-10-28 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** INIT-023/SPEC-009 stopped after the verbs. The live server has no `upsert_noun_forms`, and `generate_audio` there has no `lemmaId`.

This plan is written in the current checkout. The noun tool and the word page exist only as uncommitted work here. A new worktree from the last commit would not contain them.

## What you get

Open a noun or an adjective you have already met and see its forms, each with a clip. The live site can store those forms only after the build that already contains the tool is published. This initiative does not rewrite that tool.

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-service-publish-noun-tools.md) | service | Publish the build that stores and shows noun forms | — | 2h | in_progress |
| [SPEC-002](specs/SPEC-002-content-noun-and-adjective-forms.md) | content | Store the forms for the words already met | SPEC-001 | 2h | planned |
| [SPEC-003](specs/SPEC-003-content-form-audio.md) | content | Voice the new forms and check the word page | SPEC-002 | 1h | planned |

**Total:** 5h. SPEC-002 and SPEC-003 run only after the live server has the tool. Audio waits until the pointing table in SPEC-002 is accepted.

## Depends on / unblocks

- Depends on the uncommitted INIT-023 tree: migration 019, `upsert_noun_forms`, `lookup` returning forms, `generate_audio` with `lemmaId`, and `/words/{id}` for nouns and adjectives. Those specs are complete in the working tree and are not on https://hebrew.ibhacked.us.
- Verb forms from INIT-023/SPEC-009 are already in the live lexicon. Publishing the build shows them on the verb page. This initiative does not write them again.
- INIT-023/SPEC-009 stays blocked for the noun half. That half is this initiative.
- The one-time listen replay stays out. It still needs a production backup before anyone runs it.

## Done when

- The live MCP lists `upsert_noun_forms`, and `lookup` of a stored noun returns its forms with `hasAudio`.
- The twelve words below are handled as classified. Nouns and adjectives have their forms. הַכּוֹל and יָשָׁר do not get a paradigm.
- Each new form plays in the male voice.
- `/words/{id}` on the live site shows the noun grid or the adjective agreement, and a cell plays.
