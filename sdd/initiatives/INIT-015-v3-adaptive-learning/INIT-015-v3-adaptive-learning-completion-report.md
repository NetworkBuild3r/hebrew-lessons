# INIT-015 Completion Report: v3 Adaptive Learning

**Initiative:** INIT-015 · `INIT-015-v3-adaptive-learning`
**Closed:** 2026-10-19 (Mode E, `complete-initiative`)
**Branch:** `v3` (local `844f16d`, pushed to `origin/v3`). `v3` shares no history with `main` (ADR-0018 D6). SDD records live in the `main` checkout, untracked, per the operator workflow.
**Deployed baseline:** `origin/v3` at `9e58365`. **Delta:** 60 commits, 396 files, +82,251 and -7,973 lines.

## Outcome

**25 of 26 specs are `complete`. SPEC-025 (cutover to production, `plan_only`) is `in_progress`:** the runbook, smoke script, `hebrew migrate --check` and `hebrew backup --to` shipped, but its two acceptance criteria that need the operator (ac-2, a smoke run against staging with a copy of production data; ac-4, a rehearsed rollback) are **not done**. Nothing was run against the cluster or a real database. The initiative is complete as a build. It is not cut over.

Acceptance criteria: **116 in total. 109 met, 4 partial (SPEC-011 ac-1, SPEC-012 ac-1 and ac-7, SPEC-017), 3 open (SPEC-010 ac-4 Figma frame; SPEC-025 ac-2 and ac-4).** Each gap is disclosed in the spec's completion summary.

What shipped:

1. **The line (SPEC-001, SPEC-002):** ADR-0018 and `docs/north-star.md`; `v3` cut, CI moved to it, publishing `ghcr.io/networkbuild3r/hebrew` only.
2. **Evidence and learner model (SPEC-003, 004, 005, 013, 014):** attempts, exposures and skill-directional cards; one attempt recorder with one grading rule; learner-model tables; states and diagnosis; a daily snapshot and regression job on an in-process scheduler.
3. **Placement (SPEC-006, 007, 011, 012):** a 200-word verified deck with 206 audio clips (manifest, not yet generated on the cluster); a real placement engine with no feedback until the review (D1) and only right answers becoming cards (D2); a Hebrew on-screen keyboard; the placement screens.
4. **Design system and shell (SPEC-008, 009, 010, 016, 017, 018, 024):** Lamplight tokens (generated, contrast-checked), a motion system with reduced-motion fallbacks, icons and illustrations, activity screens and the review card, the app shell (Today with one next action, five-destination dock and rail, lesson frame, report path), Progress per skill with table fallbacks, and the visual and usability regression checks.
5. **Adaptive lessons (SPEC-015, 019, 020, 021, 022, 023, 026):** a skill-aware activity engine; `LessonBrief`; brief checks and auto-publish safety (D5, no human step); learner and coach MCP tools with roles and rate limits; one authoring skill for every AI; a weekly report and the coach automation definition; two-voice dialogue (D8, female and male).
6. **Cutover plan (SPEC-025):** `docs/ops/v3-cutover.md`, `scripts/smoke-v3.sh`, `hebrew migrate --check`, `hebrew backup --to`.

## Session & Timer Metrics (Phase 2)

No timer file exists for this initiative (`specs/.sdd-timer.json` was never created; `initiative_inventory_check.py` reports `NO TIMER FILE`). Work ran as general-purpose sub-agents in git worktrees. Per-spec session minutes were not captured, so none are reported here. Wall-clock span: all 60 commits are dated 2026-10-19 (the initiative was created that day).

## Requirements Traceability

Source: the spec files' acceptance criteria (`[x]` met, `[~]` partial, `[ ]` open).

| Spec | Domain | Title | Status | AC met | Partial | Open |
|------|--------|-------|--------|--------|---------|------|
| SPEC-001 | docs | ADR-0018: the v3 line and the north-star decisions | complete | 3 | 0 | 0 |
| SPEC-002 | infrastructure | Cut v3 and move CI to it | complete | 4 | 0 | 0 |
| SPEC-003 | data | Evidence tables and card skills | complete | 4 | 0 | 0 |
| SPEC-004 | data | Learner-model tables | complete | 3 | 0 | 0 |
| SPEC-005 | service | One attempt recorder and the grading rule | complete | 4 | 0 | 0 |
| SPEC-006 | data | Placement deck: 200 verified words with audio | complete | 4 | 0 | 0 |
| SPEC-007 | service | Placement engine v3 | complete | 7 | 0 | 0 |
| SPEC-008 | frontend | Lamplight tokens and components | complete | 4 | 0 | 0 |
| SPEC-009 | frontend | Motion system with reduced-motion fallbacks | complete | 4 | 0 | 0 |
| SPEC-010 | frontend | Icons, brand mark, and illustrations | complete | 3 | 0 | 1 |
| SPEC-011 | frontend | Hebrew on-screen keyboard | complete | 3 | 1 | 0 |
| SPEC-012 | frontend | Placement screens | complete | 5 | 2 | 0 |
| SPEC-013 | service | Learner model: states and diagnosis | complete | 4 | 0 | 0 |
| SPEC-014 | scheduled | Daily snapshot and regression job | complete | 4 | 0 | 0 |
| SPEC-015 | service | Activity engine and skill-aware session mix | complete | 5 | 0 | 0 |
| SPEC-016 | frontend | Activity screens and the review card | complete | 5 | 0 | 0 |
| SPEC-017 | frontend | App shell: Today, dock and rail, lesson frame | complete | 5 | 1 | 0 |
| SPEC-018 | frontend | Progress per skill | complete | 4 | 0 | 0 |
| SPEC-019 | service | LessonBrief | complete | 4 | 0 | 0 |
| SPEC-020 | service | Brief checks and auto-publish safety | complete | 5 | 0 | 0 |
| SPEC-021 | api | MCP tools for the learner model and briefs | complete | 5 | 0 | 0 |
| SPEC-022 | docs | One authoring skill for every AI | complete | 3 | 0 | 0 |
| SPEC-023 | scheduled | Weekly report and the coach automation | complete | 3 | 0 | 0 |
| SPEC-024 | frontend | Visual and usability regression checks | complete | 4 | 0 | 0 |
| SPEC-025 | infrastructure | Cutover to production | in_progress | 2 | 0 | 2 |
| SPEC-026 | service | Two-voice dialogue (one male voice, one female voice) | complete | 8 | 0 | 0 |
| **Total** | | | | **109** | **4** | **3** |

Completion summaries: 26 of 26, none thin (each is 31 lines or more).

## Verification evidence

- **Go suite:** `go test ./...` passes in every package on the merged tree (`gofmt` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `templ generate` leaves no diff). About 1,040 test functions.
- **Browser suites (headless Chromium, Playwright 1.49.1):** Shell, Placement, Dialogue, Motion, Activity and Progress, all passing on the merged tree at `b2b7d88`.
- **Regression matrix (SPEC-024):** `TestShots` captures 63 screens in light and dark at 360, 390, 1024 and 1440 px (504 PNGs) plus a reduced-motion pass, checked with axe-core 4.10.2, one gold action, no stray feedback colours, 44 px targets, no horizontal scroll and no running animation. It passed on the worker's own run on content identical to `844f16d`.
- **Mutation checks:** every spec's worker ran mutations and restored the tree (for example SPEC-018 about 65, SPEC-025 24, SPEC-024 six). The orchestrator re-ran mutations for the fixes in the security review.
- **CI on the pushed commit:** run [39365515384](https://github.com/NetworkBuild3r/hebrew-lessons/actions/runs/39365515384) was in progress at close; its result is not recorded here.
- **Not re-run at close:** the orchestrator's final full re-run of the merged tree was stopped by the operator. `b2b7d88` passed the full suite; `844f16d` adds only SPEC-024.

## Security Review (Phase 3.5, `INIT-015/SECURITY-REVIEW`, `SECSCAN-015`)

See `INIT-015-security-review.md`. **No unresolved Critical or High finding.** Four findings were fixed with tests (a limiter gap, a read-only token that could unpublish, an uncapped placement `Replace`, and a migration number collision with `origin/v3`). **The review was partial.** `internal/mcpstore`, `internal/units`, most web handlers and templates, and `internal/auth` cookie flags were not reviewed by any pass. This is a recorded gap, not an approved risk acceptance: no approver was named and no expiry set, so no risk acceptance exists.

## Architecture Diagrams (Phase 3.6, `INIT-015/DESIGN-DIAGRAMS`)

See `design/INIT-015-v3-adaptive-learning-architecture.md`: system context, container, data model, two sequences, data flow with trust boundaries, and deployment. Component and agent-topology diagrams were left out.

## Decisions and deviations

- **D1 to D8** are in the initiative doc. **D9 (new, recorded 2026-10-19):** migrations 007 to 010 became 008 to 011, because `origin/v3` shipped `007_oauth_client_issued.sql`. The next free number is 012. Older completion summaries (SPEC-004, 005, 007, 013, 014) still use the pre-D9 numbers; read them as +1 for 007 to 010.
- **SPEC-010 ac-4:** the "Brand & illustrations" Figma frame was not drawn (read-only Figma access). The SVGs were hand-written.
- **SPEC-012 ac-1:** on the pick screens the orb is the gold element and there is no `Primary` button. SPEC-024's gold rule now defines this ("orb" screens). ac-7 is by inspection only.
- **SPEC-017:** the Figma match is partial. The Today, flashcard and dark frames are out of date against the shipped app.
- **SPEC-018:** the Progress frames (`182:204`, `183:204`) were drawn before the code but never compared with the running app.
- **SPEC-022:** contradictions noted: brief `activities` names are not unit types; north-star says the validator needs a gloss and transliteration but the code checks only gloss; dialogue is omitted from some brief rule descriptions; prefix handling differs; step 6 needs `learnerId` for admin callers.
- **SPEC-023:** the spec's "learner OAuth token for the automation" cannot work (the brief and publish tools are admin-only). Authorization was not changed. The automation must stay off until the operator picks a credential.
- **SPEC-025:** `plan_only`; two ACs need the operator (see Outcome).
- **Dehydration skipped:** `scripts/dev/rehydrate_specs.py` is not in this repo, so the spec files and completion summaries stay in `specs/`. `render_report_pdf.sh` needs pandoc (see the results report for the render outcome).

## Challenges and what the process caught

- **Migration collision (D9):** found while merging `origin/v3`, before anything shipped. A duplicate version stops the app, and production already holds version 7. Fixed with a renumber and a regression test that upgrades from the deployed schema.
- **A 90-day chart showing 89 days (SPEC-018):** caught by a mutation, not by the original tests. The window started 89 days back but the job writes yesterday.
- **A 4 px sideways scroll at 360 px (SPEC-024):** the hidden `.vh` label overflowed its box on placement typing. Found by the no-scroll check, fixed in `base.css`.
- **A read-only token that could write, and an uncapped `Replace` (security review):** both fixed.
- **Rollback is one-way (SPEC-025):** the old image opens a migrated database and passes `/healthz`, but card queries fail with `no such column: dir`. Rollback is the previous tag plus the backup file, and everything done after the cutover is lost.
- **A dirty worktree after an aborted run:** an aborted verification run left seven tracked files rewritten with older content (in-place test mutations killed mid-run). The diff was saved to `%TEMP%\pw\dirty-worktree-after-abort.patch` and the tree restored to `HEAD` before the push.

## Gates and controls

- `db_migration` (SPEC-003, 004, 025): migrations 006 and 008 to 011 run only on temporary databases. **Never run against production or a copy of it.**
- `deploy` and `merge` (SPEC-025): not performed by the agents. The operator pushed `v3` on 2026-10-19, after being told that a push builds an image both Image Updaters deploy and that production would then migrate with no backup.
- `v3` **was merged into `main`** on 2026-10-19 at the operator's explicit request (asked twice), **overriding ADR-0018 D6** (`v3` never merges into `main`). The histories are unrelated, so a plain merge was impossible. The merge commit `1351938` has parents `c06f722` (old `main`) and `844f16d` (`v3`) and takes `v3`'s tree. It was pushed as a fast-forward (`c06f722..1351938`), with no force. Old `main` stays reachable as an ancestor (local tag `main-node-final`). `main`'s Node-era workflows are gone from the tree, so a push to `main` publishes nothing; `ci.yml` runs only on pushes to `v3` and on pull requests. ADR-0018 D6 and the initiative's constraint now disagree with what was done and need an edit or a superseding decision.
- CI never publishes `ghcr.io/networkbuild3r/hebrew-lessons`. No GitHub-hosted runners were used.

## What the operator must do before and after the cutover

1. **Production auto-deploys on the push.** Both Image Updaters track the same `sha-` tag, so the push of `844f16d` rolls production and staging. Pausing a host is `ignoreTags: ["*"]` in `home_k3` (`apps/hebrew/README.md`).
2. **Back up the production database** with `hebrew backup --to` from the v3 image in a one-off pod (the old image has no `backup`). Verify the checksum. Rehearse the 006 to 011 migration with `hebrew migrate --check` on a copy.
3. **Add `ELEVENLABS_VOICE_ID_MALE`** to the ExternalSecrets and the deployment env in `home_k3`. The `home_k3` CI check rejects that name until diff 2 of the runbook is applied.
4. **Generate the placement audio:** `hebrew audio-ensure --dry-run`, then the real run, in the pod. The 206 clips are not on the cluster. The six letter clips and the male voice have not been listened to.
5. **Choose the coach automation credential** (admin-role OAuth session or a server authorization change). Keep the automation off until then.
6. **Decide:** how to copy production data for a dry run (helper pod, Longhorn snapshot or a synthetic database); whether to rewrite existing `story` units as `dialogue`; whether sign-in should have a gold main action.

## Risks and follow-ups

- Review coverage gap (above). A follow-up pass over `internal/mcpstore`, `internal/units`, the web handlers and `internal/auth` closes it.
- Figma is behind the shipped app for Today, flashcards, dark variants and the Progress states.
- A reported shared unit still shows in the MCP outline until the web filter lands.
- The slip window can skew by up to a day (the page asks for the UTC date; the job writes days in `America/Chicago`).
- New learner copy is English only; the new Hebrew nav labels are not Pealim-verified. Hebrew pixels have not been reviewed by a person.
- Only Chromium was tested (no Firefox, WebKit or screen reader).
- The limiter is in-process, so it is not shared across replicas.
- Cluster commands in the runbook are unrun: the `kubectl` and Vault commands, a second pod attaching to the same ReadWriteOnce Longhorn volume, `kubectl cp` through busybox, and `ignoreTags` in the installed Image Updater. Migration time and outage length are unknown.
- Three older redundant screenshot tests (`TestShellScreenshots`, `TestActivityScreenshots`, `TestProgressScreenshots`) remain beside `TestShots`.
