# Database volume and single-replica layout (home_k3)

SQLite and session data live on a **second** PVC, separate from lessons and audio. Do not remount or move the existing `hebrew-lessons` volume at `/data/worksheets`.

Apply these changes in the home_k3 repo. Do not bake `.env` or API keys into the image.

## PersistentVolumeClaim (database)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: hebrew-data
  namespace: <hebrew-namespace>
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Gi
```

## Deployment (additions — keep lessons mount unchanged)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hebrew
  namespace: <hebrew-namespace>
spec:
  replicas: 1
  strategy:
    type: Recreate
  template:
    spec:
      securityContext:
        # node user on node:22-alpine is uid/gid 1000
        fsGroup: 1000
      containers:
        - name: hebrew
          securityContext:
            runAsUser: 1000
            runAsGroup: 1000
            runAsNonRoot: true
          env:
            - name: HEBREW_ROOT
              value: /app
            - name: DATA_DIR
              value: /data/app
            - name: LESSONS_DIR
              value: /data/worksheets
            - name: COOKIE_SECURE
              value: "1"
            - name: PUBLIC_ORIGIN
              value: https://<public-hebrew-host>
            - name: MCP_AUTH_TOKEN
              valueFrom:
                secretKeyRef:
                  name: hebrew-secrets
                  key: MCP_AUTH_TOKEN
            - name: ELEVENLABS_API_KEY
              valueFrom:
                secretKeyRef:
                  name: hebrew-secrets
                  key: ELEVENLABS_API_KEY
            - name: OPENAI_API_KEY
              valueFrom:
                secretKeyRef:
                  name: hebrew-secrets
                  key: OPENAI_API_KEY
            - name: ADMIN_USERNAME
              valueFrom:
                secretKeyRef:
                  name: hebrew-secrets
                  key: ADMIN_USERNAME
            - name: ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: hebrew-secrets
                  key: ADMIN_PASSWORD
            - name: ELEVENLABS_VOICE_ID
              value: "6WRlPmU1fqPwYbSxeP16"
            - name: ELEVENLABS_MODEL
              value: eleven_v3
            # Plain env, not a Secret. ladder (default) or fsrs. See
            # docs/operations/language-program-rollout.md (INIT-006/SPEC-020).
            - name: SRS_SCHEDULER
              value: ladder
          volumeMounts:
            - name: lessons
              mountPath: /data/worksheets
            - name: app-data
              mountPath: /data/app
      volumes:
        - name: lessons
          persistentVolumeClaim:
            claimName: hebrew-lessons
        - name: app-data
          persistentVolumeClaim:
            claimName: hebrew-data
```

`replicas: 1` and `strategy: Recreate` are hard limits: SQLite plus in-memory throttling do not support multiple writers. Do not add a second replica. Do not change `strategy: Recreate`. `fsGroup` and `runAsUser` stay `1000`. The process already runs as that user and creates its own directories on this volume.

Generated images use this same `hebrew-data` PVC. With `DATA_DIR=/data/app`, live files are `/data/app/media/` and files kept before a delete are `/data/app/media/_archive/`. Do not add a second volume for media, and do not put media on `hebrew-lessons`. Recordings (`.mp3`) stay on the lessons volume.

`OPENAI_API_KEY`, the ElevenLabs key, `ADMIN_USERNAME`, and `ADMIN_PASSWORD` are injected only via `secretKeyRef` (Secret name `hebrew-secrets`, keys with those same names). This document names `OPENAI_API_KEY`. It does not contain a value. The image does not contain it. The operator adds that key to the existing Secret in the cluster; do not replace the Secret, and do not write the value into git or into this file. `ADMIN_PASSWORD` is read only when the database has no admin; after the first boot, the database account is the one that signs in.

## Before the first rollout that enables the database volume

On the **live** pod (before cutover), capture audio inventory so recordings that exist only on the live volume are not lost:

```bash
kubectl exec -it deploy/<name> -- npm run audio:inventory
```

Keep that output with the rollout notes.

## First admin

There is no public sign-up. Create the first admin inside the pod:

```bash
kubectl exec -it deploy/<name> -- npm run user -- create <username> --admin
```

## Backup

Two layouts on this same `hebrew-data` volume (INIT-006/SPEC-020):

- `node scripts/backup-db.js` — WAL checkpoint, then one flat file `DATA_DIR/backups/hebrew-<ISO stamp>.sqlite`. Prints path and size only. Use for the language-program pre-rollout DB copy.
- `npm run db:backup` — one directory under `DATA_DIR/backups/hebrew-<stamp>/` with `hebrew.sqlite`, `media/`, and `media/_archive/`. Needs free space for a second copy of `media/`.

Neither command reads or requires `.mp3` files on the lessons volume, and neither moves or deletes them.

```bash
kubectl exec -it deploy/<name> -- node scripts/backup-db.js
kubectl exec -it deploy/<name> -- npm run db:backup
# Copy the whole media bundle directory (database and both media trees from the same run):
kubectl cp <namespace>/<pod>:/data/app/backups/hebrew-<stamp> ./hebrew-<stamp>
```

## Restore

1. Stop writers (scale to zero or use a maintenance window — still `replicas: 1`). Do not delete anything under `/data/worksheets`.
2. Copy that same backup directory back onto the data volume.
3. With the process stopped, remove leftover `hebrew.sqlite-wal` and `hebrew.sqlite-shm` so SQLite cannot replay them over the restored file. Replace `/data/app/hebrew.sqlite` with the backup's `hebrew.sqlite`. Replace `/data/app/media` with the backup's `media` tree (it includes `media/_archive`).
4. Start the pod. `npm run db:check` should print `ok`. Confirm the files under `/data/app/media` match this backup, and that an approved image still resolves.

```bash
kubectl cp ./hebrew-<stamp> <namespace>/<pod>:/data/app/backups/hebrew-<stamp>
kubectl exec -it deploy/<name> -- sh -c 'rm -f /data/app/hebrew.sqlite-wal /data/app/hebrew.sqlite-shm && cp /data/app/backups/hebrew-<stamp>/hebrew.sqlite /data/app/hebrew.sqlite && rm -rf /data/app/media && cp -a /data/app/backups/hebrew-<stamp>/media /data/app/media'
```

Confirm the app comes back healthy and sign-in works before ending the maintenance window. The `rm` and `cp` above apply only to the database file and `DATA_DIR/media` on `hebrew-data`.

## Lessons volume (unchanged)

The existing `hebrew-lessons` PVC remains mounted at `/data/worksheets` with `LESSONS_DIR=/data/worksheets`. See `deploy/k8s-lessons-volume.md`. Do not change that mount path.
