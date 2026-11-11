# Options Matrix: Lesson Types, Done Right (BRAIN-002)

**Date:** 2026-11-10
**Prepared By:** Brainstorm Session Panel
**For:** Brian's personal Hebrew study web app

---

## Quick Summary

Six specialists scored four options on six criteria. Each score runs from 1 (poor) to 5 (excellent). The top score for one specialist is 30. The top score for the panel is 180.

| Option | Panel total (of 180) | Mean per specialist (of 30) | Preferred by | Recommendation |
| --- | ---: | ---: | --- | --- |
| A. Full five types, built in sequence | 74 | 12.3 | none | Not recommended |
| B. Word lesson plus easy lesson now | 102 | 17.0 | none | Not recommended |
| C. Study first, build one thing | 114 | 19.0 | Devil's Advocate | Acceptable (runner-up) |
| **D. Hybrid: backfill and study now, word lesson first** | **121** | **20.2** | Product Strategist, UX Designer, Technical Architect, Data Analyst, QA Advocate | **Recommended** |

**Recommended option:** D, amended by information injection R1 (see the note below).

**Agreement:** 5 of 6 prefer D. The Devil's Advocate prefers C, by one point (21 to 20). His dissent is about scope inside slice 1, not about direction.

---

## Option Definitions

These come from the end of `dialogue.md`.

- **A. Full five types, built in sequence.** All five lesson types, one after another, as in `docs/design/lesson-architecture.md`.
- **B. Word lesson plus easy lesson now.** Code-built word lesson (grid, meaning check, tutor-line passage, typed write) and the easy lesson. Review first on Today. Conjugation next. Listening waits for a voice. Spelling is cut.
- **C. Study first, build one thing.** Backfill and record the newest classes' words. Study them for one week in today's review. Then build the word lesson only. Every other type needs a trigger agreed in advance.
- **D. Hybrid.** Backfill and study now, as data entry only. After the gate, the word lesson is the only first slice. It has typed write with accept-sets and a flag control that voids, flags, and suspends one item. The retire rule and the "same again" tripwire are agreed first. The easy lesson comes only if heavy-review days appear. Conjugation is third. Listening waits for a voice. Spelling is cut.

---

## Evaluation Criteria

All six criteria carry equal weight (one sixth each). Scores are simple sums.

| Criterion | Description | Weight |
| --- | --- | --- |
| Learning effect | Is the lesson backed by research? Will Brian learn from it? | Equal |
| Trust | No right answer marked wrong. No untaught words on screen. | Equal |
| Time to new study | Does Brian get something new to study in about 2 weeks? | Equal |
| Build cost and treadmill risk | How few specs and rules does it need? | Equal |
| Fixes "feels the same" | Does it fix Brian's complaint that lessons feel alike? | Equal |
| Measurable | Can we tell whether it works? | Equal |

---

## Detailed Scoring

Columns: **Prod** = Product Strategist. **UX** = UX Designer. **Arch** = Technical Architect. **Data** = Data Analyst. **QA** = QA Advocate. **Devil** = Devil's Advocate.

Each specialist's cell scores come from their own file. Where a specialist wrote a reason, the main ones are in the notes under each table.

### Option A. Full five types

| Criterion | Prod | UX | Arch | Data | QA | Devil | Sum | Mean |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Learning effect | 3 | 3 | 4 | 3 | 3 | 3 | 19 | 3.2 |
| Trust | 2 | 2 | 2 | 2 | 2 | 2 | 12 | 2.0 |
| Time to new study | 1 | 1 | 1 | 1 | 1 | 1 | 6 | 1.0 |
| Build cost and treadmill | 1 | 1 | 1 | 1 | 1 | 1 | 6 | 1.0 |
| Fixes "feels the same" | 3 | 2 | 4 | 4 | 4 | 2 | 19 | 3.2 |
| Measurable | 2 | 2 | 2 | 2 | 2 | 2 | 12 | 2.0 |
| **Total (of 30)** | **12** | **11** | **14** | **13** | **13** | **11** | **74** | |

Notes:
- All six agree on three scores: Trust 2, Time 1, Build 1. Five types means five sets of graded keys, and months of work.
- Typed grading marks the tutor's spelling wrong today. Five types would ship more typed keys on top of that.
- A has the most variety on paper. But it arrives last, and listening cannot run without a voice.

### Option B. Word lesson plus easy lesson now

| Criterion | Prod | UX | Arch | Data | QA | Devil | Sum | Mean |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Learning effect | 4 | 4 | 4 | 4 | 3 | 3 | 22 | 3.7 |
| Trust | 3 | 3 | 3 | 3 | 3 | 2 | 17 | 2.8 |
| Time to new study | 3 | 2 | 2 | 2 | 2 | 2 | 13 | 2.2 |
| Build cost and treadmill | 3 | 2 | 2 | 2 | 2 | 2 | 13 | 2.2 |
| Fixes "feels the same" | 3 | 4 | 3 | 3 | 3 | 3 | 19 | 3.2 |
| Measurable | 3 | 3 | 3 | 3 | 3 | 3 | 18 | 3.0 |
| **Total (of 30)** | **19** | **18** | **17** | **17** | **16** | **15** | **102** | |

Notes:
- B has no backfill-first step and no T0 gate. It grades from a lexicon nobody has checked.
- B tries to build a lesson before the 2026-11-30 gate, which the Stage 2 rule forbids (QA Advocate).
- B is two builds before any evidence (Devil's Advocate).

### Option C. Study first, build one thing

| Criterion | Prod | UX | Arch | Data | QA | Devil | Sum | Mean |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Learning effect | 3 | 3 | 3 | 4 | 4 | 4 | 21 | 3.5 |
| Trust | 3 | 3 | 4 | 3 | 3 | 3 | 19 | 3.2 |
| Time to new study | 2 | 3 | 2 | 2 | 3 | 4 | 16 | 2.7 |
| Build cost and treadmill | 4 | 4 | 4 | 4 | 4 | 4 | 24 | 4.0 |
| Fixes "feels the same" | 2 | 3 | 2 | 2 | 2 | 2 | 13 | 2.2 |
| Measurable | 4 | 3 | 3 | 4 | 3 | 4 | 21 | 3.5 |
| **Total (of 30)** | **18** | **19** | **18** | **19** | **19** | **21** | **114** | |

Notes:
- All six give C a 4 on build cost. One build and one trigger list is the cheapest plan.
- The panel splits on time. The Devil's Advocate gives C a 4: new words reach review at once, and there is one build. Product Strategist, Technical Architect, and Data Analyst give it a 2: a study week comes first, so the first new lesson lands about 3 to 4 weeks out.
- C names no accept-set fix and no flag control. The study week runs on the unfixed typed grader (UX Designer).

### Option D. Hybrid

| Criterion | Prod | UX | Arch | Data | QA | Devil | Sum | Mean |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Learning effect | 4 | 4 | 3 | 4 | 4 | 4 | 23 | 3.8 |
| Trust | 4 | 3 | 4 | 4 | 4 | 4 | 23 | 3.8 |
| Time to new study | 4 | 3 | 3 | 3 | 3 | 3 | 19 | 3.2 |
| Build cost and treadmill | 3 | 3 | 4 | 3 | 3 | 3 | 19 | 3.2 |
| Fixes "feels the same" | 2 | 3 | 2 | 2 | 2 | 3 | 14 | 2.3 |
| Measurable | 4 | 4 | 4 | 4 | 4 | 3 | 23 | 3.8 |
| **Total (of 30)** | **21** | **20** | **20** | **20** | **20** | **20** | **121** | |

Notes:
- D is the only option with tiers (T0 keys), typed accept-sets, and a one-item void, flag, and suspend control in front of the first graded step.
- D gets new words into Brian's review within days. The backfill is data entry only.
- The Devil's Advocate gives D a 3 on measurable. D changes several things at once (passage source, flag control, new screens), so attribution is weak.
- The UX Designer gives D a 3 on trust. The study week runs on the typed grader that marks tutor spelling wrong, until the fix lands.

---

## Mean Score per Dimension

Mean of the six specialist scores. These match `consensus.md`.

| Criterion | A | B | C | D | Leader |
| --- | ---: | ---: | ---: | ---: | --- |
| Learning effect | 3.2 | 3.7 | 3.5 | 3.8 | D |
| Trust | 2.0 | 2.8 | 3.2 | 3.8 | D |
| Time to new study | 1.0 | 2.2 | 2.7 | 3.2 | D |
| Build cost and treadmill | 1.0 | 2.2 | 4.0 | 3.2 | C |
| Fixes "feels the same" | 3.2 | 3.2 | 2.2 | 2.3 | A and B (tie) |
| Measurable | 2.0 | 3.0 | 3.5 | 3.8 | D |
| **Sum of means (of 30)** | **12.3** | **17.0** | **19.0** | **20.2** | |

Reading the table:
- D leads on learning effect, trust, time, and measurement.
- C leads on build cost.
- A and B lead on "feels the same", but only at 3.2. They also score worst on time and build cost.

---

## Trade-off Analysis

### D vs C: build cost and time to study

D and C are 7 points apart on 180. They share the same core:
- the backfill;
- studying in review now;
- one lesson type first;
- tutor lines as the passage;
- a cut rule set in advance.

**Build cost: C wins, 4.0 to 3.2.**
- Every specialist scored C a 4. Only the Technical Architect scored D a 4.
- D adds work to slice 1: the flag control (with one new void reason in a small migration), typed accept-sets, a release check for code-built lessons, and the retire rule agreed first.
- This is the Devil's Advocate's worry. He sees the treadmill coming back through trust work.
- The panel's answer:
  - The old validator is not deleted. Code-built lessons get their own three-test release check.
  - The flag control stays small. It extends `internal/web/also_right.go`. There is no flag table until there is real flag data.
  - The accept-set fix is treated as a trust bug inside Stage 1 (INIT-038), not as a new initiative. Brian decided this on 2026-11-10.
  - The Product Strategist asked that these rules live inside the word-lesson spec, not as separate specs.

**Time to study: D wins, 3.2 to 2.7.**
- With D, the backfilled words start spaced review within days of the data entry.
- With C, the backfill is the same. But C holds the lesson build until after a study week.
- Both options wait for the 2026-11-30 gate before any new screen. The first new lesson screen lands about 3 weeks from now, not 2 (UX Designer, Data Analyst).
- A lesson type must start by 2026-12-07 to be judged on proof before S2 (2027-01-04). That leaves about a week of slack.
- The Devil's Advocate is the one specialist who scored C ahead of D on time (4 to 3). He counts the one-build plan as faster than D's extra slice 1 work.

**Trust, learning effect, and measurement: D wins.**
- D adds T0-only keys, accept-sets, the flag control, and a retire rule agreed before the build.
- C names none of these. A wrong gloss or a wrong "wrong" mark would surface only by chance.

### Why no option fixes "feels the same" well

- **No option scored above 3.2.** The best scores are A and B at 3.2. D is 2.3 and C is 2.2.
- **A and B change the screen shape the most.** But A needs five builds, and two of its five types cannot run without a voice. B's second shape is only a fallback.
- **C and D change the content, not the shape.** Each class has its own tutor passage, so the topic and sentences change. But the lesson stages stay fixed. This is a bet. The Devil's Advocate argued it directly in the Round 4 challenge, and the panel agreed.
- **The bet stays a bet.** All six scored D or C at 3 or below here. The score does not go up by adding more lesson types. A and B score higher only on paper.

**The tripwire.** It was agreed before slice 1.
- If Brian says lessons feel the same in week 2, change the content mix. That means the passage source, the word count, and the form of the "use it" step.
- Do **not** add a lesson type.

### Option A: Full five types
- **Excels at:** Most variety of screen shape. Covers every research strand.
- **Struggles with:** Everything else. Five builds, five sets of graded keys, no voice for listening, and the BRAIN-001 treadmill again.
- **Best when:** There is a large team, a working voice, and a checked lexicon. None of these is true now.

### Option B: Word lesson plus easy lesson now
- **Excels at:** Two lesson shapes early. Good learning-effect score.
- **Struggles with:** No backfill step, no T0 gate, and two builds before any evidence. It collides with the Stage 2 gate.
- **Best when:** The lexicon is already checked and the gate is open.

### Option C: Study first, build one thing
- **Excels at:** Lowest build cost. A clean baseline: words studied in review only.
- **Struggles with:** The first new screen comes late. No accept-set fix or flag control is named. One lesson shape for weeks.
- **Best when:** Build time is the main limit, and the typed grader is already fixed.

### Option D: Hybrid
- **Excels at:** Trust, time, and measurement. New words in review within days. One build first.
- **Struggles with:** Build cost is higher than C. It does not fix "feels the same". Its first screen waits for the gate.
- **Best when:** Trust in the answer keys matters more than the fewest rules, and a gate sits between now and the build. That is Brian's case.

---

## Recommendation

**Recommended option:** D, amended by R1.

**Main differentiators:**
1. **Highest total and widest support.** D scores 121 of 180. Five of six specialists prefer it.
2. **It leads on four of six criteria.** Those are learning effect, trust, time, and measurement. Trust is where a wrong answer costs Brian the most.
3. **It is the only option that puts the key-proof gates in front of the first graded step.** Those are T0 keys, typed accept-sets, and the three-test release bar.
4. **It starts now without breaking the gate.** The backfill is data entry through the existing MCP tools. It needs no new spec.

**Modifications suggested** (these are in the amended recommendation in `consensus.md`):
- Fix typed grading inside Stage 1 (INIT-038) as a trust bug. Brian decided this on 2026-11-10.
- Agree the retire rule and the "same again" tripwire before slice 1.
- Keep the flag control small: void, flag, and suspend one item. No flag table yet.
- Keep the old validator. Give code-built lessons their own three-test release check.
- Keep typed write in slice 1 (the Devil's Advocate dissents). Keep the Devil's Advocate's "one build, study first" principle.

### Note on information injection R1

R1 was Brian's input: "I won't be doing this just after a tutor lesson. So we need lessons that don't always align to my tutor's lessons but can help me keep learning."

- R1 came **after scoring**. It **amended D's content**. Examples: the easy lesson moved into slice 1 as the heavy-day lesson. The word pool now covers all classes, ranked by need. Today always offers one lesson. T0 is decided per sheet. Conjugation starts after the word lesson's day-14 use check.
- **All six specialists changed details. None changed their preferred option.** Five still prefer D. The Devil's Advocate still prefers C, and D now absorbs his "one build, study first" principle.
- **The scores were not re-run.** The tables above score D as written in `dialogue.md` (before R1). Scoring ran on Haiku 5.5.
- All six lowered their confidence in R1. The reasons are the same in most files. The word pool is larger and mostly unchecked: 75 of about 85 sheets have not been audited. The pool depth is unknown.
- The amended D adds some build (the easy lesson on the thin player, a minimum-lesson floor). It also adds some variety (topic rotation across classes). The size of these effects on the scores is **not** measured here.

---

## Score Checks

I recomputed every total and every mean from the six specialist files.

- **Per-specialist totals:** all 24 totals (6 specialists x 4 options) match `consensus.md`.
- **Panel sums:** A 74, B 102, C 114, D 121. All match.
- **Per-dimension means:** all 24 means match `consensus.md` after rounding to one decimal.
- **Numeric discrepancies found: none.**

Two small points that are not number differences:
1. **File layout.** `technical-architect.md` has no heading named "Phase 5: Consensus Position". Its scores sit under "### Option Scores" inside its Phase 4 section, just before its Final Position. The numbers match.
2. **Wording in `consensus.md`.** It says "A scores best on 'feels the same'". A and B tie at 3.2. This matrix lists both as the leaders.

**Score Legend:** 5 = Excellent, 4 = Good, 3 = Acceptable, 2 = Concerning, 1 = Problematic.
