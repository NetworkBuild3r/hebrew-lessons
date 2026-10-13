# ADR-0005: Stack and architecture

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-08 |
| **Authored by** | `INIT-003/SPEC-007` |
| **Initiative** | INIT-003 · lesson-system-foundations |
| **Research grounding** | `RSCH-004` and `RSCH-005` via [ADR-0003](ADR-0003-course-and-lesson-plan-language.md); `RSCH-006` for direction, contrast, target size, and reduced motion. None of those reports names a framework. |
| **Supersedes** | — |
| **Superseded by** | Superseded in part by [ADR-0007](ADR-0007-single-page-app-structure.md) — the page model only (one HTML document per signed-in screen). Option A (Node, no framework, no build step, `shared/ui/`, tokens) stays. Replacement wording is ADR-0007. The sentences in this file are unchanged. |
| **Extended by** | [ADR-0011](ADR-0011-ai-first-plan-builder.md), Accepted 2026-10-12: 5 (the one-implementation table gains rows for list, create, assign, revisions, diff, restore, and issues). |

## Summary

Keep the Node 22 server, `node:sqlite`, and the in-process MCP. Add the lesson-plan language as one operations module that both the MCP and the builder call, and add a design system as tokens plus tested functions that return escaped HTML. Do not add a front-end framework, a build step, or a second runtime.

Option B (Preact and htm, no build) scores two points higher on the weighted table. That gap is smaller than the lightest criterion's weight, so a one-point rescore reverses it. Between those two, this record takes the lower migration cost. Phoenix LiveView and a Next.js rewrite lose on the MCP SDK, the database, the tests, and the image, not on reputation. The operator confirmed Option A on 2026-10-08.

---

## Context

### Problem statement

The operator asked whether this is the right codebase, whether the CSS approach is right, and whether Phoenix or another stack would be better, including a ground-up rebuild if the criteria say so. The look itself is ADR-0006. This record chooses the runtime and the rendering approach that can carry ADR-0003's plan language and ADR-0004's media rules, maintained by one person with agents writing most of the code, on the existing one-replica Docker layout.

### Current state

Checked on 2026-10-08 against commit `2f1f0b0ad87ad940ed8593af6d4a6e5c05554f58`, which is both `HEAD` and `origin/main`. The uncommitted INIT-003 documents are not part of these counts.

| Fact | Evidence |
|------|----------|
| Runtime | `package.json` `start` is `node server/start.js`. `"type": "commonjs"`. One direct dependency, `@modelcontextprotocol/sdk` `^1.32.1`. |
| Process | `server/start.js` loads the environment, refuses a bad `DATA_DIR`, requires `MCP_AUTH_TOKEN`, opens SQLite, migrates, then calls `server/web.js` `listen()` and `mcp/http.js` `listen()` in the same process. |
| Ports | Web `PORT` or 8080 (`server/web.js`). MCP `MCP_PORT` or 8787 (`mcp/http.js`). `Dockerfile` exposes both. |
| Database | `server/db/index.js` is the only `require('node:sqlite')`, class `DatabaseSync`, file `hebrew.sqlite` under `DATA_DIR`. |
| Front end | `shared/**/*.js`: 30 files, 9,409 physical lines. Markup is built with string concatenation (`shared/app-shell.js` `renderHtml`). Pages load those files as scripts. |
| Stylesheet | `shared/worksheet.css`: 1,472 physical lines, 158 hex colors, one `@media (min-width: 800px)`, no `prefers-color-scheme`. |
| Tests | `test/*.test.js`: 44 files, run by `npm test` (`node --test test/*.test.js`). Ten further scripts, `test/chrome-*-smoke.js`, drive Chrome and are not in that glob. `retry-before-reveal-smoke.test.js` is one of the 44 and also drives Chrome. |
| Lessons | Three lesson directories under `worksheets/`: `my-morning`, `anal-sex-vocab`, `shai-date-indirect-speech`. `_audio`, `_review`, and `_template` are not lessons. |
| Audio | 65 `.mp3` files in the tree, outside `node_modules`. |
| Image | `Dockerfile`: `node:22-alpine`, `npm ci --omit=dev`, `USER node`, `CMD ["node", "server/start.js"]`. |
| Cluster | `deploy/k8s-database.md`: Deployment `hebrew`, `replicas: 1`, strategy `Recreate`, `DATA_DIR=/data/app` on PVC `hebrew-data`, lessons and audio on PVC `hebrew-lessons` at `/data/worksheets`. Secrets by `secretKeyRef`. The image does not contain them. |

MCP tools registered today in `mcp/tools.js`: `list_lessons`, `get_lesson`, `list_words`, `list_activity_types`, `list_users`, `upsert_lesson`, `set_lesson_visibility`, `generate_lesson_audio`. `server/authoring.js` already funnels browser saves through `mcp/store.upsertLesson`, so the legacy worksheet has one write path. ADR-0003 says `upsert_lesson` is not an edit operation for the new plan language.

`shared/app-shell.js` sets `dir="rtl"` and `lang="he"` on the shell, and `dir="ltr"` on the English span. That is the bidi pattern `RSCH-006` requires. It is not yet on the document element, and the stylesheet still uses physical left and right in places.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | A weighted criteria list covering the thirteen items in Decision 1. |
| R2 | Options A–E, each with a concrete shape, each scored on every criterion. |
| R3 | For any option that moves the server or the database: how accounts, progress, the 65 clips, the three lessons, the MCP, the tests, and the Docker and Kubernetes layout carry over, with no step that deletes or overwrites a recording without an archive. |
| R4 | One implementation of each ADR-0003 operation, called by both the MCP and the builder API. |
| R5 | The learner load path for a plan, and the fate of the script-tag constraint, in agreement with ADR-0003. |
| R6 | A CSS and component approach: tokens, dark mode, reduced motion, right-to-left, and stable class and `data-*` hooks. |
| R7 | Which tests stay, which are ported, and how the Chrome scripts keep running. |
| R8 | Whether INIT-004 builds beside the current site or edits it in place, with a rollback at each step, and every INIT-002 audio and data invariant kept. |
| R9 | Every INIT-002 gating, session, and CSRF rule kept, or a named replacement. No migration step serves the site without sign-in. |

### Constraints

- Plan only. No application code in this spec.
- Weights are proposed here. The operator accepted the plan defaults on 2026-10-08 and can change a weight at this gate.
- The maintainer is the operator alone. Agents write most of the code.
- Hosting stays the INIT-002 image and the one-replica layout.
- Do not put secrets, learner names, or transcript text in this document.
- SPEC-006 is still `draft`. This record does not choose a look. It must leave room for one.

---

## Options considered

| Option | Shape scored below | Why this shape |
|--------|--------------------|----------------|
| A | Node server kept. Design system and a hand-written component layer. No new direct dependency. | The incumbent plus the two things the product still lacks: plan operations, and a token-based UI. |
| B | Node server kept. Preact + htm, no build step, vendored. | The no-build library [Preact's getting-started guide](https://preactjs.com/guide/v10/getting-started) documents (accessed 2026-10-08). Web components are discussed and not scored as a second row. |
| C | Node server kept. Vite, React, and Tailwind. A Docker build stage emits static files the current server gates. | The build-based front end from the spec. React is the scored UI library because agents write it more reliably than Svelte; Svelte would not change the server, the image shape, or the winner. |
| D | Elixir and Phoenix LiveView replace the server and the UI. | Judged on the criteria. Real-time push is described in the LiveView docs and is not a product requirement. |
| E | Next.js (App Router, React, Node) replaces the HTTP server and the UI. | The most plausible other full-stack, because it stays on Node, so `node:sqlite` and the TypeScript MCP SDK can still run. Rails, Django, and Nuxt were set aside: Rails and Django break the language the SDK is written in, and Nuxt is the same rewrite with a framework this repo has never used. |

Detail, scores, and the carry plans are Decisions 2 and 3. The choice is Decision 4.

---

## Decision

### 1. Criteria and weights

Weights sum to 100. A score is an integer from 1 to 5. The weighted total is the sum of score times weight. The maximum is 500.

| | What 5 means | What 1 means |
|--|--------------|--------------|
| Scale | The criterion is met with the system that already exists, or with a small addition. | The criterion is met only by replacing the thing it is about. |

| Id | Criterion | Weight | Why this weight |
|----|-----------|--------|-----------------|
| C1 | Renders the six ADR-0003 block types | 8 | The learner has to see the plan. |
| C2 | Part-level edits and a live preview in the builder | 7 | Authors edit one part and look at it. |
| C3 | One implementation of those operations, shared by MCP and builder | 12 | ADR-0003 forbids a second copy. This is the architecture constraint. |
| C4 | Right-to-left and bidirectional text | 6 | `RSCH-006`: `dir` on the element, logical properties, English glosses wrapped `dir="ltr"`. |
| C5 | Accessibility | 6 | `RSCH-006`: WCAG 2.2 contrast and targets, alt text, reduced motion. |
| C6 | Media upload and gated serving | 7 | ADR-0004's pipeline, including approval and `DATA_DIR/media`. |
| C7 | SQLite and the existing accounts, progress, and clips | 9 | The file and the 65 recordings have to survive. |
| C8 | Hosts or talks to the MCP | 9 | The official SDK is already in this process. |
| C9 | Test story | 5 | The 44 files and the Chrome scripts have to keep meaning during the move. |
| C10 | One person can maintain it | 8 | Operator, 2026-10-08. |
| C11 | Agents write and edit it correctly | 8 | Agents write most of the code. |
| C12 | Docker, one replica | 7 | The image and `replicas: 1` / `Recreate` stay the hosting target. |
| C13 | Migration cost (5 means the smallest cost) | 8 | A rewrite is allowed when it earns its cost. It does not win a tie. |

**How a close total is read.** The lightest weight is 5. If a challenger leads option A by less than 5, the table has not separated them: the spec's own check allows a reviewer to move any score by one point. The separated choice is then the lower migration cost that still delivers the plan language and a design system.

### 2. The five options

#### A — Node, plain front end, design system by hand — **chosen**

**Shape.** `server/start.js`, `node:sqlite`, and `mcp/http.js` stay. New plan behavior is a Node module both callers import (Decision 5). The UI gains CSS custom properties and one function per component in `shared/ui/`, each returning an escaped HTML string, loaded the way `shared/app-shell.js` is loaded. No new direct dependency. No build step. `npm test` stays `node --test`.

**Pros.** The plan language, the media pipeline, the accounts file, the clips, the MCP, the tests, and the image all stay in the process that already enforces them. A design system does not require a second markup language.

**Cons.** Escaped HTML strings are the weakest surface for an agent (C11 is 3). Nested blocks will be verbose. Dark mode and logical properties have to be written into the one stylesheet by hand.

#### B — Node, plus Preact and htm, no build

**Shape.** The server is identical to A. New screens are Preact function components written with [htm](https://preactjs.com/guide/v10/getting-started), the tagged-template syntax Preact documents as the no-build stand-in for JSX (accessed 2026-10-08). The packages are vendored and loaded from this origin. The guide's example imports `https://esm.sh/preact` and `https://esm.sh/htm`; this site will not load a third-party CDN into a signed-in page.

**Web components, not scored as their own row.** A shadow root hides descendants from `document.querySelector`, which is how the Chrome scripts find `data-*` hooks, and it does not receive the shared stylesheet's selectors. A light-DOM custom element is option A's function plus a registry. Scoring it separately would double-count A.

**Pros.** A component function per block is a natural preview: render the working copy. htm is closer to JSX than string concatenation, which is the C11 gain.

**Cons.** Two direct dependencies, an import map beside the existing scripts, and a second markup dialect until every screen has moved. SPEC-009's `dependency_add` gate opens. The server work is the same as A.

#### C — Node, plus Vite, React, and Tailwind

**Shape.** The Node server, SQLite, and MCP stay. Vite builds a React front end. Tailwind is the styling toolchain. The Docker image gains a build stage; the runtime command can remain `node server/start.js` serving the built files under the existing sign-in gate. Svelte or Vue would be the same shape with a different component syntax. They are not given a second score.

**Pros.** Agents write React and Tailwind more reliably than string HTML (C11 is 5). The server and the data do not move.

**Cons.** A permanent build, a much larger lockfile, and utility class names that do not match the class selectors the Chrome scripts already use. The operator maintains three tools on top of the server.

#### D — Elixir and Phoenix LiveView

**Shape.** A Phoenix application replaces `server/web.js`, the auth modules, the SQLite wrapper, and the browser UI. LiveView renders the builder and the learner screens. HEEx components are the design system. The official MCP TypeScript SDK does not run on the BEAM. MCP is either a community Elixir library or a Node process that calls the Phoenix HTTP API and does not open `hebrew.sqlite` itself.

What LiveView is, from its own docs, accessed 2026-10-08: "A LiveView is a process that receives events, updates its state, and renders updates to a page as diffs" ([Phoenix.LiveView](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html), v1.2.12). The Phoenix guide calls that "rich, real-time user experiences with server-rendered HTML" and "diffs over the wire" ([LiveView](https://hexdocs.pm/phoenix/live_view.html)). The first response is ordinary HTML, then a persistent connection pushes updates. That persistent push is the feature. ADR-0003's edit rule is the opposite shape: the caller sends `expectedRevision`, a mismatch rejects the write, the document is unchanged, and the caller reads again. No product requirement asks for presence, broadcast, or a server-pushed DOM.

**Pros.** Server-rendered HTML, form events, and a live preview are LiveView's real strengths (C2 is 5). One BEAM node can hold the site. HEEx can carry `dir` and alt text. An Elixir release can be a single replica.

**Cons.** The official SDK list ([MCP SDKs](https://modelcontextprotocol.io/docs/2026-07-28/sdk), accessed 2026-10-08) is TypeScript, Python, C#, Go, Rust, Ruby, Java, Swift, PHP, and Kotlin. Elixir is not on it. A community package exists (`mcp_elixir_sdk` on Hex, protocol date 2025-11-25 in its readme); it is not the `@modelcontextprotocol/sdk` 1.32.1 this process runs, and 1.32.1 is the v1 maintenance line of that TypeScript SDK ([npm](https://www.npmjs.com/package/@modelcontextprotocol/sdk), accessed 2026-10-08). The 44 Node tests, the session and CSRF modules, the `node:sqlite` wrapper, and the `node:22-alpine` image would be replaced. The operator would be the only person debugging OTP.

#### E — Next.js

**Shape.** Next.js App Router on Node, React for the UI, Tailwind for styling. Route handlers re-home the JSON API. `node:sqlite` can stay only if the runtime is Node and the edge runtime is forbidden. The MCP SDK can stay only if this process also serves it. Next's deploy docs (accessed 2026-10-08) say the Node server and the Docker image support all features, and "if needed, you can also eject to a custom server" ([Deploying](https://nextjs.org/docs/app/getting-started/deploying)). Co-hosting the MCP listener on port 8787 is that eject, or a second process in the same pod.

**Why this candidate.** It is the full-stack framework that does not throw away Node. The MCP package and `DatabaseSync` are Node APIs. A Ruby or Python framework would share Phoenix's language break. Nuxt would share Next's server replacement and add Vue, which this repo does not contain.

**Pros.** Agents write Next and React well (C11 is 5). The SQLite file need not be dumped. Docker is a documented target.

**Cons.** Next replaces `server/web.js`, the session gate, and the static allowlist. A cached or statically optimized response of a lesson would skip sign-in. The current image, command, and two-port process go away. Option C keeps that server and scores higher, so E pays for a server rewrite C does not need.

### 3. Evaluation

#### Scores

| Id | Weight | A | B | C | D | E |
|----|--------|---|---|---|---|---|
| C1 Blocks | 8 | 4 | 5 | 5 | 4 | 5 |
| C2 Preview | 7 | 4 | 5 | 5 | 5 | 5 |
| C3 One ops module | 12 | 5 | 5 | 5 | 3 | 4 |
| C4 RTL / bidi | 6 | 4 | 4 | 4 | 4 | 4 |
| C5 Accessibility | 6 | 4 | 4 | 4 | 4 | 4 |
| C6 Media | 7 | 5 | 5 | 5 | 4 | 4 |
| C7 SQLite and data | 9 | 5 | 5 | 5 | 3 | 4 |
| C8 MCP | 9 | 5 | 5 | 5 | 2 | 3 |
| C9 Tests | 5 | 5 | 4 | 3 | 2 | 3 |
| C10 Solo maintainer | 8 | 5 | 4 | 3 | 2 | 3 |
| C11 Agents | 8 | 3 | 4 | 5 | 2 | 5 |
| C12 Docker, one replica | 7 | 5 | 5 | 4 | 3 | 3 |
| C13 Migration cost | 8 | 5 | 4 | 3 | 1 | 2 |
| **Weighted total** | **100** | **457** | **459** | **439** | **295** | **378** |
| Lead over A | | — | +2 | −18 | −162 | −79 |

B's lead of 2 is below the weight of C9 (5). Decision 1's close-total rule applies. The choice is explained in Decision 4, not by pretending the totals are further apart.

#### Why each score

**A.** C1 is 4 because hand functions can render the six blocks, and nested string HTML is clumsier than a component call. C2 is 4 because a preview is another call with the working copy, and `shared/admin/lesson-editor.js` (1,054 lines) shows both that the pattern exists and how large a hand-built form becomes. C3 is 5 because one module serves both callers in this process. C4 is 4 because `dir` and logical properties are authoring, which `RSCH-006` already specifies, and no library will do it. C5 is 4 because alt, labels, and focus can be required arguments and still depend on tests. C6, C7, and C8 are 5 because ADR-0004's pipeline, `hebrew.sqlite`, the clip files, and the MCP listener stay. C9 is 5 because the 44 files and the Chrome scripts keep their runners if `data-*` hooks stay. C10 is 5 because the operator already runs this image, in one language, with no build. C11 is 3 because agents mishandle escaped HTML strings: unclosed tags, missed escapes, missed names. C12 is 5 because the Dockerfile and `replicas: 1` stay. C13 is 5 because the new modules are additive and the legacy player stays until a screen switches.

**B.** C1 and C2 are 5 because a Preact function per block is the render and the preview. C3, C6, C7, and C8 match A: the server does not change. C4 and C5 match A: htm does not add bidi or an accessibility tree. C9 is 4 because the server tests stay and a new render tree has to keep the hooks the Chrome scripts query. C10 is 4 because the operator takes on two libraries and an import map beside the IIFE scripts. C11 is 4 because htm is the documented JSX stand-in, easier than string concatenation and less common in agent output than compiled JSX. C12 is 5 because `npm ci` in the existing image is enough; there is no build stage. C13 is 4 because the data does not move and the front end still gains a dependency plus a second dialect.

**C.** C1, C2, C3, C6, C7, and C8 are 5 because React can render and preview, and the Node server still owns operations, media, SQLite, and MCP. C4 is 4 because Tailwind does not set Hebrew direction; `dir` on the element is still the rule, and physical `left`/`right` utilities would violate `RSCH-006`. C5 is 4 because a JSX accessibility lint catches some misses and does not finish targets, contrast, or reduced motion. C9 is 3 because utility class names break the class selectors in the Chrome scripts even when `data-*` hooks are preserved. C10 is 3 because Vite, React, and Tailwind are a second upgrade train. C11 is 5 because this is the markup agents write most reliably. C12 is 4 because the image gains a build stage and the runtime stays one Node replica. C13 is 3 because the 9,409 lines of `shared/` UI are rewritten and the server is not.

**D.** C1 is 4 because HEEx can render the blocks, and today's client audio and reveal behavior becomes LiveView events or JS hooks. C2 is 5 because a server render that updates after an event is a live preview; that score is LiveView's genuine advantage. C3 is 3 because one Elixir module can implement the operations, and the TypeScript SDK cannot call that module in-process. C4 and C5 are 4 because HEEx is HTML: `dir` and alt text work, and they are still the author's job. C6 is 4 because Plug can sniff, re-encode, and gate, as new code. C7 is 3 because the SQLite file can remain on the volume and every query and migration is re-proven on another driver; the 65 files stay only if the new server mounts the same volume and keeps the path and archive rules. C8 is 2 because the official SDK list has no Elixir, so the working integration is replaced by a community library or a Node sidecar. C9 is 2 because the 44 files import Node modules and must be ported or dropped; the Chrome scripts can stay as a black box. C10 is 2 because the operator would debug Elixir and OTP alone. C11 is 2 because this repo's patterns, tests, and MCP calls are Node, so an agent has nothing local to copy. C12 is 3 because a release image can be one replica and it replaces the Dockerfile, the workflow, and the `node` uid. C13 is 1 because the server, auth, data access, MCP boundary, UI, tests, and image all move.

**E.** C1 and C2 are 5 for the same React reason as C. C3 is 4 because one Node module can serve both callers only if MCP shares the process, which is the custom-server eject. C4 and C5 match C. C6 is 4 because the pipeline can move into route handlers, and a public cache of a lesson would break the gate. C7 is 4 because `node:sqlite` can stay on the Node runtime if the edge runtime is forbidden, after the access layer is re-wired. C8 is 3 because the SDK runs on Node and co-hosting it is the eject or a second process. C9 is 3 because imports of `server/web.js` are rewritten, domain tests can move, and the Chrome scripts are retargeted. C10 is 3 because the App Router cache and the server/client split sit beside the auth rules one person must keep straight. C11 is 5 because agents write Next and React reliably. C12 is 3 because Docker is documented and the current image, command, and replica pin are replaced. C13 is 2 because the HTTP surface, the gate, and the UI are replaced while the database file can stay.

#### Carry plan where the server or the database moves

A, B, and C do not move the server or the database. Accounts, progress, the 65 clips, the three lessons, and the MCP stay in place. C adds a build stage and does not open a second writer on `hebrew.sqlite`.

**D — Phoenix.** No step deletes an `.mp3` or writes over one without the archive-and-verify rule ADR-0002 already states. No step copies clip bytes into the image.

| Thing | Carry | Rollback |
|-------|-------|----------|
| Accounts and progress | Leave `hebrew.sqlite` on PVC `hebrew-data`. Stop writers first (`replicas: 0` or the maintenance window `docs/operations/accounts-and-admin.md` already requires). The new process opens that file read-only and checks row counts before any write. Do not restore a dump over the live file. | Point the Deployment back at the Node image. The file was not overwritten, so the Node process migrates nothing new. |
| 65 clips | Keep PVC `hebrew-lessons` mounted at `/data/worksheets`. Paths inside plans stay character for character. `scripts/audio-inventory.js` keeps running as a Node one-shot against that volume so the inventory gate does not wait for an Elixir port. | The volume was not rewritten. |
| Three lessons | Worksheet files and their `index.html` stay until a published plan snapshot exists. ADR-0003 forbids editing those `index.html` files for this work. The plan JSON is new data in the database, not a replacement of the worksheet until cutover. | Leave the worksheets mounted. Learners keep the script-tag player. |
| MCP | Prefer a Node sidecar that speaks the existing `@modelcontextprotocol/sdk` and calls the Phoenix HTTP API for every plan operation. The sidecar must not open `hebrew.sqlite`. The Elixir module is the only writer. A community Elixir SDK is a later amendment, not the default, because it is not the official SDK and not the package already integrated. | Stop the sidecar. The Node image still serves `mcp/http.js`. |
| Tests | See the port table below. | The Node suite runs against the Node image. |
| Docker and Kubernetes | New release image. Same Deployment name `hebrew`, `replicas: 1`, `strategy: Recreate`, same two PVCs, same Secret name and keys. `fsGroup` and `runAsUser` change together if the release uid is not 1000. Ports 8080 and 8787 stay on that one pod. Do not add a second replica to "scale LiveView". | Roll the Deployment back to the `node:22-alpine` image. |

| Tests under D | Fate |
|---------------|------|
| `auth-passwords.test.js`, `auth-sessions.test.js`, `auth-throttle.test.js`, `api-auth.test.js`, `http-security.test.js`, `web-gating.test.js`, `visibility.test.js`, `content-filter.test.js` | Port. Session, CSRF, throttle, sign-in, and 404-on-invisible are behavior, not Node. |
| `accounts.test.js`, `db.test.js`, `db-repos.test.js`, `user-cli.test.js` | Port the behavior. Drop assertions on `node:sqlite` APIs. |
| `progress-*.test.js`, `review-scheduler.test.js`, `review-store.test.js`, `api-progress.test.js` | Port. ADR-0001's ladder stays. |
| `audio-preservation.test.js`, `generate-audio-cache.test.js`, `word-audio-catalog.test.js`, `player-speech-clips.test.js` | Keep the file-level assertions. The inventory script stays Node. Port only the parts that import the Elixir server. |
| `authoring.test.js`, `admin-lesson-editor.test.js`, `mcp-assignment.test.js`, `mcp-auth.test.js`, `mcp-oauth.test.js` | Port against the sidecar contract: same tool names, same revision conflict, token still required. |
| `lesson-status.test.js`, `lessons-dir.test.js`, `story-gloss.test.js`, `tokenize-hebrew.test.js`, `learner-support.test.js`, `production-recall.test.js`, `review-page.test.js`, `admin-users-view.test.js`, `api-admin.test.js`, `self-hosted-assets.test.js` | Port. `progress-view.test.js` and `progress-metrics.test.js` are already in the `progress-*` row above. |
| `lesson-data.test.js`, `validate-activities.test.js`, `validate-nudges.test.js`, `vocab-index.test.js` | Keep while legacy worksheets still load. Drop each one in the change that retires the worksheet loader, the worksheet validator, or the vocab-index script. |
| `app-shell.test.js` | Rewrite against the new shell's nav and bilingual labels. Drop assertions that only match today's HTML strings. |
| `test/chrome-*-smoke.js` and `retry-before-reveal-smoke.test.js` | Keep as black-box scripts against the same URLs and the same `data-*` hooks. They do not import the server. |

**E — Next.js.** The SQLite file and the clip volume stay. The HTTP server does not.

| Thing | Carry | Rollback |
|-------|-------|----------|
| Accounts and progress | Same `hebrew.sqlite` on `hebrew-data`. Next runs on Node 22 with the edge runtime disabled, and `server/db/index.js` remains the only SQLite import. Additive migrations only, each with a rollback. | The previous image still opens the same file. |
| 65 clips | Same volume, same paths, same archive-before-overwrite. Do not emit clip URLs as public static files. | Volume untouched. |
| Three lessons | Worksheet files stay mounted and gated. Plan JSON is additive. `index.html` is not edited. | Worksheets still serve. |
| MCP | Custom server, as Next's deploy doc allows, listening on 8787 in the same process as `next start`, still requiring `MCP_AUTH_TOKEN`. It imports `server/plans/ops.js`. It does not reimplement an operation. | Drop the custom server and run `server/start.js` from the previous image. |
| Tests | `db*.test.js`, `review-scheduler.test.js`, `auth-passwords.test.js`, and other modules that do not import `server/web.js` stay. `web-gating.test.js`, `http-security.test.js`, `api-*.test.js`, and `app-shell` DOM tests are ported onto the Next router. Chrome scripts stay if URLs and `data-*` hooks stay, and they are updated in the same change as any hook they lose. | Node suite against the previous image. |
| Docker and Kubernetes | New image with `next build`. Same Deployment, `replicas: 1`, `Recreate`, both PVCs, same Secret keys. One container. Do not scale out: the in-memory throttle and SQLite writer still forbid it. | Roll back the image. |

### 4. The decision

**Chosen: A.**

B's total is 459 and A's is 457. The lead is 2, and the lightest criterion weighs 5, so Decision 1 treats them as unseparated. A one-point rise in B's test score (C9 from 4 to 5) makes the lead 7. A one-point rise in A's agent score (C11 from 3 to 4) puts A ahead by 6. The spec allows that one-point movement and asks that the difference be explained. The explanation is migration cost. Both options implement ADR-0003 in one Node module and both can carry a design system. A does it with no new direct dependency and one markup dialect. That is the lower migration cost.

**Why not B, given the two-point lead.** The points B gains are C1, C2, and C11: component syntax and a better chance an agent writes the markup. The plan store, the operations, the validator, the media pipeline, SQLite, the MCP, and the image are identical. The design system this product needs — tokens, dark mode, reduced motion, `dir` on the element, logical properties, stable `data-*` hooks — is CSS and a function contract, which option A already includes. Preact does not supply those rules. Taking B means vendored `preact` and `htm` (the documented no-build path loads a CDN, which this site will not), a `dependency_add` gate on SPEC-009, and two dialects until the 9,409 lines move. Web components do not rescue B: a shadow root hides the hooks the Chrome scripts query and blocks the shared stylesheet; light DOM is A's hand layer.

**Why the rewrites lose.** D totals 295. Its best cell is the live preview, which A and B can do by rendering the working copy again. The product does not use LiveView's pushed diffs. The official MCP SDK does not run on Elixir. The account file, the 65 clips, and the tests can be carried, and carrying them is the work of a port, not a reason to start one. E totals 378. It keeps Node, which is why it beats D, and it still throws away the HTTP server, the gate, and the player. C totals 439 and keeps that server, so E is dominated by a cheaper front-end swap, and that swap itself loses to A by 18 points: a build stage and a class-name break, before the hand component layer has been shown to fail.

**Cost of A.** New modules under `server/plans/` and `shared/ui/`. A section of tokens in the stylesheet. New tests for the operations. No new package, no image change, no data move. The C11 risk is real and is the spike's job.

**When this choice is wrong.**

| Condition | What to do |
|-----------|------------|
| SPEC-009 shows the hand components produce broken markup, missed escapes, or missing `dir` / alt / aria often enough that a block cannot land | Amend this ADR to B before INIT-004. Do not jump to C, D, or E on that evidence. |
| The operator raises C11's weight, or scores B's C10 or C11 one point higher and accepts the dependency | Confirm B at this gate. The server half of Decision 5 stays. |
| A later product need is server-pushed presence or multiplayer | Reopen D. That need does not exist now. |
| `node:sqlite` is withdrawn and the ADR-0002 `better-sqlite3` fallback is also unsuitable | Reopen the server. The fallback is unchanged by this record. |

The operator confirms A, or amends the letter, before SPEC-009 starts.

### 5. Architecture

The boundaries below are the ones INIT-004 builds. Legacy worksheet authoring stays on `server/authoring.js` → `mcp/store.upsertLesson` until that lesson is retired. `upsert_lesson` is not a plan operation.

```text
builder (admin session)  --HTTP-->  server/api/plan-routes.js --+
                                                                 +--> server/plans/ops.js
MCP tools (token)         --call-->  mcp/tools.js --------------+
                                      |
                                      +--> server/plans/validate.js
                                      +--> server/plans/store.js --> hebrew.sqlite
                                      +--> server/plans/publish.js (snapshot)
                                      +--> server/plans/personal.js (assignee)

learner (session) -- GET /api/plans/:id/published --> snapshot JSON --> shared/ui/ render
media bytes: DATA_DIR/media, ADR-0004 routes, not a plan module and not a served root
audio files: /data/worksheets, paths unchanged
```

| Module | Responsibility |
|--------|----------------|
| `server/plans/store.js` | Load and save the working copy and the published snapshot. The only plan SQL. Uses `server/db/`. |
| `server/plans/ops.js` | The six ADR-0003 operations. The only mutation logic. |
| `server/plans/validate.js` | JSON Schema plus N1–N12. Publish and the write operations both call it. |
| `server/plans/publish.js` | Copy the working document onto the snapshot, set `published`, increment `revision`. One function. |
| `server/plans/personal.js` | `audience: personal` and `assigneeId`. Checks the account, then calls `ops.js`. It does not reimplement an operation. |
| `server/api/plan-routes.js` | Session, CSRF, and role checks, then the functions above. |
| `mcp/tools.js` | Token check, then the same functions. New tool names match the operation names. |
| Media | ADR-0004 routes on the existing API router and the matching MCP tools. Bytes under `DATA_DIR/media` and `DATA_DIR/media/_archive`. |
| `shared/ui/` | Component functions. Escaped text. `dir` and `lang` as attributes. |
| `shared/player/` | Legacy player until that screen's plan path is switched. New plan screens use `shared/ui/`. |

**One implementation.** Each ADR-0003 operation is one exported function. The HTTP handler and the MCP tool are adapters. They authenticate, call the function, and map the result. They do not parse a path, apply a revision, or edit JSON themselves. A bulk import, if one is added later, is added to both adapters in the same change, as ADR-0003 requires.

| Operation | Function | MCP tool | Builder route |
|-----------|----------|----------|---------------|
| Read a part | `readPart` | `read_part` | `GET /api/plans/:id/parts` |
| Replace a part | `replacePart` | `replace_part` | `PUT /api/plans/:id/parts` |
| Insert a block | `insertBlock` | `insert_block` | `POST /api/plans/:id/blocks` |
| Move a block | `moveBlock` | `move_block` | `POST /api/plans/:id/blocks/move` |
| Remove a block | `removeBlock` | `remove_block` | `DELETE /api/plans/:id/parts` |
| Set a field | `setField` | `set_field` | `PATCH /api/plans/:id/fields` |

`publishPlan` backs MCP `publish_plan` and `POST /api/plans/:id/publish`. It is not a seventh edit operation. It is one function for the same reason.

**Conflict transport.** ADR-0003 left the status code to this record. A write whose `expectedRevision` does not match the stored working-copy revision returns HTTP **409** with `{ "error": "revision_conflict", "revision": <stored> }`. The document is unchanged. The MCP tool returns an error with the same `revision_conflict` name and the stored revision. There is no merge. A read does not check the revision; it returns the part and the current revision so the caller can send it back.

**Where state lives.**

| State | Where |
|-------|--------|
| Accounts, sessions, progress, visibility, audit | `hebrew.sqlite` under `DATA_DIR`, as today |
| Plan working copy, revision, status, snapshot | New tables in that same file, via `server/db/` |
| Image bytes | `DATA_DIR/media`, outside every served root |
| Audio | Existing relative `.mp3` paths on the lessons volume |
| Browser | Renders a snapshot. It is not the source of truth. The review queue stays as ADR-0002 left it. |

**How the learner loads a plan.** A signed-in client `fetch`es `GET /api/plans/:id/published`. The handler requires a session. A draft, a hidden lesson, a `selected` lesson without a grant, or a personal plan whose `assigneeId` is not the caller, answers **404**. The body is the published snapshot. The client renders it with `shared/ui/`. ADR-0002 already allows `fetch` for `/api/` JSON. This route is one of those calls. The client does not `fetch` a worksheet URL.

**Script-tag constraint.** Superseded for the plan document when this API serves it, which is the cutover ADR-0003 Decision 3 describes. Kept for legacy `window.WORKSHEET` files until those files are removed. After cutover the learner site does not load plan JSON from a script tag. A `window.LESSON_PLAN` script, if something still emits one, is a whole-file copy of the snapshot. It is not the source of truth and it is not what the operations edit. New plans do not need that file.

**Same process for the MCP.** `server/start.js` keeps calling both listeners. Plan tools are registered in `mcp/tools.js` and import `server/plans/ops.js`. There is no second plan service and no second writer.

### 6. CSS and components

**Tokens.** One stylesheet, `shared/worksheet.css`, gains a token block: color, type, space, radius, and control size as custom properties on `:root`. Component rules use the variables. New rules do not add a raw hex. Existing hex values move into tokens when a screen is touched, not in a single sweep that rewrites every selector the tests use.

**Dark mode.** `@media (prefers-color-scheme: dark)` overrides the same variables. An optional `data-theme` on the document element overrides the media query for an explicit choice. SPEC-008 may choose the palette. Both themes meet the `RSCH-006` contrast floors (4.5:1 for text). This record does not pick colors.

**Reduced motion.** `@media (prefers-reduced-motion: reduce)` removes non-essential transitions. `RSCH-006` treats WCAG 2.3.3 as the bar worth meeting.

**Right-to-left.** The document element gets `dir="rtl"` and `lang="he"`. Direction is an attribute, not a CSS `direction` property, matching `RSCH-006`. Layout uses logical properties (`margin-inline`, `padding-inline`, `inset-inline-start`). An English gloss is an element with `dir="ltr"` and `lang="en"`, which `shared/app-shell.js` already does for nav labels. Component functions take a `dir` argument when the fragment is not the page default.

**Component contract.** A component is a function `(props) => string` in `shared/ui/`. It escapes text through the existing escape helper. It sets `alt` from the plan's alt field for an introduction image and refuses to render an image with an empty alt. It sets `data-*` hooks for behavior and for tests. Class names are styling. Tests prefer `data-*`. A class name that a current test queries is unchanged except in the same change as that test.

**Targets.** Primary controls are sized toward the 44 CSS pixel bar `RSCH-006` recommends. The 24 CSS pixel AA floor is the minimum. That is a token, not a per-screen guess.

### 7. Testing

On option A every current file keeps running. `npm test` stays the 44 `test/*.test.js` files. The ten `test/chrome-*-smoke.js` scripts stay as they are and keep being launched the way they are launched today. `retry-before-reveal-smoke.test.js` stays inside `npm test`.

These are the files that must stay green on every migration step. They cover the surfaces the spec names.

| Surface | Files | How they stay green |
|---------|-------|---------------------|
| Authoring | `authoring.test.js`, `validate-activities.test.js`, `validate-nudges.test.js`, `lesson-data.test.js` | Legacy `upsert_lesson` path untouched until a lesson is retired. New plan tests are new files. |
| MCP | `mcp-assignment.test.js`, `mcp-auth.test.js`, `mcp-oauth.test.js` | Token gate, tool registration, and the existing tools stay. New plan tools get new tests that import `server/plans/ops.js` and the tool adapter. |
| Admin editor | `admin-lesson-editor.test.js`, `admin-users-view.test.js`, `api-admin.test.js` | The current editor's DOM hooks stay until that screen switches. The switch updates the test in the same change. |
| Gating | `web-gating.test.js`, `http-security.test.js`, `api-auth.test.js`, `auth-sessions.test.js`, `auth-throttle.test.js`, `auth-passwords.test.js` | No step removes the session check, the Origin check, or the throttle. New plan routes use them. |
| Visibility | `visibility.test.js`, `lesson-status.test.js`, `content-filter.test.js` | Plan reads use the same visibility rules and answer 404 when the caller must not know the id exists. |
| Chrome | `test/chrome-admin-lessons-smoke.js`, `test/chrome-admin-smoke.js`, `test/chrome-audio-smoke.js`, `test/chrome-learner-support-smoke.js`, `test/chrome-production-recall-smoke.js`, `test/chrome-progress-smoke.js`, `test/chrome-review-smoke.js`, `test/chrome-signin-smoke.js`, `test/chrome-story-gloss-smoke.js`, `test/chrome-today-lessons-smoke.js`, and `retry-before-reveal-smoke.test.js` | They keep running against the signed-in server. A screen that changes a queried class or `data-*` updates that script in the same change. While a screen is still the legacy player, the script is unchanged. |

New tests, added with the modules: revision conflict leaves the document unchanged and returns 409; both adapters call the same function (a test that spies on `ops.js`); N1–N12 severities; personal-plan 404; media routes when ADR-0004 is implemented. Those tests are INIT-004's, except the schema spike SPEC-009 already names (`node spikes/skeleton/validate.js`).

Nothing in the 44 is dropped under option A. Port-or-drop applies only to D and E, and those tables are in Decision 3.

### 8. Migration

INIT-004 adds modules beside the legacy player and switches one screen at a time, in the same process and the same image. It does not stand up a second site, and it does not rewrite `shared/` in place in one change. The database file, the lessons volume, and the Deployment stay.

| Step | What changes | What stays up | Rollback |
|------|----------------|---------------|----------|
| 1 | `server/plans/` store, ops, validator, publish. Routes mounted on the existing router, behind the existing session and CSRF checks. | Legacy pages, worksheets, MCP tools, sign-in. | Unmount the routes. No snapshot has been served. |
| 2 | Token block appended to `shared/worksheet.css`. Existing selectors keep their names. | Every page. Contrast checked in both themes before a token replaces a hex on a touched screen. | Revert the stylesheet. |
| 3 | Admin builder for one plan, `shared/ui/` plus the plan routes. Old editor remains for worksheets. | Learner site. | Hide the builder route. Worksheets still edit through `upsert_lesson`. |
| 4 | One learner screen fetches `GET /api/plans/:id/published` for a plan that has a snapshot. Other lessons stay on script tags. | Sign-in, the other lessons, review, progress. | Point that lesson's link back at the worksheet. |
| 5 | ADR-0004 media routes, bytes under `DATA_DIR/media`, archive before delete. | The static server's allowlist. Media is not added to it. | Stop the routes. Archived files stay archived. |
| 6 | Retire a worksheet only after its snapshot is what learners open and the Chrome script for that screen is green. | The other worksheets. | Restore the worksheet link. The files were not deleted in the earlier steps. |

Audio and data invariants on every step:

- No `.mp3` is deleted, moved, or overwritten. A future overwrite still archives and verifies first.
- Paths and URLs of existing clips do not change.
- `hebrew.sqlite` is not replaced. Plan tables are additive migrations with a down path.
- `DATA_DIR` stays outside every served root. `server/start.js` already refuses the bad layout.
- `replicas` stays 1. `strategy` stays `Recreate`.
- `MCP_AUTH_TOKEN` stays required. The MCP does not gain a second, unauthenticated listener.
- No step serves a page, a lesson file, a plan, a clip, or a media byte without the sign-in check, except the login page and its assets, as ADR-0002 already states.

### 9. Carried constraints

ADR-0003 §9 is the list. Option A does not reopen it. The script-tag row is the one this stack acts on.

| Source | Constraint | Under option A |
|--------|------------|----------------|
| ADR-0001 #1, already superseded by ADR-0002 | Progress only in `localStorage`; no accounts | **Kept** as ADR-0002 left it. Plans do not store progress. |
| ADR-0001 #2; ADR-0002 carried row; INIT-002 REQ-014 | No `fetch` for lesson or vocab data; script tags only | **Superseded** for the plan document when `GET /api/plans/:id/published` serves it. **Kept** for legacy `window.WORKSHEET` files until they are removed. |
| ADR-0001 #3; REQ-014 | No edits to existing lesson `index.html` | **Kept**. |
| ADR-0001 #4; REQ-014 | No root or binyan speed drills | **Kept**. |
| ADR-0001 #5; REQ-014 | No forced vowel fading; points on by default | **Kept**. |
| ADR-0001 #6; REQ-014 | Bilingual UI labels | **Kept** for chrome. |
| ADR-0001 #7; REQ-014 | No account names, transcripts, or secrets in lesson files, glosses, the vocab index, or ADRs | **Kept**. |
| ADR-0001 | `exactKey` identity | **Kept**. |
| ADR-0002 | Server is authoritative for the review schedule | **Kept**. |
| ADR-0002 | `fetch` only for `/api/` | **Kept**, with plan GET as one of those `/api/` calls, as ADR-0003 already changed the scope. |
| ADR-0002 | New lessons default to `hidden`; draft is `status`, not `hidden` | **Kept** as ADR-0003 changed it. |
| ADR-0002 | Visibility `hidden` / `everyone` / `selected`; grants in the account database | **Kept**. |
| ADR-0002; REQ-014 | Audio paths and URLs unchanged; archive before overwrite; inventory and verify | **Kept**. |
| ADR-0002; REQ-014 | ElevenLabs key only in server env | **Kept**. This record does not copy it. |
| ADR-0002, as ADR-0004 amended the progress sentence | No instructor role. MCP reads only the ADR-0004 §1 snapshot, and only when that learner's switch is on | **Kept**. ADR-0003 marked the progress sentence "until ADR-0004". ADR-0004 has landed, and this row uses that wording. |
| ADR-0002 | One replica; `DATA_DIR` outside served roots | **Kept**. |
| ADR-0004 | Media sniff, re-encode, random name, gated fetch, human approval, generation key in the environment | **Kept**. Implemented on this server, not on a new one. |

---

## Consequences

### Positive

- The plan language lands in the process that already has accounts, gating, and the MCP.
- MCP and the builder cannot drift: there is one function per operation.
- The 44 tests and the Chrome scripts keep their runners.
- The image and the single replica do not change.
- SPEC-009 can falsify the weak cell (hand-written markup) without a port.

### Negative / trade-offs

- Agents get less help with markup than they would from React or htm. That is accepted until the spike says otherwise.
- The stylesheet starts with 158 hex values. Tokens replace them as screens are touched, so the old and new look coexist for a while.
- `node:sqlite` stays experimental, with the same wrapper and the same `better-sqlite3` fallback ADR-0002 recorded.
- B remains two points away. A reviewer who moves one score can prefer it. The gate exists for that reason.

### Follow-ups (out of scope here)

- The MCP SDK's v2 line (`@modelcontextprotocol/server`) is a later dependency decision. This record does not upgrade 1.32.1.
- Content-Security-Policy remains the follow-up ADR-0002 left open.
- ADR-0006 chooses colors. It uses the tokens in Decision 6.
- `db:backup` still does not copy `DATA_DIR/media` until INIT-004, as ADR-0004 already says.

### Rollback

Delete this file. Set SPEC-007 `status` back to `ready`. No application file, database, or recording changes with this spec.

---

## Implementation plan (downstream)

| Spec | Consumes from this ADR |
|------|------------------------|
| SPEC-009 | Option A. One unit rendered with `shared/ui/` functions and tokens. No new direct dependency unless the spike falsifies C11, in which case this ADR is amended to B first. |
| SPEC-008 | The look is tokens, `dir` attributes, dark-mode variables, and reduced motion on this stylesheet. |
| SPEC-010 | INIT-004's module map is Decision 5. The migration steps are Decision 8. |

---

## Security considerations

### Authentication and authorization

Option A keeps the INIT-002 controls. No row is replaced.

| Control | Where it stays |
|---------|----------------|
| Sign-in on every path except the login page and its assets | `server/web.js` gating. Plan and media routes use the same session check. |
| scrypt password hashes | `server/auth/passwords.js` |
| HttpOnly session cookie, revocation | `server/auth/sessions.js` |
| Login throttle, one replica | `server/auth/throttle.js`. Decision 8 does not raise `replicas`. |
| CSRF: JSON `Content-Type`, Origin check, `SameSite=Lax` | `server/http/security.js`, as ADR-0002 recorded it. Plan writes go through it. |
| Admin checks on the server | Builder routes require `admin`. A `user` session does not get `ops.js` writes. |
| Invisible lesson answers 404 | Plan GET uses the same rule for draft, hidden, ungranted `selected`, and a personal plan's non-assignee. |
| `MCP_AUTH_TOKEN` required | `server/start.js` still exits if it is missing. Plan tools use the same check. |
| MCP progress limited to the ADR-0004 snapshot | Unchanged. Plan tools do not read progress. |
| `DATA_DIR` outside served roots | `server/start.js` still refuses a bad layout. Media bytes are not added to the static allowlist. |

### Data protection

- Plan text is escaped at render. The schema still rejects an HTML tag start, as ADR-0003 states.
- Image and audio URLs are not stored in the plan. Images are media ids. Audio paths stay relative.
- Generation and ElevenLabs keys stay in the server environment. They are not copied into this document, the image, or a client bundle. Option A adds no client bundle.
- No new direct dependency, so the supply-chain surface does not grow. A later amendment to B opens `dependency_add` and a review of those two packages.

### Migration

Decision 8 has no step in which the site is served without sign-in. New routes are mounted already wrapped in the session check. Rollback unmounts them. It does not open the allowlist.

### Residual risks (accepted)

1. Hand-written HTML strings can drop an escape or an attribute. Mitigation: component helpers, tests on `dir` and alt, and the SPEC-009 spike. If the spike fails, the amendment is B.
2. `node:sqlite` remains experimental. Mitigation: the existing wrapper and the documented `better-sqlite3` fallback.
3. No CSP yet. Unchanged from ADR-0002. Option A does not add inline event handlers as a new pattern; existing pages keep their current scripts until a screen is switched.
4. Token migration is gradual, so some screens stay on the old hex palette until they are touched. Contrast is checked when a screen moves, not claimed for the untouched sheet.

---

## References

- [ADR-0002: Accounts, server-side progress, and site map](ADR-0002-accounts-server-progress-site-map.md) — session, CSRF, gating, one replica, audio invariants
- [ADR-0003: Course and lesson-plan language](ADR-0003-course-and-lesson-plan-language.md) — operations, revision, script-tag cutover, constraint table
- [ADR-0004: Learner data, media, and secrets](ADR-0004-learner-data-media-and-secrets.md) — media pipeline and the MCP snapshot
- `RSCH-006` — `dir`, logical properties, contrast, target size, reduced motion
- `package.json`, `Dockerfile`, `server/start.js`, `server/db/index.js`, `server/web.js`, `mcp/http.js`, `mcp/tools.js`, `shared/app-shell.js`, `shared/worksheet.css`, `deploy/k8s-database.md` — counts and layout as of `2f1f0b0`, 2026-10-08
- Preact, "Getting Started" (no-build route and htm): <https://preactjs.com/guide/v10/getting-started>. Accessed 2026-10-08.
- Phoenix.LiveView v1.2.12: <https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html>. Accessed 2026-10-08.
- Phoenix guide, "LiveView": <https://hexdocs.pm/phoenix/live_view.html>. Accessed 2026-10-08.
- Model Context Protocol, "SDKs": <https://modelcontextprotocol.io/docs/2026-07-28/sdk>. Accessed 2026-10-08.
- npm `@modelcontextprotocol/sdk` 1.32.1 (v1 maintenance line): <https://www.npmjs.com/package/@modelcontextprotocol/sdk>. Accessed 2026-10-08.
- Next.js, "Deploying" (Node server, Docker, custom server): <https://nextjs.org/docs/app/getting-started/deploying>. Accessed 2026-10-08.
- Node.js, "SQLite" (`node:sqlite`): <https://nodejs.org/api/sqlite.html>. Accessed 2026-10-08. The image is Node 22; the repo's wrapper is `server/db/index.js`.

## Changelog

| Date | Change |
|------|--------|
| 2026-10-08 | Initial Proposed record — `INIT-003/SPEC-007`. Weights are the plan defaults the operator accepted the same day. The letter is A, confirmed at the gate before SPEC-009. |
| 2026-10-08 | Status Accepted — operator confirmed Option A. |
| 2026-10-09 | Superseded in part by ADR-0007 (`INIT-005/SPEC-001`). Metadata note and this changelog row only; Option A and the decision sentences in this file are left as written. The page model (one HTML file per signed-in screen) is replaced by one shell document and a hand-built router. |
| 2026-10-12 | ADR-0011 (`INIT-008/SPEC-001`, Accepted) extends 5: the one-implementation table gains rows for the plan-builder actions. Metadata row and this row only. |
