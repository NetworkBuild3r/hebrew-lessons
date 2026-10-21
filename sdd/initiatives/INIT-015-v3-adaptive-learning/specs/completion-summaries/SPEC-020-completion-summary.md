# SPEC-020 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-020 (service) · **Status:** complete · **Date:** 2026-10-19

## Change

Commit `72f2493` on local branch `v3-spec-020` (31 files), merged into local `v3` as `355be19` with no conflicts. Not pushed. No migration (the worker was told to stop if one looked necessary).

- **`internal/units`:** `ValidateWithBrief(ctx, doc, lookup, *BriefFacts)` and a plain `BriefFacts` struct owned by `units`. This avoids the `brief` to `db` to `units` import cycle SPEC-019 found. `brief.Brief` is converted to `BriefFacts` in `internal/mcpstore/learner.go`. Rule ids are documented in `internal/units/doc.go` and catalogued in `Rules`.
- **Rules (all block):** `brief.coverage`, `brief.text_too_long`, `brief.new_intro_before_test`, `brief.new_min_uses`, `brief.struggle_reuse`, `brief.gloss_out_of_brief`, `brief.audio_all`, `unit.no_markup` (also folds full-width forms, strips spaces and control characters inside `javascript:`), and `brief.used`, `brief.not_found`, `brief.expired`.
- **`internal/db`:** `AutoPublishLearnerUnit` (publish, claim the brief and write the audit row in one transaction; the brief is claimed with a single conditional UPDATE), `UnpublishLearnerUnit`, `ReportUnit` (the `report_hide` path for SPEC-017), `HiddenUnitIDs`.
- **`internal/mcp`:** `put_unit` gains input `briefId` and output `publishReason`. A learner-scoped unit with no `briefId` stays a draft with the reason in the result.

## Acceptance criteria

| AC | Result | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `TestBriefGoldens` (11 fixtures, exact rule id and pointer, no other rule fires); `TestBriefGoldenPerRule` fails if a rule has no fixture |
| ac-2 | PASS | `TestAutoPublishPassingUnit` (one audit row, warnings recorded in `issues_json`) |
| ac-3 | PASS | `TestAutoPublishNoBriefIDStaysDraft` |
| ac-4 | PASS | `TestUnpublishRefusedOutsideScope`, `TestUnpublishLearnerUnit`, `TestUnpublishLearnerOwnUnit` |
| ac-5 | PASS | `TestAutoPublishBriefReuse`; missing, expired and foreign briefs are block issues (`TestAutoPublishBriefMissingExpiredOrForeign`) |
| Security | PASS | Concurrency with real goroutines: 24 at db level and 12 through `put_unit`, each with exactly one winner and one audit row. `TestAutoPublishMarkupInEveryField` (8 fields), `TestNoMarkupCoversEveryStringField` (reflection: fails when a new string field has no check). |

Orchestrator checks on merged `v3` at `355be19`: `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok, `go test -race` on `internal/units`, `internal/mcp` and `internal/mcpstore` ok. Code read by the orchestrator: scope is re-read and compared inside the transaction, the brief claim is keyed on `user_id`, and the revision published is the revision validated.

## Deviations and decisions

- **Coverage uses the staged figure** `tier.min_known_share` (0.60, 0.90, 0.95), not the spec's flat 0.95 (operator-delegated decision, recorded in the brief to the worker). A missing or non-positive figure falls back to 0.95. At a flat 0.95 a beginner brief could never pass.
- **v2 `coverage.story` and `coverage.listening` are dropped** in `ValidateWithBrief`: they judge against earlier shared units and would block a unit built from the learner's own comfortable words. `Validate` itself is unchanged.
- **`publish_unit` now refuses learner-scoped units.** It ran only the v2 rules, so an admin could have put a learner unit on Today with no brief checks, which breaks und-2. Behaviour change; tested.
- **Existing test changed:** `TestPutUnitPublishFlag` expected a learner unit with no `briefId` to publish, which ac-3 forbids. Added an omitempty `Gloss` field to `glex` in `validate_golden_test.go`; the existing goldens are byte-identical.
- **Audit covers successes only** (publish, unpublish, report-hide). `publish_audit.action` has no value for blocked attempts and adding one needs a migration. Blocked attempts return their issues to the caller.
- **An exact repeat** of a successful publish is idempotent and writes no second audit row.
- **Prefix matching** is written inside `internal/units` (exact, then one prefix letter, a second only after ו or ש, at least two letters left). `internal/lexicon/prepare.go` does not strip prefixes, contrary to the spec's aud-1.

## Open risks

- **Coverage can over-count.** A token that is another word with a prefix letter removed can match a known word wrongly and count as known. This makes the 0.6 to 0.95 gate somewhat lenient. A real tokenizer would fix it.
- **Report hide is incomplete until SPEC-017.** `unit_reports` is the hide, and `HiddenUnitIDs` lists it, but no list reads it yet. A reported shared unit still shows for the reporter.
- **Brief reuse after unit deletion** (from SPEC-019): the foreign key sets `used_by_unit_id` to NULL, which makes the brief usable again.
- **Function words** come from a closed list of free-text POS values, because the lexicon has no tag column.
- **Not verified:** `brief.Build` driven end to end through `put_unit` (tests insert hand-built briefs), concurrency on anything but the single-connection SQLite store, and any web code.
- No personas, timer or independent verifier. No security-review agent has run on this spec (its spec asks for a deep review).

## Gates

None. Do not push `v3`: a push publishes an image that may auto-deploy.
