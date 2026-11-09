# technical_analysis — Voice input and pronunciation/prosody feedback for an adult English speaker learning Modern Hebrew, without storing the recording (RSCH-012)

**Research type:** `technical_analysis` \
**Subject:** How an adult English speaker learning Modern Hebrew can use voice input that is accurate enough to be useful, and get feedback on pronunciation and on prosody (lexical stress, intonation), without the app storing the recording \
**Date:** 2026-11-08 \
**Requestor:** Project operator (name withheld by design), via the project's research dispatch \
**Identifier:** `RSCH-012` \
**Access scope:** `NARROW` — public web sources only (see Appendix A). Shared research is not available; this session stays local to this project.

---

## 1. Findings

**Design constraints taken as given (not re-researched):** the app stores no recordings; audio is processed in the browser and discarded, or the report must say a cloud option breaks that rule; speech recognition is not built; a human tutor exists as a fallback; synthetic TTS is not a substitute for a human model of the sounds.

### 1.0 Recommendation (5 sentences)

1. Do not plan around a pronunciation-assessment API for Hebrew: the three vendors checked (Azure Speech, SpeechSuper, Speechace) do not list Hebrew for pronunciation scoring, and Azure's prosody scoring is en-US only [8][9][10][11].
2. Run a short spike that runs Hebrew speech-to-text entirely in the browser (the 66M-parameter streaming Zipformer2 model that ivrit.ai mirrors, through sherpa-onnx/WASM) only as a "did it hear the target word" check, because that keeps audio on the device and scores about 5–9% WER on two Hebrew test sets that are not learner speech (23% on the harder read-speech FLEURS set, which [3] attributes partly to reference errors; Low), whereas Chrome's Web Speech API sends audio to a server by default and documents no on-device Hebrew pack, and Firefox 160's local recognizer (still in beta on 2026-11-08) lists Hebrew only through a larger multilingual model that scores 17–46% on the same sets [1][3][17][19][20][21][23][44][45].
3. In the same spike, test an in-browser acoustic comparison of the learner against a human-recorded model, using vowel duration for lexical stress (the one reliable Hebrew stress cue in the published phonetics) and pitch only for phrase-level intonation; this is the only option that gives stress feedback without training a new model, but its on-device and cost properties are untested design assumptions [33][34].
4. Treat every automatic verdict as a hint that the tutor confirms, because no figure for adult non-native Hebrew speech was found in the searches run, ASR transcripts are blind to Hebrew stress and vowel quality, and recognizers tend to correct what a learner actually said [4][5][14][15].
5. Reject every cloud recognizer for this feature (it sends audio off the device, which breaks the stated rule even where the vendor says nothing is stored at rest), and fix the spike's pass bar with the tutor before running it, measured as agreement with the tutor's judgments on a labelled set of learner attempts [38][39].

**Overall confidence: Low** (see §2). The landscape facts (no Hebrew pronunciation-scoring API found among three vendors; on-device Hebrew ASR exists; Hebrew stress is carried mainly by duration) are **Moderate**; the central learner question (does any of this accuracy transfer to adult non-native Hebrew) has **no direct evidence**, and option F's standing rests on cells the spike must test.

### 1.1 Options matrix (weighted additive scoring)

Scale 0–5. "?" = unknown (no source retrieved); the weighted score is a range: **lower** treats "?" as 0, **upper** treats "?" as 5. The tutor is not scored (see note T): it is the premise/fallback the operator fixed, not a researched option.

Scale anchors, fixed before scoring: **Hebrew coverage** (depends on measured accuracy, best test set, non-learner speech) 5 = published Hebrew WER of 10% or less, 4 = published WER above 10% up to 20%, 3 = Hebrew listed or claimed without a published figure for that engine, or a language-independent method with a documented Hebrew cue for the property scored (a single self-reported source is capped at 3 until replicated), 2 = published WER above 20% on its best set, 0 = not supported. **Non-native evidence** 0 = none, 1 = only evidence of degraded non-native accuracy in other languages, 2 = positive controlled learner studies for the method in other languages, 3 or more = Hebrew non-native evidence (none found in two searches). **Phoneme feedback** 5 = documented Hebrew phoneme-level scores, 2 = self-reported only, 1 = word-level match only, 0 = none. **Stress/intonation feedback** 5 = documented Hebrew scoring, 3 = feasible and indirectly evidenced, 1 = research model exists with no learner test, 0 = documented absent. **Audio stays on device** 5 = recognition runs inside a page the app controls, 4 = a browser-provided local mode documented by a first-party source but untested, 0 = audio leaves the device or cannot be forced local for Hebrew. **Cost shape** 4 = no metered fee, 2 = self-reported server GPU cost. **Maturity** 4 = GA product with Hebrew listed, 3 = working but community or early, 2 = shipped browser feature whose Hebrew support is not claimed by the model's own card, 1 = self-reported single build or a pre-release browser feature, 0 = none for Hebrew; "?" = no source for a browser or Hebrew build. The upper bound counts "?" as 5 even where an anchor tops out at 4 (cost, maturity), so upper bounds are generous by up to 0.1 per such cell.

| Criterion (weight) | A1: Web Speech API, Chrome | A2: Web Speech API, Firefox 160+ on-device | B: On-device Hebrew ASR (Zipformer2 / Whisper ONNX) | C: Cloud recognizer (Azure Speech) | D: Pronunciation-assessment API (Azure PA / SpeechSuper / Speechace) | E: Build a Hebrew phoneme recognizer + GOP | F: In-browser acoustic comparison vs human reference |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Hebrew coverage (0.15) | 3 [22] | 4 [1][44] | 5 [1][23] | 3 [8] | 0 [8][10][11] | 3 [14] | 3 [33][34] |
| Non-native accuracy evidence (0.10) | 1 [4][5] | 1 [4][5] | 1 [4][5][6] | 1 [4][5] | 0 | 1 [4][5] | 2 [29][30][31] |
| Phoneme feedback (0.15) | 1 | 1 | 1 [15] | 1 [15] | 0 [9] | 2 [14] | 0 |
| Stress/intonation feedback (0.15) | 0 [15] | 0 [15] | 0 [15] | 0 [15] | 0 [9][11] | 1 [14][15] | 3 [33][34][29] |
| Audio stays on device (0.25) | 0 [17][18][20] | 4 [19][44] | 5 [23][24] | 0 [38] | 0 [9] | ? [14] | ? |
| Cost shape (0.10) | ? | 4 [44] | 4 [23] | ? | ? | 2 [14] | ? |
| Maturity (0.10) | 4 [19][22] | 1 [19][44][45][46] | 3 [23][26] | 4 [8] | 0 [8] | 1 [14] | ? |
| **Weighted score (lower–upper)** | **1.10–1.60** | **2.35** | **2.95** | **1.10–1.60** | **0.00–0.50** | **1.30–2.55** | **1.10–3.35** |

**Notes on cells.**

- **A.** Chrome's Web Speech API documents `he-IL` among supported languages [22] but the documented on-device (`processLocally`) list omits Hebrew [20][21]; with `processLocally = true` and no pack, `start()` fails with `language-not-supported` [18]. MDN states Chrome's default engine sends audio to a web service [17]. Hebrew accuracy of Chrome's engine was not found, so the non-native and cost cells are "?".
- **A2.** Firefox's on-device recognition list (main branch of its source, retrieved 2026-11-08) accepts `he-IL` through one multilingual model, Nemotron 3.5 ASR Streaming 0.6B (785 MB download after a permission prompt), and the file's own comment places Hebrew in the model's "adaptation-ready" tier, which the card does not claim as broad coverage [44]. That model scores 17.1% / 36.5% / 46.0% on eval-d1 / WhatsApp / FLEURS on the leaderboard [1] (about three times the WER of the Zipformer2 model at 5.3% / 8.8% / 23.4%). The Nemotron card says its "adaptation-ready" locales, Hebrew among them, need fine-tuning before production transcription [1][44]. Firefox 160 is the first release listed with `processLocally` [19] and was still in beta on 2026-11-08, with Mozilla's beta notes saying features may not reach the final release and the stable release due 2026-11-10 [45][46] (read by the verifier, not by the author); the beta notes state a 40-language count without listing languages, so whether the release build exposes Hebrew is unconfirmed, and Firefox for Android is not supported per [19]. Hence maturity = 1. Edge's documentation puts its local model in Canary/Dev builds behind a flag for six languages, none Hebrew [47] (also read by the verifier).
- **B.** `zipformer2-66m-streaming-he` was developed by one individual and is mirrored by ivrit.ai with extra browser/ONNX exports; CC-BY-4.0; ivrit.ai's live-transcribe site uses it for on-device transcription, and int8 on WASM was about as fast as fp16 on WebGPU on an integrated GPU [23]. Larger Hebrew Whisper builds run in-browser on WebGPU but need about 1.6 GB downloads [25]. WebGPU support was about 85% globally as of March 2026 [27]. The "1" for non-native evidence reflects only cross-language evidence that non-native speech degrades accuracy [4][5][6]; no Hebrew learner figure was found.
- **C.** Azure Speech lists `he-IL` for speech-to-text [8]; no Azure Hebrew accuracy figure was retrieved, so coverage is 3, not 4–5. Microsoft documents that real-time speech-to-text keeps audio only in server memory and logging is off by default [38][39]. Audio still leaves the device, so the "no audio leaves the app" rule fails; retention terms do not change that. Google's Chirp 3 lists Hebrew (`iw-IL`) as Preview only [40]. No price was retrieved, so cost is "?".
- **D.** Azure's pronunciation assessment lists 33 locales and Hebrew is not among them; its prosody scoring (stress, intonation, rhythm) is en-US only, and phoneme names exist for en-US (IPA) and en-US/zh-CN (SAPI) only [8][9]. SpeechSuper lists eight languages without Hebrew [10]. Speechace lists English, Spanish and French only, and its stress/intonation scoring is English-only [11]. Rosetta Stone's TruAccent is advertised as comparing a learner with native speakers in its Hebrew course, but it is a closed product, not an API, and the retrieved pages give no phoneme- or stress-level claim for Hebrew [12][13] (marketing, Tier 3).
- **E.** One agency blog (self-reported, October 2026) describes a Hebrew pronunciation scorer built from a wav2vec2 XLS-R phoneme recognizer with goodness-of-pronunciation scoring, run on a server GPU, trained and checked on native speech only; the same post says stress is not judged yet and learner behaviour is not yet measured [14]. Separately, a research paper released Hebrew audio-to-IPA models that can represent stress, trained on about two hours from two speakers and evaluated on 150 synthetic utterances (24.71% WER, 5.47% CER), not on learners [15] — hence stress = 1. The figures are those in the PDF served at the cited URL on 2026-11-08; a later revision read by a verifier describes different training (a larger Whisper model, first on about 30 hours of SASPEECH), so cite the revision before relying on the figures. Zero-shot cross-lingual phoneme recognition without training on the target language reached about 22.9% phoneme error on held-out European languages [16], so it is not a drop-in. Browser feasibility is unknown.
- **F.** Pitch-contour feedback improved learner production in controlled studies of English intonation and Mandarin tones, none in Hebrew and several with small groups [29][30][31][32]. Hebrew lexical stress is carried mainly by vowel duration; pitch marks phrase-level boundaries [33][34]. Whether such analysis can run fully in the page, and at what build cost, is a design assumption that no retrieved source tests, so those cells (and maturity, for which no source shows a browser or Hebrew build) are "?". Coverage of 3 reflects a language-independent method plus a documented Hebrew stress cue, with no Hebrew learner trial.
- **T. Tutor (not scored).** Premise from the operator. Its cells are qualitative: phoneme and stress feedback by a human who hears the learner live; no recording stored by the app if the session is live. The mixed evidence on human versus visual feedback is in §1.2 SQ4.

**Weight sensitivity (research-qa §4).** Taking the sourced (lower-bound) scores, moving the highest-weighted criterion (audio on device, 0.25) to 0.15 or 0.35 and rescaling the rest proportionally keeps B first and A2 second: B 2.68 / 3.22 and A2 2.13 / 2.57; with the stress weight at zero (rescaled), B 3.47 and A2 2.76. This is a lower-bound statement: E's upper bound (2.55) exceeds A2's score (2.35) in the base case and F's upper bound (3.35 base; 3.13 and 3.57 at the two audio weights) exceeds B's, so the unknown cells could change the order. Below B and A2 the lower-bound order moves with the weights (E, C, A1 and F sit within about 0.4 of each other), so this report does not rank them. **B and A2 are the only options with no unknown cells; B's lead over A2 is 0.60 and comes from B's lower WER (5.3% against 17.1% on eval-d1, [1]), from working in any browser rather than a pre-release Firefox feature, and from running in a page the app controls.** F's range (1.10–3.35) overlaps B's 2.95, so the data neither show F above B nor rule it out. F could pass B only if its on-device, cost and maturity cells score high in the spike.

**Recommended option: B as the base, with F added only as a spike test for stress**, because B has the highest fully sourced score (2.95; Firefox's built-in recognizer, still in beta, is next at 2.35) and F is the only option that gives stress feedback without training a new model. **[Confidence: Moderate for B's Hebrew coverage and on-device fit (first-party model card plus a benchmark that shares authors with the model); Low for F, whose standing depends on untested cells.]**

### 1.2 Evidence by sub-question

**SQ1a — Published Hebrew speech-to-text accuracy, speakers not described as learners (answered; Moderate).**

- On ivrit.ai's public leaderboard (WER, lower is better; test sets are podcast, WhatsApp voice messages, read speech, news; none is described as learner speech and the sources read do not label speakers by first language), the project's own Whisper large-v3 fine-tune scores 5.1% / 7.2% / 17.4% on eval-d1 / WhatsApp / FLEURS-he; stock Whisper large-v3 scores 9.8% / 13.2% / 26.2%; Amazon Transcribe (batch) 6.6% / 10.4% / 23.0%; Soniox 4.8% / 9.0% / 17.7%; Google Speech 21.1% / 35.2% / 38.5% (so 21.1% is eval-d1 and 38.5% is FLEURS); the Zipformer2 66M streaming model scores 5.3% / 8.8% / 23.4% [1][23].
- A separate site that re-scores FLEURS-he [3] (only partly independent of the leaderboard) reports, in its headline column, lower WER for the same systems: Whisper large-v3 20.3%, Amazon batch 14.3%, Google 33.0%, Zipformer2 17.3% (leaderboard FLEURS column [1]: 26.2%, 23.0%, 38.5%, 23.4%). The site excludes 62 of 792 FLEURS-he clips it judges wrongly transcribed (stated by the verifier, who could load the page; the author's fetch returned an empty script-rendered page), which would explain part of the gap; treat the explanation as Low. FLEURS-he figures therefore depend on reference cleaning; treat 17–26% for the best systems as a range, not comparable with the leaderboard's other test sets.
- The Interspeech 2025 paper behind ivrit.ai reports 8–29% lower error than other Whisper versions, and warns that WER counts spelling variants and numerals as errors, so figures are not directly comparable across datasets [2].
- Small stock Whisper models are not usable for Hebrew: one community card, scored with a harness it says reproduces the ivrit.ai leaderboard, lists stock whisper-small at 29.9% / 39.6% and whisper-base at 48.3% / 56.6% on eval-d1 / WhatsApp [26]. That card's own 8.59% / 15.94% for a 61M distilled model is a single self-reported source.
- Caveat on independence: the leaderboard, the paper, and the strongest model share authors [1][2][23].

**SQ1b — Published accuracy for non-native adult learners of Hebrew (not answered; open question; band capped Low).**

- **No Hebrew non-native figure was found** in two targeted searches; the Hebrew ASR papers retrieved evaluate general or native speech, corpora, or datasets [2]. This is an absence after a bounded search, not proof that none exists.
- Cross-language evidence says to expect worse: across ten ASR systems, the gap between non-native and native English was 2.6–5.9×, with tonal-L1 speakers worst [4]; Whisper scored 28.7% WER on Japanese learners' spontaneous English and tended to correct erroneous verb and noun forms in the transcript [5]; Whisper large-v3 averaged 14% WER on L2 Spanish, and the authors conclude accurate pronunciation assessment needs a separate phonetic analysis [6]; on non-native Arabic, Whisper large-v3 reached 66% WER (human listeners scored 67%), with errors tied to emphatic and guttural sounds [7]. Arabic is a different language, but it shares the guttural consonants an English speaker finds hard in Hebrew.

**SQ2 — Pronunciation-assessment products that score Hebrew phonemes, stress, or intonation (answered; Moderate for the absence finding among the three vendors checked).**

- Azure Speech: pronunciation assessment covers 33 listed locales, none Hebrew, while speech-to-text lists `he-IL`; prosody (stress, intonation, rhythm) is en-US only [8][9]. SpeechSuper and Speechace: no Hebrew; stress/intonation scoring English-only at Speechace [10][11]. A third-party practitioner blog says no vendor sells a Hebrew pronunciation scorer as of October 2026 [14]; that agrees with the vendor documents but is not independent evidence of them. Other vendors were not checked.
- Product claims that exist: Rosetta Stone says TruAccent gives real-time feedback on Hebrew [12][13] (Tier 3 marketing; **Very Low**, no evidence of accuracy, phoneme detail, or stress). The agency blog's self-built Hebrew scorer reports 7.0% sound error on held-out native speech and sound-level feedback, but is **Very Low–Low** (single self-reported source, native clips only, stress unjudged) [14].
- Hebrew ASR itself cannot see stress: a paper on Hebrew TTS (arXiv 2506.12311, first posted 2025, revised 2026) notes that standard Hebrew ASR outputs unvocalized text, blind to vowel quality and stress placement; the same paper trains audio-to-IPA models that do capture such features, on very little data and tested on synthetic speech only [15].
- Note on method: the search tool's own AI summary for the Azure query wrongly said Hebrew was supported for pronunciation assessment; the vendor's own language table was read directly and does not list it [8]. No finding rests on that summary.

**SQ3 — What can run on-device, and the privacy consequence (answered for browsers; partly open for phones; Moderate for browsers).**

- **Web Speech API (Chrome and Firefox).** Chrome's default engine sends audio to a web service [17]. A `processLocally` mode and `available()`/`install()` language-pack methods exist in Chrome/Edge 139+ and Firefox 160+, not Safari [18][19]. The documented Chrome on-device language list has 17 languages and not Hebrew [20]; the ChromeOS language-pack installer source file also has no Hebrew entry (full file searched for `he-IL`, `he`, `iw`, "Hebrew") [21]. Both are Google/Chromium sources, so they are not independent and they speak only for Chrome. **Firefox differs:** its on-device model list includes `he-IL` through the Nemotron 785 MB model, with Hebrew in the "adaptation-ready" tier and no latency measured in the file; the same model scores 17.1% / 36.5% / 46.0% on the Hebrew leaderboard [1][44]. Edge's documentation lists no Hebrew for its local model, which is a Canary/Dev flag only [47]. Consequence: a Hebrew web-speech feature cannot be forced local in Chrome today, so it breaks the rule there; in Firefox 160+ (beta on 2026-11-08) it may be local but at a much higher error rate than option B, and Hebrew in the release build is unconfirmed [45][46]. The spike should still call `SpeechRecognition.available({langs:["he-IL"], processLocally:true})` on target browsers to confirm.
- **On-device Hebrew models.** ivrit.ai lists ONNX/browser builds, a whisper.cpp build, and the 66M streaming Zipformer2 for "real-time / on-device streaming" [24]. Audio never leaves the page if the model files are served by the app and recognition runs in a Worker; this is a design consequence, not something a source tested for this app. The model card says the int8 export is half the download of the fp16 build [23] (no absolute size was read), against about 1.6 GB for large Whisper builds [25]; the model was trained on about 20k hours of ivrit.ai audio labelled by Whisper models, not on learner speech [23]. Streaming decodes in 1.28 s chunks [23].
- **Phones.** The `processLocally` mode is not supported in Chrome for Android, Firefox for Android, or iOS Safari [19]. Native phone recognizers are a different path from a web page: Android's on-device recognizer lists downloadable languages through `RecognitionSupport` (Android 13+) but the retrieved reference does not say Hebrew is included [41]; Apple's recognizer reports on-device support per locale at runtime and notes that some languages need an internet connection [42]; a 2022 test on one iPhone listed 22 on-device locales and no Hebrew (one device, iOS 15.4, dated, Tier 3) [43]. **Hebrew on-device recognition on phones is therefore unconfirmed.** Whether the 66M model runs acceptably in a phone browser (WASM or WebGPU) was not checked.
- **Cloud.** Real-time Azure speech-to-text holds audio in server memory only and logging is off by default [38][39]; the audio still leaves the device. Google Chirp 3 lists Hebrew as Preview [40].

**SQ4 — What feedback format do published studies find useful (partly answered; Low–Moderate).**

- Across 15 studies of ASR in English pronunciation teaching, explicit corrective feedback (waveform/spectrogram comparison with a native speaker, or messages about the error) had g = 0.86 against g = 0.50 for indirect feedback such as a transcript or right/wrong flag; it reports large effects for segmental and small for suprasegmental pronunciation, and also notes that a separate computer-assisted-pronunciation meta-analysis found the opposite pattern (suprasegmental 0.89 vs segmental 0.47), so that gap is contested [28]. **Low** for "explicit comparison beats a bare transcript": one meta-analysis, English ESL/EFL only, and its subgroup difference is not reported as significant in the text retrieved.
- Visual pitch-contour feedback: English-intonation training for Japanese learners with interactive F0 feedback beat a no-feedback group [30]; a four-week web training with instant pitch feedback improved Chinese learners' spontaneous-speech comprehensibility and confidence while controls did not (abstract-level; learners rated individualized evaluation from the researcher above self-made Praat feedback) [31]; English-speaking learners of Mandarin tones using F0 displays moved closer to native mean F0 and range than controls (small study, cohorts compared across years) [29]. Another Mandarin-tone study found a Praat-feedback group made larger gains than a group given feedback by the researcher [32]. **Low:** small trials, other languages, and a split between "human feedback preferred" and "Praat group did better".
- **Hebrew stress phonetics.** Duration is the reliable cue for primary stress in Modern Hebrew; there is no realized secondary stress [33]; at utterance end duration is again the word-level cue and F0 marks higher prosodic units [34]. Consequence for the display: a pitch curve alone would not show a stress error; a duration (vowel-length) view would. **Moderate** (two peer-reviewed studies, overlapping research group; abstract-level).
- **Minimal-pair and phoneme-highlight formats were not separately evidenced** in what was retrieved; this part of SQ4 is an open question (Low).
- **English-speaker Hebrew stress errors** (initial-stress habit on words like *shalom*) appear only in practitioner pages [35][36] (**Very Low**); the peer-reviewed study retrieved is of Arabic-L1 learners and found they discriminate Hebrew stress less accurately than native speakers [37].

**SQ5 — Options matrix and recommended spike.** See §1.1 and §4.

### 1.3 Disconfirmation record (research-qa §4)

- **Leading answer stated first:** on-device Hebrew ASR as a word check, plus an in-browser pitch comparison against a human recording for stress and intonation, with the tutor as authority and no scoring API.
- **Evidence that would make it wrong, written before looking:** (a) a Hebrew-capable pronunciation API with phoneme and stress scores; (b) evidence Hebrew ASR cannot run acceptably in a browser; (c) evidence that acoustic displays do not help or do not carry Hebrew stress.
- **Outcome:** (a) searched Azure, SpeechSuper, Speechace documents, a consumer product, and a Hebrew mispronunciation-detection query; none found (Rosetta Stone's claim has no detail). (b) Found model cards showing a 66M model running in-browser on WASM, self-reported; not independently tested. (c) **Partly found:** Hebrew stress is carried by duration, not pitch [33][34], so the answer was revised from a pitch curve to a duration view with pitch for phrase intonation; the ASR meta-analysis shows smaller effects for suprasegmentals but reports that another meta-analysis found the reverse [28]. Verification round 1 also showed F's top rank rested on unsourced cells; F's scores were reduced to ranges and the "F above B" claim was withdrawn. Verification round 2 found that Firefox 160+ lists `he-IL` for on-device recognition through a 785 MB multilingual model [44], which contradicted the Chrome-only scoring of Web Speech; Web Speech was split into Chrome and Firefox columns, the coverage anchor was tied to measured WER, and B's lead over Firefox was restated as 0.50 with its cause.

---

## 2. Confidence

**Overall confidence:** `Low`.

**Per-finding bands (technical-analysis §5 rubric):**

| Finding | Band | Basis |
| --- | --- | --- |
| Hebrew ASR on non-learner speech: best open models about 5–9% WER on podcast/WhatsApp sets, 17–26% on FLEURS depending on reference cleaning; Google 21.1% / 35.2% / 38.5% on eval-d1 / WhatsApp / FLEURS | Moderate | Benchmark and paper share authors [1][2][23]; one partly independent re-run [3]; one reproduction by a community card [26]; coverage 0.8 |
| No published Hebrew non-native ASR figure found | Low (open question) | Absence after two searches; coverage 0.3 (< 0.5) |
| Cross-language non-native ASR degradation | Moderate | Four independent papers agree on direction [4][5][6][7]; none Hebrew |
| Azure, SpeechSuper and Speechace do not document Hebrew pronunciation scoring, and none scores Hebrew stress or intonation | Moderate | Three vendor documents, retrieved 2026-11-08 [8][9][10][11]; other vendors unchecked; coverage 0.9 |
| Rosetta Stone TruAccent for Hebrew | Very Low | Vendor marketing only [12][13] |
| A self-built Hebrew phoneme scorer is feasible | Very Low–Low | One self-reported agency blog [14] |
| Chrome sends Web Speech audio to a server by default | Moderate | MDN [17] plus the explainer's on-device framing [20] |
| Chrome has no on-device Hebrew pack on its documented list | Low | Two Google/Chromium sources, not independent [20][21]; Edge covered separately below |
| Firefox 160+ lists `he-IL` for on-device recognition through a 785 MB multilingual model with 17–46% WER | Low | First-party model list [44] plus the leaderboard row [1]; Firefox 160 was beta on 2026-11-08 and release exposure is unconfirmed [45][46] |
| Edge's local model does not list Hebrew | Low | One Microsoft page, read by the verifier [47] |
| Hebrew on-device recognition on phones | Very Low (open) | No retrieved source confirms it; one dated single-device test lacks Hebrew [43] |
| On-device Hebrew ASR can run in the browser | Moderate for existence, Low for speed and size claims | First-party card [23] and community cards [25][26]; self-reported timings |
| Explicit comparison feedback beats a bare transcript | Low | One English-only meta-analysis [28]; contested subgroup pattern |
| Visual pitch feedback helps learners | Low | Small trials in other languages [29][30][31][32]; mixed human-versus-visual results |
| Hebrew lexical stress is carried mainly by duration | Moderate | Two peer-reviewed studies [33][34], overlapping authors |
| B scores highest on sourced cells; F's standing is open | Low | F's on-device and cost cells are unknown; ranges overlap |

**Justification and budget.** The Phase 3.2 cap (~15–20 calls) was **exceeded**: 27 web search/fetch calls were made before synthesis was forced, because sub-questions 1b, 3 (phones) and 4 stayed under-evidenced and the Hebrew stress-cue finding changed the recommendation. SQ1b (non-native Hebrew accuracy) scored 0.3 and SQ4 scored 0.6; both are carried as open questions, as is the phone part of SQ3. The matrix lower/upper range is shown rather than averaged. Pricing for cloud options was not retrieved; no price appears in this report.

---

## 3. Verification

> Produced by a fresh, independent verifier sub-agent per `research` Phase 4.5. Filled in after verification.

**Verification status:** `NOT PASSED — 3 loops used; round-3 fixes unverified`.

**Verifier model:** Claude Opus 5.5 (requested slug `claude-opus-5-5-xhigh`, accepted), a fresh read-only sub-agent in each round, none of which wrote the report. The author applied all fixes.

| Round | Verdict | Main defects found | Fix applied |
| --- | --- | --- | --- |
| 1 | FAIL | 11 defects: a Google WER range that mixed two test sets; a dropped source ([29] in that draft, Wiley CAPT meta-analysis, returned 403, not public) and its effect size; F ranked above B on unsourced cells; the leaderboard FLEURS caveat missing; a contested suprasegmental effect not reported; absence claims worded too strongly; no phone coverage; the audio-to-IPA model omitted | Sources and cells corrected; F's unknown cells shown as ranges; phone paragraph added; absence claims reworded; sources renumbered to [1]–[43] |
| 2 | FAIL | One major: Web Speech was scored from Chrome-only sources, but Firefox's source lists `he-IL` for on-device recognition through a 785 MB model; plus uneven anchors for E, A and F and one overstated cause | Web Speech split into Chrome and Firefox columns; coverage anchor tied to measured WER; E, A and F rescored; [44] added; B's lead restated |
| 3 | FAIL | Two major: "B is the only option with no unknown cells" was false (Firefox also has none); Firefox 160 was treated as shipped but was beta on 2026-11-08. Minor: order claims true only on lower bounds; "only option that addresses stress" conflicted with E's score; [15] training detail; anchor gaps; Edge and [3] details | All applied: wording corrected, A2 maturity set to 1 (A2 now 2.35), anchors extended, [15] revision caveat added, [45]–[47] added, Edge and [3] details added |

**What the verifiers confirmed:** every WER figure quoted from the leaderboard; the vendor locale tables; the Web Speech and Chrome sources; the matrix arithmetic and the sensitivity figures at each round; that no claim cites a number outside the reference list; no long verbatim quotes (no `untrusted-transcript` fences needed).

**Per-citation verdicts at the last full check (round 3, 27 references sampled):** supported, except [15] (partly supported: later revision differs from the cited PDF text, now disclosed) and [3] (unverifiable by the author; the verifier read it and confirmed the headline figures). [29] in the round-1 draft was dropped as not public.

**Scope verdict:** all five sub-questions are addressed and the open parts are stated. **Confidence verdict:** `Low` is justified (no Hebrew learner evidence; F rests on untested cells; several key sources are single and self-reported; Firefox's Hebrew support is pre-release).

**Not re-verified:** the round-3 fixes. Per the three-loop limit, the following are held at their stated bands and are marked as unverified by a fresh reader: the A2 maturity re-score and the lead of 0.60; the Edge statement [47]; the explanation for the FLEURS gap via [3]; the [15] revision caveat. None changes the overall confidence or the recommended spike.

**Disclosures:** the verifier's disconfirmation searches (other pronunciation vendors, non-native Hebrew accuracy, Edge/Firefox packs, Hebrew visual feedback) found no Hebrew pronunciation scorer and no non-native Hebrew accuracy figure, which supports the absence statements; the Hebrew-specific acoustic study that attributes most stress-variance to duration (a ResearchGate abstract found in round 2) agrees with [33][34] but is not cited.

---

## 4. Recommended Next Steps

**Spike (a throwaway page, not a build commitment):**

- [ ] **Agree the pass bar with the tutor before running anything.** Build a labelled set of learner attempts (target word read correctly; deliberately wrong consonant such as ח/כ; deliberately wrong stress, using stress minimal pairs such as *ókhel* "food" versus a final-stress form, taken from a Hebrew grammar page [35] and confirmed by the tutor) judged by the tutor, and define success as agreement with the tutor's judgments, not as WER.
- [ ] **B — in-browser word check.** Run `zipformer2-66m-streaming-he` int8 via sherpa-onnx WASM in a Web Worker, model files served from the app origin; keep audio in memory and discard it after the check; confirm in the network panel that no audio bytes leave the page. Record how often it accepts a deliberately wrong attempt (the "autocorrect" risk) [5][14]. Also record load time and speed on a phone browser, which no source covers.
- [ ] **A — one-line check.** Call `SpeechRecognition.available({langs:["he-IL"], processLocally:true})` on target desktop browsers (Chrome, Edge, and Firefox 160+) and record the answer; on Firefox also record the model download size and its error rate on the labelled set against B [18][20][44].
- [ ] **F — acoustic comparison.** Compare a learner attempt with a human-recorded model on syllable timing/vowel duration (stress) and pitch contour (phrase intonation), computed in the page; record build effort and whether it runs fully in the page; have the tutor judge whether the display's verdict matches theirs on the labelled set [33][34].
- [ ] **Keep the tutor as authority** for phoneme-level and stress-level corrections; show the automatic result as a hint.

**Open questions to carry:** accuracy on adult non-native Hebrew speech (none found); whether duration displays change learners' stress production (no Hebrew trial); minimal-pair and phoneme-highlight formats (not separately evidenced); Hebrew on-device recognition on phones (unconfirmed); price of cloud options (not retrieved).

- [ ] **Recommend `create-initiative` initiative:** `No` — findings support a spike only; the learner-outcome evidence is too thin to commit a build.

**Provenance:** `RSCH-012` — this report's citable identifier.

---

## 5. References

All entries retrieved 2026-11-08. Tier: 1 = primary/first-party docs, 2 = peer-reviewed or analyst/benchmark, 3 = secondary/vendor marketing/blog.

[1] "benchmark.csv — ivrit-ai/hebrew-transcription-leaderboard." *Hugging Face Space (ivrit.ai)*. <https://huggingface.co/spaces/ivrit-ai/hebrew-transcription-leaderboard/blob/main/benchmark.csv>. Retrieved 2026-11-08. **Tier:** 2 (benchmark run by a model author).
[2] Marmor et al., "Building an Accurate Open-Source Hebrew ASR System through Crowdsourcing." *Interspeech 2025 (ISCA Archive)*. <https://www.isca-archive.org/interspeech_2025/marmor25_interspeech.pdf>. Retrieved 2026-11-08. **Tier:** 2.
[3] "scribeil — FLEURS Hebrew benchmark." *scribeil.com*. <https://scribeil.com/bench/dataset/fleurs>. Retrieved 2026-11-08. **Tier:** 3.
[4] "Scaling Conformation Bias in Automatic Speech Recognition." *SLaTE 2025 (ISCA Archive)*. <https://www.isca-archive.org/slate_2025/roll25_slate.pdf>. Retrieved 2026-11-08. **Tier:** 2.
[5] "Accuracy of Automatic Speech Recognition in L2 Learners' Spontaneous Speech." *Language Education & Technology (J-STAGE)*. <https://www.jstage.jst.go.jp/article/letcj/35/0/35_15/_article/-char/en>. Retrieved 2026-11-08. **Tier:** 2.
[6] Sarymsakova & Martín-Rodilla, "Assessment of L2 Spanish pronunciation accuracy via automatic speech recognition." *Phonica 21 (2025)*. <https://revistes.ub.edu/index.php/phonica/article/download/49237/44105/148972>. Retrieved 2026-11-08. **Tier:** 2.
[7] "Measuring linguistic bias in ASR: Whisper large-v3 on non-native speech versus human perception." *Procedia Computer Science (2026)*. <https://doi.org/10.1016/j.procs.2026.01.080>. Retrieved 2026-11-08 (abstract, via search excerpt; the DOI page returned an empty body to a later fetch). **Tier:** 2.
[8] "Language and Voice Support for Azure Speech." *Microsoft Learn*. <https://learn.microsoft.com/en-us/azure/ai-services/speech-service/language-support>. Retrieved 2026-11-08. **Tier:** 1.
[9] "Use pronunciation assessment." *Microsoft Learn*. <https://learn.microsoft.com/en-us/azure/ai-services/speech-service/how-to-pronunciation-assessment>. Retrieved 2026-11-08. **Tier:** 1.
[10] "SpeechSuper API & SDK Documentation." *SpeechSuper*. <https://docs.speechsuper.com/#/>. Retrieved 2026-11-08. **Tier:** 1.
[11] "Supported Languages." *Speechace API Documentation*. <https://api-docs.speechace.com/getting-started/supported-languages>. Retrieved 2026-11-08. **Tier:** 1.
[12] "Hebrew Words." *Rosetta Stone blog*. <https://blog.rosettastone.com/hebrew-words/>. Retrieved 2026-11-08. **Tier:** 3.
[13] "All About Foundations." *Rosetta Stone Support*. <https://support.rosettastone.com/all-about-foundations/>. Retrieved 2026-11-08. **Tier:** 3.
[14] "How We Built Baba Learn's Hebrew Pronunciation Engine." *ZipLyne (agency blog), 2026-10-15*. <https://ziplyne.agency/blog/how-we-built-baba-learns-hebrew-pronunciation-engine>. Retrieved 2026-11-08. **Tier:** 3.
[15] "Phonikud: Overcoming Phonetic Underspecification for Hebrew Text-To-Speech." *arXiv 2506.12311*. <https://arxiv.org/pdf/2506.12311>. Retrieved 2026-11-08. **Tier:** 2.
[16] Xu et al., "Simple and Effective Zero-shot Cross-lingual Phoneme Recognition." *arXiv 2109.11680 / Interspeech 2022*. <https://arxiv.org/abs/2109.11680>. Retrieved 2026-11-08. **Tier:** 2.
[17] "SpeechRecognition — Web APIs." *MDN Web Docs*. <https://developer.mozilla.org/en-US/docs/Web/API/SpeechRecognition>. Retrieved 2026-11-08. **Tier:** 1.
[18] "SpeechRecognition: install() static method." *MDN Web Docs*. <https://developer.mozilla.org/en-US/docs/Web/API/SpeechRecognition/install_static>. Retrieved 2026-11-08. **Tier:** 1.
[19] "SpeechRecognition API: processLocally." *Can I use*. <https://caniuse.com/mdn-api_speechrecognition_processlocally>. Retrieved 2026-11-08. **Tier:** 2.
[20] "On-device speech recognition explainer." *WebAudio/web-speech-api (GitHub)*. <https://github.com/WebAudio/web-speech-api/blob/main/explainers/on-device-speech-recognition.md>. Retrieved 2026-11-08 (the researcher's direct fetch returned an empty body and the content was read from a search excerpt; an independent re-fetch of the raw file succeeded). **Tier:** 1.
[21] `components/soda/soda_installer_impl_chromeos.cc`. *Chromium source (googlesource.com)*. <https://chromium.googlesource.com/chromium/src.git/+/refs/heads/main/components/soda/soda_installer_impl_chromeos.cc>. Retrieved 2026-11-08. **Tier:** 1.
[22] "Voice Driven Web Apps: Introduction to the Web Speech API." *Chrome for Developers*. <https://developer.chrome.com/blog/voice-driven-web-apps-introduction-to-the-web-speech-api>. Retrieved 2026-11-08 (undated article from the Chrome 25 era). **Tier:** 1.
[23] "ivrit-ai/zipformer2-66m-streaming-he." *Hugging Face (mirror of a model developed by Yakov Kolani)*. <https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he>. Retrieved 2026-11-08. **Tier:** 1.
[24] "ivrit-ai (ivrit.ai) — open Hebrew speech & language AI." *Hugging Face*. <https://huggingface.co/ivrit-ai>. Retrieved 2026-11-08. **Tier:** 1.
[25] "instush/ivrit-whisper-large-v3-turbo-timestamped-onnx." *Hugging Face*. <https://huggingface.co/instush/ivrit-whisper-large-v3-turbo-timestamped-onnx>. Retrieved 2026-11-08. **Tier:** 3.
[26] "itayinbar/Ozen-v1." *Hugging Face*. <https://huggingface.co/itayinbar/Ozen-v1>. Retrieved 2026-11-08. **Tier:** 3.
[27] "Running models on WebGPU." *Transformers.js documentation (Hugging Face)*. <https://huggingface.co/docs/transformers.js/guides/webgpu>. Retrieved 2026-11-08. **Tier:** 1.
[28] "The effectiveness of automatic speech recognition in ESL/EFL pronunciation: A meta-analysis." *ReCALL (Cambridge Core)*. <https://www.cambridge.org/core/journals/recall/article/effectiveness-of-automatic-speech-recognition-in-eslefl-pronunciation-a-metaanalysis/A915444CF252B61D14961D2FE733822D>. Retrieved 2026-11-08. **Tier:** 2.
[29] "Effects of F0 Feedback on the Learning of Chinese Tones by Native Speakers of English." *Interspeech 2005 (ISCA Archive)*. <https://www.isca-archive.org/interspeech_2005/zhang05_interspeech.pdf>. Retrieved 2026-11-08. **Tier:** 2.
[30] "Effect of interactive visual feedback on the improvement of English intonation of Japanese EFL learners." *CiteSeerX*. <http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.161.8536>. Retrieved 2026-11-08 (abstract). **Tier:** 2.
[31] "Web-based intonation training helps improve ESL and EFL Chinese students' oral speech." *Computer Assisted Language Learning 36(3) (Taylor & Francis)*. <https://www.tandfonline.com/doi/abs/10.1080/09588221.2021.1931342>. Retrieved 2026-11-08 (abstract). **Tier:** 2.
[32] "Computer-aided feedback on the pronunciation of Mandarin Chinese tones: using Praat to promote multimedia foreign language learning." *Computer Assisted Language Learning 37(3) (Taylor & Francis)*. <https://www.tandfonline.com/doi/full/10.1080/09588221.2022.2037652>. Retrieved 2026-11-08 (abstract). **Tier:** 2.
[33] "The acoustics of primary and secondary stress in Modern Hebrew." *Brill*. <https://brill.com/view/journals/aall/10/1/article-p5_2.xml>. Retrieved 2026-11-08 (abstract; the publisher page can return an error to automated fetches, the same abstract is at doi.org/10.1163/18776930-01001001). **Tier:** 2.
[34] "Word stress at utterance-final position." *Brill*. <https://doi.org/10.1163/18776930-01401002>. Retrieved 2026-11-08 (abstract). **Tier:** 2.
[35] "Stress: מלרע vs מלעיל." *Elon.io Hebrew Grammar*. <https://elon.io/grammar/hebrew/pronunciation/stress-milra-milel>. Retrieved 2026-11-08. **Tier:** 3.
[36] "Stress and Rhythm in Hebrew: Why You Sound Like a Beginner." *Heb4You Blog*. <https://heb4you.com/blog/posts/stress-and-rhythm-in-hebrew-why-you-sound-like-a-beginner.html>. Retrieved 2026-11-08. **Tier:** 3.
[37] "Influence of the native language on sensitivity to lexical stress." *Studies in Second Language Acquisition (Cambridge Core)*. <https://www.cambridge.org/core/journals/studies-in-second-language-acquisition/article/influence-of-the-native-language-on-sensitivity-to-lexical-stress/2A33EBFA90CF7BE19E355C2A3E1BC674>. Retrieved 2026-11-08. **Tier:** 2.
[38] "Data, privacy, and security for Speech to text." *Microsoft Learn*. <https://learn.microsoft.com/en-us/azure/foundry/responsible-ai/speech-service/speech-to-text/data-privacy-security>. Retrieved 2026-11-08. **Tier:** 1.
[39] "How to log audio and transcriptions for speech recognition." *Microsoft Learn*. <https://learn.microsoft.com/en-us/azure/ai-services/speech-service/logging-audio-transcription>. Retrieved 2026-11-08. **Tier:** 1.
[40] "Chirp 3 Transcription: Enhanced multilingual accuracy." *Google Cloud Documentation*. <https://docs.cloud.google.com/speech-to-text/docs/models/chirp-3>. Retrieved 2026-11-08. **Tier:** 1.
[41] "RecognitionSupport." *Android Developers API reference*. <https://developer.android.com/reference/android/speech/RecognitionSupport>. Retrieved 2026-11-08 (via search excerpt). **Tier:** 1.
[42] "SFSpeechRecognizer." *Apple Developer Documentation*. <https://developer.apple.com/documentation/speech/sfspeechrecognizer.md>. Retrieved 2026-11-08 (via search excerpt). **Tier:** 1.
[43] "Available languages in on-device speech recognition on iOS in 2022." *Medium (toru_furuya)*. <https://medium.com/@toru_furuya/available-languages-in-on-device-speech-recognition-on-ios-in-2022-8c6383fac9f2>. Retrieved 2026-11-08 (via search excerpt). **Tier:** 3.
[44] "models.yaml — Web Speech recognition on-device models." *Mozilla Firefox source (dom/media/webspeech/recognition)*. <https://raw.githubusercontent.com/mozilla-firefox/firefox/main/dom/media/webspeech/recognition/models.yaml>. Retrieved 2026-11-08. **Tier:** 1.
[45] "Firefox 160 beta release notes." *Mozilla*. <https://www.firefox.com/en-US/firefox/160.0beta/releasenotes/>. Retrieved 2026-11-08 (read by the verifier). **Tier:** 1.
[46] "Firefox 160 for developers." *MDN Web Docs*. <https://developer.mozilla.org/en-US/docs/Mozilla/Firefox/Releases/160>. Retrieved 2026-11-08 (read by the verifier). **Tier:** 1.
[47] "Speech recognition API in Microsoft Edge." *Microsoft Learn*. <https://learn.microsoft.com/en-us/microsoft-edge/web-platform/speech-recognition-api>. Retrieved 2026-11-08 (read by the verifier). **Tier:** 1.

---

## Appendix A — Access-Gate Result

```text
Access Gate — deep research
  Domains:  external only. repo_tree and application_data are out of scope (the app has a sign-in
            layer, so the single-maintainer file permission does not hold; the learner database was
            never in scope).
  Bindings: multi_user_authz for the external domain, from rung 1 (config §23
            research.access_gate.bindings); entitlement, scope and segment inapplicable for public pages.
  Verify:   resolved by the caller; the binding was confirmed by the operator on 2026-11-08.
  Checked:  Class A principal resolved from git identity (value not printed here or in any query) ·
            Classes B/C inapplicable for public pages · Class D inapplicable.
  Result:   NARROW — public web only
  Proceeding at: public web sources only
```

**Result:** `NARROW`. **Scope narrowing:** the repository tree, any database, learner records, production, and `docs/decisions/` were not read. Prior research reports were not opened (the index shows RSCH-002 is TTS and RSCH-001/003 are learning science, none on speech recognition or pronunciation feedback). **Sources dropped as not public:** the Wiley page for Mahdi & Al Khateeb (2019), a computer-assisted pronunciation training meta-analysis, returned 403 Forbidden to the independent verifier; it was cited in the first draft and was removed in remediation round 1, together with the effect size taken from it. No other source returned 401/403, a login redirect, or a paywall block to a full-text request; several journal pages (Taylor & Francis, Cambridge, Brill, Elsevier DOI) exposed only abstracts or errored on automated fetch, and findings from them are stated at abstract level. The Journal of Phonetics article on Israeli Hebrew stress showed only a title and reference fragments, so it is not cited. No person's name, email, learner id, or the app's hostname appeared in any query. **Shared research is not available; this session stays local to this project.**

## Appendix B — Scope & Research Type

**Research type selected:** `technical_analysis` — `findings_pattern`: `options_matrix`.

**Why this type:** a comparison of voice-input and feedback options against fixed constraints (config §23 `research.research_types`; named by the operator).

**Research Index duplicate check:** index read; RSCH-001..011 cover learning science, TTS audio (RSCH-002), course design, interface and practice layouts; none addresses speech recognition or pronunciation feedback, so proceeding independently.

**Subject confirmed:** Modern Hebrew (he / he-IL), adult English-speaking learner; no near-miss ambiguity (not Biblical Hebrew, not Yiddish).

**Sub-questions (written before searching):** SQ1 published Hebrew STT accuracy, native and non-native; SQ2 pronunciation-assessment products for Hebrew phonemes/stress/intonation; SQ3 on-device options (browser or phone) and privacy consequence; SQ4 feedback formats that studies find useful, apart from vendor marketing; SQ5 options matrix and spike. All five in scope.

**Coverage scores (research-qa §5):** SQ1a 0.8; SQ1b 0.3 (open); SQ2 0.9; SQ3 0.7 for browsers (Chrome, Firefox and Edge checked), phones open; SQ4 0.6 (minimal-pair/phoneme-highlight part open); SQ5 not scored (synthesis).

## Appendix C — Methodology

**Corpus doc used:** `.claude/skills/sdd-research/assets/research-corpus/technical-analysis.md` (with `research-qa.md` §1, §2, §4, §5, §6).

**Tools used:** `WebSearch` and `WebFetch` for public sources; a local scratch calculation (outside the repository) for the weighted scores and sensitivity; no repository reads beyond the skill files and the Research Index. Search-tool AI summaries were treated as leads only; every cited fact was checked against the retrieved page text.

**Iteration budget:** 27 web calls / ~15–20 (`research` Phase 3.2) — **exceeded**; see §2.

**Remediation:** verification rounds 1, 2 and 3 each returned FAIL (verifier: Claude Opus 5.5, a fresh sub-agent each time); fixes after each round were applied by the researcher agent itself rather than by a separate fix sub-agent. Three loops is the maximum, so the fixes made after round 3 were **not** re-verified by a fresh verifier (see §3).

**PDF render:** skipped — `pandoc` is not installed. Markdown report is the source of truth.
