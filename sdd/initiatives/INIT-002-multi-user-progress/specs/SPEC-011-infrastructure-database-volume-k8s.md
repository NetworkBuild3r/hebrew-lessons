# SPEC-011 · Infrastructure: Database volume, non-root container, k8s layout

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-002
title: Database volume, non-root container, k8s layout, MCP auth required
domain: infrastructure
status: complete
primary_prompt: .claude/agents/principal-devops-engineer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []   # no deploy here; the k8s rollout is a separate operator `deploy`
reversibility_class: revertible
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-003, SPEC-004, SPEC-008]
blocks: [SPEC-018]
```

---

## Description

**Context:**
- **Local:** `docker-compose.yml` bind-mounts `.:/app`, publishes 8080 and 8787 on loopback, and reads `.env`.
- **Image:** the Dockerfile runs as root, with no `USER` line (INIT-001 LOW-1).
- **k8s:** the deployment mounts PVC `hebrew-lessons` at `/data/worksheets`, with `LESSONS_DIR=/data/worksheets` (`deploy/k8s-lessons-volume.md`).

The database needs its own persistent location, outside every served root, and the lessons-and-audio volume must not be remounted or moved.

**Scope (in):**
- `Dockerfile`: create `/app/data` owned by `node` and add `USER node`. The image must not contain `.env` or `data/` (both stay in `.dockerignore`).
- `docker-compose.yml`:
  - Add `DATA_DIR: /app/data` (inside the existing bind mount; `data/` is gitignored), `COOKIE_SECURE: "0"`, and `PUBLIC_ORIGIN: http://localhost:8080`.
  - Keep loopback-only port publishing.
  - **MCP auth required (closes INIT-001 MED-1).** Because the MCP can now assign lessons (REQ-016), remove `MCP_ALLOW_NO_AUTH` and pass `MCP_AUTH_TOKEN: ${MCP_AUTH_TOKEN}` from `.env`. Startup must fail with a clear message if the token is missing.
  - `.cursor/mcp.json`: send the token as a bearer header by environment-variable reference. The token value is never written into the file, and the file is not read or printed while doing this.
- `deploy/k8s-lessons-volume.md` (or a new `deploy/k8s-database.md`):
  - **Storage:** a second PVC, `hebrew-data`, mounted at `/data/app`, with `DATA_DIR=/data/app` and `fsGroup` set for the `node` user.
  - **Cookies and origin:** `COOKIE_SECURE=1`, plus `PUBLIC_ORIGIN` set to the public URL.
  - **Single replica:** `replicas: 1`, with `strategy: Recreate` (one replica is a hard limit: SQLite plus in-memory throttling).
  - **First admin:** create it with `kubectl exec -it deploy/<name> -- npm run user -- create <username> --admin`.
  - **Backup:** `kubectl exec … npm run db:backup`, then copy the file out.
  - **Before the first rollout:** run `npm run audio:inventory` on the live pod and keep the output, because the live volume may hold recordings the repo doesn't.
- Local check: `docker compose up -d --build` starts, and the container user is `node`.

**Scope (out):**
- Applying anything to the cluster: that is a separate operator-initiated `deploy`.
- OAuth changes to the MCP.
- The CI image workflow, which keeps its current trigger.

---

## Understood (agent's read of the intent)

- [ ] und-1: The lessons PVC and its audio stay exactly where they are. The database gets a separate volume.
- [ ] und-2: The ElevenLabs key is still injected from `.env` locally, and from a `secretKeyRef` in k8s. It is never baked into the image.

## Acceptance Criteria

- [ ] ac-1 (REQ-013): `docker compose exec hebrew id -un` prints `node`. The server starts and can write `DATA_DIR`.
- [ ] ac-2 (REQ-013): after `docker compose up -d --build`, the database file exists under `./data/` on the host. `git status` does not list it, and `docker image inspect` layers do not include `data/` or `.env`. The second claim is checked by listing `/app` in a fresh container started without the bind mount.
- [ ] ac-3 (REQ-013): the k8s doc lists the second PVC, `DATA_DIR`, `COOKIE_SECURE=1`, `PUBLIC_ORIGIN`, `replicas: 1` with `Recreate`, first-admin bootstrap, backup and restore, and the pre-rollout audio inventory step. The ElevenLabs key is referenced only by `secretKeyRef` name, never by value.
- [ ] ac-4 (REQ-012): the lessons volume mount path and `LESSONS_DIR` are unchanged in every file.
- [ ] ac-6 (REQ-016): in the rebuilt container, an MCP request with no token or a wrong token gets 401. The container refuses to start when `MCP_AUTH_TOKEN` is unset. With the right token, the operator's AI can call `list_users` (checked by the operator in Cursor after a restart).
- [ ] ac-5: signed out, `curl -i http://127.0.0.1:8080/` against the rebuilt container returns 302 to `/login/`.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Bind-mount ownership on Windows Docker Desktop | bind mounts are writable by the container user; if not, compose sets `user:` to match | ambiguous | no — verify during execution |

---

## Deliverables

- [ ] `Dockerfile`, `docker-compose.yml`, `.dockerignore`, `.gitignore`, `.cursor/mcp.json` (header by env reference only)
- [ ] `deploy/k8s-lessons-volume.md` and/or `deploy/k8s-database.md`

---

## Technical Requirements (domain-specific)

- [ ] No secret values in any committed file; reference env var names only.
- [ ] Keep `node:22-alpine` pinned.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none. A local container rebuild is not a deploy.
- **Blocking checks:** local `docker compose up -d --build`, then ac-1, ac-2, and ac-5. `npm test` on the host.
- **Rollback plan:** revert the listed files and rebuild. `./data/` can be deleted to reset.

---

## Security

- **Sensitivity:** required. Triggers matched: container privilege, secrets injection, cookie `Secure`, persistent storage of PII.
- **Review depth:** full. Security acceptance: ac-1, ac-2, ac-3, and ac-6.

---

## Verification Strategy

- **Claim:**
  - The container runs as non-root.
  - The database persists outside the image and outside git.
  - The k8s guide leaves the lessons volume untouched and documents a single-replica, TLS-cookie deployment.
- **Check + executor:**
  - `mechanized`: the shell commands in ac-1, ac-2, ac-4, and ac-5 against the local container.
  - `human`: the operator reads the k8s guide against the ac-3 checklist.
- **Pass condition:** ac-1, ac-2, ac-4, and ac-5 commands produce the stated outputs. The operator ticks every ac-3 checklist item as present in the doc.

---

## Integration Points

- Consumed by SPEC-018 (operator guide), and by the operator's later k8s rollout.
- **Provenance tag:** `INIT-002/SPEC-011`
