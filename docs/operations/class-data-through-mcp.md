# Loading class data through MCP

The class spreadsheet stays on the operator's machine. Nothing from it is committed to this repo. It reaches the course through the MCP server, one table at a time, and only once the backfill is under way.

The sample so far is four classes in ten tables. Before the first load, list those tables and match each one to a row below. A table that matches no row is not loaded.

## When loading may start

Loading waits for the operator's decision that the backfill is under way. Record it here when it is made.

- Backfill under way: not yet. Decided by: — Date: —

Until that line is filled in, load nothing.

## Which tool loads which table

Load in this order. A later load depends on the earlier ones.

| Class table | Scope | Tool | State |
| ----------- | ----- | ---- | ----- |
| Words, with root and verb pattern | shared | `upsert_lexemes` (`root`, `binyan`) | ready (INIT-016/SPEC-003) |
| Grammar patterns, each with what must come first | shared | patterns tool | not loaded yet — INIT-017/SPEC-002 |
| Explainers, one per pattern | shared | explainers tool | not loaded yet — INIT-017/SPEC-002 |
| Class order | shared | `put_unit` positions, then `check_path_order` | not loaded yet — the path check is INIT-017/SPEC-002 |
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
