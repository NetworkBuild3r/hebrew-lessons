# Spike decision — walking skeleton

**Initiative:** INIT-003
**Spec:** SPEC-010
**Date:** 2026-10-08
**Source:** `spikes/skeleton/RESULTS.md`

## Decision

Adopt the spike as INIT-004's base.

`spikes/skeleton/ops.js`, `validate.js`, and the render and escape behavior are the starting point for `server/plans/` and `shared/ui/`. The production modules are a port into the paths ADR-0005 §5 names. The running site does not import `spikes/`.

## ADR amendments

None.

`RESULTS.md` says the hostile page did not execute a script or inject a tag, so the spike does not justify moving off ADR-0005 Option A. Recall stays the ADR-0006 word-cluster state. The checkpoint stays `spaced-review-checkpoint`. The four picture ids in the spike validator (`coffee-cup`, `water-glass`, `milk`, `sugar`) are a spike catalog. They are not a schema change. INIT-004's validator must not copy that set.

## What is not adopted

- Listening on a public port. The spike bound `127.0.0.1:8099` for a local check. Production stays the existing server.
- A hardcoded media catalog inside the validator. Resolving a media id is the media store and the renderer, which refuse an id they cannot resolve.
- Deleting the spike in the first INIT-004 slice. It stays until the ported tests cover the same cases. `spikes/` stays out of the Docker image and out of served folders, as SPEC-009 already required.

## Rewrite or delete

Not rewritten from scratch. Not deleted.
