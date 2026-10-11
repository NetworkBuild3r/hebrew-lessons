# Persistent lessons volume (home_k3)

Lessons written by the MCP must survive image rebuilds. The container image still ships seed lessons under `/app/worksheets`. At runtime, set `LESSONS_DIR` to a mounted volume and let `server/start.js` copy any missing seed folders into that volume on boot.

## Manifest change (apply in the home_k3 repo)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: hebrew-lessons
  namespace: <hebrew-namespace>
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Gi
---
# On the hebrew Deployment / Pod template:
spec:
  containers:
    - name: hebrew
      env:
        - name: LESSONS_DIR
          value: /data/worksheets
        - name: HEBREW_ROOT
          value: /app
        # Vault/ExternalSecrets should sync into this Secret. Key name must match.
        - name: ELEVENLABS_API_KEY
          valueFrom:
            secretKeyRef:
              name: hebrew-secrets
              key: ELEVENLABS_API_KEY
              # If the vault key is 11_labs_api_key, either rename in the Secret or add:
              # optional: true
        # First admin. Read only when the database has no admin user.
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
        # Plain env, not a Secret. ladder (default) or fsrs.
        # Full Deployment env lives in deploy/k8s-database.md (INIT-006/SPEC-020).
        - name: SRS_SCHEDULER
          value: ladder
      volumeMounts:
        - name: lessons
          mountPath: /data/worksheets
  volumes:
    - name: lessons
      persistentVolumeClaim:
        claimName: hebrew-lessons
```

Word audio is stored under `LESSONS_DIR/_audio/words/` on this volume so clips survive image rebuilds and are shared across lessons. Never bake `.env` or the API key into the container image (`.dockerignore` already excludes `.env`).

## Before the first deploy that enables the volume

If a lesson exists only on the current pod (for example שגרה 1 — הבוקר שלי), pull it into git first:

```bash
# Against the live MCP, with a token:
MCP_AUTH_TOKEN=... node scripts/rescue-lesson.js <slug> https://<mcp-host>
git add worksheets/<slug> worksheets/catalog.js
git commit -m "Rescue <slug> before persistent volume cutover"
```

After the volume is mounted, MCP writes go to `/data/worksheets` and the next image push will not wipe them.

Pictures for lessons are not stored on this volume. They live on the data PVC `hebrew-data` at `DATA_DIR/media/` and `DATA_DIR/media/_archive/` (`/data/app/media` when `DATA_DIR=/data/app`). `.mp3` recordings stay here. The Deployment in `deploy/k8s-database.md` names `OPENAI_API_KEY` as a `secretKeyRef` on Secret `hebrew-secrets` and does not contain a value. Do not add a second replica. Do not change `strategy: Recreate`.
