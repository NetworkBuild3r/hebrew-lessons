# INIT-027 security review

**Provenance:** INIT-027/SECURITY-REVIEW
**Reviewer:** independent generalPurpose read, model grok-4.7-high. Not an implementer. [INIT-027 security review](5c166a30-8c2c-42c4-b718-f3a1320b44bc)
**Result:** PASS
**Counts:** Critical 0 / High 0 / Medium 0 / Low 0

Checked the new drill answers, implied credit, placement-list seeding, and `POST /_hx/review/drill`:

- The learner id comes from the signed-in session, never the form.
- A drill pick must be one side of the served pair or "don't know". Right or wrong is decided against the letter the server chose. Latency is measured on the server.
- Implied credit runs only after a server-graded right answer on the card just served, stores Good, and writes no attempts row. Another learner's card is refused.
- Seeding inserts new cards with `INSERT OR IGNORE` and does not change an existing card or write taught words, attempts, exposures, or review log rows.
- The pick and the done screen stay templ-escaped. No new MCP tool and no role change.
- No secret, token, or localhost model URL was added.

No open findings.
