# RSCH-007: What makes a language program feel easy, enjoyable, and intuitive for new users

| Field | Value |
|---|---|
| Research type | prior_art_review |
| Subject | New-user ease, enjoyment, and intuitiveness for this Hebrew learning app (internal) against published UX precedent (external) |
| Date | 2026-10-09 |
| Requestor | Brian Nelson (operator) |
| Access scope | repo_tree (working tree: code, ADRs, prior RSCH) + external (public web only). application_data (learner DB/answers) excluded — unbound, untouched. |
| Findings pattern | narrative_bluf |

## BLUF

Published precedent says new-user ease comes from **value before signup, one visible next action, and icon+visible-label navigation** — and our app currently conflicts with the first of those (everything is login-gated), conforms to the second (Today page), and conforms to the third only as of today's icon polish. Enjoyment without points/streaks (an operator constraint) has a real theoretical substitute — Self-Determination Theory's autonomy/competence/relatedness via informational feedback — but the commercial evidence for zero-gamification retention is thin, and that is the honest risk in this direction. [Moderate overall: strong precedent on mechanics, thin evidence on the no-gamification bet]

## Findings

### 1. First-run ease: skip the tutorial, give value before signup — we currently do the opposite (CONFLICTS)

NNg's onboarding guidance is explicit: drop users into the real UI, guide one essential action with short contextual help, reveal the rest on demand, and never front-load a tutorial — users who read mobile tutorials perceived tasks as *more* difficult with no success advantage [1][2][3]. The commercial archetype is Duolingo's inversion: pick a language, play a real first lesson with zero account, and only then "create a profile to save your progress" — signup framed as loss-aversion [4][5][6].

Our app conflicts with this precedent at the front door: `server/web.js` `handleStatic` redirects every signed-out HTML request to `/login/` — a new user can see nothing (no sample lesson, no value) before creating an account. RSCH-006's interface rules govern in-lesson behavior and never addressed signup gating, so this is a genuine gap, not a decided tradeoff. The single biggest ease win available is a **public sample lesson** (e.g. `my-morning` step 1, read-only, no account) with deferred signup at the "save your progress" moment. (Contrast note: one current-build teardown shows a paywall *preceding* the first lesson and its capture ends before any account moment — the inversion is the documented canonical flow [4][5], not a universal one [6].)

Confidence: **Moderate** — 2+ independent primary sources per claim (NNg articles; 3 teardowns on the inversion), but no controlled experiment proving deferred signup *causes* higher conversion (all sources descriptive) [open question 1].

### 2. One next action, recognition over recall (CONFORMS — keep it)

Two NNg heuristics do the most work for new users: *recognition rather than recall* (make actions visible, minimize memory load) and *aesthetic minimalism* (every extra unit of information competes) [1]. Our Today page already conforms: `shared/today-page.js` renders at most Continue / Next-new / Review-due cards with one primary button each, and RSCH-006 rule 16 / ADR-0006 already require "one next action" on the home screen. The polish pass (hero Continue card, level color-tops) extends this without changing the rule.

The related "error prevention" heuristic [1] points at our known rough edges: mixed-language labels (INIT-005 REQ-016 already flags: one language per label, English only as gloss) and dead-end empty states. Fix copy before adding features.

Confidence: **Moderate** — one high-quality external primary plus code-verified conformance (conformance is not an independent primary).

### 3. Enjoyment without points/streaks: SDT is the legitimate substitute, but the commercial evidence is thin (DIVERGES — eyes open)

Ryan & Deci (2000) is unambiguous on the mechanism: autonomy, competence, and relatedness drive intrinsic motivation; controlling extrinsic devices (imposed goals, surveillance, pressuring rewards) can *undermine* it [7]. Applied to m-learning, need-supportive apps raised perceived autonomy/competence and intrinsic motivation vs. textbook controls; the achievement effect was indirect (a trend only), positive affect was unchanged while negative affect decreased (Jeno, Adachi et al. 2018, N=58) [8]. The candidate non-points levers — self-chosen goals, optimal/adaptive difficulty, informative (not controlling) feedback, user history/self-monitoring, human connection — are author synthesis from that study's direction; the qualitative follow-up's feature decomposition was not independently verified [9]. [Low for the lever list as stated]

Against this, our app: CONFORMS on informational feedback (words-due counts, stage bars, correction-with-correct-form per RSCH-006 rule 12); DIVERGES on self-set goals and relatedness (no goal-setting, no human presence — everything is solitary drill). The operator's no-points constraint is therefore *defensible but incomplete*: SDT says cut the controlling rewards only if you feed autonomy/competence/relatedness through other channels. Cheapest SDT wins within our architecture: (a) let the learner set a tiny goal ("10 min" / "one story") on first run and reflect it back on Today; (b) "words you can now use" count (already allowed, INIT-005 A1); (c) real-life-task framing in blurbs ("order coffee") instead of drill names.

Confidence: **Moderate** — foundational theory High-consensus; the app-level levers rest on one qualitative study plus a decomposition I did not independently verify; zero-gamification *commercial* retention evidence not found [open question 2].

### 4. Findability: icon+label, 3–5 destinations, persistent position (CONFORMS as of today)

Precedent converges: Apple HIG and Material 3 both say 3–5 persistent destinations of equal importance [10][11]; NNg plus one lab study (Leung et al. 2011: labels greatly help young and old initially use icons) say icon+visible-label beats icon-only for novices [12]. Our shell has exactly 4 learner destinations (Today/Lessons/Review/Progress + admin-only) and now renders icon + Hebrew label + English gloss per link (`shared/app-shell.js` `renderHtml`), which conforms. Remaining gap vs. precedent: on phones the bar is top-positioned; Apple/M3 precedent puts primary navigation at the bottom (thumb zone) — ADR-0006 rule 17 already requires the *primary action* in the lower area but not the nav itself. A bottom tab bar is the logical next structural move, not more top links.

Confidence: **Moderate** — M3/NNg fetched primaries agree; Apple live fetch returned shell only (mirror + cached text used — re-verify before quoting exact HIG sentences).

### 5. Hebrew/RTL: mechanics are precedented, onboarding pedagogy is not (CONFORMS on mechanics; absence noted)

Script requirements (RTL direction, nikud as combining marks) are W3C hlreq ground [13]; the layout prescription — logical start/end properties, mirror directional icons, keep numbers/Latin LTR with bidi isolation — rests on community RTL practice plus our own ADR-0006 Decision 6, which mandates exactly it [14][17]. Nikud need a shipping font, larger size, and generous line-height or they clip [14]. Our stack conforms: ADR-0006 Decision 6 mandates logical properties and `dir="rtl"`/`lang="he"`; Rubik is self-hosted with Hebrew coverage; line-height 1.5–1.7. No published study was found on Hebrew-learner *onboarding* (pointed-first vs. unpointed-first, transliteration timing) — that absence is honest, not a finding.

Confidence: **Moderate** for the mechanics (one W3C primary + one community source + ADR — below any High threshold); **Very Low** for the pedagogy absence (negative claim, capped per rubric).

## Confidence (per-sub-question)

| Sub-question | Band | Basis |
|---|---|---|
| First-run ease / value-before-signup | Moderate | 2+ primaries per claim; conversion causality unproven |
| One-action / recognition | Moderate | 1 high-quality external primary (NNg) + code-verified conformance (not an independent primary) |
| SDT enjoyment without gamification | Moderate | Theory consensus High; app-level levers single-study; lever list Low (synthesis) |
| Navigation / icon+label | Moderate | M3+NNg primaries; Apple text needs re-verification |
| Hebrew/RTL mechanics | Moderate | W3C script facts + community/ADR prescription; single-community-source figures capped |
| Hebrew onboarding pedagogy | Very Low | Absence-of-evidence within budget |

## Verification

Independent verifier dispatch (fresh agent, no researcher context): PASS after 2 remediation loops. Loop 1 (full re-derivation): FAIL — [6] unsupported as inversion evidence (paywalled, ends pre-account) → demoted to contrast note, quantitative "~4 taps" removed; [8] misquoted year/results → corrected to Jeno et al. 2018 with softened effects; [9] unverified decomposition → marked author synthesis, Low; [12] two-studies claim → one study; [13] misattributed prescription → moved to [14]+ADR-0006; Finding 2 High → Moderate; Finding 5 non-standard band → Moderate. Loop 2 (fresh verifier): PASS on citation accuracy + scope; FAIL on one residual non-standard band string ("Moderate–High"). Fix applied verbatim. Loop 3 (fresh verifier, narrow): PASS — allowed bands only, table consistent, no hyphenated bands remain.

## Recommended Next Steps

1. **Public sample lesson before signup** (Finding 1) — biggest ease lever; frame signup as "save your progress." Needs ADR-0002 visibility work (a public-lesson mode) — propose as INIT-005 follow-on spec.
2. **First-run goal micro-flow** (Finding 3) — one question ("why Hebrew? / how much time?"), reflected on Today. Feeds autonomy + relatedness-adjacent relevance without gamification.
3. **Bottom tab bar on phones** (Finding 4) — move the 4 destinations to thumb zone; keep top shell for brand/account.
4. **Copy sweep per REQ-016** (Finding 2) — one language per label before any new surface.
5. **Do not build**: upfront tutorial, coach-mark tour, points/streaks — precedent says the first two hurt and the third is excluded by operator decision.

## References

[1] "10 Usability Heuristics for User Interface Design" — Nielsen Norman Group — https://www.nngroup.com/articles/ten-usability-Heuristics/ (retrieved 2026-10-09)
[2] "Mobile-App Onboarding: An Analysis of Components and Techniques" — Nielsen Norman Group — https://www.nngroup.com/articles/mobile-app-onboarding/ (retrieved 2026-10-09)
[3] "Mobile Tutorials: Wasted Effort or Efficiency Boost?" — Nielsen Norman Group — https://www.nngroup.com/articles/mobile-tutorials/ (retrieved 2026-10-09)
[4] "Duolingo onboarding teardown: value before signup" — Wakeline — https://wakeline.io/teardowns/duolingo (retrieved 2026-10-09)
[5] "Duolingo Onboarding journey: 10 Screens" — Screenrove — https://screenrove.com/duolingo/flows (retrieved 2026-10-09)
[6] "Duolingo onboarding flow: full teardown" — Botsi — https://www.botsi.com/resources/flows/apps/duolingo (retrieved 2026-10-09)
[7] Ryan & Deci (2000), "Self-Determination Theory and the Facilitation of Intrinsic Motivation…" — American Psychologist via selfdeterminationtheory.org — https://selfdeterminationtheory.org/SDT/documents/2000_RyanDeci_SDT.pdf (retrieved 2026-10-09)
[8] Jeno, Adachi et al. (2018), "The effects of m-learning on motivation, achievement and well-being" — British Journal of Educational Technology 50(2):669–683 (N=58) via selfdeterminationtheory.org — https://selfdeterminationtheory.org/wp-content/uploads/2020/09/2019_JenoAdachiEtAl_TheEffectsMLearning.pdf (retrieved 2026-10-09; year corrected per verification — file path retains 2019 upload stamp)
[9] Jeno et al. (2022), "A qualitative investigation of psychological need-satisfying experiences…" — Computers in Human Behavior Reports via selfdeterminationtheory.org — https://selfdeterminationtheory.org/wp-content/uploads/2022/10/2022_JenoEdgelandsalGrytnes_AQualitative.pdf (retrieved 2026-10-09)
[10] "Tab bars" — Apple Developer Documentation — https://developer.apple.com/design/human-interface-guidelines/tab-bars (search-cached/mirror text 2026-10-09 — re-verify before exact quotation)
[11] "Navigation bar" — Material Design 3 — https://m3.material.io/components/navigation-bar/overview (retrieved 2026-10-09)
[12] "Icon Usability" — Nielsen Norman Group — https://www.nngroup.com/articles/icon-usability/ (retrieved 2026-10-09); corroborated by Leung et al. (2011) — https://www.cs.ubc.ca/labs/edapt/papers/leung2011.pdf
[13] "Hebrew Layout Requirements (hlreq)" — W3C — https://www.w3.org/International/hlreq/ (retrieved 2026-10-09)
[14] "hebrew-rtl-best-practices" — skills-il/localization (community) — https://raw.githubusercontent.com/skills-il/localization/HEAD/hebrew-rtl-best-practices/SKILL.md (retrieved 2026-10-09)
[15] RSCH-003 — sdd/research/README.md row (language-learning science; spacing High, SDT-adjacent)
[16] RSCH-006 — sdd/research/README.md row (learner-interface rules; word+meaning+picture Moderate, WCAG High)
[17] ADR-0006 — docs/decisions/ADR-0006-visual-language-and-interface-rules.md (rules 10, 11, 14, 16, 17; hook contract)
[18] ADR-0007 context — shared/app-shell.js NAV/renderHtml; shared/layout.js page(); shared/today-page.js cards; server/web.js handleStatic (login gating)
[19] INIT-005 — sdd/initiatives/INIT-005-learner-experience-and-identity/INIT-005-learner-experience-and-identity-initiative.md (REQ-010–017; SPEC-005 RSCH-007 plan — note: this standalone RSCH-007 now occupies that number; SPEC-005's planned session should take RSCH-008)

Provenance: INIT-005-adjacent standalone research; prior_art_review; narrative_bluf.

## Appendix A — Access-Gate Result

Gate Briefing: Domains repo_tree + external. Bindings: recorded single_maintainer (repo_tree, rung 1) re-earned by operator confirmation 2026-10-09 (this session; principal brian.nelson@ahead.com); recorded external declaration (entitlement/scope inapplicable + per-source test). Verify: config+gate doc unique; bindings block present (2 domains); class slots complete; no placeholders. CHECK/CONFIRM items: harness command primitives confirmed by execution; repo_tree zero-anchor CONFIRM resolved by operator confirmation above. application_data unlisted → excluded (not requested, untouched). Result: PROCEED at repo_tree + external.

## Appendix B — Scope & Research Type

Type: prior_art_review (registry §23; corpus prior-art-standards-review.md; pattern narrative_bluf). Index-first check: RSCH-003/006 partial overlap → operator chose fresh session. Subject identity: internal = c:\Users\BrianNelson\Projects\Hebrew (this repo); external = public UX precedent (NNg, Apple, M3, SDT literature, Duolingo teardowns). Sub-questions: §1 first-run ease; §2 enjoyment w/o gamification; §3 navigation; §4 Hebrew/RTL; §5 internal gap analysis. Disconfirmation attempted: deferred-signup causality unproven (descriptive only); SDT commercial-zero-gamification evidence thin; Duolingo's success partly IS gamification — recorded as the honest risk, capping the BLUF at Moderate.

## Appendix C — Methodology

Corpus doc: C:\Users\BrianNelson\.claude\skills\sdd-research\assets\research-corpus\prior-art-standards-review.md. External investigation ran in an isolated sub-agent (12 calls); internal grounding by researcher via Read/Grep over repo + ADRs. Saturation per sub-question (≥2 independent sources or honest single-source cap). Budget respected; open questions listed (§OPEN below). Internal anchors verified by read: server/web.js, shared/app-shell.js, shared/layout.js, shared/today-page.js, ADR-0006, INIT-005.

## Open questions

1. Drops/Memrise/Babbel teardowns to triangulate Duolingo (not found in budget).
2. Causal evidence for deferred-signup conversion lift (descriptive only).
3. SDT evidence for Hebrew/abjad or zero-gamification commercial apps.
4. Apple HIG tab-bar exact sentences (live fetch failed; re-verify).
5. Any empirical Hebrew-learner onboarding study.
