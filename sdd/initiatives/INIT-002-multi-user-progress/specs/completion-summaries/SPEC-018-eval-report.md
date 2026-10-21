# SPEC-018 AC verification

**Verifier:** orchestrator (not the author). Read `docs/operations/accounts-and-admin.md`, `.cursor/skills/hebrew-worksheet/SKILL.md`, and `README.md`. Re-ran the first-admin and backup commands against a temporary `DATA_DIR` (not the repo `data/` folder), then deleted that tree. No secret values were printed.
**Tier:** light. Pre-flight: gate_actions PASS · security_review=light PASS · domain=docs PASS · autonomy_level=plan_only PASS ⇒ light_tier_eligible=true.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | `npm run user -- create admin --admin --password-file <temp>` exited 0 and created `hebrew.sqlite`. `npm run db:backup` wrote one file under `DATA_DIR/backups/`. |
| ac-2 | PASS | The guide names `audio:inventory`, `audio:verify`, `<lesson>/audio/_archive/<stem>.<12hex>.mp3`, and inventory on the live pod before a rollout. |
| ac-3 | PASS | The skill still forbids learner names, transcripts, lesson `index.html` edits, and root/binyan drills. It adds hidden-until-assigned, `list_users`, and the audio inventory check. |
| ac-4 | PASS | The three files contain env and secret names only. No `sk-` key and no bearer token value. |

No remediation loop.
