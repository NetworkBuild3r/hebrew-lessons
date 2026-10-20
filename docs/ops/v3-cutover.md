# v3 cutover runbook

INIT-015 / SPEC-025. This is a plan. Nothing in it has been run. The operator runs it, or an agent the operator names.

Read this first:

- **Plan only.** The commands below were written from the repo, from `home_k3` (read only), and from facts checked on 2026-10-19 with read-only `kubectl`. No command in steps 1 to 6 was run against the cluster. See [What is proven and what is not](#what-is-proven-and-what-is-not).
- **The cutover is one way.** Migration 006 rebuilds the `cards` table. The old image cannot read the result (step 5 shows the proof). The only way back is to restore the backup file. Anything the learner does after the cutover is lost on rollback.
- **Never print a secret.** Every command here keeps tokens in a shell variable and never echoes them. Do not add `set -x`. Do not paste a token or a voice id into a ticket, a chat, or a file in a repo.
- **Rules that still hold.** v2 and v3 never merge into `main`. CI never publishes `ghcr.io/networkbuild3r/hebrew-lessons`. CI runs on the `k8-build` runners only. The Hebrew PR Reviewer approves and merges pull requests, so the operator does not approve them. The Kubernetes MCP service account is read only: if it answers Forbidden, stop and use your own credentials. Do not work around it.

## Facts this plan rests on

Checked 2026-10-19 via read-only `kubectl`. Re-check them at run time (step 1 has the commands). If one differs, stop and re-plan.

| Fact | Value |
| --- | --- |
| Production | namespace `hebrew`, Deployment `hebrew`, 1 replica, strategy `Recreate` |
| Production image | `ghcr.io/networkbuild3r/hebrew:sha-7ad62e8dfeb94091ad0cd32fc98ae23ccb093e02` (`origin/v3` commit 7ad62e8, "Shuffle lesson choices (#99)") |
| Production data | PVC `hebrew-v2-data` at `/data` (`DATA_DIR=/data`), `/tmp` is an `emptyDir`, root file system read only, uid 65532, distroless (no shell, no `sqlite3`, no `tar`) |
| Init container | `import-tanakh` runs `hebrew import-tanakh` before the app, on the same image and the same volume |
| Staging | namespace `hebrew-next`, Deployment `hebrew-next`, PVC `hebrew-next-data`, the same image |
| Hosts | `hebrew.ibhacked.us` (production), `hebrew-next.ibhacked.us` (staging), MCP at `hebrew-mcp.ibhacked.us` (production) and `hebrew-mcp-next.ibhacked.us` (staging) |
| Environment on the pod | `PUBLIC_ORIGIN`, `COOKIE_SECURE`, `TRUSTED_PROXIES`, `DATA_DIR`, `MCP_PUBLIC_URL`, `ELEVENLABS_API_KEY` (secret `hebrew-elevenlabs`), `MCP_AUTH_TOKEN` and `HEBREW_SESSION_SECRET` (secret `hebrew-mcp-auth`), `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` (secret `hebrew-google-oauth`), `MCP_OAUTH_ALLOWED_EMAILS`, `ADMIN_EMAILS` |
| Not set | `ELEVENLABS_VOICE_ID` (the code default is the female voice, which is right), `ELEVENLABS_MODEL` (default `eleven_v3`, right), **`ELEVENLABS_VOICE_ID_MALE` (missing, needed for the second voice in dialogues)**, `MCP_READONLY_TOKEN`, `HEBREW_JOBS` (the daily scheduler is on by default) |
| D7 | Production runs the v3-line image `hebrew`, not `hebrew-lessons`, so D7 branch 1 applies: keep the SQLite file and migrate it forward |
| Migrations on the production database | recorded: 1, 2, 3, 4, 5, 7 (`007_oauth_client_issued.sql` came from `origin/v3`). Not recorded: 6. INIT-015 numbers (D9) are 006, 008, 009, 010, 011, so the runner will apply **6, 8, 9, 10, 11** |
| Argo CD | Applications `hebrew` and `hebrew-next` read `git@github.com:NetworkBuild3r/home_k3.git`, paths `apps/hebrew/manifests` and `apps/hebrew-next/manifests` (kustomize `images.newTag`) |
| Image Updater | Two `ImageUpdater` objects track `ghcr.io/networkbuild3r/hebrew`, newest `sha-<40 hex>`, and write back to the kustomization. **A push to `v3` therefore rolls production and staging both, and staging is not a separate stage today.** Step 4 fixes that before the push. |
| `origin/v3` | Four PRs merged there while this line was built (#95 version endpoint, #77 OAuth prune, #99 shuffle, #97 read-only smoke token). They are merged into local `v3` (2ac89b9). Nothing is pushed. **Pushing `v3` deploys.** |
| Placement audio | 206 clips (pointed Hebrew, default female voice, `eleven_v3`) are not on the cluster. `hebrew audio-ensure` makes them (step 6). |

What I added from reading `home_k3` at commit `ececb90c` (2026-10-19, read through the GitHub API because the local clone is two days behind and has no `hebrew-next`):

- `ImageUpdater` objects are in `apps/argocd-image-updater/manifests/hebrew-imageupdater.yaml` and `hebrew-next-imageupdater.yaml`. The documented pause is `ignoreTags: ["*"]` (`apps/hebrew/README.md`, section "Pause").
- Both Argo CD Applications have `automated.prune: true` and `selfHeal: true`, and so does the `root` Application that owns them. **A `kubectl scale`, or a patch to an Application, is reverted.** Every change to replicas or to the image tag has to be a commit to `home_k3`. That is why the rollback below is written as commits.
- `scripts/check-hebrew-kustomization.sh` (a `home_k3` CI check) fails if a Deployment or ExternalSecret mentions `ELEVENLABS_VOICE_ID`. `ELEVENLABS_VOICE_ID_MALE` matches that pattern, so the male voice change cannot merge unless the check is edited too. Diff 2 does that.

## Run order

| # | When | What | Where it is written |
| --- | --- | --- | --- |
| R1 | now | Confirm image, volume, and updaters, read only | Step 1 |
| R2 | before the push | Put the male voice id in Vault (4.0) | Step 4 |
| R3 | before the push | `home_k3` PR with Diff 1 (pause the production updater) and Diff 2 (check edit and staging male voice). Merge it. Verify the pause (4.5). | Step 4 |
| R4 | the push | [Pre-push checklist](#pre-push-checklist), then `git push origin v3`. CI publishes `sha-<V3_SHA>`. Staging rolls. Production must not. | Pre-push checklist |
| R5 | after CI | Confirm the image exists, production did not move, staging is up on v3, smoke staging | Step 3.0 |
| R6 | after R5 | Back up production with the v3 image | Step 2 |
| R7 | after R6 | Staging dry run with a copy of production data. Rehearse the rollback. Clean up. | Step 3 and 5.R |
| R8 | the cutover | `home_k3` PR with Diff 3 (production male voice and the pin). Migrations run when the pod starts. | 4.4 |
| R9 | after R8 | Verify, audio, automation, first daily job, soak, clean up | Step 6 |
| any | on failure at R8 or R9 | Roll back | Step 5 |

The backup (R6) comes after the push (R4) because the production image has no `backup` command, no `sqlite3`, and no `tar`. The v3 image has the command, so the v3 image makes the backup. Production keeps running the old image the whole time, because Diff 1 stops the updater.

## Variables

Set these in a POSIX shell (Linux, macOS, WSL, or Git Bash). Use your own `kubectl` credentials, not the read-only service account.

```sh
export IMG=ghcr.io/networkbuild3r/hebrew
export OLD_SHA=7ad62e8dfeb94091ad0cd32fc98ae23ccb093e02   # the tag production runs today. Re-check in step 1.
export V3_SHA=                                            # fill in at R5: the 40 hex characters of the v3 commit CI published
export BK="$HOME/hebrew-backups"                          # operator-only directory, outside every repo
mkdir -p "$BK" && chmod 700 "$BK"
```

## Step 1. Confirm the image and the volume for each host

Read only. Run it at R1 and again right before the push (R4) and right before the cutover (R8).

```sh
for ns in hebrew hebrew-next; do
  echo "== $ns"
  kubectl -n "$ns" get deploy "$ns" -o jsonpath='{.spec.replicas}{" replicas, "}{.spec.strategy.type}{"\n"}'
  kubectl -n "$ns" get deploy "$ns" -o jsonpath='{range .spec.template.spec.initContainers[*]}init {.name}{" "}{.image}{"\n"}{end}{range .spec.template.spec.containers[*]}app  {.name}{" "}{.image}{"\n"}{end}'
  kubectl -n "$ns" get deploy "$ns" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="data")].persistentVolumeClaim.claimName}{"\n"}'
  kubectl -n "$ns" get deploy "$ns" -o jsonpath='{range .spec.template.spec.containers[0].env[*]}{.name}{"\n"}{end}' | sort | tr '\n' ' '; echo
done
curl -s https://hebrew.ibhacked.us/version
echo
kubectl -n argocd get application hebrew hebrew-next
kubectl -n argocd get imageupdater hebrew hebrew-next
```

Expected:

- Production: `1 replicas, Recreate`. `init import-tanakh` and `app  hebrew`, both `ghcr.io/networkbuild3r/hebrew:sha-7ad62e8dfeb94091ad0cd32fc98ae23ccb093e02`. Volume `hebrew-v2-data`. The env names listed under "Environment on the pod" above, and no `ELEVENLABS_VOICE_ID_MALE`.
- Staging: the same shape, the same tag, volume `hebrew-next-data`.
- `/version` on production prints JSON whose `commit` starts with `7ad62e8`.
- Both Applications `Synced` and `Healthy`. Both `ImageUpdater` objects exist.

Also check that the pause field exists in the installed CRD:

```sh
kubectl explain imageupdaters.spec.applicationRefs.images.commonUpdateSettings.ignoreTags
```

Expected: a field description (a list of strings).

**STOP if:**

- the image name is `ghcr.io/networkbuild3r/hebrew-lessons`. D7 branch 2 applies. This runbook does not cover it. Archive the v1 database read only, start fresh with a placement, and write a new plan.
- the tag is not `sha-7ad62e8...`. Set `OLD_SHA` to what is actually running. That tag is the rollback target, so it must be the real one. Read the new tag's commit and check it carries no migration this plan does not know about (`git ls-tree origin/v3 migrations/`).
- the volume is not `hebrew-v2-data` (production) or `hebrew-next-data` (staging). Every step below names those volumes.
- the `ignoreTags` field does not exist. The pause in Diff 1 would do nothing. Do not push `v3`. (A fallback is to narrow `allowTags` in the production `ImageUpdater` to the one old tag. `check-hebrew-kustomization.sh` rejects that, so that fallback needs its own `home_k3` change.)
- `kubectl` answers Forbidden. Use your own credentials. Do not try to get around it.

## Step 2. Back up the production SQLite file, with a checksum

The spec says `sqlite3 .backup`. The image has no `sqlite3` and no `tar`, so the backup is `hebrew backup --to <file>`. It runs `VACUUM INTO` on a read-only connection, so it is one consistent snapshot even while the app writes. It creates the file with mode 0600, refuses to overwrite, applies no migration, and prints the byte count and SHA-256 only. The old production image does not have it, so a one-off pod runs the v3 image against the production volume. The pod is pinned to the node the app pod is on, because the volume is ReadWriteOnce.

Run at R6 (the v3 image exists, production still runs the old one).

```sh
export NODE=$(kubectl -n hebrew get pod -l app=hebrew -o jsonpath='{.items[0].spec.nodeName}')
export BACKUP_NAME="hebrew-prod-pre-v3-$(date -u +%Y%m%dT%H%M%SZ).sqlite"
echo "node=$NODE file=$BACKUP_NAME"

kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: v3-backup
  namespace: hebrew
spec:
  restartPolicy: Never
  nodeName: $NODE
  automountServiceAccountToken: false
  imagePullSecrets:
    - name: ghcr-pull-secret
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    runAsGroup: 65532
    fsGroup: 65532
    seccompProfile:
      type: RuntimeDefault
  initContainers:
    - name: backup
      image: $IMG:sha-$V3_SHA
      args: ["backup", "--to", "/data/backups/$BACKUP_NAME"]
      env:
        - name: DATA_DIR
          value: /data
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
      volumeMounts:
        - name: data
          mountPath: /data
        - name: tmp
          mountPath: /tmp
  containers:
    - name: shell
      image: busybox:1.37
      command: ["sleep", "3600"]
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: hebrew-v2-data
    - name: tmp
      emptyDir: {}
EOF

kubectl -n hebrew wait --for=condition=Initialized pod/v3-backup --timeout=180s
kubectl -n hebrew logs v3-backup -c backup
```

Expected: `bytes=<N> sha256=<64 hex>` and nothing else. Keep the digest:

```sh
export SHA_REMOTE=$(kubectl -n hebrew logs v3-backup -c backup | sed -n 's/^bytes=[0-9]* sha256=\([0-9a-f]\{64\}\)$/\1/p')
[ -n "$SHA_REMOTE" ] && echo "digest captured" || echo "NO DIGEST"
```

Copy it off the cluster and compare:

```sh
kubectl -n hebrew cp -c shell "v3-backup:/data/backups/$BACKUP_NAME" "$BK/$BACKUP_NAME"
chmod 600 "$BK/$BACKUP_NAME"
LOCAL=$(sha256sum "$BK/$BACKUP_NAME" | cut -d' ' -f1)
[ "$LOCAL" = "$SHA_REMOTE" ] && echo "MATCH" || echo "MISMATCH"
echo "$SHA_REMOTE  $BACKUP_NAME" >> "$BK/SHA256SUMS"
```

Expected: `MATCH`. (`shasum -a 256` on macOS.)

Check the file with the same binary, read only, on your own machine:

```sh
docker run --rm -v "$BK:/b" "$IMG:sha-$V3_SHA" migrate --check --db "/b/$BACKUP_NAME"
```

Expected, for the production database as it is checked today:

```
migrate-check: files=11 applied=6 pending=5
applied: 1 2 3 4 5 7
pending: 006_evidence.sql 008_learner_model.sql 009_confusion_other.sql 010_daily_job.sql 011_placement_v3.sql
pending-below-applied: 6 (the runner applies by number, so these still run)
integrity: ok
result: ok
```

Then release the volume:

```sh
kubectl -n hebrew delete pod v3-backup
```

The backup file stays on the volume at `/data/backups/$BACKUP_NAME`. That is the fast path for a restore (step 5). Delete it at the end of the soak (step 6.7).

**STOP if:**

- the pod stays `Pending` or `ContainerCreating` with a Multi-Attach error. The pod is not on the app's node. Re-read `NODE` and start again. If Longhorn still refuses to attach a second pod, use an ephemeral container on the running pod instead and do not improvise further.
- the `backup` init container fails. `kubectl -n hebrew logs v3-backup -c backup` has the reason. A backup that was not made is the end of the plan. Production is untouched.
- the digest does not match, or `migrate --check` prints `problem:`, or the applied list is not `1 2 3 4 5 7`. Do not go on with this file. Delete the local copy, delete `/data/backups/$BACKUP_NAME` through a shell pod, and take a new one.
- the backup directory is readable by anyone but you. It must be `700`, outside every repo, and not in a synced folder. The file holds a learner's whole history.

Optional, recommended before R4 as a second layer: take a Longhorn snapshot of `hebrew-v2-data`. I did not verify the Longhorn tooling on this cluster, so use whatever you already use for Longhorn snapshots.

## Step 3. Dry run on staging with a copy of production data

This follows the spec: copy the backup to staging, run v3 on it, run `hebrew migrate --check` and the smoke script, and do the signed-in checks in a browser. Staging is disposable by its own manifests (`apps/hebrew-next/manifests/pvc-data.yaml`: "Clean staging database and media"), so this overwrites its database.

An alternative is a scratch pod with an `emptyDir`. It leaves no copy of production data on a volume with `Retain`, but sign-in does not work on it (Google redirects go to the real staging host), so the signed-in checks cannot be done there. It is not written out here.

### 3.0 Staging on v3 with its own data (R5)

After the push, CI publishes the image and staging rolls on its own.

```sh
export V3_SHA=""     # set it: the 40 hex characters of the v3 commit CI published
docker buildx imagetools inspect "$IMG:sha-$V3_SHA" | head -n 4
kubectl -n hebrew-next rollout status deploy/hebrew-next --timeout=300s
kubectl -n hebrew-next get deploy hebrew-next -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
kubectl -n hebrew get deploy hebrew -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
kubectl -n hebrew-next logs deploy/hebrew-next -c import-tanakh --tail=5
kubectl -n hebrew-next logs deploy/hebrew-next -c hebrew --tail=20
```

Expected: the image exists. Staging runs `sha-<V3_SHA>`. **Production still runs `sha-7ad62e8...`.** The staging app logs show `starting` with `male_voice=true`, then `listening`.

Smoke staging with its own data:

```sh
export BASE_URL=https://hebrew-next.ibhacked.us
export MCP_URL=https://hebrew-mcp-next.ibhacked.us/mcp
export MCP_TOKEN=$(vault kv get -field=mcp_token kv/apps/staging/hebrew-next)
export SMOKE_LEARNER_ID=1     # an admin or static token needs the id of a learner row that exists
export EXPECT_COMMIT=$V3_SHA
sh scripts/smoke-v3.sh
```

Expected: `smoke-v3: 7 passed, 0 failed, 0 skipped`.

**STOP if:**

- production runs anything but `sha-7ad62e8...`. The pause did not hold, and production has migrated without a backup. Go to step 5 now and treat the Longhorn snapshot, if you took one, as the restore source.
- staging's init container `import-tanakh` is in `CrashLoopBackOff` or `Error`. Migrations run there (it opens the database before the app does), so a failing migration shows up as a stuck init container, not as an app crash. `kubectl -n hebrew-next logs <pod> -c import-tanakh` names the file. Fix it in the repo. Do not go on.
- `male_voice=false`. The Vault key or the staging diff is missing. Fix before step 3.2.
- a smoke check fails. The script names it.

### 3.1 Freeze staging (Diff 4)

Staging's updater would move the tag under you during the dry run, and `kubectl scale` is reverted by Argo CD. Freeze it with a commit: Diff 4 in step 4.

```sh
kubectl -n hebrew-next get pods -l app=hebrew-next
```

Expected after the sync: `No resources found in hebrew-next namespace.`

**STOP if** a pod is still there after three minutes. Look at `kubectl -n argocd get application hebrew-next`. Do not touch the volume while a pod holds it.

### 3.2 Load the production copy

```sh
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: v3-shell
  namespace: hebrew-next
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    runAsGroup: 65532
    fsGroup: 65532
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: shell
      image: busybox:1.37
      command: ["sleep", "3600"]
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: hebrew-next-data
EOF
kubectl -n hebrew-next wait --for=condition=Ready pod/v3-shell --timeout=180s

kubectl -n hebrew-next cp -c shell "$BK/$BACKUP_NAME" "v3-shell:/data/incoming.sqlite"
kubectl -n hebrew-next exec v3-shell -c shell -- env SUM="$SHA_REMOTE" sh -c '
  set -e
  cd /data
  echo "$SUM  incoming.sqlite" | sha256sum -c -
  mkdir -p backups
  cp incoming.sqlite backups/pre-v3.sqlite
  rm -f hebrew.sqlite hebrew.sqlite-wal hebrew.sqlite-shm
  mv incoming.sqlite hebrew.sqlite
  ls -l hebrew.sqlite backups/pre-v3.sqlite
'
kubectl -n hebrew-next delete pod v3-shell
```

Expected: `incoming.sqlite: OK`, then two file listings. The copy at `backups/pre-v3.sqlite` is the pristine copy the rollback rehearsal restores in 5.R.

**STOP if** the checksum line says FAILED. Nothing was replaced (`set -e` stops before the `rm`). Delete `incoming.sqlite` and copy again.

### 3.3 Start v3 on the copy

Commit Diff 5 (staging replicas back to 1; see step 4). Then:

```sh
kubectl -n hebrew-next rollout status deploy/hebrew-next --timeout=300s
kubectl -n hebrew-next logs deploy/hebrew-next -c import-tanakh --timestamps
kubectl -n hebrew-next get pod -l app=hebrew-next -o jsonpath='{.items[0].status.initContainerStatuses[0].state.terminated.startedAt}{" to "}{.items[0].status.initContainerStatuses[0].state.terminated.finishedAt}{"\n"}'
kubectl -n hebrew-next exec deploy/hebrew-next -c hebrew -- /usr/local/bin/hebrew migrate --check
```

Expected: the init container finishes (this is where 006, 008, 009, 010, 011 run, one transaction each). The two timestamps give the time the migrations and the import took. Write it down: production will be down for about that long plus the image pull and the app start, because the strategy is `Recreate`. `migrate --check` then prints `applied: 1 2 3 4 5 6 7 8 9 10 11`, `pending: (none)`, `integrity: ok`, `result: ok`.

Run the daily job once on the copy:

```sh
kubectl -n hebrew-next exec deploy/hebrew-next -c hebrew -- /usr/local/bin/hebrew jobs daily
```

Expected: one `daily job` log line with `learners=` the number of learners and no error. The command is safe to repeat. The scheduler also runs a catch-up at start (at most 14 days), so rows may already be there.

**STOP if** the init container fails. Staging holds a copy, so this is the safe place to find out. Collect `kubectl -n hebrew-next logs <pod> -c import-tanakh --previous`. Do not cut over until the cause is fixed and this step passes again from 3.2. The log names a migration file and a SQLite error. It does not print row data, but read it before you share it.

### 3.4 Smoke and the signed-in checks

```sh
export BASE_URL=https://hebrew-next.ibhacked.us
export MCP_URL=https://hebrew-mcp-next.ibhacked.us/mcp
export MCP_TOKEN=$(vault kv get -field=mcp_token kv/apps/staging/hebrew-next)
export SMOKE_LEARNER_ID=1     # set it to your own learner id if it is not 1
export EXPECT_COMMIT=$V3_SHA
SMOKE_STRICT=1 sh scripts/smoke-v3.sh
```

Expected: `smoke-v3: 7 passed, 0 failed, 0 skipped`, exit status 0. `get_learner_model` returning a page of words proves the migrated tables read on real data.

The script cannot sign in. Do these by hand in a browser at `https://hebrew-next.ibhacked.us`, signed in with your Google account:

1. Today loads, and its due count looks like the production count.
2. Open Review, answer one card, and confirm the answer saves (the card leaves the due list, and the Progress page changes).
3. Open a lesson and finish one step.
4. Open Progress.
5. Start a placement. This needs the 206 clips on staging. Run the dry run first: `kubectl -n hebrew-next exec deploy/hebrew-next -c hebrew -- /usr/local/bin/hebrew audio-ensure --dry-run`. If `would_generate` is more than 0, the clips are missing and the placement start will fail on listening items. Making them costs ElevenLabs credits (step 6.1 has the real run). Decide at [Open decisions](#open-decisions).

**STOP if** anything returns a 5xx, shows `no such column`, shows an empty Today for a learner who has due cards, or loses the review you just made. A smoke failure here blocks the cutover.

Do not clean up staging yet. 5.R rehearses the rollback on it, and cleans up.

## Step 4. The exact `home_k3` changes

None of these is applied. They are diffs against `home_k3` commit `ececb90c1ef47f022851a89bada875948e8e8996` (master, 2026-10-19). I applied them in order to a scratch copy of the files at that commit. Each applies cleanly, and `scripts/check-hebrew-kustomization.sh` (the `home_k3` CI check) passes after each. Two things I could not run: `kustomize build` and the Argo CD and External Secrets behaviour. Commit them through your usual `home_k3` process, and re-read the files at the current `master` first (`git apply --check` tells you).

**Files read:** `apps/argocd-image-updater/manifests/hebrew-imageupdater.yaml`, `hebrew-next-imageupdater.yaml`, `apps/hebrew/manifests/kustomization.yaml`, `deployment.yaml`, `elevenlabs-externalsecret.yaml`, `pvc-v2.yaml`, `apps/hebrew-next/manifests/kustomization.yaml`, `deployment.yaml`, `elevenlabs-externalsecret.yaml`, `pvc-data.yaml`, both `application.yaml` files, `apps/root/application.yaml`, `apps/hebrew/README.md`, `apps/hebrew-next/README.md`, `apps/argocd-image-updater/README.md`, `scripts/check-hebrew-kustomization.sh`.

### 4.0 Put the male voice id in Vault (R2)

The value comes from your secret store. Do not write it in a file, a ticket, or this repo. `vault kv patch` adds one key and keeps the others (`vault kv put` would replace them all).

```sh
read -r -s MALE_VOICE_ID          # paste the id, press Enter. Nothing is echoed.
vault kv patch kv/apps/production/hebrew ELEVENLABS_VOICE_ID_MALE="$MALE_VOICE_ID"
unset MALE_VOICE_ID
vault kv get -format=json kv/apps/production/hebrew | jq '.data.data | has("ELEVENLABS_VOICE_ID_MALE")'
```

Expected: `true`. The property is the same one for production and staging (both ExternalSecrets read `apps/production/hebrew`).

**STOP if** it prints `false`, or if the existing keys (`ELEVENLABS_API_KEY`, `mcp_token`, `session_secret`) are gone. If `ELEVENLABS_VOICE_ID_MALE` is not in Vault when Diff 2 syncs, the External Secrets Operator reports a sync error for `hebrew-elevenlabs`. The old Secret stays, and the pods still start (the env entry is `optional: true`), but there is no male voice.

SPEC-026 recorded that the male voice has only been tried once, directly, with `eleven_v3`. It has not been tried through the app. Generate one dialogue line after the cutover and listen to it (step 6.1).

### 4.1 Diff 1. Pause the production updater (R3)

Without this, the push to `v3` also rolls production. With it, the updater ignores every tag, and the production tag stays where `kustomization.yaml` has it.

```diff
diff --git a/apps/argocd-image-updater/manifests/hebrew-imageupdater.yaml b/apps/argocd-image-updater/manifests/hebrew-imageupdater.yaml
index 6fb5257..21f5ce6 100644
--- a/apps/argocd-image-updater/manifests/hebrew-imageupdater.yaml
+++ b/apps/argocd-image-updater/manifests/hebrew-imageupdater.yaml
@@ -43,6 +43,10 @@ spec:
             updateStrategy: newest-build
             # Full git sha only. Excludes buildcache and short tags.
             allowTags: "regexp:^sha-[0-9a-f]{40}$"
+            # PAUSED for the v3 cutover (INIT-015/SPEC-025). Every tag is ignored, so a push to the
+            # v3 branch rolls hebrew-next only. Remove this list to resume auto-deploy.
+            ignoreTags:
+              - "*"
             pullSecret: pullsecret:argocd/ghcr-pull-secret
             # image.yml publishes linux/amd64.
             platforms:
```

### 4.2 Diff 2. Allow the male voice in the check, and add it to staging (R3)

Ship it in the same pull request as Diff 1. The check edit is the first change in the diff: `\b` after `ELEVENLABS_VOICE_ID` stops the pattern matching `ELEVENLABS_VOICE_ID_MALE` (underscore is a word character), so `ELEVENLABS_VOICE_ID` and `ELEVENLABS_MODEL` are still refused. Without the check edit the `home_k3` CI check fails with "hebrew-next must not require ELEVENLABS_VOICE_ID or ELEVENLABS_MODEL" (I ran it both ways).

The env entry is `optional: true`, in the repo's own style ("A missing unused Vault key must not block the pod"). The cost: if the key is ever missing, the pod starts without the male voice. The app logs `male_voice=false` when it starts, so check for that (4.5).

```diff
diff --git a/apps/hebrew-next/manifests/deployment.yaml b/apps/hebrew-next/manifests/deployment.yaml
index 8a531ea..a9ac595 100644
--- a/apps/hebrew-next/manifests/deployment.yaml
+++ b/apps/hebrew-next/manifests/deployment.yaml
@@ -97,6 +97,15 @@ spec:
                 secretKeyRef:
                   name: hebrew-elevenlabs
                   key: ELEVENLABS_API_KEY
+            # Voice for speaker b of a dialogue (INIT-015 D8). No code default: when it is
+            # unset a speaker b clip fails and the female voice is never used for it.
+            # optional, so a missing Vault property does not keep the pod from starting.
+            - name: ELEVENLABS_VOICE_ID_MALE
+              valueFrom:
+                secretKeyRef:
+                  name: hebrew-elevenlabs
+                  key: ELEVENLABS_VOICE_ID_MALE
+                  optional: true
             - name: MCP_AUTH_TOKEN
               valueFrom:
                 secretKeyRef:
diff --git a/apps/hebrew-next/manifests/elevenlabs-externalsecret.yaml b/apps/hebrew-next/manifests/elevenlabs-externalsecret.yaml
index c95e188..b47dbf2 100644
--- a/apps/hebrew-next/manifests/elevenlabs-externalsecret.yaml
+++ b/apps/hebrew-next/manifests/elevenlabs-externalsecret.yaml
@@ -1,8 +1,9 @@
 # ElevenLabs API key. Same Vault path as prod (kv/apps/production/hebrew
 # property ELEVENLABS_API_KEY). v2 also reads optional ELEVENLABS_VOICE_ID
 # (default 6WRlPmU1fqPwYbSxeP16) and ELEVENLABS_MODEL (default eleven_v3).
-# Both have code defaults and are not set here, so this ExternalSecret
-# still syncs only the API key. Values stay in Vault.
+# Both have code defaults and are not set here. The second voice is the exception:
+# ELEVENLABS_VOICE_ID_MALE (speaker b of a dialogue, INIT-015 D8) has no code default,
+# so it is synced from Vault too. Values stay in Vault.
 apiVersion: external-secrets.io/v1
 kind: ExternalSecret
 metadata:
@@ -27,3 +28,10 @@ spec:
         conversionStrategy: Default
         decodingStrategy: None
         metadataPolicy: None
+    - secretKey: ELEVENLABS_VOICE_ID_MALE
+      remoteRef:
+        key: apps/production/hebrew
+        property: ELEVENLABS_VOICE_ID_MALE
+        conversionStrategy: Default
+        decodingStrategy: None
+        metadataPolicy: None
diff --git a/scripts/check-hebrew-kustomization.sh b/scripts/check-hebrew-kustomization.sh
index 5eaff07..7633c8d 100644
--- a/scripts/check-hebrew-kustomization.sh
+++ b/scripts/check-hebrew-kustomization.sh
@@ -320,8 +320,8 @@ check_hebrew_next_clean() {
     echo "hebrew-next Deployment is missing ELEVENLABS_API_KEY"
     fail=1
   fi
-  if noncomment "$dir/deployment.yaml" | grep -q 'ELEVENLABS_VOICE_ID\|ELEVENLABS_MODEL' \
-    || noncomment "$dir/elevenlabs-externalsecret.yaml" | grep -q 'ELEVENLABS_VOICE_ID\|ELEVENLABS_MODEL'; then
+  if noncomment "$dir/deployment.yaml" | grep -q 'ELEVENLABS_VOICE_ID\b\|ELEVENLABS_MODEL' \
+    || noncomment "$dir/elevenlabs-externalsecret.yaml" | grep -q 'ELEVENLABS_VOICE_ID\b\|ELEVENLABS_MODEL'; then
     echo "hebrew-next must not require ELEVENLABS_VOICE_ID or ELEVENLABS_MODEL"
     fail=1
   fi
@@ -524,8 +524,8 @@ if ! grep -q 'name: ELEVENLABS_API_KEY' "$deploy"; then
   echo "prod Deployment is missing ELEVENLABS_API_KEY"
   fail=1
 fi
-if noncomment "$deploy" | grep -q 'ELEVENLABS_VOICE_ID\|ELEVENLABS_MODEL' \
-  || noncomment "$dir/elevenlabs-externalsecret.yaml" | grep -q 'ELEVENLABS_VOICE_ID\|ELEVENLABS_MODEL'; then
+if noncomment "$deploy" | grep -q 'ELEVENLABS_VOICE_ID\b\|ELEVENLABS_MODEL' \
+  || noncomment "$dir/elevenlabs-externalsecret.yaml" | grep -q 'ELEVENLABS_VOICE_ID\b\|ELEVENLABS_MODEL'; then
   echo "prod must not require ELEVENLABS_VOICE_ID or ELEVENLABS_MODEL"
   fail=1
 fi
```

### 4.3 Diff 3. Production male voice and the pin (R8, the cutover)

This is the commit that cuts production over. Do not merge it before step 3 and 5.R have passed. It does three things: it sets the production tag to the v3 commit, it adds the male voice, and (as a side effect of the pin) it restarts the pod. One restart, so the migrations run once.

`<V3_SHA, 40 hex>` must be the real value (the check requires `newTag: sha-<40 hex>`). Save the diff to `diff3.patch`, replace the placeholder, then `git apply --check diff3.patch && git apply diff3.patch`. The updater stays paused from Diff 1, so the pin is not overwritten.

```diff
diff --git a/apps/hebrew/manifests/deployment.yaml b/apps/hebrew/manifests/deployment.yaml
index 4f283a2..9b4aa09 100644
--- a/apps/hebrew/manifests/deployment.yaml
+++ b/apps/hebrew/manifests/deployment.yaml
@@ -97,6 +97,15 @@ spec:
                 secretKeyRef:
                   name: hebrew-elevenlabs
                   key: ELEVENLABS_API_KEY
+            # Voice for speaker b of a dialogue (INIT-015 D8). No code default: when it is
+            # unset a speaker b clip fails and the female voice is never used for it.
+            # optional, so a missing Vault property does not keep the pod from starting.
+            - name: ELEVENLABS_VOICE_ID_MALE
+              valueFrom:
+                secretKeyRef:
+                  name: hebrew-elevenlabs
+                  key: ELEVENLABS_VOICE_ID_MALE
+                  optional: true
             - name: MCP_AUTH_TOKEN
               valueFrom:
                 secretKeyRef:
diff --git a/apps/hebrew/manifests/elevenlabs-externalsecret.yaml b/apps/hebrew/manifests/elevenlabs-externalsecret.yaml
index 504f916..80e3604 100644
--- a/apps/hebrew/manifests/elevenlabs-externalsecret.yaml
+++ b/apps/hebrew/manifests/elevenlabs-externalsecret.yaml
@@ -1,8 +1,9 @@
 # ElevenLabs API key. Source of truth: Vault kv/apps/production/hebrew
 # property ELEVENLABS_API_KEY. v2 also reads optional ELEVENLABS_VOICE_ID
 # (default 6WRlPmU1fqPwYbSxeP16) and ELEVENLABS_MODEL (default eleven_v3).
-# Both have code defaults and are not set here, so this ExternalSecret
-# still syncs only the API key. A missing unused Vault key must not block
+# Both have code defaults and are not set here. The second voice is the exception:
+# ELEVENLABS_VOICE_ID_MALE (speaker b of a dialogue, INIT-015 D8) has no code default,
+# so it is synced from Vault too. A missing unused Vault key must not block
 # the pod. Shared with hebrew-next. Values stay in Vault.
 apiVersion: external-secrets.io/v1
 kind: ExternalSecret
@@ -28,3 +29,10 @@ spec:
         conversionStrategy: Default
         decodingStrategy: None
         metadataPolicy: None
+    - secretKey: ELEVENLABS_VOICE_ID_MALE
+      remoteRef:
+        key: apps/production/hebrew
+        property: ELEVENLABS_VOICE_ID_MALE
+        conversionStrategy: Default
+        decodingStrategy: None
+        metadataPolicy: None
diff --git a/apps/hebrew/manifests/kustomization.yaml b/apps/hebrew/manifests/kustomization.yaml
index 7996b40..4efb27e 100644
--- a/apps/hebrew/manifests/kustomization.yaml
+++ b/apps/hebrew/manifests/kustomization.yaml
@@ -28,4 +28,4 @@ resources:
   - ingressroute.yaml
 images:
   - name: ghcr.io/networkbuild3r/hebrew
-    newTag: sha-7ad62e8dfeb94091ad0cd32fc98ae23ccb093e02
+    newTag: sha-<V3_SHA, 40 hex>
```

Note: `deployment.yaml` also has `image: ...:sha-a936686d...` on the container and init container. That is the match key only. Kustomize rewrites the tag from `newTag` for both. Do not edit those lines.

### 4.4 Cutover (R8)

Run this only when: step 2 produced a verified backup, step 3 and 5.R passed, the pre-push checklist items still hold, and you have time to watch it. Re-run step 1 first.

```sh
# in a home_k3 checkout
git pull --ff-only
git apply --check diff3.patch && git apply diff3.patch
bash scripts/check-hebrew-kustomization.sh          # expect: ...cannot cross-match
git diff --stat                                      # expect 3 files
# commit and merge through your usual home_k3 process, then:
kubectl -n argocd get application hebrew
kubectl -n hebrew rollout status deploy/hebrew --timeout=600s
kubectl -n hebrew get pods -l app=hebrew
kubectl -n hebrew logs deploy/hebrew -c import-tanakh --tail=10
kubectl -n hebrew logs deploy/hebrew -c hebrew --tail=20
```

Expected: the old pod ends first (`Recreate`). The new pod runs `import-tanakh`, which opens the database and applies 006, 008, 009, 010, 011. Then the app starts and logs `starting` with `male_voice=true`, then `listening`. Roll-out ends with `successfully rolled out`.

**STOP and go to step 5 if** the init container or the app does not become Ready, `import-tanakh` ends with an error, or the log shows a migration or SQLite error. A migration is one transaction per file, so a failing file leaves the earlier files applied. That database is half migrated, and the old image cannot read it either. Restore the backup. Do not retry in place.

### 4.5 Checks after Diff 1 and Diff 2 (R3)

```sh
kubectl -n argocd get imageupdater hebrew -o jsonpath='{.spec.applicationRefs[0].images[0].commonUpdateSettings.ignoreTags}{"\n"}'
kubectl -n hebrew get deploy hebrew -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
kubectl -n hebrew-next get secret hebrew-elevenlabs -o jsonpath='{.data}' | jq 'has("ELEVENLABS_VOICE_ID_MALE")'
kubectl -n hebrew-next get externalsecret hebrew-elevenlabs
```

Expected: `["*"]`. The production image is still `sha-7ad62e8...`. `true`. The ExternalSecret reports `SecretSynced`.

**STOP before the push if** any of those is wrong. The `ImageUpdater` object may take a minute to sync from Git.

### 4.6 Diff 4. Freeze staging for the dry run (step 3.1)

Pauses the staging updater and scales staging to zero, so the volume is free. It is one commit, and the sync is automatic.

```diff
diff --git a/apps/argocd-image-updater/manifests/hebrew-next-imageupdater.yaml b/apps/argocd-image-updater/manifests/hebrew-next-imageupdater.yaml
index c3c6efd..aef30b0 100644
--- a/apps/argocd-image-updater/manifests/hebrew-next-imageupdater.yaml
+++ b/apps/argocd-image-updater/manifests/hebrew-next-imageupdater.yaml
@@ -36,6 +36,9 @@ spec:
           commonUpdateSettings:
             updateStrategy: newest-build
             allowTags: "regexp:^sha-[0-9a-f]{40}$"
+            # PAUSED for the staging dry run (INIT-015/SPEC-025). Remove this list to resume.
+            ignoreTags:
+              - "*"
             pullSecret: pullsecret:argocd/ghcr-pull-secret
             platforms:
               - linux/amd64
diff --git a/apps/hebrew-next/manifests/deployment.yaml b/apps/hebrew-next/manifests/deployment.yaml
index a9ac595..87d5276 100644
--- a/apps/hebrew-next/manifests/deployment.yaml
+++ b/apps/hebrew-next/manifests/deployment.yaml
@@ -14,7 +14,7 @@ spec:
   labels:
     app: hebrew-next
 spec:
-  replicas: 1
+  replicas: 0
   strategy:
     type: Recreate
   selector:
```

The later staging commits are one-line edits. These do not depend on line numbers:

```sh
# GNU sed. On macOS use: sed -i ''
sed -i 's/^  replicas: 1$/  replicas: 0/' apps/hebrew-next/manifests/deployment.yaml     # Diff 4 (freeze)
sed -i 's/^  replicas: 0$/  replicas: 1/' apps/hebrew-next/manifests/deployment.yaml     # Diff 5 (start again)
sed -i "s|newTag: sha-[0-9a-f]\{40\}|newTag: sha-$OLD_SHA|" apps/hebrew-next/manifests/kustomization.yaml    # rehearsal: old tag
sed -i "s|newTag: sha-[0-9a-f]\{40\}|newTag: sha-$V3_SHA|" apps/hebrew-next/manifests/kustomization.yaml     # cleanup: v3 tag
grep -c '^  replicas: ' apps/hebrew-next/manifests/deployment.yaml                      # expect 1 before each edit
```

Staging commits in order: **Diff 4** (3.1, freeze), **Diff 5** (3.3, start on the copy), **Diff 6** (5.R, `replicas: 0` and the old tag), **Diff 7** (5.R, `replicas: 1`), **Diff 8** (5.R cleanup, `replicas: 0` and the v3 tag), **Diff 9** (5.R cleanup, `replicas: 1` and remove the `ignoreTags` lines from `hebrew-next-imageupdater.yaml`).

## Step 5. Rollback

**The old image does not tolerate the new database. Rolling back the image alone is not enough.** I checked this on 2026-10-19 by running the code of commit 7ad62e8 (the production image) against a database that the v3 binary had migrated:

- `internal/db/migrate.go` in 7ad62e8 loops over the files it embeds (001 to 005 and 007) and skips a version that is already recorded. It never reads a version it does not have a file for. So it **starts without complaint** on a database with versions 6 and 8 to 11 recorded. `/healthz` would pass.
- But migration 006 replaced `cards.dir` with `cards.skill`, and the old code selects `dir`. Against the migrated file, `ListDueCards` and `ListStudyCards` failed with `SQL logic error: no such column: dir`. Reviews and Today would be broken while the pod looks healthy.
- Against the pre-migration copy of the same file, the same two calls returned the card.

So a rollback is **the previous tag and the backup file together**. Every review, placement result, and snapshot written after the backup is lost, because the old schema cannot hold it. It stays in the file you set aside (`hebrew.sqlite.after-v3-*`), and nothing can merge it back.

**This rollback has NOT been rehearsed.** Staging was not touched by the person who wrote this runbook. 5.R is the rehearsal, and it is the operator's to run (it is acceptance criterion ac-4). Treat everything below as untested until 5.R has passed.

### 5.1 Production

Variables: `BACKUP_NAME` and `SHA_REMOTE` from step 2 (the backup file is on the volume at `/data/backups/$BACKUP_NAME`; if it is not, upload it from `$BK` first, as in step 3.2). `OLD_SHA` is the tag step 1 recorded.

1. Scale to zero and pin the old tag in one commit. The updater is already paused (Diff 1). If you resumed it, pause it first, or it will move the tag forward again.

```sh
# in a home_k3 checkout
git pull --ff-only
sed -i 's/^  replicas: 1$/  replicas: 0/' apps/hebrew/manifests/deployment.yaml
sed -i "s|newTag: sha-[0-9a-f]\{40\}|newTag: sha-$OLD_SHA|" apps/hebrew/manifests/kustomization.yaml
git diff --stat      # expect 2 files, 2 insertions, 2 deletions
git commit -am "hebrew: roll back to $OLD_SHA and scale to 0 for the database restore (INIT-015/SPEC-025)"
# merge through your usual home_k3 process
kubectl -n hebrew get pods -l app=hebrew
```

   Expected: `No resources found in hebrew namespace.` **STOP if** a pod is still there after three minutes. Never touch the database while a pod holds it.

2. Restore the file.

```sh
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: v3-shell
  namespace: hebrew
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    runAsGroup: 65532
    fsGroup: 65532
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: shell
      image: busybox:1.37
      command: ["sleep", "3600"]
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: hebrew-v2-data
EOF
kubectl -n hebrew wait --for=condition=Ready pod/v3-shell --timeout=180s
kubectl -n hebrew exec v3-shell -c shell -- env NAME="$BACKUP_NAME" SUM="$SHA_REMOTE" sh -c '
  set -e
  cd /data
  echo "$SUM  backups/$NAME" | sha256sum -c -
  [ ! -e hebrew.sqlite ] || mv hebrew.sqlite "hebrew.sqlite.after-v3-$(date -u +%Y%m%dT%H%M%SZ)"
  rm -f hebrew.sqlite-wal hebrew.sqlite-shm
  cp "backups/$NAME" hebrew.sqlite.restoring
  mv hebrew.sqlite.restoring hebrew.sqlite
  echo "$SUM  hebrew.sqlite" | sha256sum -c -
  ls -l hebrew.sqlite*
'
kubectl -n hebrew delete pod v3-shell
```

   Expected: two `OK` lines. **The `-wal` and `-shm` files must go.** A stale WAL next to a restored file can replay old pages into it. **STOP if** either checksum fails. Nothing was swapped before the first check (`set -e`). If the second fails, the file is damaged: repeat from the copy in `$BK`.

3. Start the old image on the restored file.

```sh
sed -i 's/^  replicas: 0$/  replicas: 1/' apps/hebrew/manifests/deployment.yaml
git commit -am "hebrew: start on $OLD_SHA with the restored database (INIT-015/SPEC-025)"
# merge through your usual home_k3 process
kubectl -n hebrew rollout status deploy/hebrew --timeout=300s
kubectl -n hebrew get deploy hebrew -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
```

4. Check it, web only. The old image has no learner-model tool, so MCP is not part of this check.

```sh
BASE_URL=https://hebrew.ibhacked.us EXPECT_COMMIT=$OLD_SHA sh scripts/smoke-v3.sh
```

   Expected: `3 passed, 0 failed, 1 skipped`, exit 0 (`/healthz`, `/version` with the old commit, sign-in). Then sign in, open Today, answer one card, and confirm it saves.

5. Afterwards keep `hebrew.sqlite.after-v3-*` on the volume until you have decided what to do with it. It is a copy of the learner's v3 activity.

### 5.R Rehearsal on staging (acceptance criterion ac-4)

Do this after step 3 passes. It uses the same commands as 5.1 on the staging names: namespace `hebrew-next`, Deployment and Application `hebrew-next`, volume `hebrew-next-data`, files under `apps/hebrew-next/manifests`. The pristine copy is `backups/pre-v3.sqlite` (loaded in 3.2). Its checksum is `SHA_REMOTE`.

1. **Diff 6.** `replicas: 0` and the old tag (the staging updater is paused since Diff 4). Wait until no pod is left.
2. Run the restore script from 5.1 step 2 with `-n hebrew-next`, `claimName: hebrew-next-data`, and `NAME=pre-v3.sqlite`. (The script checks `backups/$NAME`, so the same text works.)
3. **Diff 7.** `replicas: 1`. Staging now runs the old image on the restored copy.
4. `BASE_URL=https://hebrew-next.ibhacked.us EXPECT_COMMIT=$OLD_SHA sh scripts/smoke-v3.sh` expects `3 passed, 0 failed, 1 skipped`. Sign in, open Today, answer one card.
5. Cleanup (security criterion: the staging copy of production data is deleted after the dry run). **Diff 8:** `replicas: 0` and the v3 tag. Wait until no pod is left. Create the `v3-shell` pod on `hebrew-next-data` again (the manifest in 3.2), then:

```sh
kubectl -n hebrew-next exec v3-shell -c shell -- sh -c '
  cd /data
  rm -f hebrew.sqlite hebrew.sqlite-wal hebrew.sqlite-shm hebrew.sqlite.after-v3-* backups/pre-v3.sqlite incoming.sqlite
  rmdir backups 2>/dev/null || true
  ls -la /data
'
kubectl -n hebrew-next delete pod v3-shell
```

   Then **Diff 9:** `replicas: 1`, and remove the staging `ignoreTags` lines so the updater resumes. Staging starts on an empty database. Check there are no snapshots of `hebrew-next-data` that were taken while it held production data (Longhorn snapshots and backups), and delete any you find. `longhorn-fast` is `Retain`, and deleting a file does not scrub the blocks.
6. Confirm the file is gone: the `ls -la /data` above should list no `.sqlite` files other than what the new pod creates later.

If 5.R passes, the rollback in 5.1 is rehearsed as far as a staging copy can show. The Argo CD commits, the restore script, and the old image on the restored file are all exercised. What it cannot show is production's own timing and load.

## Step 6. After the cutover

### 6.1 Make the placement clips (R9)

```sh
kubectl -n hebrew exec deploy/hebrew -c hebrew -- /usr/local/bin/hebrew audio-ensure --dry-run
```

Expected: `audio-ensure dry run: would_generate=206 skipped=0 total=206`. It needs no API key and calls no provider. Read `would_generate`. A text that has a clip only in some other voice is made again in the default voice (`docs/audio-ensure.md`), so a number other than 206 needs a look before you spend credits.

```sh
kubectl -n hebrew exec deploy/hebrew -c hebrew -- /usr/local/bin/hebrew audio-ensure
```

Expected: `audio-ensure: generated=206 skipped=0 failed=0 total=206`, with `audio-ensure: batch N of 21` lines on stderr (batches of at most 10, because the server's `WriteTimeout` is 30 s). The command prints counts only. It is safe to run again: stored clips are skipped, so a second run makes none. If `kubectl exec` drops, run it again. **STOP if** `failed` is more than 0: read the reason lines, then run it again to retry only those.

This spends ElevenLabs credits once. The pod already has `ELEVENLABS_API_KEY`. The command does not need the male voice: the placement clips are in the default voice.

Then make and listen to one dialogue line in the male voice, through the app, before you rely on it (SPEC-026 risk: only one direct test clip exists).

### 6.2 Verify

```sh
export BASE_URL=https://hebrew.ibhacked.us
export MCP_URL=https://hebrew-mcp.ibhacked.us/mcp
export MCP_TOKEN=$(vault kv get -field=mcp_token kv/apps/production/hebrew)
export SMOKE_LEARNER_ID=1     # your own learner id
export EXPECT_COMMIT=$V3_SHA
SMOKE_STRICT=1 sh scripts/smoke-v3.sh
kubectl -n hebrew exec deploy/hebrew -c hebrew -- /usr/local/bin/hebrew migrate --check
```

Expected: `7 passed, 0 failed, 0 skipped`. `applied: 1 2 3 4 5 6 7 8 9 10 11`, `pending: (none)`, `result: ok`. Then sign in and do the manual checks from 3.4, including starting a placement.

Take a post-cutover backup now, with the running pod (the v3 image has the command):

```sh
kubectl -n hebrew exec deploy/hebrew -c hebrew -- /usr/local/bin/hebrew backup --to "/data/backups/hebrew-prod-post-v3-$(date -u +%Y%m%dT%H%M%SZ).sqlite"
```

Expected: `bytes=<N> sha256=<64 hex>`. Copy it out as in step 2.

For ongoing checks without the admin token, an `MCP_READONLY_TOKEN` bound to your learner id would do (`app.ConfigureMCPReadOnly`). It is not set today. That is an operator decision.

### 6.3 Turn on the SPEC-023 automation

The weekly report and the coach automation are SPEC-023. It is a blocker for this spec, and **`docs/ops/weekly-coach.md` does not exist on this branch.** When SPEC-023 lands, follow that file. Until then there is nothing to switch on. Do not invent the steps here. Do not turn the automation on before 6.2 passes. The automation uses the admin or static token: limits are 60 calls a minute and 100 coach notes a day per learner.

### 6.4 Watch the first daily job

The scheduler runs at 03:30 in `TZ` (the pod has no `TZ`, so the code default `America/Chicago` applies). It also runs a catch-up when the app starts, for up to 14 days. A catch-up writes today's state for the days it missed (SPEC-014 disclosure), so the first rows are approximate.

```sh
kubectl -n hebrew logs deploy/hebrew -c hebrew --since=24h | grep -E 'daily job|jobs'
```

Expected: a `daily job scheduled at=03:30 zone=America/Chicago` line at start, and the morning after, a `daily job day=<date> learners=<n> written=<n> ... events=<n> band_slips=<n>` line with no error. Then `get_progress` over MCP returns that day's snapshot.

**STOP and look if** there is no `daily job` line the morning after, if it logs an error, or if the volume is close to full (`/data` is 2 Gi).

### 6.5 Decide about auto-deploy

Diff 1 left the production updater paused. While it stays paused, nothing reaches production until someone changes `newTag`. If you resume it (remove the `ignoreTags` lines), every merge to `v3` rolls production, and every migration runs against the real database with no backup in between. See [Open decisions](#open-decisions).

### 6.6 Soak, then clean up

After the soak period you choose (a week is reasonable; the first daily jobs and a weekly report should have run):

- Delete the backup files from the production volume: `/data/backups/*` and any `hebrew.sqlite.after-v3-*`, from a `v3-shell` pod on `hebrew-v2-data`.
- Keep the copy in `$BK` as long as you want the option of a restore. It holds a learner's whole history. Move it to encrypted storage, or delete it.
- Make sure no staging copy of production data is left (5.R step 5).

## Pre-push checklist

Check each item before `git push origin v3`. The push publishes an image, and staging rolls on it. Production must not.

- [ ] Step 1 ran today. The production image is still the one this plan assumes, and the volume names match.
- [ ] `git status` is clean on `v3`, and `git log origin/v3..v3 --oneline` is the INIT-015 line you expect, with the four `origin/v3` PRs merged in (head 2ac89b9 or later).
- [ ] SPEC-023 and SPEC-024 are on `v3`. They block this spec. At the time of writing, `docs/ops/weekly-coach.md` does not exist on this branch, so SPEC-023 is not here yet.
- [ ] The full suite passes on the commit you will push: `gofmt -l .` is empty, `go vet ./...` is clean, `go run ./cmd/tokens -check` is ok, `go test ./...` is all ok (Docker: `golang:1.26`).
- [ ] The security review for the line has run, or you have decided to push without it. The SPEC-021 summary says it is scheduled for the whole line and had not run. SPEC-025 has `security_review: required`.
- [ ] This runbook's pull request was merged by the Hebrew PR Reviewer. The operator did not approve it.
- [ ] CI for `v3` runs on `k8-build` only and publishes only `ghcr.io/networkbuild3r/hebrew` (`grep -n 'runs-on' .github/workflows/ci.yml` shows `k8-build`, and `scripts/check-image-name.sh --self-check` passes). No GitHub-hosted runner is involved.
- [ ] v2 does not publish: its CI builds and tests but pushes no image (PR #92). One branch publishes `sha-` tags at a time.
- [ ] **`home_k3` Diff 1 and Diff 2 are merged, and 4.5 passes:** the production `ImageUpdater` shows `["*"]`, production still runs the old tag, and the staging Secret has the male voice key.
- [ ] The male voice id is in Vault (4.0).
- [ ] You have a way back for the database even though the step 2 backup cannot exist yet: take a Longhorn snapshot of `hebrew-v2-data` (recommended).
- [ ] You know who watches R4 to R9, and when. The production pod is down for the migration time plus the image pull and app start (3.3 measures it).
- [ ] The Open decisions below have answers.

## Open decisions

1. **Auto-deploy after the cutover.** Keep the production updater paused and pin by pull request (safe, manual), or resume it (every merge to `v3` deploys to production and migrates the real database)? Today's rule is that the Hebrew PR Reviewer merges, so every reviewer merge would be a production deploy. This runbook assumes paused.
2. **Dry-run method.** Real staging with a copy of production data (this runbook, six staging commits in `home_k3` and a cleanup), or a scratch pod with an `emptyDir` (fewer commits, no copy on a `Retain` volume, but no sign-in, so no signed-in checks on real data)?
3. **Clips on staging.** Run `audio-ensure` on staging (206 clips of ElevenLabs credits, once more than production needs) so the placement start can be checked there, or skip that one manual check on staging and check it on production after the cutover?
4. **Where the backup lives, and for how long.** `$BK` on your machine is an operator-only directory. Decide on encrypted off-machine storage and a retention time. The file holds a learner's whole history.
5. **The security review.** Push before or after the line's security review? It is required for this spec and had not run.
6. **SPEC-023 and SPEC-024.** This spec is blocked by both. Do you cut over without them? The runbook assumes they are merged.
7. **The male voice.** Confirm the voice id is the one you want with `eleven_v3`. It has only been tried directly.
8. **A read-only smoke token.** Set `MCP_READONLY_TOKEN` and `MCP_READONLY_LEARNER_ID` so the smoke script can run without the admin token? It needs a Vault key, a secret, an env entry, and another `home_k3` diff that this runbook does not write.
9. **A window.** The cutover takes production down for the migration time (measured in 3.3) plus the pull and start. Pick a time when you are not studying.
10. **Staging updater.** After 5.R, the staging updater resumes (Diff 9), and staging follows every `v3` merge, which is the point of a staging stage. Keep it that way?

## What is proven and what is not

Proven (on 2026-10-19, on a Windows host with Go in Docker, against temp files only; no real database was touched):

- `hebrew migrate --check` and `hebrew backup --to`: unit tests in `internal/db/inspect_test.go` and `cmd/hebrew/ops_test.go`, and by mutation (every mutation that matters failed a test; one was equivalent). `migrate --check` on a database at versions 1 to 5 and 7 reported `pending: 006 008 009 010 011` and then, after the v3 binary opened it, `applied: 1..11`.
- The old image's code (7ad62e8) opens a v3-migrated database without error and then fails on `cards.dir`. Restoring the pre-migration copy fixes it.
- `scripts/smoke-v3.sh` against a local server built from this branch (temp database, no TTS key, a throwaway token that was never written to disk or a log): it passes 7 of 7, and it fails for a wrong port, a wrong token, a wrong commit, and a missing learner id. The token never appeared in a `curl` argument. `shellcheck` is clean. The script was mutation-checked (a token on the command line, an exit status that is always 0, and two weakened checks were each detected).
- The three `home_k3` diffs apply in order to the files at `ececb90c`, and the `home_k3` CI check passes with them and fails without the check edit.

Not proven:

- **Everything that touches the cluster.** No `kubectl`, Vault, Argo CD, Longhorn, or External Secrets command here was run. The commands, the pod manifests, the same-node attach of a second pod to a ReadWriteOnce Longhorn volume, `kubectl cp` through `busybox`, `ignoreTags` in the installed Image Updater, and the Argo CD behaviour (selfHeal reverting a scale) come from reading the repos and the READMEs.
- **ac-2.** The smoke script has not run against staging with a copy of production data. That needs a real backup, which does not exist yet.
- **ac-4.** The rollback has not been rehearsed on staging. The restore mechanics (file swap, old code on the restored file) were shown locally, but the Argo CD commits and the Kubernetes parts were not.
- **The migration on real data.** 006 to 011 have only run on temp databases built by the tests. Their first run on a copy of the real file is step 3.3.
- **Timings.** Migration time, outage length, and `audio-ensure` duration are unknown.
- **The signed-in flows** (sign in, Today, a review, a placement start). The smoke script cannot do them. They are manual checks in 3.4 and 6.2.
- **The male voice through the app.**
