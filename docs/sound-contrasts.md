# Sound contrasts for "hear the difference"

**Provenance:** `INIT-036/SPEC-008`

## Draft status — do not serve

This file is a **draft inventory**. It is waiting for a **native speaker of Modern Hebrew** or the **learner's tutor** to review every row, listen to a sample clip for each listed pair in **both voices**, and decide which contrasts are worth training.

- Every contrast below is marked `unsure`.
- **No row may be served** by the drill, MCP tool, or data layer until a reviewer sets that row to an approved state and records their name and date here.
- This document does **not** invent a reviewer name or a review date. Those fields stay empty until a person fills them in.

**Reviewer:** _(none yet)_  
**Review date:** _(none yet)_

### Why a person has to check

`RSCH-001` (English speakers learning Modern Hebrew) found **no published list** of English speakers' Hebrew production or perception errors, and no Hebrew training study that prevents them (`RSCH-001` §1, line 19). General second-language perception training helps hard sounds elsewhere (`RSCH-003`), but that is not a Hebrew error inventory. Until a speaker who knows the language signs this list, every row stays unverified.

---

## Analyst working view (not evidence)

The following letter-pair sketch is an **analyst working view only**. It is **not** evidence, not a published finding, and not a native-speaker judgment. It is recorded so a reviewer can confirm or strike it.

| Working-view group | Letters | Working-view note (unverified) |
| ------------------ | ------- | ------------------------------ |
| Likely spelling, not hearing | ט/ת, כּ/ק, ח/כ (many speakers), ס/שׂ, א/ע, ב/ו | In much modern speech these pairs are near-homophones; a listening drill on them may train spelling rather than perception. |
| Candidate true perception targets | ח vs ה vs כ/ק; ר; צ vs ס; stress minimal pairs; prefix and clitic boundaries | May be worth a listening drill for English speakers — only if a reviewer agrees. |

Stress example from the working view (`בִּירָה` with alternating stress) was **not** found as a lexicon entry (see Omissions). Do not treat that example as a servable pair.

---

## Fields every contrast uses

| Field | Meaning |
| ----- | ------- |
| `slug` | Stable id for later data / tools (`SPEC-009`+) |
| What to listen for | The distinction under review |
| Why it may matter | Hypothesis for an English speaker — not proven |
| Lexicon pairs | Pointed `text_nfc` spellings verified in the course lexicon |
| `status` | Always `unsure` in this draft |
| Listening check | Empty until a person listens in both voices |

---

## Contrast inventory

### Homophone letter-pair candidates (working view: may be spelling, not hearing)

These rows exist so a reviewer can **reject them for listening**, keep them, or reclassify them. This draft does **not** mark them `rejected` or `approved`.

#### `tet-tav`

- **What to listen for:** ט vs ת
- **Why it may matter (hypothesis):** English has one /t/; Hebrew spelling keeps two letters that are often the same sound in modern speech.
- **Lexicon pairs (verified):**
  1. `טוֹב` (good) — `תּוֹדָה` (thanks)
  2. `טַעַם` (taste; flavor) — `תָּמִיד` (always)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `kaf-dagesh-qof`

- **What to listen for:** כּ vs ק
- **Why it may matter (hypothesis):** Often the same /k/ in modern speech; useful as a spelling trap, maybe not as a hearing target.
- **Lexicon pairs (verified):**
  1. `כָּל` (every; all) — `קוֹל` (voice; sound)
  2. `כּוֹס` (cup; glass) — `קֶמַח` (flour)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `chet-kaf-soft`

- **What to listen for:** ח vs soft כ (no dagesh), when a speaker merges or nearly merges them
- **Why it may matter (hypothesis):** Working view treats many speakers' ח/כ as near-homophones; a reviewer must say whether any residual difference is worth training.
- **Lexicon pairs (verified):**
  1. `כֵּן` (yes) — `חֵן` (charm; grace)
  2. `כּוֹס` (cup; glass) — `חוֹף` (beach; shore)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `samekh-sin`

- **What to listen for:** ס vs שׂ
- **Why it may matter (hypothesis):** Both often /s/ in modern speech; spelling contrast more than hearing.
- **Lexicon pairs (verified):**
  1. `סֵפֶר` (book) — `שָׂמֵחַ` (happy)
  2. `סִיר` (pot) — `מִשְּׂמֹאל` (on the left)
  3. `סַבָּא` (grandfather) — `שְׂמֵחָה` (happy, f.)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `alef-ayin`

- **What to listen for:** א vs ע
- **Why it may matter (hypothesis):** Many modern speakers weaken or drop the distinction; English speakers also lack ע.
- **Lexicon pairs (verified):**
  1. `אוֹרֶז` (rice) — `עוֹרֵב` (raven)
  2. `אוֹ` (or) — `עוֹד` (more; another; still)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `bet-vav`

- **What to listen for:** soft ב (/v/) vs consonantal ו (/v/)
- **Why it may matter (hypothesis):** Shared /v/ can make spelling, not hearing, the hard part.
- **Lexicon pairs (verified):** No clean pair of content words where one is soft ב and the other is consonantal ו was verified in the lexicon. Soft-ב examples that **were** verified: `אָבִיב` (spring), `בָּרוּךְ` (blessed). Prefixes `בְּ` and `וְ` are listed under `prefix-clitic` instead of inventing a ב/ו content pair.
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

---

### Candidate perception targets (working view)

#### `chet-he`

- **What to listen for:** ח vs ה (and how each differs from a plain vowel onset)
- **Why it may matter (hypothesis):** English has /h/ but not Hebrew ח; confusing ח with ה is a plausible English-speaker error — unverified.
- **Lexicon pairs (verified):**
  1. `חַם` (hot) — `הֵם` (they, m.)
  2. `חָלָב` (milk) — `הַיּוֹם` (today)
  3. `חוּם` (brown) — `הוּא` (he; it, m.)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `chet-kaf-qof-back`

- **What to listen for:** ח vs the back/stop family כּ/ק (when the reviewer treats this as a real hearing target, not only spelling)
- **Why it may matter (hypothesis):** English speakers may flatten ח toward /k/ or /h/; needs a speaker's ear.
- **Lexicon pairs (verified):**
  1. `חֵן` (charm; grace) — `כֵּן` (yes)
  2. `חָלָב` (milk) — `כֶּלֶב` (dog)
  3. `חוֹף` (beach; shore) — `קוֹל` (voice; sound)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `resh`

- **What to listen for:** Hebrew ר (often a uvular or tapped quality) vs the learner's English /ɹ/
- **Why it may matter (hypothesis):** ר is a common production and perception hurdle for English speakers in many languages; Hebrew-specific evidence was not found in `RSCH-001`.
- **Lexicon pairs (verified):** pairs here are resh-bearing words for discrimination practice, not English-r spellings:
  1. `רֹאשׁ` (head; top) — `רַע` (bad; badly)
  2. `רַב` (much; many) — `רִאשׁוֹן` (first; Sunday)
  3. `רַעַשׁ` (noise) — `רַעְיוֹן` (idea)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `tsadi-samekh`

- **What to listen for:** צ (/ts/) vs ס (/s/)
- **Why it may matter (hypothesis):** English has /s/ and /ts/ sequences, but צ as a single letter and Hebrew phonotactics may still confuse learners.
- **Lexicon pairs (verified):**
  1. `צֶבַע` (color) — `סַבָּא` (grandfather)
  2. `צֶמֶר` (wool) — `סֵפֶר` (book)
  3. `צָעִיר` (young) — `סִיר` (pot)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

#### `prefix-clitic`

- **What to listen for:** whether a prefix or clitic is present, and where the word boundary falls (הַ־, בְּ־, וְ־, fused forms)
- **Why it may matter (hypothesis):** English speakers often miss short prefixes in fast speech.
- **Lexicon pairs (verified):**
  1. `בַּיִת` (house) — `הַבַּיְתָה` (home; homeward)
  2. `בְּ` (in; at, prefix) — `וְ` (and, prefix)
  3. `בַּחוּץ` (outside) — `הַכּוֹל` (everything; all)
- **status:** `unsure`
- **Listening check (both voices):** _(not done)_

---

## Omissions (looked up, not invented)

These working-view ideas were **not** given lexicon pairs because a suitable pair could not be verified with a lexicon hit:

| Idea | What was tried | Result |
| ---- | -------------- | ------ |
| Stress minimal pair (`בִּירָה` / alternate stress) | `lookup` query `בירה` | No hit; suggestions were unrelated (`בֵּיצָה`, `דִּירָה`, `סִירָה`). Omitted. |
| `ב/ו` content-word minimal pair | `ורד`, `וילון`, `תקווה`, `וגם` | No consonantal-ו content hit suitable to pair with soft ב. Soft-ב words and the prefixes `בְּ` / `וְ` are noted above instead. |
| Standalone `אָח` (brother) as a ח example | queries `אח`, `אָח` | No exact hit for bare `אָח` (only related forms like `אַחֵר`, `אַחֲרֵי`). Not used. |
| Morning `בֹּקֶר` | pointed and bare queries | No hit for morning; `בָּקָר` (cattle) hit instead. Not used as "morning". |

---

## Serving rule (for later specs)

`SPEC-009` (data), `SPEC-010` (MCP), and `SPEC-011`/`SPEC-012` (drill / screen) may store or show a contrast **only** when this document's row for that `slug` has been human-approved. While every row is `unsure` and no reviewer is named, **serve nothing**.

---

## How lexicon pairs were verified

Every pointed form listed under "Lexicon pairs (verified)" was confirmed with a **hit** from the Hebrew lessons MCP `lookup` tool (read-only) against the live course lexicon. Suggestions-only results were not treated as entries. Omitted candidates are listed in Omissions.
