# Accounts and admin

Operator guide for first run, day-to-day accounts, lesson sharing, database backup, and audio safety. Commands below are for Windows PowerShell locally, and for `kubectl exec` on the cluster. Env var and k8s secret **names** only — never paste secret values into this file or into shell history.

Provenance: `INIT-002/SPEC-018`. Cluster layout details: [deploy/k8s-database.md](../../deploy/k8s-database.md).

## First run (local, PowerShell)

There is no public sign-up. Create the first admin before anyone can use the site.

1. Point `DATA_DIR` at a writable folder (not the lessons tree). Example:

```powershell
$env:DATA_DIR = "C:\path\to\app-data"
```

2. Create the admin. The password comes from a **TTY prompt** (no echo) or from `--password-file`. Never pass `--password=` on the command line (the CLI refuses it).

```powershell
npm run user -- create <name> --admin
```

Or with a password file (useful in scripts; delete the file afterward):

```powershell
npm run user -- create <name> --admin --password-file C:\path\to\password.txt
```

3. Start the app (for example `docker compose up --build`) and sign in at the site origin.

On Kubernetes, create the first admin inside the pod (TTY prompt):

```bash
kubectl exec -it deploy/<name> -- npm run user -- create <username> --admin
```

## Day to day

### Accounts (CLI)

```powershell
npm run user -- create <username>
npm run user -- create <username> --display-name "Display Name"
npm run user -- reset <username>
npm run user -- disable <username>
npm run user -- enable <username>
npm run user -- role <username> user|admin
npm run user -- list
```

`create` and `reset` take a password from a TTY prompt or `--password-file`, never `--password=` on the command line.

### Share lessons (admin UI)

Sign in as an admin. Open the **Lessons** tab. For each lesson set visibility to:

- **hidden** — not shown to learners
- **everyone** — all learners
- **selected** — only the learners you pick

New lessons stay hidden until you assign them (in this tab, or via MCP — see below).

## Backup and restore

### Local

With `DATA_DIR` set to the same folder the app uses:

```powershell
npm run db:backup
```

Backups land under `DATA_DIR/backups/` as files named like `hebrew-<timestamp>.sqlite`.

Check integrity:

```powershell
npm run db:check
```

To restore: stop the app (no writers), replace `DATA_DIR/hebrew.sqlite` with a copy of the backup file, then start again and run `npm run db:check`. Confirm sign-in works before treating the restore as done.

### Kubernetes

From [deploy/k8s-database.md](../../deploy/k8s-database.md):

```bash
kubectl exec -it deploy/<name> -- npm run db:backup
# Then copy the backup file out of the pod (path under DATA_DIR/backups/):
kubectl cp <namespace>/<pod>:/data/app/backups/<backup-file> ./<backup-file>
```

Restore (still `replicas: 1`; stop writes first — scale to zero or a maintenance window):

```bash
kubectl cp ./<backup-file> <namespace>/<pod>:/data/app/backups/<backup-file>
kubectl exec -it deploy/<name> -- sh -c 'cp /data/app/backups/<backup-file> /data/app/hebrew.sqlite'
```

Then start the pod again and verify with `npm run db:check` if available. Confirm the app is healthy and sign-in works before ending the maintenance window.

## Audio safety

Before regenerating lesson audio, or before a rollout that touches the live lessons volume:

```powershell
npm run audio:inventory
```

After generation (or to check the live tree against a saved inventory):

```powershell
npm run audio:verify
```

When a lesson clip is replaced, the previous bytes are kept at:

`<lesson>/audio/_archive/<stem>.<12hex>.mp3`

**Before a rollout that mounts or changes the live lessons volume**, run inventory on the **live** pod and keep the output with the rollout notes:

```bash
kubectl exec -it deploy/<name> -- npm run audio:inventory
```

## Environment variables (names only)

| Name | Role |
| --- | --- |
| `DATA_DIR` | SQLite and backups directory (`hebrew.sqlite`, `backups/`) |
| `COOKIE_SECURE` | `0` locally; `1` on HTTPS / k8s (Secure cookie + `__Host-` prefix) |
| `PUBLIC_ORIGIN` | Public site origin used for CSRF Origin checks |
| `MCP_AUTH_TOKEN` | Required for MCP assignment tools (`list_users`, visibility on `upsert_lesson`, `set_lesson_visibility`) |
| `ELEVENLABS_API_KEY` | TTS; never commit or bake into the image |

**Local compose:** `COOKIE_SECURE=0`, `PUBLIC_ORIGIN=http://localhost:8080`.

**Kubernetes:** `COOKIE_SECURE=1`, `DATA_DIR=/data/app`, `LESSONS_DIR=/data/worksheets`. `MCP_AUTH_TOKEN` and `ELEVENLABS_API_KEY` come from Secret `hebrew-secrets` via `secretKeyRef` (keys with those same names). Full Deployment snippet: [deploy/k8s-database.md](../../deploy/k8s-database.md). Do not paste secret values into docs or YAML.

## Privacy: who can see what

- **Admins** can see everything a learner does, including the full answer history.
- **Learners** are told this on the sign-in page.
- **The AI and MCP tools cannot** read progress or answer history.
- **`list_users`** returns only usernames and display names of active accounts (nothing else).

## Working with the AI (MCP)

Useful tools once `MCP_AUTH_TOKEN` is set:

- `list_users` — usernames and display names of active accounts
- `upsert_lesson` — optional `visibility`: `"everyone"` or `{ "users": ["username", ...] }`
- `set_lesson_visibility` — `"everyone"`, `"hidden"`, or `{ "users": ["username", ...] }`

If `MCP_AUTH_TOKEN` is unset or empty, those assignment tools refuse. A lesson created with no assignment stays **hidden** until you share it in the admin Lessons tab or call `set_lesson_visibility` / pass `visibility` on upsert.

## Known limits

- **One replica** — SQLite (and in-memory sign-in throttling) do not support multiple writers. Keep `replicas: 1` and `strategy: Recreate` on the cluster.
- **`file://` no longer works** — open the site through the HTTP(S) origin (local compose or the public host).
- **Hash-named word mp3s** under the shared words cache require sign-in, but are **not** filtered per lesson.
- **No Content-Security-Policy** yet.

## OAuth later

Columns `auth_provider` and `provider_subject` on `users` are reserved for a future OAuth/OIDC path. Do not design that flow here; local username/password remains the current path.
