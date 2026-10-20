# SPEC-002 · infrastructure: Cut `v3` and move CI to it

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-015
title: "Cut v3 and move CI to it"
domain: infrastructure
status: complete
primary_prompt: .claude/agents/devops-engineer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_branch
gate_actions: [merge]
reversibility_class: revertible
security_review: required
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003, SPEC-006, SPEC-008]
```

## Description

**Context:** `.github/workflows/ci.yml` on `origin/v2` runs on push to `v2`, only on ARC `k8-build`, and publishes `ghcr.io/networkbuild3r/hebrew:sha-<sha>`. A guard step refuses any other image name. Its header says Argo Image Updater deploys the newest `sha-` tag of that image. If v2 and v3 both published `sha-` tags, the updater would deploy whichever pushed last.

**Scope (in):**

- Create branch `v3` from `origin/v2`.
- In `ci.yml` on `v3`: trigger on `branches: [v3]`, keep the image-name guard, keep `runs-on: k8-build`, keep action pins, and change the header to say v3 and cite ADR-0018.
- On `v2`: a one-line change to `ci.yml` that stops publishing (build and test only). It lands in the same window as the `v3` CI change.
- `README.md` on `v3`: change the first line to v3.
- A handoff note `docs/ops/v3-hosts.md` on `v3`. It names the home_k3 change needed, if any, for `hebrew-next.ibhacked.us` to track v3. It does not make that change.

**Scope (out):** Editing home_k3. Production cutover (SPEC-025).

## Understood (agent's read of the intent)

- [x] und-1: Only one branch publishes `sha-` tags at any time.
- [x] und-2: Every v2 CI safety rule carries over unchanged.

## Acceptance Criteria

- [x] ac-1: `v3` exists on `origin`, created from the `origin/v2` head. — covered: no
- [x] ac-2: A push to `v3` runs CI on `k8-build` and publishes `ghcr.io/networkbuild3r/hebrew:sha-<sha>`. The guard still fails the job for any other image name. — covered: no
- [x] ac-3: A push to `v2` after this change runs tests and does not push an image. — PASS: PR #92 squash-merged as 9af9193 on 2026-10-19; CI run 39328565351 on that push: test success, publish skipped, build-and-smoke skipped.
- [x] ac-4: `ci.yml` on `v3` contains no `pull_request_target`, no GitHub-hosted runner label, and the string `hebrew-lessons` only in the "never" comment and the guard. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Which host tracks the `hebrew` image | Not knowable from this repo. The handoff note asks the operator to confirm in home_k3. | assumed | no |
| aud-2 | Tag scheme | Keep `sha-<sha>`, so no updater change is needed. | decided | yes |

> **Readiness** = 90. aud-1 does not block CI. It blocks only SPEC-025.

## Deliverables

- [x] Branch `v3`
- [x] `.github/workflows/ci.yml` on `v3`, and on `v2` as PR #92 (merged as 9af9193)
- [x] `README.md` on `v3`
- [x] `docs/ops/v3-hosts.md`

## Technical Requirements (domain-specific)

- [ ] `fetch-depth: 1` stays. The registry cache settings stay. Do not set `DOCKER_HOST`.
- [ ] Forks are still refused.

## Gates & Controls

- **Autonomy level:** auto_branch. The agent may push the `v3` branch. The CI change to `v2` goes through a PR.
- **Gate actions triggered:** merge. Hebrew PR Reviewer approves and merges.
- **Blocking checks:** the first `v3` CI run is green and pushes one `sha-` tag.
- **Rollback plan:** revert the `v2` change and delete the `v3` branch.

## Security

- **Sensitivity:** required — triggers matched: CI, registry credentials, deploy trigger.
- **Security acceptance criteria:**
  - [ ] No new secret. No change to token permissions in the workflow.
  - [ ] The image-name guard is byte-identical to v2's.
- **Review depth:** standard · **focus areas:** supply chain, CI trigger scope.

## Verification Strategy

- **Claim:** v3 publishes through the same guarded pipeline, and v2 stops publishing.
- **Check + executor:** mechanized — `gh run list --branch v3` and `gh run view` on the first run. `git diff origin/v2 origin/v3 -- .github/workflows/ci.yml`.
- **Pass condition:** the run is green and the log shows one push to `ghcr.io/networkbuild3r/hebrew:sha-<sha>`. The diff touches only the trigger branch, the header, and comments.

## Integration Points

- Every later spec works on `v3`.
- SPEC-025 reads `docs/ops/v3-hosts.md`.
- **Provenance tag:** `INIT-015/SPEC-002`
