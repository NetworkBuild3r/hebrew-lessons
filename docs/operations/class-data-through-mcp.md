# Loading class data through MCP

The class spreadsheet stays on the operator's machine. Nothing from it is committed to this repo. It reaches the course through the MCP server, one table at a time, and only once the backfill is under way.

The sample so far is four classes in ten tables. Before the first load, list those tables and match each one to a row below. A table that matches no row is not loaded.

## When loading may start

Loading waits for the operator's decision that the backfill is under way. Record it here when it is made.

- Backfill under way: not yet. Decided by: — Date: —

Until that line is filled in, load nothing.

The five starter explainers are original notes, not class rows: present tense agreement (`present-tense`), past tense with ti (`past-tense-ti`), going to plus a verb (`going-to`), efshar plus a verb (`efshar`), and prepositions with endings (`preposition-endings`). The app inserts them through the explainer store when none are stored. They name no person. Further explainers wait for the backfill and go through `upsert_explainer`.

## Which tool loads which table

Load in this order. A later load depends on the earlier ones.

| Class table | Scope | Tool | State |
| ----------- | ----- | ---- | ----- |
| Words, with root and verb pattern | shared | `upsert_lexemes` (`root`, `binyan`) | ready (INIT-016/SPEC-003) |
| Grammar patterns, each with what must come first | shared | `upsert_patterns` | ready (INIT-017/SPEC-002). Not loaded yet. |
| Explainers, one per pattern | shared | `upsert_explainer` | ready (INIT-017/SPEC-002). Five original starters are inserted when the table is empty. Class explainers are not loaded yet. |
| Class order | shared | `put_unit` positions, then `check_path_order` | ready (INIT-017/SPEC-002). Not loaded yet. |
| Phrases with an English speaking prompt | shared | `put_unit`, using the `say` step | not loaded yet — INIT-020/SPEC-002 |
| Verb forms by tense and person | shared | verb forms tool | not loaded yet — INIT-019/SPEC-002 |
| Mistakes, personal, with how often they return | one learner | `record_mistakes` | not loaded yet — INIT-019/SPEC-002 |
| Mistakes, general | shared | general mistakes tool | not loaded yet — INIT-019/SPEC-002 |

Lexicon first, then patterns, then units. Verb forms and mistakes can follow in either order once their tools exist.

## Shared and personal

- The shared course gets words, patterns, explainers, the path, phrases, verb forms, and the general mistakes list.
- A personal mistakes row goes only to the learner it belongs to. The learner id comes from the token. An admin names the learner explicitly.
- Nothing is copied from a personal row into a shared table.

## Checks before a row is written

Run these on every row. A row that fails one is skipped and listed, not written.

1. A shared row contains no person's name, no transcript, and no verbatim answer.
2. A general mistakes row states a pattern, such as what learners often say, and carries no one's actual response.
3. A personal mistakes row names exactly one learner, and that learner is the one the call is for.
4. A root is 2 to 4 Hebrew consonants and a binyan is one of `paal`, `nifal`, `piel`, `pual`, `hifil`, `hufal`, `hitpael`. `upsert_lexemes` rejects anything else and writes nothing.

## Dry run and check after

- Before any `put_unit`, call `validate_unit` with the same document. Write only when it returns ok.
- After a words load, call `lookup` for a sample of the rows and confirm the root and binyan came back.
- After a personal load, call `get_learner_model` for that learner and confirm the change is theirs alone.
