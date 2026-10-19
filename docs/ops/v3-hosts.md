# v3 hosts and image tracking

This note is a handoff for the operator. It makes no change outside this repo.

## What CI does on v3

- CI on `v3` publishes `ghcr.io/networkbuild3r/hebrew:sha-<sha>`.
- Argo Image Updater deploys the newest `sha-` tag of that image.
- CI never publishes `ghcr.io/networkbuild3r/hebrew-lessons`.
- Only one branch may publish `sha-` tags at a time (ADR-0018, D6). v2 is frozen. Its CI builds and tests but does not push an image.

## Hosts

- `hebrew.ibhacked.us` and `hebrew-next.ibhacked.us` both answer today.
- Which host tracks the `hebrew` image is set in the `home_k3` repo.
- `home_k3` is outside this repo. This work does not edit it.

## Operator action requested

Please confirm in `home_k3`:

1. Which host or hosts track `ghcr.io/networkbuild3r/hebrew`.
2. Whether `hebrew-next.ibhacked.us` needs a change to follow v3.

Record the answer here or in SPEC-025.

## Status of this assumption

- This is assumption `aud-1` in INIT-015 SPEC-002.
- It does not block v3 CI.
- It blocks only SPEC-025 (the production cutover).

## Rollback

1. Revert the v2 pull request that stopped publishing. v2 then publishes again.
2. Delete the `v3` branch.

Do both in the same window. Otherwise two branches publish `sha-` tags at once.
