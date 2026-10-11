# ADR-0004: Learner data, media, and secrets

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Proposed |
| **Date** | 2026-10-08 |
| **Authored by** | `INIT-003/SPEC-005` |
| **Initiative** | INIT-003 · lesson-system-foundations |
| **Research grounding** | `RSCH-005` (AI-image failure modes and the human review step); operator decisions 2026-10-08 for the snapshot, the provider handoff, the switch default, and approval |
| **Supersedes** | [ADR-0002](ADR-0002-accounts-server-progress-site-map.md) in part — the rule that the MCP never reads learner progress. Replacement wording is §11. ADR-0002's other decisions stay in force. |
| **Superseded by** | — Superseded in part by [ADR-0009](ADR-0009-learner-model-and-language-program.md), Accepted 2026-10-09: §1 (the snapshot becomes the aggregate profile), the "what the operator is accepting" paragraph in §2, the disclosure sentence in §3, the snapshot rate-limit and matrix row in §4, the `learner_snapshot_read` audit row in §10, and the MCP wording in §11. The full list with line numbers is ADR-0009 §9. |

## Summary

Personal lesson plans need a small, named snapshot of what a learner has already passed. That snapshot may leave this server for the operator's AI provider only when an admin has turned AI planning on for that learner. The switch starts off. Images may be uploaded or generated. Generated images stay out of a published lesson until an admin has approved the pixels and the alt text. The image call uses a server environment key, a dry-run cost preview, and spend caps. The provider is OpenAI's Images API.

---

## Context

### Problem statement

INIT-003 lets the operator's AI draft the next lesson from what a learner already knows, and lets admins and that AI add pictures by upload or by generation. ADR-0002 and INIT-002 REQ-016 forbid the MCP from reading learner progress. Generation adds an outbound call, a second secret, a spend risk, and a file type the site does not serve today. Both changes need a decision before any code.

### Current state

- Accounts, sessions, review rows, practice events, and lesson-round blobs live in SQLite under `DATA_DIR`. Admins can read a learner's full answer history. Learners are told that on the sign-in page and in the account menu (`docs/operations/accounts-and-admin.md`).
- MCP tools are `list_users`, `upsert_lesson`, and `set_lesson_visibility`. They require `MCP_AUTH_TOKEN`. `list_users` returns usernames and display names of active accounts. No MCP tool reads progress.
- Memory stage for one direction is already defined in `server/progress.js` (`stageFor`): `learning` or `longTerm`, matching ADR-0002 §10. A pass is a `practice_events.result` of `pass`.
- Lesson-round state is the JSON blob on `lesson_progress`, including `completedRounds`.
- Audio generation is admin-only, after a dry-run (`audio-plan`), and the ElevenLabs key is an environment variable. `db:backup` copies the SQLite file only.
- There is no media store, no image route, and no image-generation key.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | The snapshot contract names every returned field and every excluded field. INIT-004's tool tests check this table. |
| R2 | A per-learner switch, default off, gates the snapshot. Learners are told. Every read and every switch change is audited. |
| R3 | Uploads accept PNG, JPEG, and WebP only, by bytes, and are re-encoded before anything is stored. |
| R4 | A generated image is invisible to learners until an admin approves it. The approval uses the `RSCH-005` failure list. |
| R5 | The generation key follows the ElevenLabs rule: server environment only. |
| R6 | ADR-0002 and INIT-002 REQ-016 change only where §11 says so. |

### Constraints

- No code in this spec. Plan-language media fields belong to ADR-0003. The stack belongs to ADR-0005.
- No secret values, learner names, or transcript text in this record.
- The operator confirmed on 2026-10-08: the minimum snapshot; the snapshot may reach the AI provider when the switch is on; the switch defaults off; generated images need human approval; the provider is chosen here by a short public-doc comparison.
- MCP authentication stays `MCP_AUTH_TOKEN`. There is no anonymous MCP mode.

---

## Options considered

The eleven decision areas are in **Decision**. Two comparisons are not repeated elsewhere:

- What the model may see about a learner: no learner data, the minimum snapshot, or a richer snapshot (§1).
- Who generates images: OpenAI Images, Google Imagen on Vertex AI, or Stability AI (§7). Prices and policy quotes below were read from the vendor pages on 2026-10-08.

---

## Decision

### 1. What the AI may read

**Options considered:**

1. **No learner data.** The AI drafts only from the shared course. This keeps ADR-0002's MCP rule intact. It cannot draft from what this learner has passed, which is the personal-plan requirement (INIT-003 REQ-006).
2. **Minimum snapshot.** Word ids the learner has passed, memory stage per direction, and lessons completed. **Chosen.** Operator 2026-10-08.
3. **Richer snapshot.** Answer history, timestamps, wrong answers, typed text, accuracy, and trouble words. Rejected. Those fields are the admin view ADR-0002 kept off the MCP, and a next-lesson draft does not need them.

**Chosen snapshot.** One object, returned only when §3's switch is on for that learner. `word_id` is the ADR-0001 identity key (NFC pointed Hebrew). Stage strings are `learning` and `longTerm`, the values `stageFor` already returns. A direction with no pass is omitted. `new` is omitted. A word with no pass in either direction is omitted.

| Field | Returned |
|-------|----------|
| `username` | The account username. Present on every snapshot that is returned. |
| `words[].word_id` | Identity key of a word with at least one `pass`. |
| `words[].he_en_stage` | `learning` or `longTerm` when `he_en` has a pass. Omitted when it does not. |
| `words[].en_he_stage` | `learning` or `longTerm` when `en_he` has a pass. Omitted when it does not. |
| `lessons_completed` | Lesson slugs whose stored `completedRounds` is greater than zero. The array is present even when empty. |

| Never returned |
|----------------|
| Display name, numeric user id, role, password hash, session token, or any other account secret |
| `practice_events` rows: result, time, lesson slug, activity, client event id |
| Review-row schedule fields: `due`, `step`, `streak`, `lapses`, `last_result`, `last_at`, sources |
| Direction accuracy, trouble words, miss counts |
| Lesson-blob fields other than the completed-slug list: `wordMarks`, `missedLast`, `activityState`, `completedSteps`, `stepIndex`, `roundStartedAt`, typed answers, free text |
| English glosses or any other meaning text |
| Timestamps of answers or of lesson completion |

This table is the contract. A snapshot response that contains a name from the "Never returned" table fails the INIT-004 tool test.

### 2. Where that data goes

**Options considered:**

1. **Keep the snapshot on this server** and let only a local model read it. Rejected. The operator's authoring agent is the existing MCP client. Its model runs at the provider on the other side of that client.
2. **Send the snapshot to that provider when the switch is on.** **Chosen.** Operator 2026-10-08. The operator accepts this handoff.
3. **Replace `word_id` with an opaque integer** so the provider sees no Hebrew. Rejected as the contract. The identity key is the Hebrew form, and a personal plan has to name those words. English glosses are still withheld (§1).

**What the operator is accepting.** For a learner whose switch is on, a call to `get_learner_snapshot` puts that learner's username, passed-word identity keys, the two stage strings, and completed lesson slugs into the MCP client's context. The client's model provider therefore receives them. Hebrew forms of passed words are included, because they are the identity keys. Data already sent is not recalled when an admin later turns the switch off. Further reads stop at the next call (§4).

The snapshot is not written into lesson files, image files, prompts, or logs beyond the audit row in §10.

### 3. Consent, disclosure, and control

**Options considered:** (1) per-learner admin switch, default off, with disclosure beside the existing admin-visibility notice — **chosen**; (2) on for every learner — rejected (operator: default off); (3) learner self-serve toggle — rejected (admins create accounts; there is no public sign-up, and the operator asked for an admin switch).

| Rule | Decision |
|------|----------|
| Who sets it | An admin, on one account at a time |
| Default | Off for every current account and every account created later |
| Off means | `get_learner_snapshot` returns no snapshot fields (§4) |
| Disclosure | The sign-in page and the account menu each show the sentence below, in addition to the existing sentence that admins can see all activity. The new sentence does not replace the admin-visibility sentence. |
| Audit | Every change writes the row in §10 |

Disclosure sentence, same words in both places:

> If an admin turns on AI planning for your account, a short summary of words you have passed and lessons you have finished is sent to the operator's AI provider so it can draft your next lesson. The summary does not include your answers or anything you typed.

### 4. Access limits and the role × route matrix

**Options considered:** (1) token-authenticated MCP, a rate limit, an audit row, and an empty response when the switch is off — **chosen**; (2) anonymous MCP reads — rejected (ADR-0002 removed `MCP_ALLOW_NO_AUTH`); (3) reuse `GET /api/admin/users/:id/events` for the model — rejected (that route is the full answer history).

**Authentication.** Every MCP tool, including the new ones in the matrix, refuses unless `MCP_AUTH_TOKEN` is configured and the request presents it. A missing or wrong token is a refusal with no snapshot body.

**Switch off.** The tool returns a refusal named `planning_off` and no field from §1. An empty `words` array is reserved for a learner whose switch is on and who has no passes yet. The refusal still writes an audit row (§10).

**Snapshot rate limit.** 60 reads per clock hour for the MCP credential. A read over the limit is refused and writes an audit row with no snapshot body.

**Matrix.** Roles are the signed-out browser, `user`, `admin`, and the `mcp` actor. Browser routes use the session. MCP tools use the token. A blank cell is a defect. `404` is the response when the caller must not learn whether an id exists.

| Route group | Representative route | Signed out | `user` | `admin` | `mcp` |
|-------------|----------------------|------------|--------|---------|-------|
| Media upload | `POST /api/media`, MCP `upload_media` | 401 | 403 | Allow | Allow with token |
| Media fetch | `GET /api/media/:id` | 401 | 200 only when a lesson this user may see references the id; otherwise 404 | 200 when the id exists, including an unreferenced item; 404 when it does not | 200 when any lesson references the id; 404 when nothing references it or the id is missing |
| Media approval | `POST /api/admin/media/:id/approval` | 401 | 403 | Allow | 403 |
| Generation | `GET /api/admin/images/image-plan`, `POST /api/admin/images`, MCP `plan_image`, MCP `generate_image` | 401 | 403 | Allow, under §7 | Allow with token, under §7 |
| Snapshot read | MCP `get_learner_snapshot` | Refuse (no token) | No browser route | 403. Admin progress routes stay as ADR-0002 wrote them | Allow with token when that learner's switch is on. Switch off: `planning_off` and no §1 fields |
| AI-planning switch | `PATCH /api/admin/users/:id/ai-planning` | 401 | 403 | Allow | 403 |

### 5. Upload security

**Options considered:** (1) sniff, re-encode, strip metadata, store outside served roots under a random id — **chosen**; (2) trust the multipart filename and `Content-Type` and write the bytes under the lesson folder — rejected (content-type confusion, SVG script, path under a served root); (3) allow SVG — rejected (SVG is active content).

**Allowed input.** PNG, JPEG, and WebP. The check is the file bytes. The filename and the client `Content-Type` are ignored. SVG is refused, including a file whose name ends in `.png`.

**Sniff, then decode.** Accept only a buffer that starts as PNG (`89 50 4E 47`), JPEG (`FF D8 FF`), or WebP (`RIFF` and `WEBP`). Then decode. Refuse a buffer the decoder rejects, a buffer with more than one frame, and any animated PNG, animated WebP, or multi-frame JPEG.

**Limits, stated once.**

| Limit | Value |
|-------|-------|
| Maximum input bytes | 5242880 |
| Maximum width and maximum height | 4096 pixels on either side |
| Stored image | PNG, longest side 2048 pixels |
| Stored name | 32 cryptographic random bytes, hex, plus `.png` |
| Upload rate | 40 uploads per actor per clock hour |

**Ingest.** Measure the byte length before decode. Measure width and height from the decoded image. Re-encode to a single-frame PNG at the stored size above. Do not copy EXIF, GPS, device, or other metadata. Do not keep the original filename, including in the database. If re-encode fails, store nothing.

**Where bytes live.** `DATA_DIR/media/` for the live file and `DATA_DIR/media/_archive/` for §9. `DATA_DIR` is already required to sit outside every served root (ADR-0002). Media must not be written under `worksheets/`, `shared/`, a page directory, or any other path the static server can answer by URL.

**Response.** `Content-Type: image/png` and `X-Content-Type-Options: nosniff` on every media response. The original declared type is never the response type.

### 6. Media serving and visibility

**Options considered:** (1) one gated route, lesson visibility for referenced items, admins only for unreferenced items — **chosen**; (2) a public `/media/` folder — rejected (bypasses lesson visibility); (3) bake images into the lesson folder under the original filename — rejected (§5).

Fetch authorization is the matrix row **Media fetch**. A `user` fetches an image only when they may see a lesson that references it, the same rule as lesson files. An image that no lesson references is fetchable by an admin only. Missing ids and forbidden ids both answer 404.

A lesson with visibility `everyone` or `selected` must not reference an image whose status is `pending`. The publish path refuses that lesson. A `hidden` lesson may reference a `pending` image so an admin can preview it. Learners do not see `hidden` lessons, so they do not receive that image.

### 7. AI image generation

**Options considered:** OpenAI Images, Google Imagen on Vertex AI, and Stability AI. Compared on 2026-10-08 from the pages in References. The preference was a server-side key in the same pattern as `ELEVENLABS_API_KEY`.

| Provider | How the server authenticates | Cost cited | Content rules cited | Data terms cited |
|----------|------------------------------|------------|----------------------|------------------|
| OpenAI Images, model `gpt-image-1` | Environment key on `Authorization: Bearer`, endpoint `POST /v1/images/generations` | Low quality, 1024×1024: $0.011 per image. Medium $0.042. High $0.167. Same page, same size. | Usage policies effective 2025-10-29 prohibit sexual violence, non-consensual intimate imagery, and sexual content involving anyone under 18. | API data is not used to train models unless the customer opts in (in force since 2023-03-01). For `/v1/images/generations`: training "No", abuse-monitoring retention 30 days, application-state retention none, Zero Data Retention eligible for `gpt-image-1`. |
| Google Imagen on Vertex AI | Cloud credentials. Documented API keys apply to `generateContent` and `streamGenerateContent`, not to Imagen's `:predict` call. | Imagen 4 $0.04 per image, Imagen 4 Fast $0.02, Imagen 4 Ultra $0.06. | Safety filters include a sexual category. `personGeneration` can block people and faces. | Not used to choose. The authentication mismatch is enough to reject this option, and this record does not paraphrase a data-governance page it did not retrieve. |
| Stability AI Platform | Environment key on `Authorization`. Getting Started documents that header. | 1 credit = $0.01. Stable Image Core is 3 credits. Stable Image Ultra is 8 credits. Stable Diffusion 3.5 Flash is 2.5 credits. | Acceptable Use Policy prohibits non-consensual intimate imagery, illegal pornography, and content relating to sexual intercourse or sexual acts. | Privacy Policy effective 2026-09-30: inputs and outputs are kept as long as reasonably necessary for the service, disputes, and legal duties. The privacy center tells API customers to set the account toggle "Training: Improve the Model for Everyone" to No if they do not want inputs and outputs used for training. |

**Chosen: OpenAI Images, pinned to `gpt-image-1` at the low-quality size and unit price in the table.**

Rationale. The call takes one environment key, which matches the ElevenLabs pattern. Google's Imagen predict call does not. OpenAI's API terms exclude training unless the customer opts in, and the table names the abuse-monitoring window for this endpoint. Stability's retention is open-ended, and training is a toggle the customer must turn off. The pinned OpenAI price is a published per-image amount, so the dry-run can multiply it. The model page calls `gpt-image-1` the previous image model. If that model id is rejected by the provider, generation stays disabled until this ADR is amended. The server must not substitute another model. The model card for `imagen-4.0-generate-001` lists discontinuation on 2026-06-30, which is before this decision, while the pricing page still lists Imagen 4. That unsettled lifecycle is a further reason Imagen was not chosen. This record does not rest the Google rejection on training terms.

**Call site and key.**

| Rule | Decision |
|------|----------|
| Where the call runs | This server only. The browser and the MCP process do not hold the key and do not call the provider themselves. |
| Key | Environment variable `OPENAI_API_KEY`. Same rule as `ELEVENLABS_API_KEY`: never in a source file, container image layer, log line, HTTP response, lesson file, or prompt record. |
| Missing key | The server refuses plan and generate and does not call out. |
| Prompt scope | One ordinary object or action for a vocabulary sense (`RSCH-005`). The server refuses a prompt that asks for a sexual act, a minor, or a real person's likeness. Adult lessons stay text. Learner free text is never the prompt. |

**Dry-run and caps, stated once.**

| Limit | Value |
|-------|-------|
| Dry-run | `image-plan` / `plan_image` returns the image count and the estimated US dollars (count times the pinned unit price in the comparison table). It does not call OpenAI. |
| Generate | A separate call. It carries `confirm: true` and the estimate from the dry-run of that same prompt. Without that confirm, the server does not call the provider. |
| Per-hour cap | 15 generation calls per clock hour, one cap shared by admin and `mcp` |
| Daily spend cap | $1 of estimated generation cost per UTC day |

A generate call that would pass either cap is refused before the provider is contacted. A call that does contact the provider counts toward both caps, including a provider error after the request was sent. The dry-run counts toward neither.

**Record and approval.** Each generated image stores the prompt, the provider name `openai`, the model id `gpt-image-1`, the actor, and the time. Status starts at `pending`. A generated image can appear in a published lesson (visibility `everyone` or `selected`) only after the human approval in §8. This is mandatory. `RSCH-005` rates teaching images as often wrong while looking real, and its review step is a person, not the generator.

The admin checks each failure mode from `RSCH-005` before approving:

| Failure | Check |
|---------|-------|
| Wrong object or action | The depicted thing is the gloss |
| Extra story | No object the gloss does not need |
| Cultural mismatch or stereotype | Ordinary referent; no stock role for a kind of person |
| Text inside the image | No letters or captions in the pixels |
| Ambiguous depiction | One word, not two |
| Looks photographic | Plausibility is not approval |
| Inconsistent picture | One approved image per word unless the lesson teaches a different sense |

### 8. Alt text and accessibility

**Options considered:** (1) English alt text required, draftable by the admin or the model, approved by an admin — **chosen**; (2) optional alt text — rejected (`RSCH-005` picture rule: the first-language meaning has to survive if the image fails); (3) model-approved alt text — rejected (the same human gate as the pixels).

Every image that a lesson may reference has alt text: a short English statement of the word's meaning. The admin or the model may draft it. An admin approves the pixels and the alt text in one action. Approval is refused while alt text is empty. The `mcp` actor cannot approve (§4). The uploading admin may also be the approving admin. The approval is still its own audited action, so an upload is not itself a publish.

### 9. Retention and backup

**Options considered:** (1) keep media under `DATA_DIR`, archive before delete, include the files in backup — **chosen**, extending INIT-002's audio archive rule to images; (2) delete an image as soon as no lesson references it — rejected (a later lesson may need the approved picture, and deletion would skip the archive); (3) rely on today's `db:backup` — rejected (`db:backup` copies `hebrew.sqlite` only).

| Rule | Decision |
|------|----------|
| Live bytes | Kept under `DATA_DIR/media/` until an admin deletes them |
| Unreferenced image | Kept. Dropping a lesson reference does not delete the file. Fetch stays admin-only (§6). |
| Delete | Admin only. Copy the bytes to `DATA_DIR/media/_archive/` and verify the copy, then remove the live file. |
| Backup | The backup set includes `DATA_DIR/media/` and `DATA_DIR/media/_archive/` together with the SQLite file. INIT-004 adds that copy. Until it ships, `npm run db:backup` does not cover images. |
| Restore | Stop writers, restore the database and the media trees from the same backup, then check that approved images still resolve. |

### 10. Audit

**Options considered:** (1) one `audit_log` row per action below, actors `admin:<username>` and `mcp` — **chosen**, the same log ADR-0002 already has; (2) a separate media log — rejected (two logs to forget); (3) audit mutations only — rejected (snapshot reads are the sensitive new read).

| Action | Actor | Row records |
|--------|-------|-------------|
| `media_upload` | `admin:<username>` or `mcp` | actor, action, media id, time |
| `media_generate` | `admin:<username>` or `mcp` | actor, action, media id, time. The prompt is stored on the media row (§7), not copied into a log line that might be printed. |
| `media_approve` | `admin:<username>` | actor, action, media id, time |
| `media_delete` | `admin:<username>` | actor, action, media id, time |
| `learner_snapshot_read` | `mcp` | actor, learner username, time, the field names from §1 that were included, `word_count`, `lesson_count`. The word ids themselves are not copied into the audit row. A `planning_off` refusal and a rate-limit refusal record the learner and `fields: none`. |
| `ai_planning_switch` | `admin:<username>` | actor, learner username, time, previous value, new value |

### 11. Changes to earlier decisions

ADR-0002's body is not rewritten. Its metadata and changelog point here. The sentences below are the wording this ADR puts in force. Where a line is "unchanged", the quoted sentence remains the rule.

**New wording for the MCP progress rule:**

> MCP reads only the snapshot in item 1, for learners whose switch is on. That snapshot is ADR-0004 §1: word ids the learner has passed, memory stage per direction, and lessons completed. The MCP still never reads answer history, passwords, sessions, or admin-only views.

| Source | Line | Wording in force before this ADR | Wording this ADR puts in force |
|--------|------|----------------------------------|--------------------------------|
| ADR-0002 metadata, **Superseded by** | 13 | `—` | Superseded in part by ADR-0004. This is the note written into ADR-0002. |
| ADR-0002 requirement R9 | 55 | "MCP never reads progress (REQ-009)." | The MCP reads only the ADR-0004 §1 snapshot, and only when that learner's switch is on. Admin full answer history, and the learner notice about that history, are unchanged. |
| ADR-0002 authorization matrix, MCP tools note | 265 | "never progress/events" | The three existing tools still return no progress and no events. `get_learner_snapshot` is added and returns only §1, and only when the switch is on. |
| ADR-0002 §10 options | 336 | "(3) MCP-readable progress for tutoring — rejected (operator: MCP never reads learner progress)." | The rejection stands for metrics, events, trouble words, and answer history. It yields for the §1 snapshot only. |
| ADR-0002 §10 MCP row | 345 | "Never reads learner progress, metrics, events, or trouble words" | Reads only the §1 snapshot, and only when the switch is on. Still never reads metrics, events, or trouble words. |
| ADR-0002 consequences | 433 | "MCP can assign lessons safely only when token-authenticated, without reading progress." | Token authentication stays. The MCP may read the §1 snapshot when the switch is on. |
| ADR-0002 security | 494 | "MCP tools are token-gated and cannot read progress." | MCP tools stay token-gated. They read progress only as the §1 snapshot, and only when the switch is on. |
| INIT-002 REQ-016 | `INIT-002-multi-user-progress-initiative.md` line 73, last sentence | "The MCP never reads learner progress." | The new wording in the blockquote above. The rest of REQ-016 (list users, upsert, visibility, hidden-by-default) is unchanged. |

**Still in force**

| Rule | Where it stays |
|------|----------------|
| The MCP never reads answer history | ADR-0002 §10 admin full view; §1 "Never returned" |
| The MCP never reads passwords, hashes, or sessions | ADR-0002 §3 and §4 |
| The MCP has no admin events route and no admin progress route | ADR-0002 matrix row "Admin progress & activity"; this ADR's snapshot row gives `admin` 403 on `get_learner_snapshot` |
| `list_users` returns usernames and display names of active accounts only | ADR-0002 §11. The snapshot adds username and withholds display name. |
| ElevenLabs key in server env only | ADR-0002 carried constraints |
| Learners are told that admins can see all activity | ADR-0002 §10. §3 adds a second sentence. |
| No learner names, transcripts, or secrets in lesson files | ADR-0002 carried constraint 7 |

`docs/operations/accounts-and-admin.md` still says the AI cannot read progress. Updating that guide is INIT-004's job. This spec does not edit it.

---

## Consequences

### Positive

- Personal plans have a testable field list.
- Learners stay out of the provider's input until an admin opts that account in, and they are told where.
- Image bytes cannot arrive as SVG, as a declared type, or under a served path.
- Generation cost is previewed, capped, and tied to a key that is not in the repo.

### Negative / trade-offs

- For opted-in learners, Hebrew identity keys and usernames reach the operator's model provider, and abuse-monitoring copies can remain there for the window §7 cites.
- `gpt-image-1` is documented as the previous image model. A later removal disables generation until this ADR is amended.
- `db:backup` does not cover images until INIT-004 extends it.
- One operator can approve their own upload. The approval is audited and is still unavailable to `mcp`.

### Follow-ups (out of scope here)

- ADR-0003's media fields in the plan language.
- ADR-0005's runtime for decode, re-encode, and the outbound image call.
- INIT-004 implementation, tests against §1 and §4, backup of `DATA_DIR/media/`, and the operations-guide sentence.
- Setting `OPENAI_API_KEY` (`secret_change`) and any OpenAI organization verification the image models require. No key is created in this spec.

### Rollback

Delete this file and revert the ADR-0002 metadata cell and changelog row. No code lands here.

---

## Implementation plan (downstream)

| Consumer | What it takes from this ADR |
|----------|----------------------------|
| SPEC-007 | The stack must store media outside served roots, keep an env key, and make a server-side outbound image call |
| SPEC-010 | INIT-004's security classification: snapshot, upload, generation, approval, switch |
| INIT-004 | Routes in §4, tests that the §1 table is exact, dry-run and caps, backup of the media trees, operations-guide update |

---

## Security considerations

### Authentication and authorization

The §4 matrix is the authorization contract for the six new route groups. Snapshot reads require the MCP token and the per-learner switch. Approval and the switch are admin-only. Generation and upload are admin or token-authenticated MCP. Fetch follows lesson visibility, with unreferenced bytes limited to admins.

### Data protection

The snapshot is the only new learner data that leaves the server, and only when the switch is on. The "Never returned" column in §1 is closed: answer history, timestamps, wrong answers, free text, and names other than the username stay on the server. The generation key is an environment variable and is never written into source, images, logs, responses, or lesson files. Uploaded bytes are sniffed, decoded, re-encoded, stripped of metadata, renamed, and stored outside served roots.

### Residual risks (accepted)

1. **Provider receipt of the snapshot** — accepted by the operator on 2026-10-08 for learners whose switch is on, including the provider's abuse-monitoring retention cited in §7.
2. **`gpt-image-1` lifecycle** — the published per-image price is why it is pinned. If the id disappears, generation stops.
3. **Media backup gap** — until INIT-004, image files are not in `db:backup`.
4. **Admin full answer history** — unchanged from ADR-0002. The MCP still does not receive it.
5. **Single-admin self-approval** — accepted for this deployment, audited, and closed to the MCP actor.

---

## References

- `sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md` — failure modes and the human review step. Retrieved with the report, 2026-10-08.
- [ADR-0002: Accounts, server-side progress, and site map](ADR-0002-accounts-server-progress-site-map.md)
- [ADR-0001: Learner review model](ADR-0001-learner-review-model.md) — identity key
- INIT-002 REQ-016 — `sdd/initiatives/INIT-002-multi-user-progress/INIT-002-multi-user-progress-initiative.md` line 73
- OpenAI, "GPT-Image-1" (per-image prices, `POST /v1/images/generations`): <https://developers.openai.com/api/docs/models/gpt-image-1>. Accessed 2026-10-08.
- OpenAI, "Data controls in the OpenAI platform" (training opt-in, `/v1/images/generations` row): <https://developers.openai.com/api/docs/guides/your-data>. Accessed 2026-10-08.
- OpenAI, "How your data is used to improve model performance" (API default, in force since 2023-03-01): <https://openai.com/policies/how-your-data-is-used-to-improve-model-performance/>. Accessed 2026-10-08.
- OpenAI, "Usage policies", effective 2025-10-29: <https://openai.com/policies/usage-policies/>. Accessed 2026-10-08.
- Google Cloud, "Authenticate to Agent Platform" (API keys limited to `generateContent` and `streamGenerateContent`): <https://docs.cloud.google.com/vertex-ai/docs/authentication>. Accessed 2026-10-08.
- Google Cloud, "Imagen 4" (model card; discontinuation date is stated in §7): <https://docs.cloud.google.com/vertex-ai/generative-ai/docs/models/imagen/4-0-generate>. Accessed 2026-10-08.
- Google Cloud, "Agent Platform Pricing" (Imagen 4, Fast, Ultra per-image prices): <https://cloud.google.com/gemini-enterprise-agent-platform/generative-ai/pricing>. Accessed 2026-10-08.
- Google Cloud, "Responsible AI and usage guidelines for Imagen": <https://cloud.google.com/vertex-ai/generative-ai/docs/image/responsible-ai-imagen>. Accessed 2026-10-08.
- Stability AI, "Getting Started" (Authorization header): <https://platform.stability.ai/docs/getting-started>. Accessed 2026-10-08.
- Stability AI, "Pricing" (credit prices): <https://platform.stability.ai/pricing>. Accessed 2026-10-08.
- Stability AI, "Privacy Policy", effective 2026-09-30: <https://stability.ai/privacypolicy>. Accessed 2026-10-08.
- Stability AI, "Privacy Center" (training toggle): <https://stability.ai/privacy-center>. Accessed 2026-10-08.
- Stability AI, "Acceptable Use Policy": <https://stability.ai/use-policy>. Accessed 2026-10-08.

## Changelog

| Date | Change |
|------|--------|
| 2026-10-08 | Initial Proposed record — `INIT-003/SPEC-005`. Operator defaults aud-1 through aud-5 recorded as chosen. |
| 2026-10-09 | ADR-0009 (`INIT-006/SPEC-001`, Proposed) would replace the §1 snapshot with an aggregate learner profile (word stage, strength and weak flags, per-concept accuracy, journey label), with a new disclosure sentence, a shared rate limit, and new audit rows. The switch, the token, the default-off rule, §5 to §9 (uploads, media, image generation, alt text, retention) are unchanged. Metadata note and this row only; the sentences in this file are left as written. Takes effect only when ADR-0009 is Accepted. |
| 2026-10-09 | ADR-0009 Accepted by the operator. The replacement in the row above is in force. Metadata note and this row only. |
