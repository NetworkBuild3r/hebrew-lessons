# SPEC-017 · Infrastructure: Media volume, backup, and the generation secret

## Metadata

```yaml
spec_id: SPEC-017
initiative_id: INIT-004
title: "Media volume, backup, and OPENAI_API_KEY"
domain: infrastructure
status: complete
primary_prompt: .claude/agents/principal-devops-engineer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [secret_change, deploy]
reversibility_class: one_way
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-008]
blocks: [SPEC-020]
```

---

## Description

**Context:** ADR-0004 §7 and §9. `db:backup` today copies `hebrew.sqlite` only. INIT-004 adds `DATA_DIR/media/` and `DATA_DIR/media/_archive/`. The generation key is a cluster secret, same pattern as `ELEVENLABS_API_KEY`. The image does not contain it.

**Scope (in):**

- Document and, where the repo already describes the Deployment (`deploy/k8s-database.md` and the lessons volume doc), add the media directory on the existing `hebrew-data` PVC (`DATA_DIR=/data/app` at `origin/main` as ADR-0005's current-state table states). Do not add a second replica. Do not change `strategy: Recreate`.
- `OPENAI_API_KEY` as a `secretKeyRef` next to the existing keys. The manifest references the key name. It does not contain a value.
- Extend the backup script so the set includes the SQLite file and both media trees. Restore instructions: stop writers, restore the database and the media trees from the same backup, then check that approved images still resolve.
- `.dockerignore` continues to exclude `.env`. A test or a grep in the spec's verification fails if `OPENAI_API_KEY=` with a non-empty value appears in a tracked file this spec adds.
- Local `DATA_DIR` layout the server already refuses when it is wrong stays in force. Media is not a served root.

**Scope (out):** creating a real key in a cluster (the operator does that at the gate); calling OpenAI; copying recordings into the image.

---

## Understood (agent's read of the intent)

- [x] und-1: The secret value is never written into git. The spec and the manifest name the variable only.
- [x] und-2: Backup of media does not move or delete `.mp3` files. Those stay on the lessons volume.

## Acceptance Criteria

- [x] ac-1 (REQ-011): the backup command's file list includes `hebrew.sqlite`, `media/`, and `media/_archive/` under `DATA_DIR`. A test with a temporary directory asserts those paths are copied and an `.mp3` outside `DATA_DIR` is not required for the command to succeed.
- [x] ac-2 (REQ-011): the Deployment snippet keeps `replicas: 1` and `Recreate`, and adds `OPENAI_API_KEY` only as a secret reference. A test reads the manifest text and fails if a line assigns a key value.
- [x] ac-3 (REQ-013): `npm run audio:verify` still passes. This spec does not write under `LESSONS_DIR`.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Which volume | The existing data PVC, subdirectory `media`, because ADR-0004 puts bytes under `DATA_DIR` and ADR-0005 keeps that volume. | resolved | yes — ADR-0004 §5, ADR-0005 §8 |

> **Readiness** = 100.

---

## Deliverables

- [x] Backup script change and restore note in `docs/operations/` or the deploy doc the repo already uses for backup
- [x] Deployment manifest or documented patch for the secret reference
- [x] Tests: `test/media-backup.test.js`

---

## Technical Requirements (domain-specific)

- [x] `fsGroup` and `runAsUser` stay as they are unless the media directory is not writable by the existing uid, in which case the spec stops and asks. Do not change the uid in the same unattended edit.
- [x] No secret in the image layer. Do not add a `ENV OPENAI_API_KEY` with a value in the Dockerfile.

---

## Gates & Controls

- **Autonomy level:** `auto_tests` for the script and the manifest text. Applying the secret and rolling the Deployment waits.
- **Gate actions triggered:** `secret_change`, `deploy`.
- **Blocking checks:** `node --test test/media-backup.test.js`, `npm test`, `npm run audio:verify`. The operator confirms the cluster secret exists before deploy. The agent does not print the value.
- **Rollback plan:** point the Deployment back at the previous manifest. The backup script's previous behavior remains in git. Media files already written stay on the volume.

---

## Security

- **Sensitivity:** required. Triggers matched: generation key, backup of user-uploaded images.
- **Security acceptance criteria:**
  - [x] No secret value in git.
  - [x] Media backup paths stay under `DATA_DIR`, which is outside served roots.
- **Review depth:** thorough · **focus areas:** secrets, backup scope.

---

## Verification Strategy

- **Claim:** backup copies the media trees with the database, and the manifest does not embed a key.
- **Check + executor:** `mechanized` — `node --test test/media-backup.test.js` plus a grep of the manifest for an assignment pattern. `human` — the operator, at the deploy gate, confirms the secret exists in the cluster without pasting it into the chat.
- **Pass condition:** the test exits 0; the manifest contains `secretKeyRef` for `OPENAI_API_KEY` and no literal key; `replicas` is 1.
- **Test levels:** unit over the backup function with a temporary directory. No cluster call from the test. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- SPEC-008 writes the files this backup must include. SPEC-020 does not cut over generation-dependent lessons until this secret gate is done if those lessons need new pictures. Plan text cutover does not require the key.
- Produces **Change:** backup and secret reference.
- **Provenance tag:** `INIT-004/SPEC-017`
