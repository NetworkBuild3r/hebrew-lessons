# Illustration style: per-word pictures

**Status:** draft for operator review Â· **Provenance:** `INIT-005/SPEC-006` Â· **Requirement:** REQ-011

One page. It makes every AI-generated word picture look like it belongs to the same family. It applies to pictures made through INIT-004 SPEC-009 (`server/media/openai-images.js`, model `gpt-image-1`, 1024 x 1024, low quality).

## The style in one line

Flat, friendly line pictures of one plain object or action: thick rounded dark-slate outline, solid flat fills in the brand blues with one small amber accent, on a warm paper background, nothing else in the frame.

## Rules

| Topic | Rule |
|-------|------|
| **Palette** | Outline and details: ink `#1e293b`. Main fills: brand blue `#2563eb` / `#1d4ed8`, indigo `#4338ca`, and soft tints of them (`#eff6ff`, `#e0e7ff`). One small accent in sun amber `#fbbf24` (at most about 10 % of the subject). Green `#059669` or rose `#e11d48` only when the word itself is that colour or thing. |
| **Background** | Plain warm paper `#faf8f4`, flat, edge to edge. No gradient, no vignette, no ground shadow, no scene. The picture must sit on the lesson card without a visible box. |
| **Line** | One uniform, thick, rounded outline, about 3 % of the image width (roughly 30 px at 1024). Round caps and joins. No thin hatching and no sketchy double lines. |
| **Fill** | Solid flat colour. No gradients, no texture, no photo realism, no glow, no 3D rendering. |
| **Composition** | One subject. Centred. The subject fills about 60 % of the frame with at least 15 % clear margin on every side so it survives a round or square crop. Simple three-quarter or side view. |
| **Detail level** | It must read at 160 px wide. If a detail disappears at that size, leave it out. |
| **Text** | **None.** No letters, Hebrew or Latin, no numbers, no signs, no labels, no logos, no watermark. The word is shown by the lesson, never by the picture. |
| **People** | **No human faces and no realistic people.** For an action, show the object and, if a hand is needed, a plain hand and forearm with no face. No crowds. No children. This also keeps every prompt inside the ADR-0004 refusals (no minor, no likeness). |
| **Animals** | Allowed for animal words. Natural, side view, simple face with no human expression. No cartoon mascot, no clothes, no speech. |
| **Tone** | Neutral and calm. No jokes, no puns, no characters with personality (operator rule, REQ-015). No religious or national symbols unless the word is exactly that thing. |
| **Words that are not things** | Abstract words (justice, because) and grammar words are not pictured. The lesson uses text and audio only. Do not generate a decorative stand-in. |
| **Alt text** | Every stored picture gets short alt text in the page language, saying what is shown ("×›×•×¡ ×ž×™×"), not "illustration of". The picture never carries the only meaning. |
| **Dark mode** | The picture keeps its own paper background and sits on a paper-coloured tile (`--color-picture-bg`). Do not re-colour it per theme. |

## The prompt sentence

Append this exact sentence to every image-generation prompt. The subject goes first, in plain words, and the style sentence goes last:

> Flat vector-style illustration of a single subject, centered with generous empty margin, drawn with one uniform thick rounded dark slate (#1e293b) outline and solid flat fills in indigo (#4338ca) and blue (#2563eb) with one small amber (#fbbf24) accent, on a plain warm off-white (#faf8f4) background, with no gradients, no shadows, no texture, no text, no letters, no numbers, no logos, and no human faces.

Example final prompt for the word "×›×•×¡":

```
A drinking glass of water. Flat vector-style illustration of a single subject, centered with generous empty margin, drawn with one uniform thick rounded dark slate (#1e293b) outline and solid flat fills in indigo (#4338ca) and blue (#2563eb) with one small amber (#fbbf24) accent, on a plain warm off-white (#faf8f4) background, with no gradients, no shadows, no texture, no text, no letters, no numbers, no logos, and no human faces.
```

The sentence is about 400 characters. `MAX_PROMPT_LENGTH` is 4000 and the operator's subject text sits well under it, so length is not a problem.

The subject part stays "one ordinary object or action", as INIT-004 SPEC-009 already requires. Learner free text is never the subject.

## Where to append the sentence

**Done in `INIT-005/SPEC-010`:** `buildGenerationBody` in `server/media/openai-images.js` appends the sentence (exported as `IMAGE_STYLE_SENTENCE`) after the operator's subject. `test/openai-images.test.js` checks that the constant matches this page word for word. The notes below are the analysis that led to that choice.

Looked at before the change:

- **Entry points (all pass the prompt through unchanged today):**
  - Admin plan builder: `shared/admin/plan-builder.js` sends `state.estimatePrompt` (the dry-run prompt) in `generateImage()` (about lines 1064 and 1095).
  - HTTP: `server/api/media-routes.js`, `images.planImage(input, ...)` (about line 661) then `images.generateImage(input, ...)` (about line 695).
  - MCP: `mcp/tools.js`, the dry-run tool (about line 1166) and the generate tool (about line 1184), which call the same two functions.
- **One choke point:** `server/media/openai-images.js`.
  - `readPrompt(input, apiKey)` (about line 198) validates the operator's subject text: length, control characters, key leak, and the ADR-0004 refusal list. It returns `{ prompt }`.
  - `buildGenerationBody(prompt)` (about line 142) builds the outbound request `{ model: 'gpt-image-1', prompt, n: 1, size, quality }`.

**Recommendation:** append the sentence inside `buildGenerationBody` (or in `outboundRequest`, about line 382), as `prompt + ' ' + STYLE_SENTENCE`, kept as one exported constant. Reasons:

1. Every route (admin UI, HTTP, MCP) gets the style with one edit and cannot forget it.
2. It runs **after** `readPrompt`, so the refusal check still sees only the operator's subject, and the fixed sentence ("no human faces") cannot trip the word list or the length limit.
3. The dry-run and confirm still compare the same operator prompt (`state.estimatePrompt` equals the prompt sent to `generateImage`), so the estimate-match rule in SPEC-009 is unaffected.
4. The stored media row's `promptText` stays the subject only (it is `fields.prompt`, not the outbound body). If the operator wants the full text stored for audit, store the constant's version id (for example `style: 'illustration-v1'`) next to it rather than repeating 400 characters per row.

The existing test `test/openai-images.test.js` asserts the outbound body. It will need one added case that the body prompt ends with the sentence. That change belongs to the spec that edits the code, not to SPEC-006.

## Checking a picture

Accept a generated picture only if all of these are true. If not, regenerate or reject it.

1. One subject, centred, with visible margin.
2. Background is flat paper, no scene.
3. Outline is thick and uniform; fills are flat.
4. Only ink, blue, indigo and at most a small amber accent (and a word-specific colour where needed).
5. No text of any kind, and no human face.
6. Still clear at 160 px.
7. Alt text written.
