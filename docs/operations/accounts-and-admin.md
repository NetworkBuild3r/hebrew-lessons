# Accounts and admin

Operator guide for first run, day-to-day accounts, lesson sharing, database backup, pictures, and audio safety. Commands below are for Windows PowerShell locally, and for `kubectl exec` on the cluster. Env var and k8s secret **names** only — never paste secret values into this file or into shell history.

Provenance: `INIT-002/SPEC-018`. Pictures: `INIT-004/SPEC-019`. AI planning and the profile loop: `INIT-006/SPEC-019`. Plan builder: `INIT-008/SPEC-017`. Teacher screens: `INIT-011/SPEC-001`. Cluster layout details: [deploy/k8s-database.md](../../deploy/k8s-database.md).

## First run

There is no public sign-up. On startup, if the database has **no admin user**, the server creates one from the environment and logs the username only:

| Name | Required when no admin exists |
| --- | --- |
| `ADMIN_USERNAME` | Yes |
| `ADMIN_PASSWORD` | Yes. At least 10 characters, and not a common password |
| `ADMIN_DISPLAY_NAME` | No |

Locally those live in `.env` (see `.env.example`). On Kubernetes, inject `ADMIN_USERNAME` and `ADMIN_PASSWORD` from Secret `hebrew-secrets` the same way as `MCP_AUTH_TOKEN` (see [deploy/k8s-database.md](../../deploy/k8s-database.md)). Do not put the password in the Deployment YAML.

If any admin already exists, including a disabled one, startup ignores these variables. Changing the Secret later does not change the password. Reset it with the CLI below, or delete the database and restart to bootstrap again.

The CLI still works when you want another account, or a reset. The password comes from a **TTY prompt** (no echo) or from `--password-file`. Never pass `--password=` on the command line (the CLI refuses it).

```powershell
npm run user -- create <name> --admin
npm run user -- reset <name>
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

After step 2 of the [lesson-plan rollout](lesson-plan-rollout.md), this tab no longer limits who can read the published plan of `my-morning`, `anal-sex-vocab`, or `shai-date-indirect-speech`: the plan route reads the visibility inside the plan. To restrict one of those, change and publish its plan first, as that checklist describes, then this tab.

## Plan builder

Provenance: `INIT-008/SPEC-017`. Teacher screens and worksheet map: `INIT-011/SPEC-001`. Decisions: [ADR-0011](../decisions/ADR-0011-ai-first-plan-builder.md). Migration and rollout: [plan-builder-rollout.md](plan-builder-rollout.md).

Lesson plans are built by the AI through the MCP and fixed by you in the admin **Plans** tab. The AI creates, edits, and publishes plans without waiting for you. You can open any plan, change any field, and put back an earlier value in one step. Every screen action here has an MCP tool, and every plan tool has an admin route, so the AI and you work on the same plan with the same rules.

### Teacher workflow (two screens)

The teacher page and MCP call the **same** plan operations. There is one write path and one plan document. A button on the page is not a second kind of edit.

Four steps, start to end:

1. **New plan.** Title, and who it is for: one learner, or everyone.
2. **The plan screen** (Figma node `19:2304` in file [`NJ8HfTPCQgrFty2VVQBMhY`](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY)). Lessons in order, one wide row each. Add lesson asks for a title and one sentence. Reuse lesson copies a lesson from this plan or another. Move and Open lesson sit on the row. The word bank on the side lists each word and the lesson that added it.
3. **The lesson screen** (Figma node `19:2592` in the same file). Tabs for the lessons in this plan. Word bank on the left, activities in the middle, the learner preview on the right. An activity opens with the fields to fill and is saved when it is valid.
4. **Publish.** Warnings do not stop it.

**Add lesson** and **Reuse lesson** both call `insert_lesson` — the function `insertLesson` behind `POST /api/plans/:id/lessons` in `server/plans/ops.js`. Reuse is that same operation with a copy of the lesson. There is no separate copy tool.

English chrome is English only. Hebrew chrome is Hebrew only. A lesson title in Hebrew stays Hebrew in both languages, because it is the lesson, not the chrome. A teacher does not type a path, a block id, an index, or a recipe slug. Ids are made from the title.

What a worksheet exercise becomes (teacher activity names; the AI maps them through `shared/admin/activity-map.js`):

| What the teacher makes | Activity |
| --- | --- |
| Word bank, then complete the sentences | Words (with fill-in-the-blank) |
| Match column A to column B | Match |
| Choose the correct sentence | **Not held** (open decision O1 — teach as a Story question until a later schema change; no choice block) |
| A short reading and questions | Story (with questions) |
| Write about yourself / discussion prompts | Writing |
| A verse with glosses | Verse |
| Verb rows to chant | Pattern |
| English cue, say the Hebrew | Type it (Fluency for the timed round) |
| A teacher note | Notes |
| A pasted list of Hebrew–English lines | Word bank (paste) |

A drawing task (for example “read and draw a family tree”) is not held. Do not invent a drawing block. The AI recipe for turning a Drive worksheet into a lesson is [worksheet-to-lesson](../../.cursor/skills/hebrew-worksheet/recipes.md#worksheet-to-lesson).

### The Plans list and New plan

Sign in as an admin, open **Admin** (`/admin/`), then the **Plans** tab. Each row is one plan: title and id, draft or published (with an "unpublished changes" mark when the working copy is ahead of what learners read), audience and assignee, revision, how many fields a person last wrote, and the last update. Open a plan from its title.

**New plan** asks for an id (a lowercase slug), a title, and optionally a recipe. It makes a draft at revision 1, shared, with the lesson **hidden** and one placeholder block. The recipe sets the lesson's starting recipe; it writes no Hebrew and no blocks. Insert a real block, then remove the placeholder. The AI makes plans the same way with `create_plan`, so a plan you did not create may already be in the list.

**Delete** on a row, or **Delete plan** in the editor, removes that plan, its published copy, and its history. A worksheet lesson with the same id stays on the Lessons tab until you delete it there. The AI does the same with `delete_plan`.

### The editor

The form is generated from the plan JSON Schema (`docs/decisions/ADR-0003-assets/lesson-plan.schema.json`), so it covers every block type, including `explainer`, `conjugation-table`, `personal-question`, `verse-reading`, and `letter-intro`, and the lesson's `recipe`, `newWords`, `buildsOn`, `recycles`, and `visibility`. A field the form has no widget for is edited as JSON. Each field has its own **Save field** button; a save sends one operation and the revision in the toolbar goes up by one. Hebrew fields are right-to-left; keep the full vowel points.

If you save while the AI has written since you loaded the plan, you get `revision_conflict` and the revision on the server. Your typed text stays on the screen. **Reload** takes the server's copy and drops unsaved text. Nothing merges by itself.

Plan **audience** and **assignee** are not form fields. They change only through the assign control under the toolbar: shared, or personal for one active account.

### History, Compare, and Restore

Open **History** on a plan. Each row is one change: the revision, who made it, when, the operation, and the path and field. The writer is **a person** (your account name), **AI** (with the tool name), or **system** (a baseline row, or a script). Filter by writer, and load older rows.

**Compare** shows two states side by side, field by field: any two revisions, a revision against the working copy, or the working copy against what is published. Hebrew values keep their direction.

**Restore** puts an earlier value back in one step: "Restore this value" for a field, "Restore this part" for a block or item, or "Restore the whole plan to this revision" on any row. You confirm once, inline. A restore is itself a **new revision**: it does not rewind history, the AI can see and undo it, and a whole-plan restore keeps the plan's current audience and assignee. A restore uses the revision shown in the toolbar, so if the AI wrote in between you get `revision_conflict` and nothing changes. A field a person last wrote carries an **Edited by a person** mark that opens to who, when, and the value that person replaced, with "Restore the earlier value". The mark is information. It never disables a field.

While a plan exists, history is never edited. No screen or tool rewrites a revision. Deleting the plan removes that history with it.

### The Issues panel

**Issues** lists everything the server finds in the plan, in the plan's working copy, refreshed after each save. Hard errors come first (they stop a publish). Below them are warnings, each with a stable code:

- `pointing-form` — vowel points in an impossible or non-canonical order. The suggested value is the canonical one.
- `field-language` — Hebrew in an English field, or the reverse.
- `lexicon-form` — a word pointed differently from its lexicon entry, or a meaning that differs from a **verified** entry. **Use this value** applies the lexicon form in one save. Only a verified entry is an authority; an unverified one is a lead.
- `pointing-consistency` — one word pointed two ways in a lesson. Pick one form; both may be right.
- `audio-stale` — text changed since the last publish while its clip stayed. A text fix leaves the old clip attached; no tool regenerates audio for a plan field, so the fix is to remove the stale clip path or set a new existing `.mp3`.

Warnings never block a save or a publish. They are leads. A link on each issue opens the field.

### Preview, publish, and assign

The **preview** beside the form draws the selected block (or the lesson card) the way a learner's screen does, from the text in the form, saved or not. Its label says "Preview — not saved" or "Preview — as saved". It is a view inside the admin page and no learner ever reaches it: learners read only the last **published** snapshot of a plan, and a draft answers "not found" to a learner.

**Publish** sends the working copy to learners (if the lesson's `visibility` is `everyone` or `selected`, or the plan is personal and assigned). The line above the button shows the counts of hard errors and warnings and says warnings do not stop a publish. A hard error (`validation`) or a pending picture on a lesson visible to learners (`pending_media`) refuses it. Pictures are still approved by you, one at a time (see [Pictures](#pictures)).

### What the AI can do without you

With `MCP_AUTH_TOKEN`, the AI can create a plan, edit any field, publish, set a lesson's `visibility` (so it can expose a lesson to learners the moment it publishes), restore an earlier revision, and assign a plan to an active account. No approval step stands in the way. The protection is the record: History shows every change and its writer, Compare shows exactly what changed, and **Restore** undoes it in one step. The skill asks the AI to keep new lessons `hidden` unless you asked otherwise; that is guidance, not a lock the server enforces.

It still cannot mark a lexicon form **verified** (`set_lexicon_verified` answers `forbidden`; you tick the forms checklist), approve a picture (only your admin session can), or turn on a learner's **AI planning** switch (`set_ai_planning` answers `forbidden`).

### Who made a change

The AI's tools all arrive on one shared bearer token, so every AI client shows as `mcp`, and two AI clients cannot be told apart. Your own edits show your account name. The AI is not told your name: its tools show a person's change as `human`. Nothing the AI sends can claim to be you.

### When something looks wrong

Open the plan, then **History**, find the row that changed the field, use **Compare** to see before and after, and **Restore** the value. If a learner saw it, publish again once it is fixed. To see what the AI has been doing, filter History by writer. The audit log also holds one `plan.*` row per change, with the writer and the path, never the text.

## Backup and restore

### Local

With `DATA_DIR` set to the same folder the app uses:

```powershell
npm run db:backup
```

Backups land under `DATA_DIR/backups/hebrew-<timestamp>/`. Each directory contains `hebrew.sqlite`, `media/`, and `media/_archive/`. The command does not read lesson `.mp3` files, and it does not move or delete them.

Check integrity:

```powershell
npm run db:check
```

To restore: stop the app (no writers). From that same backup directory, replace `DATA_DIR/hebrew.sqlite` and replace `DATA_DIR/media` (including `media/_archive`). Remove `hebrew.sqlite-wal` and `hebrew.sqlite-shm` while the process is stopped so they are not replayed over the restored file. Do not delete recordings under the lessons folder. Start again, run `npm run db:check` (expect `ok`), and confirm an approved image still resolves. Confirm sign-in works before treating the restore as done.

### Kubernetes

From [deploy/k8s-database.md](../../deploy/k8s-database.md):

```bash
kubectl exec -it deploy/<name> -- npm run db:backup
# Copy the whole directory (hebrew.sqlite, media/, and media/_archive/):
kubectl cp <namespace>/<pod>:/data/app/backups/hebrew-<stamp> ./hebrew-<stamp>
```

Restore (still `replicas: 1`; stop writes first — scale to zero or a maintenance window). Full commands, including the media tree and the WAL files, are in [deploy/k8s-database.md](../../deploy/k8s-database.md). Do not delete `/data/worksheets`.

Then start the pod again and verify with `npm run db:check` (expect `ok`). Confirm the restored `media/` files match that backup and an approved image still resolves. Confirm the app is healthy and sign-in works before ending the maintenance window.

## Pictures

Images live under `DATA_DIR/media/`. They are not a public folder. Upload and generation store a pending PNG. Backup copies that tree and `DATA_DIR/media/_archive/` with the database. Restore stops writers first, restores both trees from the same backup, then checks that an approved image still resolves. Commands are under [Backup and restore](#backup-and-restore).

### Upload

An admin uploads with `POST /api/media`. The MCP tool `upload_media` stores the same pending image and does not approve it. Send PNG, JPEG, or WebP bytes. The filename and the declared content type are ignored. SVG is refused. The server re-encodes one PNG under `DATA_DIR/media/` and does not keep the original filename. A learner cannot upload.

You may send an English alt-text draft with the upload. Empty alt text cannot be approved later.

### Approval

Only an admin session can approve: `POST /api/admin/media/:id/approval` with JSON `altText`. Alt text is a short English statement of the word's meaning. Approval is refused while alt text is empty. The MCP actor cannot approve. Uploading is not approval, even when the same admin does both.

A lesson set to **everyone** or **selected** cannot reference a pending image. A **hidden** lesson may, so you can preview it.

Before you approve a generated picture, check that the object matches the gloss, the frame has nothing the gloss does not need, the pixels contain no letters, and the picture is one word. A photographic look is not a reason to approve. Keep one approved image per word unless the lesson teaches a different sense.

### Generate

The server reads the name `OPENAI_API_KEY` from the environment (see the table below). Do not put a value in this file, a command, or a log. If that name is unset, dry-run and generate both refuse and do not call out.

Dry-run first. An admin calls `GET /api/admin/images/image-plan` with the prompt. MCP calls `plan_image`. The answer is the image count and the estimated dollars. It does not call the provider and does not spend a cap.

Generate is a separate call. It must send `confirm` true and the dollar estimate from that dry-run of the same prompt. Admin: `POST /api/admin/images`. MCP: `generate_image`. Without that confirm, the server does not call the provider. The new image stays pending.

Caps, one pool shared by admin and MCP:

- 15 generation calls per clock hour
- $1 of estimated generation cost per UTC day

A call that would pass either cap is refused before the provider is contacted. A call that does contact the provider counts toward both caps, including a provider error after the request was sent. The dry-run counts toward neither.

The prompt is one ordinary object or action. The server refuses a prompt that asks for a sexual act, a minor, or a real person's likeness. Learner free text is never the prompt. Adult lessons stay text.

### Delete

Only an admin can delete an image: `POST /api/admin/media/:id/delete`. Delete copies the bytes to `DATA_DIR/media/_archive/`, checks the copy, then removes the live file. Dropping a lesson reference does not delete the file.

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
| `DATA_DIR` | SQLite, backups, and media (`hebrew.sqlite`, `backups/`, `media/`, `media/_archive/`). Not a served root. |
| `COOKIE_SECURE` | `0` locally; `1` on HTTPS / k8s (Secure cookie + `__Host-` prefix) |
| `PUBLIC_ORIGIN` | Public site origin used for CSRF Origin checks |
| `MCP_AUTH_TOKEN` | Required for MCP assignment and picture tools (`list_users`, visibility, `upload_media`, `plan_image`, `generate_image`) |
| `ELEVENLABS_API_KEY` | TTS; never commit or bake into the image |
| `OPENAI_API_KEY` | Image generation; never commit or bake into the image. In k8s, Secret `hebrew-secrets` key `OPENAI_API_KEY` |
| `ADMIN_USERNAME` | First admin, only when the database has none |
| `ADMIN_PASSWORD` | Password for that first admin. Ignored once an admin exists |
| `ADMIN_DISPLAY_NAME` | Optional display name for that first admin |
| `SRS_SCHEDULER` | `ladder` (default) or `fsrs`. Not a secret. Read once when the server process starts. See [Ladder fallback](#ladder-fallback). |
| `TRUSTED_PROXIES` | Comma-separated IPv4 or IPv6 addresses or CIDRs. The login throttle uses `CF-Connecting-IP` only when the socket peer is in this list; otherwise it uses the socket address. Empty trusts nobody. Not a secret. Invalid entries are ignored at startup with one warning. Set Traefik's pod CIDR before a production deploy — see [deploy/k8s-database.md](../../deploy/k8s-database.md). |

**Local compose:** `COOKIE_SECURE=0`, `PUBLIC_ORIGIN=http://localhost:8080`.

**Kubernetes:** `COOKIE_SECURE=1`, `DATA_DIR=/data/app`, `LESSONS_DIR=/data/worksheets`. `SRS_SCHEDULER` is a plain environment variable, not a secret; `ladder` is the fallback. `TRUSTED_PROXIES` is also plain env. Production is Cloudflare, then Traefik. Note for the team lead (Hebrew Dev) and CI/CD: set `TRUSTED_PROXIES` to Traefik's pod CIDR before or with the deploy, and confirm Traefik passes `CF-Connecting-IP` through untouched. Until that value is set, every sign-in is keyed on the Traefik socket address, so all users share one throttle bucket. `MCP_AUTH_TOKEN`, `ELEVENLABS_API_KEY`, `OPENAI_API_KEY`, `ADMIN_USERNAME`, and `ADMIN_PASSWORD` come from Secret `hebrew-secrets` via `secretKeyRef` (keys with those same names). Media files are `/data/app/media/` and `/data/app/media/_archive/` on PVC `hebrew-data`. Full Deployment snippet: [deploy/k8s-database.md](../../deploy/k8s-database.md). Do not paste secret values into docs or YAML.

## Privacy: who can see what

- **Admins** can see everything a learner does, including the full answer history.
- **Learners** are told this on the sign-in page.
- **The AI and MCP tools** follow the rule in [AI planning](#ai-planning) below. That section is the only statement of what they may read.
- **`list_users`** returns only usernames and display names of active accounts (nothing else).

## AI planning

An admin turns AI planning on for one account at a time. The switch starts off. While it is off, `get_learner_profile`, `check_text_coverage`, and `add_library_words` return `planning_off` and nothing else. The MCP cannot flip the switch: `set_ai_planning` always returns `forbidden`.

The rule this guide puts in force (ADR-0009 §9):

> MCP reads only the profile in ADR-0009 §1, for learners whose switch is on. That profile is aggregates: word ids with stage, strength, weak flag and a capped lapse count; accuracy per grammar concept; a journey label; and lessons completed. The MCP still never reads answer history, typed text, timestamps, passwords, sessions, or admin-only views.

What that call can return, and what it must never return, are the two tables in [ADR-0009 §1](../decisions/ADR-0009-learner-model-and-language-program.md). This guide does not copy those tables. The disclosure sentence for the sign-in page, the account menu, Progress, and the admin confirm step is:

> If an admin turns on AI planning for your account, a summary of your progress and your username are sent to the operator's AI provider so it can draft your next lesson. The summary covers the words you have practiced and how well you know them, the grammar you have practiced and how accurate you are, the lessons you have finished, and how often you read with vowel points off. It does not include your individual answers, anything you typed, or when you practiced.

That sentence does not replace the notice that admins can see all activity.

### View it on Progress

Sign in as the account you want to inspect. Open **Progress** (`/progress/`) and read **What your AI sees**. That section is `GET /api/me/profile` for the signed-in account: the same object the MCP would receive, plus whether AI planning is on, whatever the switch says. There is no browser page that loads another learner's profile. The sample account id in this guide is `example-learner`.

### Turn it on for your own account

Sign in as an admin. Open **Admin** (`/admin/`) and the **Users** tab. On your own row, read the disclosure sentence above, then turn AI planning on. The request is `PATCH /api/admin/users/:id/ai-planning` with a JSON body `{ "enabled": true }`. Turning it off sends `{ "enabled": false }`. The new state is whatever the response returns. Do not ask the lesson AI to change the switch.

### Forms checklist

Starter verb forms stay unverified until you check the pointing. The list is [SPEC-018-forms-checklist.md](../../sdd/initiatives/INIT-006-language-program/specs/completion-summaries/SPEC-018-forms-checklist.md). Tick a line (`[x]`) only after you have checked that form. Then, from the repo root, with `DATA_DIR` set to the database you mean to update:

```powershell
node scripts/seed-starter-content.js
```

That run marks the ticked forms verified and leaves every other form unverified. It does not publish a plan. Untick a line and run the command again to clear that form. The MCP cannot do this step.

### Ladder fallback

Review scheduling uses `SRS_SCHEDULER`. Unset, empty, or `ladder` keeps the five-step ladder. `fsrs` selects FSRS-6. The process reads the variable once at startup, so a change applies on the next start.

To fall back, set `SRS_SCHEDULER` to `ladder` and start the server again. No down migration is required. The variable is not a secret; do not put a token or a password in it, and do not paste secret values into the environment file while you edit the name.

## Working with the AI (MCP)

Useful tools once `MCP_AUTH_TOKEN` is set:

- `list_users` — usernames and display names of active accounts
- `upsert_lesson` — optional `visibility`: `"everyone"` or `{ "users": ["username", ...] }`
- `set_lesson_visibility` — `"everyone"`, `"hidden"`, or `{ "users": ["username", ...] }`
- `upload_media` — store one pending image from base64 bytes. Does not approve.
- `plan_image` — dry-run: image count and estimated dollars. Does not call the provider.
- `generate_image` — only when `confirm` is true and `estimate` matches that dry-run. The image stays pending.
- Plan builder (see [Plan builder](#plan-builder)): `list_plans`, `create_plan`, `read_part`, `replace_part`, `insert_lesson`, `insert_block`, `move_block`, `remove_block`, `set_field`, `list_plan_issues`, `list_plan_revisions`, `get_plan_revision`, `diff_plan`, `restore_plan_revision`, `assign_plan`, `publish_plan`, `delete_plan`. New lessons are plans; `upsert_lesson` stays for a worksheet that has not moved. Add lesson and Reuse lesson on the teacher page both call `insert_lesson` (`INIT-011/SPEC-001`). `delete_plan` removes the plan and its history. `delete_lesson` removes a worksheet from the shelf and archives its recordings.
- `delete_lesson` — the Lessons tab **Delete** does the same. **Edit** on that row opens the worksheet. Neither one deletes a plan that happens to use the same id.

Approving a picture, and deleting an image, are admin session routes, not MCP tools. If `MCP_AUTH_TOKEN` is unset or empty, the assignment tools and the picture tools refuse. A lesson created with no assignment stays **hidden** until you share it in the admin Lessons tab or call `set_lesson_visibility` / pass `visibility` on upsert.

## Known limits

- **One replica** — SQLite (and in-memory sign-in throttling) do not support multiple writers. Keep `replicas: 1` and `strategy: Recreate` on the cluster.
- **`file://` no longer works** — open the site through the HTTP(S) origin (local compose or the public host).
- **Hash-named word mp3s** under the shared words cache require sign-in, but are **not** filtered per lesson.
- **No Content-Security-Policy** from this app. A reverse proxy in front of the site may add one. Pages load no stylesheet, script, or font from another host (Rubik is served from `shared/fonts/`, icons are inline SVG), so a strict policy such as `style-src 'self' 'unsafe-inline'` works. Keep it that way: `test/self-hosted-assets.test.js` fails if a page links an outside host, including the new-lesson template.
- **Public paths** are only `/login/`, the web-app manifest and icon (`/manifest.webmanifest`, `/icon.svg`), and the font files under `/shared/fonts/`. Everything else needs a sign-in.

## Google sign-in

App sign-in and MCP consent use one Google client, the same client oauth2-proxy uses for Tautulli. Password sign-in stays. A new Google account becomes a learner: the username comes from the email, and the display name comes from the Google profile. An admin can still attach a Google email to an existing user (Users → Google). MCP consent is a separate allowlist, `MCP_OAUTH_ALLOWED_EMAILS`. Redirect URIs to add on that client are listed in `deploy/k8s-database.md`.
