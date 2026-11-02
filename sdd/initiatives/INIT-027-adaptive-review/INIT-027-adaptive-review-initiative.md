# INIT-027: Review that moves on when you know it

**Initiative ID:** INIT-027 · **Status:** on_track · **Created:** 2026-11-01 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `adaptive-review` (already created)
**Source:** Brian, 2026-11-01: "review is the same shit over and over. It's not dynamic in learning. It keeps asking me if I know what letters mean. I clearly do but it keeps asking them over and over. It's not analyzing my response times or my response." He chose all four parts (drills that end, speed, move up, new words with accuracy) and the 5 PM fix in the same push. Evidence: [RSCH-003](../../research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md) — spacing and retrieval (High), longer gaps help delayed retention (Moderate), interleaving (Moderate), and "count a same-session repeat as practice, not as the review" (checklist line 173). RSCH-003 has no evidence on answer speed. The speed rules here are product rules, as in ADR-0020.

Cited lines are `origin/main` at `d2a979867234072b38e09402f0774ae68b74bf4d`, which is live. INIT-025 is on branch `one-recall-card`, not on `main`, and touches the lesson player and shell, not review. `.claude/agents/` is absent, so specs execute inline with no persona. ADR-0023 and migration 021 are free on every local branch, including `one-recall-card`.

## What we found

Live data for learner 1 on 2026-11-01: 1,060 lexemes, 65 words touched, 133 cards, 55 due now, 22 `new` cards waiting. Last 50 reviews 78% Good or Easy. Reading cards median stability 3.2 days, recall 0.2 days, spell never asked.

| Problem | Why | Line |
| --- | --- | --- |
| Letter drills never stop | Two letter-pair misses in 7 days add a 4-step drill to **every** session for 7 days, up to 20 steps. The drill is a self-graded flashcard, and its grade is thrown away. Active now: א/ע, ו/י, ו/מ (12 steps a session). | `internal/session/session.go:275`, `internal/session/mix.go:103`, `:109`, `:113` |
| Typos make letter drills | A typed answer within two letters logs each wrong letter as a pair. ו/מ is a slip, not a confusion. | `internal/activity/check.go:196` |
| Known words you were not due for | The known quota (about 16 of 20, about 19 of 20 below 80% accuracy) is filled with known cards that are **not due yet** when due ones run short. | `internal/session/mix.go:546`, `:126` |
| The same due word comes back every session | An answer grades the card of the skill asked. A due reading card asked through listening stays due. | `internal/session/session.go:101`, `internal/db/evidence.go:108` |
| Strong words never move up | The planner picks the weakest skill. A reading card you know is still asked as reading. Nothing counts typing it from English as reading it. | `internal/activity/plan.go:100`, `:180` |
| New words do not follow how you do | New cards are min(5, max(2, floor(0.25 × size))) whatever your accuracy. Review cannot introduce a word no lesson taught. `FreqRank` is never loaded. | `internal/session/mix.go:587`, `:180` |
| Review breaks at 5 PM Denver | The queue is rebuilt, and answers are refused with 400, when the **UTC** date changes. | `internal/web/learn.go:434`, `web.go:499`, `activity.go:115`, `:191` |

Speed already counts on checked activities: faster than your median is Easy (held at Good until the card is in review), slower than twice it is Hard (`internal/evidence/grade.go:33`). Drills and flashcards are not timed.

## What you get

- A letter drill asks you to pick the right letter in a real word you missed, with two buttons, timed. Three right in a row, none slow, ends that drill until you miss the pair again. At most 6 drill steps a session.
- A typing slip makes a letter drill only for a pair that looks or sounds alike, such as ב/כ, ד/ר, or א/ע.
- A session holds only cards that are due, plus new words. A known word you are not due for waits.
- A due card leaves the queue once answered. Once you know a word's reading, it is asked the harder way, by typing it from English. A right answer there also counts for the reading card.
- New words per session follow your last 50 reviews: 2 below 80%, 5 from 80% to 90%, 8 above 90%. When you are at 85% or better and fewer new cards are waiting than allowed, review adds the next words from the frequency-ordered placement list, with an intro screen.
- The done screen says what changed: words moved up, letter pairs cleared, slow words coming back sooner, new words met.
- Review keeps working past 5 PM. Your sitting lasts until your 4:00 study-day rollover, in your zone.

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-docs-adaptive-review.md) | docs | ADR-0023: review that moves on when you know it | — | 1h | pending |
| [SPEC-002](specs/SPEC-002-data-drill-answers-and-implied-reviews.md) | data | Migration 021: drill answers, cleared pairs, implied reviews | — | 2h | pending |
| [SPEC-003](specs/SPEC-003-service-drills-that-end.md) | service | Drills that are graded, timed, capped, and end | SPEC-002 | 4h | pending |
| [SPEC-004](specs/SPEC-004-service-due-cards-and-move-up.md) | service | Only due cards; a due card leaves; strong words move up | SPEC-002, SPEC-003 | 4h | pending |
| [SPEC-005](specs/SPEC-005-service-new-words-follow-accuracy.md) | service | New words follow accuracy and come from the frequency list | SPEC-004 | 4h | pending |
| [SPEC-006](specs/SPEC-006-frontend-review-screens.md) | frontend | Letter pick drill, done-screen changes, and the study-day queue | SPEC-003, SPEC-004, SPEC-005 | 4h | pending |

**Total:** 19h. **Critical path:** SPEC-002 → SPEC-003 → SPEC-004 → SPEC-005 → SPEC-006 = 18h. Realistic ×1.3 ≈ 23h.

**Order.**
1. SPEC-001 and SPEC-002 run together.
2. SPEC-003, SPEC-004, and SPEC-005 all edit `internal/session`, so they run in order.
3. SPEC-006 is the only spec that edits `internal/web`.

## Decisions to confirm

- **A cleared pair is three right in a row.** Counted across sessions since the pair's last miss. A right answer slower than twice your drill median (once 10 timed drill answers exist) does not count toward the three. A new miss after clearing starts over, and a drill needs two misses after the clear.
- **Look-alike and sound-alike pairs only, for typing slips.** The look-alike set is the drill cue table in `internal/web/views/alefbet.go:107` with points removed, plus ו/י, kept in a new `internal/letters` package with a test that keeps the two in step. The sound-alike set is א/ע, ט/ת, כּ/ק, ח/כ, ב/ו, שׂ/ס. A wrong **pick** of another word one letter off still logs the pair, as today. Existing `confusion_misses` rows stay.
- **Implied credit goes one way.** Recall (typing from English) right also schedules the word's due reading card as Good. Spell (writing from audio) right also schedules its due listening card as Good. Never Easy, never the other way, and a wrong answer never touches the other card. The review_log row is marked `implied`, and accuracy counts skip implied rows.
- **Move up only from strength.** The harder skill is used for a due card only when the card's own skill is known or solid, and the harder skill's card has had no Again or Hard in the last 7 days. Otherwise the card is asked through its own skill.
- **Shorter sessions are fine.** Removing the not-due known padding can make a session shorter than 20. New words fill the room only as far as the accuracy tier allows.
- **New words from review get an intro.** Seeded words get the same cards a finished lesson gives (read and recall, plus listen when a clip exists). They are verified placement words with a gloss, lowest band first, in list order. The placement list is 200 words. A larger word source is later work.
- **Pattern drills stay self-graded,** but the grade now counts: Good or Easy is right. Only letter drills become a pick.

## Threat model

- **No new trust boundary.** Every write comes from the signed-in learner's own review session, through the existing CSRF- and Origin-checked `/_hx/review/*` handlers. The learner id comes from the session, never the form.
- **Input.** A drill pick posts one letter or "don't know". The letter must be one side of the served pair, as `askedInPair` checks today (`internal/web/web.go:563`). Anything else is 400 and writes nothing. Right or wrong is decided against the letter the server chose and stored at serve time, never a posted value, and latency is measured on the server.
- **Integrity.** Implied credit only follows a server-graded right answer in the same visit, for the card the session served. Seeding creates only `new` cards for the learner's own id, with `INSERT OR IGNORE`, so an existing card is never changed.
- **Output.** Every screen stays templ-escaped. No new MCP tool or role change.

## Depends on / unblocks

- Builds on the session mix (INIT-015/SPEC-016), the checked activities (INIT-015/SPEC-015), the speed rule (INIT-022, ADR-0020), and the learner day (INIT-026, ADR-0022).
- Out of scope: new MCP fields for drills (the coach still reads confusions through `get_context`), a word source beyond the placement list, changing FSRS weights, timing flashcards, retry questions for tutor corrections.

## Done when

- A letter pair answered right three times in a row stops appearing, and a typing slip between two unlike letters, such as ו/מ, makes no drill.
- A session built with 5 due known cards and 40 known cards not yet due holds those 5 and none of the 40.
- A due reading card of a word whose reading is known is asked as typing from English, and a right answer leaves both cards scheduled into the future.
- At 92% accuracy, a session with no `new` cards waiting introduces 8 placement words. At 78%, it introduces at most 2.
- At 17:30 MST (00:30 UTC), an answer to a card served at 16:50 MST is graded, not refused.

## Plan quality

Completeness 8, accuracy 8, clarity 8, feasibility 7, guardrails 8. Confidence 0.78. Single-domain specs, acyclic graph, each with a Claim, a Check with its executor, and a Pass condition. Lowest score: feasibility. SPEC-004 changes the mix that carries the most existing tests (`internal/session/mix_test.go`, `session_test.go`), and several of those tests pin the not-due padding on purpose.
