# SPEC-007 eval report

**Spec:** `INIT-013/SPEC-007` (`SPEC-007-data-multi-tense-verb-workshop-and-interleaved-drill.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `cursor-grok-4.5-high-fast`, read-only constraint, did not author the files).

## Per-AC verdicts (Loop 1 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | Test `ac-1 and ac-2: 1.2.0 plan validates, 88 forms stay unverified` (`test/verb-tense-workshop.test.js:108-174`) passes on a temp DB (`fs.mkdtempSync` at `:72`); `schemaVersion: "1.2.0"` (`scripts/seed-verb-tense-workshop.js:662`); `validation.blocks.length === 0` and `N14` unverified warnings present when unticked (`:171-173`). |
| ac-2 | PASS | `result.forms === 88`, `verified === 0`, 4 lemmas (`לְהַחְלִיט`, `לִבְחוֹר`, `לְהַאֲמִין`, `לְנַצֵּחַ`), tense counts `16 present + 36 past + 36 future = 88` (`test/verb-tense-workshop.test.js:108-138`), each tagged with `tense`, `cell`, `binyan`, `root`, `root_class`, `accept[]`, and `verified = 0` (`scripts/seed-verb-tense-workshop.js:337-355,393-395`). |
| ac-3 | PASS | `SPEC-007-forms-checklist.md:15-141` lists all 88 forms unticked (`88 × - [ ]`, `0 × - [x]`). Test `ac-3: ticking present, past, and future verifies only those forms` (`test/verb-tense-workshop.test.js:196-229`) ticks 3 forms across `present`, `past`, and `future`, confirms `verified === 3`, then unticks one and confirms `verified === 2`. |
| ac-4 | PASS | Test `ac-4: lesson has five stages and drillQueue interleaves verbs and tenses` (`test/verb-tense-workshop.test.js:251-315`) verifies all 5 stages (`explainer` with 3-binyan table `:503-554`, `conjugation-table` `study` in `present`/`past`/`future` `:555-557`, `conjugation-table` `drill` across verbs & tenses `:558-561`, `exercise` `choose` & `translate` `:562-628`, `assignment` + `personal-question` `:629-658`) and confirms `drillQueue()` interleaves across `>=2` lemmas and `>=2` tenses (`:301-306`). |
| ac-5 | PASS | Test `ac-5: no personal names and no class-doc pointing typos` (`test/verb-tense-workshop.test.js:317-358`) confirms zero personal names (`Brian`, `Nelson`, `Aaron`, `Shai`, `Shay`) and zero class-doc pointing typos (`הֶחְלַטֶתִי`, `בַּחָרתִי`, `מְנָּצֵחַ`), while verifying correct dictionary pointing (`הֶחְלַטְתִּי`, `בָּחַרְתִּי`, `מְנַצֵּחַ`). |

## Remediation-loop log

- **Loop 1 (fresh independent verifier):** all-PASS (`ac-1` through `ac-5`, Security criteria, and Verification Strategy; `11 pass / 0 fail` across `test/verb-tense-workshop.test.js` and `test/starter-content.test.js`).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` HALT · `security_review=light` PASS · `domain=data` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
