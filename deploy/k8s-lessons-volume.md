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
