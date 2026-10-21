# SPEC-007 · Docs: ADR-0005 — stack and architecture decision

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-003
title: "ADR-0005: stack and architecture decision"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus   # pinned: the stack trade-off
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-004, SPEC-005]
blocks: [SPEC-009, SPEC-010]
```

---

## Description

**Context:** The operator asks whether this is the right code base, whether the CSS approach is right, and whether Phoenix "or something else" would be better, and says to rebuild from the ground up if needed. What exists today, from a read of the repository on 2026-10-08:

- Plain Node 22 (`node:http`, `node:sqlite`), one dependency (`@modelcontextprotocol/sdk`), no build step and no framework.
- About 9,400 lines of front-end JavaScript as plain globals that build markup with string templates, plus one stylesheet of about 1,470 lines with colour tokens, 158 hard-coded hex values, one breakpoint, and no dark mode.
- A server that also hosts the MCP in the same process, with 44 test files and 10 Chrome smoke scripts, some of which assert on class names and `data-*` attributes.
- Data that a rewrite must carry: accounts and progress in SQLite, 65 recorded audio clips, and three lessons.

The look problem is mostly a design and CSS problem, which any stack would have to solve. The real stack question is which runtime and rendering approach suits a typed lesson-plan language (ADR-0003), part-level edits shared by the MCP and a builder, a media pipeline (ADR-0004), and one operator maintaining it. This ADR decides it on written criteria, with a rewrite and Phoenix judged fairly.

**Scope (in):** `docs/decisions/ADR-0005-stack-and-architecture.md` (number claimed at plan time by directory scan, disclosed). It decides:

1. **Criteria.** A weighted list, derived from requirements, such as: renders the block vocabulary of ADR-0003; supports part-level operations and a live preview in the builder; one implementation of the operations shared by the MCP and the builder; right-to-left and bidirectional text; accessibility; media upload and gated serving; works with SQLite and the existing data; hosts or talks to the MCP; test story; operator's ability to maintain it alone; AI agents' ability to write and edit it correctly; deployment (Docker, one replica); and the cost of migration.
2. **Options.** At least these, each described with a concrete shape:
   - A. Keep Node and plain front end, add a real design system (tokens, components, dark mode) and a tested component layer by hand.
   - B. Keep the Node server, add a small no-build component library for the front end (for example Preact with htm, or web components).
   - C. Keep the Node server, move the front end to a build-based toolchain (for example Svelte, or Vue, or React with Vite) and a CSS approach such as Tailwind.
   - D. Rewrite on Elixir and Phoenix LiveView.
   - E. Rewrite on another full-stack framework (the ADR names the single most plausible candidate and says why).
3. **The evaluation.** A scored table, with every score justified in a sentence. For each rewrite option (D and E), the plan to carry accounts and progress, the 65 clips and archive rules, the three lessons, the MCP, the 44 tests (which would be ported, which dropped), and the Docker and Kubernetes layout.
4. **The decision.** One option, with the reasons, the cost, and the conditions under which it would be wrong. The decision is the operator's and is confirmed at the gate.
5. **Architecture.** For the chosen option: the module boundaries (plan store, operations, validator, media, personal-plan service, MCP, API, front end); that the operations of ADR-0003 are implemented once and used by both the MCP and the API; where state lives; how the learner site loads a plan (and the fate of the script-tag constraint per ADR-0003); and how the same server hosts the MCP.
6. **CSS and component approach.** For the chosen option: how styling is organized (tokens, components), how dark mode and reduced motion are handled, how right-to-left is handled, and how class and data-attribute contracts are kept stable for tests.
7. **Testing approach.** Which tests are kept as they are, which are ported, and how the Chrome smokes are kept running during the transition.
8. **Migration approach.** Whether INIT-004 builds alongside the current site and switches over, or changes it in place, and the rollback at each step. All INIT-002 audio and data invariants are carried.

**Scope (out):** the lesson-plan language (ADR-0003); AI and media security (ADR-0004); the chosen look (ADR-0006); production code.

---

## Understood (agent's read of the intent)

- [x] und-1: A rewrite is allowed if it earns its cost. It does not need to win ties.
- [x] und-2: Phoenix is judged on the criteria in item 1, not on reputation. Real-time push, its main strength, is not a requirement of this product. The ADR checks whether anything else Phoenix offers is.
- [x] und-3: The ADR does not decide the look. A stack that cannot deliver the look the operator picks in SPEC-006 and SPEC-008 is a finding against that stack.
- [x] und-4: SPEC-009's spike is the cheap test of this decision. If the spike shows the choice is wrong, the ADR is amended before INIT-004 is written.

## Acceptance Criteria

- [x] ac-1 (REQ-001): ADR-0005 has a Decision section for each of the eight items above, with considered options, the chosen option, and the rationale.
- [x] ac-2 (REQ-001): options A to E all appear in the scored table with a justified score on every criterion.
- [x] ac-3 (REQ-001, REQ-010): for each option that moves the server or the database, the ADR states how accounts, progress, the 65 clips, the three lessons, and the MCP carry over, with no step that deletes or overwrites a recording without an archive.
- [x] ac-4 (REQ-011): the ADR states the fate of each carried constraint under the chosen option (kept, changed, or superseded) and agrees with ADR-0003's table.
- [x] ac-5 (REQ-003): the architecture section shows that each ADR-0003 operation has one implementation, reached by both the MCP and the builder's API.
- [x] ac-6: the ADR lists the tests that must keep passing during migration (starting from the authoring, MCP, admin editor, gating, visibility, and Chrome smoke files) and how each is kept or ported.
- [x] ac-7: the ADR contains no secret values, no learner names, and no transcript text.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Operator's tolerance for a rewrite | "From the ground up if you have to" means a rewrite is allowed when it wins on the criteria, not that it is preferred | resolved | yes — operator 2026-10-08 |
| aud-2 | Who maintains the result | The operator alone, with AI agents writing most code | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-3 | Hosting target | The existing Docker image and Kubernetes layout from INIT-002, one replica | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-4 | Weights on the criteria | Proposed in the ADR; the operator adjusts at the gate | resolved | yes — operator 2026-10-08 ("go"); plan default |

> **Readiness** = 100. aud-2 to aud-4 were resolved by the operator on 2026-10-08 ("go"), including the plan defaults for the maintainer, the one-replica host, and the weights. The letter (A) is Proposed and is confirmed at the gate before SPEC-009.

---

## Deliverables

- [x] `docs/decisions/ADR-0005-stack-and-architecture.md`
- [x] Tests: none (documentation). The spike (SPEC-009) tests the decision.

---

## Technical Requirements (domain-specific)

- [x] Use `.claude/skills/sdd/assets/templates/architecture-decision-record-template.md` section order, matching ADR-0001 and ADR-0002.
- [x] Facts about the repository (line counts, test counts, dependencies) are re-checked against `origin/main` when the spec runs, and cited with file paths, not repeated from this spec.
- [x] Any claim about a framework's capabilities cites its documentation with the access date.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the operator confirms the chosen option and answers aud-2 to aud-4 before SPEC-009 begins.
- **Rollback plan:** delete ADR-0005.

---

## Security

- **Sensitivity:** standard. Triggers matched: runtime and auth consequences of the choice (session handling, gated static serving, MCP hosting).
- **Security acceptance criteria:**
  - [x] The chosen option keeps every INIT-002 gating, session, and CSRF rule, or states its replacement for each.
  - [x] The migration approach has no step in which the site is served without sign-in.
- **Review depth:** standard · **focus areas:** auth, gating, supply chain (new dependencies).

---

## Verification Strategy

- **Claim:** the stack decision was made on written criteria, each option was scored on every criterion, and the chosen option has a migration path that loses nothing.
- **Check + executor:** `human`. A reviewer who did not write the ADR picks two criteria and re-scores two options from the ADR's own evidence, and checks the migration table row by row against the list in ac-3. `mechanized` aid: a search for the five option letters in the scored table.
- **Pass condition:** the re-scores land within one point of the ADR's, or the reviewer's difference is explained in writing; every row of the migration table is present; aud-2 to aud-4 are resolved.

---

## Integration Points

- Consumed by SPEC-009 (the spike is built on this choice), SPEC-010 (INIT-004's specs follow this architecture), and ADR-0006 (the look must be deliverable on the stack).
- Produces **Change:** one ADR.
- **Provenance tag:** `INIT-003/SPEC-007`
