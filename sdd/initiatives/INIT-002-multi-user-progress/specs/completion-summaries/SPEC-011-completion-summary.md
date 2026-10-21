# SPEC-011 Completion Summary — Container, database volume, MCP auth

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-011 · Database volume, non-root container, k8s layout, MCP auth required
**Status:** complete
**Provenance:** INIT-002/SPEC-011

## Change

The container runs as the `node` user. The database lives in `./data`, which git ignores. The process refuses to start without `MCP_AUTH_TOKEN`, and the Cursor MCP client sends that token by environment reference only.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`. No cluster deploy.
- Pre-flight: gate_actions PASS · security_review=required FAIL · domain=infrastructure FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. See `SPEC-011-eval-report.md`.

## Security

No token or API key was written into the repo. `.cursor/mcp.json` uses `Bearer ${env:MCP_AUTH_TOKEN}`. The ElevenLabs key stays in `.env` and, in the k8s guide, in a `secretKeyRef`. No Critical or High finding.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit. No `kubectl apply`. `MCP_AUTH_TOKEN` is not in `.env` yet. The running container was started with a temporary shell value. The next `docker compose up` from a shell that does not have that variable will refuse to start until the operator sets it in `.env`.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-011](095ecb73-7dfa-4e1a-8d87-1b691f7fe67f).
