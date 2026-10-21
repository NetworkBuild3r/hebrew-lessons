# SPEC-011 AC verification

**Verifier:** orchestrator (not the author). Re-read `Dockerfile`, `docker-compose.yml`, `.cursor/mcp.json`, `server/start.js`, `mcp/http.js`, and `deploy/k8s-database.md`. Re-ran the live checks below. No secret values were printed.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=required FAIL · domain=infrastructure FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | `docker compose exec hebrew id -un` printed `node`. `./data/hebrew.sqlite` exists on the host. |
| ac-2 | PASS | `git check-ignore` matches `data/hebrew.sqlite` via `.gitignore` `data/`. The worker's image listing reported no `.env` and an empty `/app/data` in a container without the bind mount. Compose does not copy those into the image (`.dockerignore` already lists both). |
| ac-3 | PASS | `deploy/k8s-database.md` has PVC `hebrew-data` at `/data/app`, `DATA_DIR`, `fsGroup: 1000`, `COOKIE_SECURE=1`, `PUBLIC_ORIGIN`, `replicas: 1`, `Recreate`, first-admin `npm run user`, backup and restore, and `audio:inventory` before rollout. The ElevenLabs key appears only as `secretKeyRef` name `hebrew-secrets`. |
| ac-4 | PASS | Compose still has the lessons path only as the commented `LESSONS_DIR: /data/worksheets`. The k8s doc keeps that mount. |
| ac-5 | PASS | `curl http://127.0.0.1:8080/` returned `302` to `/login/?next=%2F`. |
| ac-6 | PASS | No token → 401. Wrong token → 401. The container's own token, used inside the container and not printed, got past auth (`400` on an empty body, not `401`). `node server/start.js` with an empty token exited 1 with the missing-token message. Calling `list_users` from Cursor is still an operator check after `MCP_AUTH_TOKEN` is set in `.env`. |

Windows bind-mount: `USER node` wrote `./data` without a compose `user:` override.

No remediation loop.
