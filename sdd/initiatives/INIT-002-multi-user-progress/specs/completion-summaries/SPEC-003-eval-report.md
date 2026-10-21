# SPEC-003 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/db.test.js test/db-repos.test.js` and read the schema, the served-root guard, and the backup path.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=required FAIL · domain=data FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS (11 pass, 0 fail)

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | `dataDir()` throws when `DATA_DIR` is inside or equal to `shared/`, `review/`, or `LESSONS_DIR`. The default `<root>/data` is allowed. Five tests passed. |
| ac-2 | PASS | Migration creates the ADR-0002 tables and records version 1. A second run does not apply it again. |
| ac-3 | PASS | `role='owner'` fails the CHECK. Deleting a user cascades to sessions, review items, events, lesson progress, and grants. |
| ac-4 | PASS | A `'; DROP TABLE users; --` value is stored literally and the `users` table remains. |
| ac-5 | PASS | Backup opens and `integrity_check` returns `ok`. The destination is under `DATA_DIR/backups/`. |
| ac-6 | PASS | `auth_provider` defaults to `local`. A repeated `(provider, subject)` pair is rejected. Several null subjects are allowed. |

No remediation loop.
