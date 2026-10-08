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
            - name: ELEVENLABS_VOICE_ID
              value: "6WRlPmU1fqPwYbSxeP16"
            - name: ELEVENLABS_MODEL
              value: eleven_v3
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

`replicas: 1` and `strategy: Recreate` are hard limits: SQLite plus in-memory throttling do not support multiple writers.

The ElevenLabs key is injected only via `secretKeyRef` (Secret name `hebrew-secrets`, key `ELEVENLABS_API_KEY`). Never put the key value in this document or in the Deployment YAML.

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

```bash
kubectl exec -it deploy/<name> -- npm run db:backup
# Then copy the backup file out of the pod (path under DATA_DIR/backups/):
kubectl cp <namespace>/<pod>:/data/app/backups/<backup-file> ./<backup-file>
```

## Restore

1. Stop writes (scale to zero or use a maintenance window — still `replicas: 1`).
2. Copy a backup file into the pod under `/data/app/` (or replace `hebrew.sqlite` after stopping the process).
3. Restore by replacing the live database file with the backup, then start the pod again and verify with `npm run db:check` if available.

```bash
kubectl cp ./<backup-file> <namespace>/<pod>:/data/app/backups/<backup-file>
kubectl exec -it deploy/<name> -- sh -c 'cp /data/app/backups/<backup-file> /data/app/hebrew.sqlite'
```

Confirm the app comes back healthy and sign-in works before ending the maintenance window.

## Lessons volume (unchanged)

The existing `hebrew-lessons` PVC remains mounted at `/data/worksheets` with `LESSONS_DIR=/data/worksheets`. See `deploy/k8s-lessons-volume.md`. Do not change that mount path.
