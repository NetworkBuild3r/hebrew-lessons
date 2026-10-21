# SPEC-002 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-002 (infrastructure) · **Status:** complete (PR #92 merged 2026-10-19) · **Date:** 2026-10-19

## Changes

- Branch `v3` on origin, from `origin/v2` 528c75c. Commit `97192b2` "Cut v3 from v2 and move CI publishing to v3 (INIT-015/SPEC-002)". 4 files, +283 / -16: `.github/workflows/ci.yml`, `README.md`, `docs/decisions/ADR-0018-v3-adaptive-learning.md` (copied from main, unchanged), `docs/ops/v3-hosts.md`.
- Branch `ci/v2-stop-publishing`, commit `9717734`, PR [#92](https://github.com/NetworkBuild3r/hebrew-lessons/pull/92) against `v2`. 1 file, +3. It adds `false &&` to the `publish` job `if:`. Open, mergeable, not merged. Its own CI: test pass, build-and-smoke pass, publish skipped.

## Acceptance criteria

| AC | Result | Evidence (re-checked by the orchestrator) |
| --- | --- | --- |
| ac-1 | PASS | `git ls-remote`: `refs/heads/v3` = 97192b2, parent 528c75c. `origin/v2` still 528c75c. |
| ac-2 | PASS | Run 39304044756 on `k8-build`: test success, publish success, build-and-smoke skipped. The only `ghcr.io/networkbuild3r/<name>` in the log is `hebrew`. Guard is byte-identical to v2 (sub-agent compared programmatically). |
| ac-3 | PASS | PR #92 squash-merged as `9af9193`. CI run 39328565351 on that push to `v2`: `test` success, `publish` skipped, `build-and-smoke` skipped. No image was pushed. |
| ac-4 | PASS | No `pull_request_target` trigger (only a comment). Every `runs-on` is `k8-build`. `hebrew-lessons` appears in the never-comment, the module-path comment, the guard, and the push `case`, all unchanged from v2. |

## Deviations and disclosures

- **More than the trigger changed.** Publishing on v3 needed the publish job `if:`, the login, build and push step `if:`s, and the in-step push guard moved to `refs/heads/v3`. Several trigger-describing comments were updated too. Diff is +19 / -15 in `ci.yml`.
- **Added beyond the deliverables:** ADR-0018 copied onto `v3` (the CI header cites it), and ", ADR-0018" on the README line.
- **Sub-agent mistake, recovered.** Its first commit failed on PowerShell quoting and it pushed before checking, so `origin/v3` briefly pointed at 528c75c. The old workflow only triggers on v2, so nothing ran. It then committed and fast-forward pushed. No force push.
- **`docs/ops/v3-hosts.md` says v2 "does not push an image".** That is true only after #92 merges.
- **Image tag now live.** `ghcr.io/networkbuild3r/hebrew:sha-97192b2b…` is the newest `sha-` tag. Its code is identical to v2 (only CI and docs differ). If Argo Image Updater tracks this image for a host, that host may pick it up. aud-1 (which host tracks it) is still open and blocks only SPEC-025.
- **Not verified:** that the tag is visible in the registry (log shows the push step ran, not a "pushed" line), the negative test of the guard, and Go build/test locally (Go not installed here; CI's `go vet`, `go test`, and `go test -race` passed).
- `Co-authored-by: Cursor` trailer was added to both commits by the environment.
- One `gh auth status` call printed a masked token (`gho_***`). Nothing exposed.
- Same standing gaps as SPEC-001: no personas, no timer or trace scripts, general-purpose sub-agent.

## Gates

`merge` (closed): the operator told the orchestrator to merge #92 on 2026-10-19, and it was squash-merged with `gh pr merge --squash` under the operator's account. No review or approval was recorded on the PR (no review was required by branch rules). This departs from the standing rule that the Hebrew PR Reviewer approves and merges; it was done on the operator's explicit instruction.
