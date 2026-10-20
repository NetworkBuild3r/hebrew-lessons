# SPEC-019 AC Verification Eval Report — Operator guide: media and backup

**Traceability:** INIT-004/SPEC-019 \
**Verifier:** independent generalPurpose Task that did not author the guide · model `composer-2.5-fast` (`ac_eval.verifier_model` on Cursor) · [Verify SPEC-019 criteria](39952405-07fa-4cd1-b4b6-46acc4241f6e) · temperature 0 · read-only by prompt (the Task tool has no readonly flag) \
**Evaluated at:** 2026-10-09T20:50:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Guide documents upload, approval, dry-run, confirm, the caps, and delete-archives-first. The "AI cannot read progress" sentence is unchanged. | PASS | Upload `docs/operations/accounts-and-admin.md:94`. Approval, admin only, alt required `:100`. Dry-run `:110`. Confirm `:112`. Caps 15 per clock hour and $1 per UTC day `:116-117`. Delete copies to `_archive`, checks the copy, then removes the live file `:125`. Privacy sentence `:173` is "The AI and MCP tools cannot read progress or answer history." `git blame` on that line is `bbfb307` (2026-10-08), before this spec. No `hebrew-worksheet` path in the file. |
| ac-2 | Media trees are backed up with the database. Restore checks approved images after both trees are restored. Writers stop first. | PASS | Backup directory contains `hebrew.sqlite`, `media/`, and `media/_archive/` `:64`. Restore stops the app ("no writers"), replaces both trees, then confirms an approved image still resolves `:72`. Kubernetes stops writes first `:84` and checks an approved image `:86`. Pictures section states the same order `:90`. |
| ac-3 | No key assignment and no learner transcript. | PASS | `rg` for `sk-`, `ELEVENLABS_API_KEY=`, and `(OPENAI_API_KEY\|ELEVENLABS_API_KEY)\s*=` returned no matches (exit 1). `node --test test/media-backup.test.js` → 6 pass, 0 fail, including `rejects a non-empty OPENAI_API_KEY assignment` which scans this file. `OPENAI_API_KEY` is named at `:108` and `:160` with no value. No quoted learner dialogue. |

## Definition of done

- [x] Upload, approval, dry-run, confirm, caps, and archive-before-delete are in the operator guide — ac-1.
- [x] Backup and restore sentences name both media trees and the approved-image check — ac-2. Those backup sentences were already in the working copy from `INIT-004/SPEC-017`; this spec left them and restated the rule in the Pictures section.
- [x] No secret assignment and no learner transcript — ac-3 and the media-backup scan.
- [x] The progress sentence and the hebrew-worksheet skill were not rewritten.

## Tests run by the verifier

```text
rg "sk-" docs/operations/accounts-and-admin.md
rg "ELEVENLABS_API_KEY=" docs/operations/accounts-and-admin.md
rg "(OPENAI_API_KEY|ELEVENLABS_API_KEY)\s*=" docs/operations/accounts-and-admin.md
→ no matches (exit 1)

node --test test/media-backup.test.js
→ 6 passed / 6 total ; exit 0
```

The implementer re-ran `node --test test/media-backup.test.js` after one later env-table edit (the `MCP_AUTH_TOKEN` cell now names the picture tools). Same result: 6 pass, 0 fail, exit 0. That cell assigns no key.

---

## Remediation log (feedback loop)

None. First pass was all-PASS on ac-1, ac-2, and ac-3.

---

## Notes

- The verifier's strict reading of the human check ("trace media sentences to ADR-0004 §5 to §9") flagged "A learner cannot upload" (`accounts-and-admin.md:94`) as coming from the §4 role matrix, and the admin-only approval lines as §4 plus §8. The spec's pass condition is that every media sentence traces to the ADR and that the secret search is empty. Those sentences trace to ADR-0004 §4, and §8 cites §4 for the rule that MCP cannot approve. The three acceptance criteria stayed PASS. No sentence was removed.
- No AC was passed by weakening a test or the criterion.
- After the verifier returned, the `MCP_AUTH_TOKEN` row was widened so it names `upload_media`, `plan_image`, and `generate_image`. No key value was added. The media-backup scan was re-run on that text.
