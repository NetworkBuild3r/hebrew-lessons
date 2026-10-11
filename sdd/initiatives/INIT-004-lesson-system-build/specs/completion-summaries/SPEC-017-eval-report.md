# SPEC-017 AC Verification Eval Report — Media volume, backup, and the generation secret

**Traceability:** INIT-004/SPEC-017 \
**Verifier:** independent readonly sub-agent [Verify SPEC-017 criteria](7d7654ea-ef5e-4f39-94ee-cd0b323bce5e) · model `composer-2.5-fast` · temperature 0 \
**Evaluated at:** 2026-10-09T20:10:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Backup file list includes `hebrew.sqlite`, `media/`, and `media/_archive/`. An `.mp3` outside `DATA_DIR` is not required and is not copied. | PASS | `node --test test/media-backup.test.js` → 6 pass, 0 fail, exit 0. Tests `copies hebrew.sqlite, media/, and media/_archive/…` and `the backup command lists hebrew.sqlite, media/, and media/_archive/`. `server/db/index.js` `backup()` / `copyMediaTree`; `scripts/db.js` prints the three names. Source `.mp3` unchanged. |
| ac-2 | Deployment keeps `replicas: 1` and `Recreate`, and names `OPENAI_API_KEY` only as `secretKeyRef`. | PASS | Same suite, test `keeps replicas 1, Recreate, and OPENAI_API_KEY as a secret reference only`. `deploy/k8s-database.md` `replicas: 1`, `type: Recreate`, `fsGroup: 1000`, `runAsUser: 1000`, and `OPENAI_API_KEY` via `valueFrom.secretKeyRef` name `hebrew-secrets` with no `value:` in that block. |
| ac-3 | `npm run audio:verify` passes. This spec does not write under `LESSONS_DIR`. | PASS | Verifier re-ran `npm run audio:verify` → exit 0, `65 recordings accounted for`. No worksheet or `LESSONS_DIR` writes in this spec's files. |
| sec-1 | No secret value in git. | PASS | `.env.example` has empty `OPENAI_API_KEY=`. Dockerfile has no `ENV OPENAI_API_KEY`. Test `rejects a non-empty OPENAI_API_KEY assignment in the files this spec tracks` passed. |
| sec-2 | Media backup paths stay under `DATA_DIR`. | PASS | `copyMediaTree` refuses a destination outside `DATA_DIR` and a link that resolves outside `DATA_DIR/media`. Test `refuses a media symlink that leaves DATA_DIR` passed (Windows junction). |

## Definition of done

- [x] Backup directory contains the sqlite file and both media trees — tests above.
- [x] Manifest names the key and does not contain a value — manifest test.
- [x] Cluster secret and live Deployment apply were not performed — left for the operator gate.

## Tests run by the verifier

```text
node --test test/media-backup.test.js
→ 6 pass / 0 fail ; exit 0

npm run audio:verify
→ exit 0 ; 65 recordings accounted for
```

The implementing session also ran `npm test` → 509 pass, 0 fail, exit 0, before this verification. No coverage tool is configured.

## Remediation loop

None. First independent pass was all-PASS.
