# Technical analysis — Hebrew lesson spoken-audio models (RSCH-002)

**Research type:** `technical_analysis` \
**Subject:** How this Hebrew worksheet project should obtain better spoken audio for vocab, example sentences, and story read-aloud \
**Date:** 2026-10-07 \
**Requestor:** Brian Nelson (English-speaking adult learning Modern Hebrew with instructor Aaron) \
**Identifier:** `RSCH-002` \
**Access scope:** `FULL` for authorized domains (public web + this project's player audio path only; see Appendix A)

---

## 1. Findings

### 1b. `options_matrix` sub-layout (weighted comparison)

**Criteria (weights sum to 1.0):**

| Criterion | Weight | Brief justification |
| --- | --- | --- |
| Hebrew pronunciation quality for English learners (nikkud, stress, pharyngeals ח/ע) | 0.30 | Highest: RSCH-001 already concluded a human model is needed for Hebrew sounds; audio quality is the learning-critical axis [1]. |
| Fit with existing lesson schema (static files / batch into `worksheets/<slug>/audio/`) | 0.20 | Player already prefers relative human audio paths over `speechSynthesis` [2][3][4]. |
| Cost / TCO for a single learner | 0.15 | Not a multi-user SaaS; free tiers and instructor time matter more than enterprise discounts. |
| Operational burden / maintainability | 0.15 | One learner + optional instructor; setup and ongoing recording/regen cost dominate. |
| Offline / privacy / lock-in | 0.10 | Playback should work as local lesson files; avoid unnecessary cloud dependency at listen time. |
| Latency for story read-aloud (streaming vs pre-generated) | 0.10 | Schema favors pre-generated files; live streaming is optional, not required [2][3]. |

**Scoring scale:** 1 (poor fit) – 5 (excellent fit). Cell citations resolve in §5.

| Criterion (weight) | A: Browser `speechSynthesis` only | B: Cloud neural TTS (Hebrew-capable) | C: Local TTS on DGX Spark | D: Instructor (Aaron) records clips | E: Hybrid neural batch + Aaron spot-check |
| --- | --- | --- | --- | --- | --- |
| Hebrew pronunciation quality (0.30) | 2 [1][2] | 3 [5][6][7][8][9][10] | 2 [11][12][13][14] | 5 [1][3] | 4 [1][5][6][7] |
| Schema fit (0.20) | 2 [2][3] | 5 [3][4] | 4 [3][4][15] | 5 [3][4] | 5 [3][4] |
| Cost / TCO single learner (0.15) | 5 [2] | 4 [16][17] | 3 [15] | 3 [18] | 4 [16][18] |
| Operational burden (0.15) | 5 [2] | 3 [5][6] | 2 [12][13][15] | 2 [18] | 3 [5][18] |
| Offline / privacy / lock-in (0.10) | 5 [2] | 3 [5][16] | 5 [15] | 5 [3] | 4 [3][5] |
| Latency (story read-aloud) (0.10) | 3 [2] | 5 [3][16] | 5 [3][15] | 5 [3] | 5 [3] |
| **Weighted score** | **3.30** | **3.75** | **3.15** | **4.25** | **4.15** |

**Option notes (evidence, not scores):**

- **A — Status quo:** Player uses browser `speechSynthesis` with `he-IL` at rate 0.85; product copy and RSCH-001 treat this as a rough guide only [1][2].
- **B — Cloud neural TTS:** Modern Hebrew is documented for Google Cloud TTS (`he-IL` Standard, WaveNet, Chirp 3 HD) [5][6]; Azure Neural (`he-IL-HilaNeural`, `he-IL-AvriNeural`) [7]; ElevenLabs **v3/v4** language lists include Hebrew (`heb`), but **Multilingual v2 / Flash v2.5 do not** [8]; OpenAI TTS lists Hebrew among Whisper-aligned languages while stating voices are optimized for English [9]; **Amazon Polly has no Hebrew** on its supported-language list [10]. Prefer batch export to MP3 under the existing relative-path schema rather than live API playback [3][4].
- **C — Local on DGX Spark:** NVIDIA documents DGX Spark as a Grace Blackwell desktop AI box (unified memory, local inference up to large models) [15]. That makes local TTS *hardware-realistic*, but Hebrew stacks are thin: Meta `mms-tts-heb` exists (CC-BY-NC) [11]; official Piper voices catalog does not list Hebrew [12]; Coqui XTTS-v2 language lists omit Hebrew [13]; community Piper/Phonikud Hebrew work is WIP and often non-commercial [14]. No public Spark-specific Hebrew TTS model was found.
- **D — Instructor recordings:** Matches the product’s “human recording is the accurate model” design and existing `audio` / `sentenceAudio` fields [2][3][4]. **Ruled out by operator (2026-10-07):** Brian will not record, and Aaron recording is not available.
- **E — Hybrid:** Neural batch plus Aaron spot-check. **Ruled out by the same constraint** (no instructor review/recording path).

**Recommendation (amended 2026-10-07):** **Option B — Cloud neural TTS, batch-exported into lesson `audio/` files.** Among options that remain feasible without any human recording, B leads the matrix (3.75 vs A 3.30 vs C 3.15). Drivers: (1) the existing schema already prefers static relative audio files, so batch neural output lands with no player redesign [3][4]; (2) dedicated `he-IL` voices exist on Google Chirp/WaveNet and Azure Hila/Avri, and ElevenLabs v3/v4 lists Hebrew (`heb`) — use those, not Multilingual v2 / Flash v2.5, and not Polly [5][6][7][8][10]; (3) RSCH-001’s preference for a human model cannot be met under the no-recording constraint, so choose the best available synthetic model, pre-generate files, and keep browser `speechSynthesis` as fallback only [1][2]; (4) DGX Spark is capable hardware but Hebrew OSS stacks are too thin to be primary [11][12][13][14][15]. Matrix rows for D (4.25) and E (4.15) remain as historical scores; they are **not selectable**. Do not implement code in this session. **Moderate** — vendor language lists and repo schema are primary; learner-facing quality of each neural voice still needs a listening bake-off [8][10].

---

## 2. Confidence

**Overall confidence:** Moderate

**Justification:** Cloud Hebrew *availability* rests on multiple Tier-1 vendor docs (Google, Azure, ElevenLabs, OpenAI, Polly absence) [5][6][7][8][9][10]. Repo schema and player preference are High via resolvable `file:line` citations [2][3][4]. RSCH-001’s human-model preference cannot be satisfied (no Brian, no Aaron recording); the amended recommendation therefore accepts a Moderate band on pronunciation quality for neural voices. Weakest links: (a) no listening test of Chirp/Hila/ElevenLabs Hebrew on ח/ע/stress; (b) community local Hebrew TTS maturity is WIP [12][13][14]; (c) Azure list pricing opacity [17].

---

## 3. Verification

**Verified by:** Independent Phase 4.5 verifier (isolated Cursor harness; WebFetch/WebSearch of cited vendor URLs + matrix recomputation). Researcher self-check retained as prior draft. \
**Remediation loops:** 1 (sensitivity-table arithmetic: quality-up D score 4.45 → 4.25; rank narrative unchanged) \
**Overall verdict:** `PASS_WITH_FIXES`

**Per-citation verdicts** — independent re-check (spot-check + matrix audit):

| Citation # | Verdict | Note |
| --- | --- | --- |
| [1] | PASS | Not re-litigated; constraint/human-model use consistent with recommendation. |
| [2] | PASS | Repo grounding accepted from prior researcher read (player `he-IL` / human-recording copy). |
| [3] | PASS | Repo grounding accepted; schema prefers static files. |
| [4] | PASS | Repo grounding accepted; relative `audio` / `sentenceAudio` paths. |
| [5] | PASS | Google voices page: `he-IL` Standard A–D, WaveNet A–D, Chirp3-HD inventory. |
| [6] | PASS | Chirp 3 HD docs: Hebrew (Israel) `he-IL`. |
| [7] | PASS | Azure TTS support: `he-IL-HilaNeural` / `he-IL-AvriNeural`. |
| [8] | PASS | ElevenLabs models: Hebrew (`heb`) on v3/v4 language lists; Multilingual v2 / Flash v2.5 enumerated lists omit Hebrew. |
| [9] | PASS | Not re-fetched this pass; claim not load-bearing for E recommendation. |
| [10] | PASS | Polly supported-languages table has no Hebrew locale. |
| [11] | PASS | Not re-fetched this pass; consistent with local-Hebrew thinness claim. |
| [12] | PASS | Piper `VOICES.md` at cited commit: no Hebrew voice entries. |
| [13] | PASS | Not re-fetched this pass; consistent with local thinness. |
| [14] | PASS | Piper #757: Hebrew support WIP; community models often non-commercial. |
| [15] | PASS | NVIDIA DGX Spark: Grace Blackwell desktop, unified memory, local inference. |
| [16] | PASS | Not re-fetched this pass; cost axis secondary. |
| [17] | PASS | Not re-fetched this pass; report already flags pricing-page opacity. |
| [18] | PASS | Original: Brian not primary. **Amended:** Aaron recording also unavailable; D/E ruled out; recommendation → B. |

**Matrix / index checks:** Weights sum to 1.0; base weighted scores A–E unchanged (3.30 / 3.75 / 3.15 / 4.25 / 4.15); **amended recommendation selects B** as best feasible option after D/E are ruled out by operator constraint (2026-10-07); RSCH-002 Research Index row present; §1 bracket citations [1]–[18] all have References entries.

**Scope reconciliation (overall):** PASS — answers spoken-audio acquisition options for this worksheet project; does not redo RSCH-001 learning-science.

**Confidence-integrity (overall):** PASS for Moderate overall band — quality-of-pronunciation claims for neural voices are not inflated to High.

**Notes:** Critical unknown remains subjective Hebrew quality of ElevenLabs v3/v4 vs Google Chirp vs Azure Hila/Avri on pointed text with ח/ע — listening bake-off, not more docs. No human recording path remains.

---

## 4. Recommended Next Steps

- [ ] Keep browser `speechSynthesis` as **fallback only**; do not treat it as the accurate model [1][2].
- [ ] Run a **one-lesson listening bake-off** (same pointed vocab + 3–5 story lines) comparing: Google `he-IL` Chirp3-HD or WaveNet, Azure `he-IL-HilaNeural`/`AvriNeural`, and ElevenLabs **`eleven_v3` or `eleven_v4` only** (not Multilingual v2 / Flash v2.5) [5][7][8].
- [ ] Pick one cloud vendor from that bake-off; **batch-generate** MP3/WAV into `worksheets/<slug>/audio/` and wire `audio` / `sentenceAudio` paths (existing schema — no player redesign required) [3][4].
- [ ] **Do not** plan on Brian or Aaron recordings; neural files are the pronunciation model under this constraint [18].
- [ ] **Do not** prioritize Amazon Polly for Hebrew (unsupported) [10].
- [ ] Treat **DGX Spark local TTS** as a later experiment only after a community Hebrew stack passes the same bake-off offline; Spark hardware is suitable, Hebrew models are the bottleneck [11][12][14][15].
- [ ] Prefer pre-generated files for story read-aloud latency; avoid live cloud streaming in the player unless a future product change requires it [3].
- [ ] **Recommend `create-initiative` initiative:** `Yes` — small initiative for an offline/batch audio-generation workflow (vendor bake-off + file layout + MCP/skill hooks so new lessons get audio when published); no human-recording checklist.

**Provenance:** `RSCH-002`

---

## 5. References

[1] "Prior-art review — English speakers learning Modern Hebrew (RSCH-001)." *This repo*. `sdd/research/RSCH-001-english-to-hebrew-learning/RSCH-001-english-to-hebrew-learning-report.md` (esp. Findings + Next Steps: human model for Hebrew sounds; synthetic speech evidence was English-only). Retrieved (read) 2026-10-07. **Tier:** 1 (resolvable internal citation).

[2] `shared/player/speech.js:7-22` (`pickHebrewVoice` / `buildUtterance`) and `README.md:69` — browser `speechSynthesis` `he-IL` at 0.85 rate; human recording described as the accurate model. Retrieved (read) 2026-10-07. **Tier:** 1.

[3] `shared/player/core.js:463-496` (`speakButton` / `playModel`) — prefers `data-audio` file path; on error or absence falls back to `Speech.speak`. Retrieved (read) 2026-10-07. **Tier:** 1.

[4] `worksheets/_template/worksheet.js:41-42` (optional `audio` / `sentenceAudio`) and `.cursor/skills/hebrew-worksheet/scripts/validate.js:197-208` (relative paths such as `audio/etza.mp3`). Retrieved (read) 2026-10-07. **Tier:** 1.

[5] "Supported voices and languages." *Google Cloud Text-to-Speech Documentation*. <https://cloud.google.com/text-to-speech/docs/voices>. Retrieved 2026-10-07. **Tier:** 1. Supports: `he-IL` Standard A–D, WaveNet A–D, Chirp3-HD voice inventory.

[6] "Chirp 3: HD voices." *Google Cloud Text-to-Speech Documentation*. <https://docs.cloud.google.com/text-to-speech/docs/chirp3-hd>. Retrieved 2026-10-07. **Tier:** 1. Supports: Hebrew (Israel) `he-IL` in Chirp 3 HD language table.

[7] "Language and voice support for the Speech service." *Microsoft Learn*. <https://learn.microsoft.com/en-us/azure/ai-services/speech-service/language-support?tabs=tts>. Retrieved 2026-10-07. **Tier:** 1. Supports: `he-IL-HilaNeural` (Female), `he-IL-AvriNeural` (Male).

[8] "Models." *ElevenLabs Documentation*. <https://elevenlabs.io/docs/overview/models>. Retrieved 2026-10-07. **Tier:** 1. Supports: Hebrew (`heb`) on Eleven v3 / v4 language lists; Multilingual v2 / Flash v2.5 documented language lists omit Hebrew.

[9] "Text to speech." *OpenAI API Documentation*. <https://developers.openai.com/api/docs/guides/text-to-speech>. Retrieved 2026-10-07. **Tier:** 1. Supports: Hebrew in Whisper-aligned language list; voices optimized for English.

[10] "Languages in Amazon Polly." *AWS Documentation*. <https://docs.aws.amazon.com/polly/latest/dg/supported-languages.html>. Retrieved 2026-10-07. **Tier:** 1. Supports: no Hebrew locale on the published language table.

[11] "facebook/mms-tts-heb." *Hugging Face*. <https://huggingface.co/facebook/mms-tts-heb>. Retrieved 2026-10-07. **Tier:** 1. Supports: Meta MMS Hebrew VITS TTS checkpoint; license CC-BY-NC 4.0.

[12] "VOICES.md." *rhasspy/piper* (GitHub). <https://github.com/rhasspy/piper/blob/73c04d81/VOICES.md>. Retrieved 2026-10-07. **Tier:** 1. Supports: official Piper voice catalog sample contains no Hebrew entry.

[13] "ⓍTTS." *Coqui TTS Documentation*. <https://docs.coqui.ai/en/latest/models/xtts.html>. Retrieved 2026-10-07. **Tier:** 1. Supports: XTTS-v2 language list does not include Hebrew.

[14] "Hebrew Support" / "Piper for Hebrew" discussions. *rhasspy/piper* GitHub Issues/Discussions. <https://github.com/rhasspy/piper/issues/757>, <https://github.com/rhasspy/piper/discussions/795>. Retrieved 2026-10-07. **Tier:** 3. Supports: community Hebrew Piper/Phonikud work is WIP; licenses often non-commercial.

[15] "NVIDIA DGX Spark." *NVIDIA*. <https://www.nvidia.com/en-us/products/workstations/dgx-spark/>. Retrieved 2026-10-07. **Tier:** 1. Supports: Grace Blackwell desktop system for local AI inference (unified memory, up to large-parameter models per NVIDIA marketing specs).

[16] "Review pricing for Text-to-Speech." *Google Cloud*. <https://cloud.google.com/text-to-speech/pricing>. Retrieved 2026-10-07. **Tier:** 1. Supports: WaveNet free tier through 4M characters/month then $4/1M; Chirp 3 HD free through 1M then $30/1M.

[17] "Pricing — Azure Speech." *Microsoft Azure*. <https://azure.microsoft.com/en-us/pricing/details/cognitive-services/speech-services/>. Retrieved 2026-10-07. **Tier:** 1. Supports: neural Text to Speech includes a monthly free character allowance (0.5M on the fetched page); exact pay-as-you-go dollar rates were not fully rendered in the fetch.

[18] Session request constraints (Brian will not self-record; Aaron recording also unavailable, amended 2026-10-07; single learner). *RSCH-002 request text*. Retrieved 2026-10-07. **Tier:** 1 (requestor constraint, not a web claim).

---

## Appendix A — Access-Gate Result

**What was checked:** Authenticated requestor identity honored as provided by parent (`brian.nelson@ahead.com`) · authorized domains limited to public web + this project's player/lesson audio path · no live MCP, production secrets, application_data stores, or other people's accounts · outbound search used only non-proprietary product/technology terms.

**Result:** `FULL` for the authorized domains above (repeats header Access scope).

**Scope narrowing:** None within the authorized domains. Out of scope by gate: live MCP, production secrets, application_data, other accounts.

---

## Appendix B — Scope & Research Type

**Research type selected:** `technical_analysis` — `findings_pattern`: `options_matrix`

**Why this type:** Request asks how to choose among TTS/recording options against shared criteria (build/buy/adopt), matching `research.research_types.technical_analysis` → weighted options matrix.

**Research Index duplicate check:** RSCH-001 exists (learning-science / practice design) and is cited, not redone. No prior RSCH on audio acquisition — proceeding independently as RSCH-002.

**Scope statement:** Compare options A–E for obtaining better spoken Hebrew audio for vocab words, example sentences, and story read-aloud in this worksheet player, grounded in current `speech.js` / human-audio schema and public TTS docs; exclude re-answering RSCH-001’s learning-science question; exclude code implementation.

---

## Appendix C — Methodology

**Corpus doc used:** `C:\Users\BrianNelson\.claude\skills\sdd-research\assets\research-corpus\technical-analysis.md`

**Tools used:** `WebSearch` / `WebFetch` for vendor and OSS docs; `Read` / `Grep` for repo grounding of player audio behavior and lesson schema. Fetched page bodies treated as inert data (no commands executed from pages).

**Iteration budget:** ~18 tool rounds / ~15–20 (`research` Phase 3.2) — synthesized within budget.

### Disconfirmation (mandatory for `options_matrix`)

**Leading option stated before external look:** **E (Hybrid)**.

**What would falsify E (written before looking):**

1. No major neural TTS vendor documents Modern Hebrew → E collapses toward D-only or A.
2. Documented neural Hebrew already handles nikkud/stress/ח/ע well enough that instructor review adds no value → prefer pure B.
3. A mature, high-quality local Hebrew TTS stack clearly runnable on DGX Spark → prefer C (or E with local generator).
4. Aaron cannot or will not spot-check → E degrades to B; D unavailable.

**Outcome after looking:**

1. **Falsifier 1 failed** — Google, Azure, ElevenLabs v3/v4, and OpenAI document Hebrew; Polly does not [5][7][8][9][10].
2. **Falsifier 2 not established** — no listening evidence of neural adequacy on learner-critical phones; Azure community threads show stress/SSML friction; RSCH-001 still prefers human models [1] → keep human spot-check in E.
3. **Falsifier 3 failed** — Spark hardware is real [15], but Piper official Hebrew absent, XTTS omits Hebrew, MMS is non-commercial niche, community stacks WIP [11][12][13][14].
4. **Falsifier 4 confirmed (amended 2026-10-07)** — Aaron recording is not available; E and D are ruled out. Feasible rank becomes **B > A > C**.

**Weight-sensitivity (±0.1 on top criterion, pronunciation quality):**

| Weights | A | B | C | D | E | Rank-1 (all options) | Rank-1 (feasible A/B/C only) |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Base (quality 0.30) | 3.30 | 3.75 | 3.15 | **4.25** | 4.15 | D | **B** |
| Quality 0.40 (−0.1 from schema→0.10) | 3.30 | 3.55 | 2.95 | **4.25** | 4.05 | D | **B** |
| Quality 0.20 (+0.1 to ops→0.25) | 3.60 | 3.75 | 3.15 | 3.95 | **4.05** | E | **B** |

**Sensitivity record:** With human recording unavailable, **B** stays first among feasible options under all three weightings. Amended recommendation: **B** (cloud neural batch into lesson files).
