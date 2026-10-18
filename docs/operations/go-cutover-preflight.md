# Go cutover preflight

Read-only checks for a restored backup before the Go server cutover ([ADR-0016](../decisions/ADR-0016-go-server-port.md), [INIT-014](../../sdd/initiatives/INIT-014-go-server-port/INIT-014-go-server-port-initiative.md)). The command is `go/cmd/preflight`. `scripts/rollout-preflight.js` is the lesson-plan rollout check. It does not open a restored hebrew-next backup, so this command is separate.

The process opens `$DATA_DIR/hebrew.sqlite` with the read-only helper in `go/internal/db` (`mode=ro`). It does not migrate and it does not write the database or lesson files. A finding is a report. The data fix is a later decision.

Wiring this into the hebrew-next restore step is a CI/CD follow-up. This change does not edit workflows, the Deployment, or deploy config.

## Command

Paths are the ones the server already uses (`go/internal/config`, `go/internal/paths`). On hebrew-next those are the restored volumes from ADR-0016: `DATA_DIR=/data/app` and `LESSONS_DIR=/data/worksheets`. The image build compiles every `go/cmd/*` into `/usr/local/bin`, so a later image contains `preflight`. The running deployment is not changed here.

Against the hebrew-next restored backup, once that image is the one the pod runs:

```
DATA_DIR=/data/app LESSONS_DIR=/data/worksheets /usr/local/bin/preflight
```

JSON on stdout, human report on stderr:

```
DATA_DIR=/data/app LESSONS_DIR=/data/worksheets /usr/local/bin/preflight --json
```

From a checkout, against a copy of that backup, before the image contains the binary:

```
cd go
CGO_ENABLED=0 \
  DATA_DIR=/data/app \
  LESSONS_DIR=/data/worksheets \
  go run ./cmd/preflight
```

Exit 0 when every check passes. Exit 1 when a check fails or is not wired yet. Exit 2 on a usage error. There is no warn level: a check that did not run must not look like a pass.

## Checks

1. **Ledger.** `schema_migrations` is exactly versions 1–11, using `go/internal/db` `CheckLedger`. This check runs now.
2. **Lexicon keys.** Every `lexicon_entries.key` and `lexicon_entries.lemma_key` must satisfy `exactKey(stored) == stored`. `exactKey` is Node's NFC, then ECMAScript `\s` runs collapsed to one space, then trim (`server/vocab-index.js`). Two stored `key` values that share an `exactKey` are a collision even when one of them already equals that `exactKey`. The report gives counts and up to 20 examples per class, with row ids and `U+XXXX` code points. Classes, first match wins: non-ASCII whitespace (NBSP and other non-ASCII `\s`), leading/trailing whitespace, internal multi-whitespace, NFC-only, other.
3. **Lessons.** Every `$LESSONS_DIR/<name>/worksheet.js` round-trips byte-identical through `go/internal/lessondata`. Directories named `.` or `_` are skipped, the same split as `server/vocab-index.js` `listLessonNames`.

Checks 2 and 3 are behind the `Check` interface in `go/cmd/preflight`. `go/internal/textkey` is PR #37 (`cursor/learner-path-readonly-c123`). `go/internal/lessondata` is PR #25 (`cursor/go-2-lessondata-parser-97f7`). Neither package is on main, and this branch does not copy them. Until a rebase wires `productionDeps`, those two checks report `pending` and the process exits 1. The ledger result is still printed. `pending` is not a data finding.

`shared/ui/plan-render.js` `safeLexiconKey` is unchanged. It does not collapse internal whitespace, which is why a stored key such as `א  ב` would miss an `exactKey` lookup. This command only reports that.
