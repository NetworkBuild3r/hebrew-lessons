# SPEC-025 completion summary: Cutover to production (plan_only)

- **Status:** in progress. The runbook and the tools it calls are written and merged. Two acceptance criteria (ac-2, ac-4) need the operator and have not been done. Worker commit `0476bde`, merged into local `v3` as `e3898eb`. Not pushed. No schema change (the next free migration number is still 012).
- **Worker:** [SPEC-025 worker](045d41bf-0209-4aa7-bd8f-feae9867697d), a general-purpose sub-agent in worktree `Hebrew-v3-s025`. Nothing was run against the cluster, a real database, or the `home_k3` checkout.
- **Merge:** clean. The change is additive (8 files; `cmd/hebrew/main.go` gets 4 lines to route `migrate` and `backup`).

## What shipped

- `docs/ops/v3-cutover.md`: numbered steps 1 to 6, each with commands, expected output and a STOP condition. It also has the run order, ten open operator decisions, the pre-push checklist, and the `home_k3` change written as four diffs, not applied.
- `scripts/smoke-v3.sh` (mode 100755): `/healthz`, `/version` (optional expected commit), the anonymous redirect to sign-in, a 401 on an anonymous MCP call, MCP initialize, `tools/list`, and `get_learner_model`. It prints PASS, FAIL or SKIP and exits 1 on any failure. The token is read from the environment and never appears in an argument or in output. It does not sign in, load Today, review a card or start a placement; those are manual checks in the runbook.
- `hebrew migrate --check`: reports the migrations a database would apply, opening it read-only. `hebrew backup --to <path>`: writes a consistent copy, refuses to overwrite, prints a checksum. Code in `cmd/hebrew/ops.go` and `internal/db/inspect.go`, with tests.
- `docs/ops/v3-hosts.md` records the answer to its open question.

## Acceptance criteria

| AC | Result |
| --- | --- |
| ac-1 | Met. Every step has commands, expected output and a stop condition. |
| ac-2 | **Not done.** The script passes 7 of 7 against a local server and fails for a wrong port, token, commit and a missing learner id. It has not run against staging with a copy of production data, because no backup exists yet. |
| ac-3 | Met. The `home_k3` change is four diffs against the files at commit `ececb90c`, not applied. |
| ac-4 | **Not done.** The rollback was not rehearsed; staging was not touched. Step 5.R is the rehearsal for the operator. The runbook says plainly that the rollback is untested. |

## Verified by the orchestrator

On the merged `v3`: `gofmt -l` empty, `go vet ./...` clean, tokens check clean, `go test ./...` passes in all packages. I did not repeat the worker's mutations. The worker reports 20 on the Go code (each failed a test, except one equivalent case where `O_EXCL` already refuses it; one uncaught case led to a test seam in `inspect.go`) and 4 on the smoke script (token on the command line, exit status always 0, healthz accepting any status, commit mismatch ignored), all detected. `shellcheck` is clean per the worker.

## Findings that change the plan

1. **The old image cannot read a migrated database.** The worker ran the `7ad62e8` code against a database the v3 binary had migrated. It opens without error, so `/healthz` passes, but card queries fail with `no such column: dir`. Rollback is the previous tag plus the backup file, and everything a learner does after the cutover is lost. This is a one-way door.
2. **`home_k3` CI blocks the male voice.** `scripts/check-hebrew-kustomization.sh` rejects any mention of `ELEVENLABS_VOICE_ID`, which also matches `ELEVENLABS_VOICE_ID_MALE`. Diff 2 of the runbook edits that check; the worker ran it with and without the edit.
3. **The backup cannot come from the production image.** It has no `backup` command, so the v3 image makes the backup in a one-off pod. That is why the run order puts the push before the backup.
4. **Migrations run in the init container** (`import-tanakh` opens the database first). A failing migration shows as a stuck init container, not an app crash loop.
5. **Argo CD reverts `kubectl scale`.** `hebrew`, `hebrew-next` and `root` all have selfHeal and prune on, so every replicas or tag change is a `home_k3` commit.
6. **Staging is not a separate stage.** Both Image Updaters track the same image, so a push to `v3` rolls both hosts. The documented pause is `ignoreTags: ["*"]` in `apps/hebrew/README.md`. A real staging dry run costs six extra `home_k3` commits.
7. The local `home_k3` clone is two days stale and has no `hebrew-next`; the worker read the files at `ececb90c` through the GitHub API.

## Unproven

Everything that touches the cluster: the `kubectl` and Vault commands, the pod manifests, a second pod attaching to the same ReadWriteOnce Longhorn volume, `kubectl cp` through busybox, and `ignoreTags` in the installed Image Updater. Migrations 006 to 011 have only run on temp databases. Migration time and outage length are unknown. `migrate --check` and `backup` leave empty `-wal` and `-shm` files beside a database opened read-only (harmless).

## Operator decisions (full list of ten in the runbook)

- Keep production auto-deploy paused after the cutover, or resume it.
- The line's security review was partial (see the initiative file's security review record); whether that is enough before pushing.
- SPEC-024 has since landed on 3 (merge 844f16d), so that blocker is cleared.
- Real-staging dry run versus a scratch pod.
- Whether to run `audio-ensure` on staging as well as production.
- The coach-automation credential (see the SPEC-023 summary).
