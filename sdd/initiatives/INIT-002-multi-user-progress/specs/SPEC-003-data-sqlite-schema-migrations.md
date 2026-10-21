# SPEC-003 · Data: SQLite database, schema, migrations, backup

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-002
title: SQLite database, schema, migrations, backup
domain: data
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []   # becomes [dependency_add] only if ADR-0002 aud-5 picks better-sqlite3
reversibility_class: revertible
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-005, SPEC-006, SPEC-011]
```

---

## Description

**Context:** the server has no persistent state besides lesson files. ADR-0002 picks the engine (default `node:sqlite`, confirmed present in the container's Node 22.23.3) and the schema outline. This spec builds the one module every later service uses.

**Scope (in):**
- `server/paths.js`: `dataDir()`. It reads the `DATA_DIR` env, defaulting to `<root>/data`. It throws at startup if the resolved path is inside `rootDir()/shared`, `rootDir()/review`, `lessonsDir()`, or any other served root, or equal to one of them.
- `server/db/index.js`: the single wrapper around the engine.
  - **Setup:** `open()` turns on WAL, `foreign_keys=ON`, and `busy_timeout`.
  - **Interface:** `prepare`, `get`, `all`, `run`, `transaction(fn)`, and `close`.
  - **Migrations:** `migrate()` applies numbered migrations from `server/db/migrations/` once each, recorded in `schema_migrations`.
  - **Startup:** the engine's experimental warning is emitted once and not repeated.
- `server/db/migrations/001-initial.js` creates every table in the ADR-0002 schema outline:
  - `users`, `sessions`, `review_items`, `practice_events`, `lesson_progress`, `lesson_visibility`, `lesson_grants`, `audit_log`.
  - It also sets the CHECK constraints (`role IN ('user','admin')`, `status IN ('active','disabled')`, `direction IN ('he_en','en_he')`), foreign keys with `ON DELETE CASCADE` for per-user rows, and the indexes the metrics queries need: (`user_id`, `at`) on events, and (`user_id`, `due`) on review items.
  - `users` columns include `auth_provider` (default `local`) and `provider_subject` (REQ-015).
- `server/db/repos/*.js`: thin repository functions per table, with prepared statements only and no string-built SQL.
- `scripts/db.js` with `npm run db:migrate`, `npm run db:backup` (`VACUUM INTO data/backups/hebrew-<timestamp>.sqlite`), and `npm run db:check` (`PRAGMA integrity_check`).

**Scope (out):** password and session logic (SPEC-004); progress rules (SPEC-005); HTTP (SPEC-008).

---

## Understood (agent's read of the intent)

- [ ] und-1: The database must never be reachable over HTTP. A `DATA_DIR` inside a served folder is a startup error, not a warning.
- [ ] und-2: The first migration runs against an empty file, so it is revertible by deleting `data/`. Any later migration on a populated database is a `db_migration` gate.

## Acceptance Criteria

- [ ] ac-1 (REQ-013): with `DATA_DIR` set inside `LESSONS_DIR` or `shared/`, server startup throws a clear error. The default `<root>/data` starts.
- [ ] ac-2: `migrate()` on an empty file creates every ADR-0002 table and records version 1. A second run is a no-op.
- [ ] ac-3: the CHECK and foreign-key constraints hold. Inserting `role='owner'` fails, and deleting a user cascades to that user's sessions, review items, events, progress, and grants.
- [ ] ac-4 (REQ-003): no repository function builds SQL by concatenating a caller value. A test runs each repo with a quote and `; DROP TABLE` style input and finds the data stored literally.
- [ ] ac-5 (REQ-013): `db:backup` writes a consistent copy under `data/backups/` that opens and passes `integrity_check`.
- [ ] ac-6 (REQ-015): `users.auth_provider` defaults to `local`, and (`auth_provider`, `provider_subject`) is unique when `provider_subject` is not null.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Migration format | JS modules exporting `up(db)` (no down; revert = restore backup), numbered `NNN-name.js` | resolved | yes — plan default |
| aud-2 | Timestamps | ISO-8601 UTC strings, matching ADR-0001's `due`/`lastAt` | resolved | yes — plan default |

---

## Deliverables

- [ ] `server/paths.js` (`dataDir()` + served-root guard)
- [ ] `server/db/index.js`, `server/db/migrations/001-initial.js`, `server/db/repos/{users,sessions,review,events,lesson-progress,visibility,audit}.js`
- [ ] `scripts/db.js`; `package.json` scripts `db:migrate`, `db:backup`, `db:check`
- [ ] `.gitignore` and `.dockerignore`: `data/`
- [ ] Tests: `test/db.test.js`, `test/db-repos.test.js` (temp `DATA_DIR` per test)

---

## Technical Requirements (domain-specific)

- [ ] CommonJS, matching `server/*.js`. No new dependency unless ADR-0002 aud-5 picks `better-sqlite3` (then the `dependency_add` gate applies).
- [ ] All engine calls stay inside `server/db/index.js`, so swapping engines touches one file.
- [ ] Usernames are stored lowercase, with a `UNIQUE` index.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none (new empty database), or `dependency_add` per aud-5.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the listed files and delete `data/`.

---

## Security

- **Sensitivity:** required. Triggers matched: PII storage, password-hash storage, secrets at rest, injection surface.
- **Review depth:** full. Security acceptance: ac-1, ac-4, and backups landing outside served roots.

---

## Verification Strategy

- **Claim:** the schema matches ADR-0002, its constraints hold, no query can be injected, and the database can never be served.
- **Check + executor:** `mechanized`. `node --test test/db.test.js test/db-repos.test.js` runs with a fresh temp `DATA_DIR`.
- **Pass condition:** all tests pass. The served-root guard test throws for each served root. The injection test reads back the literal input. The backup opens and returns `ok` from `integrity_check`.

---

## Integration Points

- Consumed by SPEC-004, SPEC-005, SPEC-006, SPEC-007, SPEC-010, and SPEC-011.
- **Provenance tag:** `INIT-002/SPEC-003`
