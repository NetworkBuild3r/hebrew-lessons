# SPEC-030 · frontend: the Tutor section

## Metadata

```yaml
spec_id: SPEC-030
initiative_id: INIT-039
title: "The Tutor section"
domain: frontend
status: complete
primary_prompt: none
complexity: medium
complexity_reason: one new page over the existing class-sheet tables, one study route, and the section link
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-024]
blocks: []
```

## Description

Brian asked on 2026-11-12 for a tutor section that holds what his lesson with his tutor was, so it can feed the same study he does from the course. The class-sheet tables and the tool that loads them already exist (`import_class_sheet`, `db.ClassSheets`, `db.SaveClassSheet`), and the local database has no sheets yet. The four class-sheet units on `/lessons` move here in meaning: this page shows the sheets themselves, not the old units.

A sheet is imported by an outside AI through MCP, never by this page. The page is read-only plus one button that turns a class into a lesson.

## What to build

### `GET /tutor`

New file `internal/web/tutor.go`, new template `internal/web/views/tutor.templ`.

- Read `store.ClassSheets(ctx, userID)`, newest class first.
- For each sheet, split its lines into words with `units.Tokenize` and mark each distinct token:
  - **in the course** when a ready course row's `Lemma` key matches (`speech.LemmaKey`);
  - **new** when the token is not in the course but the sheet's own gloss is present for that line;
  - **not on the list** otherwise.
- The page shows:
  - the title "Your tutor's classes";
  - one card per sheet: the class date, the title, "N lines", and the first three tutor lines;
  - a count line: "{in course} words you already have, {new} new from this class";
  - a form that POSTs to `/_hx/tutor/{id}/study` with `_csrf` and the button "Study this class";
  - the button is disabled-looking and absent when the sheet has no new words;
  - when there are no sheets: one plain paragraph, "No class loaded yet", and a second paragraph telling him what to hand his AI: "Ask your AI to call import_class_sheet with the exported lines from class, one line each as tutor: Hebrew || English." That is the only instruction; do not invent a class.
- A held line is listed but marked "held", and is never studied.

### `POST /_hx/tutor/{id}/study`

- Needs the session and the CSRF token, as `postCourseReview` does.
- Reads the sheet by id through `store.ClassSheets` filtered to the learner, so another learner's sheet is 404.
- Builds the study set:
  - a token already in the course maps to its stored lexeme id;
  - a token not in the course, but with a gloss on its line, is created with `lexicon.Prepare` and `UpsertLexemes` only when no stored row has that consonant spelling. Two stored rows with one spelling means that token is skipped, and a line that loses a word is dropped from the set. This is the same rule the course sync uses.
  - the word is then studied with `s.study.BuildLesson`.
- `replaceQueue`, then redirect to `/review`.
- If nothing is studyable, redirect back to `/tutor` with `?nothing=1`, and the page shows "Nothing new to study from that class."

### The section link

Add `{Href: ui.Href("/tutor"), Label: "Tutor", Icon: "book", ID: "tutor"}` to `sectionLinks` in `internal/web/shell.go`, always, because the page has something to say even when empty.

## Tests

New `internal/web/tutor_test.go`:

- `TestTutorEmpty`. No sheets: the page shows "No class loaded yet", names `import_class_sheet`, and renders no study form.
- `TestTutorListsAClass`. Save one sheet with three tutor lines, two of whose words are in the course: the page shows the date, the line count, and "2 words you already have".
- `TestTutorStudyCreatesMissingWords`. Posting to study a class whose lines hold words not in the lexicon creates them once, and a second study of the same class creates none.
- `TestTutorStudySkipsAmbiguousSpelling`. Two stored lexemes share a spelling for a class token: that token is skipped, and no third row is created.
- `TestTutorStudyNeedsCSRF`.
- `TestTutorOtherLearnerSheetIs404`.
- `TestTutorIsAlwaysInTheSections`. The shell shows the Tutor link with no sheets.

## Acceptance criteria

- [ ] `/tutor` lists the learner's classes, marks each word as in the course, new, or not on the list, and says plainly when nothing is loaded.
- [ ] One class can be studied, and its new words enter the lexicon once, behind CSRF.
- [ ] An ambiguous spelling is skipped rather than guessed.
- [ ] Tests pass and the layout audit reports no new issues on `/tutor`.

## Verification Strategy

- **Claim:** a tutor class becomes studyable, and the page never invents a class.
- **Check + executor:** mechanized. The tests above, plus the layout audit on the local server.
- **Pass condition:** all pass.
