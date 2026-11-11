# Lesson types

**Status:** Design, 2026-11-10. The player, the brief, and the author skill still follow the older shape. This file is the target. It is built in Stage 2 ([BRAIN-001 spec skeleton §5](../../sdd/brainstorms/BRAIN-001-is-the-hebrew-app-built-right/spec-skeleton.md)), after the Stage 1 gate.
**Research:** [RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md), [RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md), [RSCH-005](../../sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md), [RSCH-001](../../sdd/research/RSCH-001-english-to-hebrew-learning/RSCH-001-english-to-hebrew-learning-report.md).

A lesson teaches. Review, on a later day, is what decides a word was remembered. Flashcards stay in My stuff. Spoken conversation stays with the tutor: the app cannot score speech, and the research that supports speaking practice needs feedback on what was said (RSCH-003 §1.3, Moderate for pronunciation with feedback; the interaction evidence is Low).

Five lesson types. Most sittings are a word lesson. The brief picks another type when a pattern is due, listening is the weak skill, too many words are still in learning, or known words are ready to be spelled from memory.

| Type | What is new | What the learner does |
| --- | --- | --- |
| Word | About 4 dictionary words (up to 8 past the beginner tier) | Sees them, matches the meaning, reads them in a story, then writes them |
| Conjugation | One set of forms, on verbs they already know | Sees the forms, practices them, reads them in sentences |
| Listening | Nothing new | Hears a passage and answers from the audio |
| Easy | Nothing new | Reads a longer passage of words they are learning, then answers |
| Spelling | Nothing new | Types words they already understand, spelling hidden |

## Who builds a lesson

The app builds every lesson. The words come from the tutor's classes, and the cards come from the lexicon. Graded answers are checked against the lexicon, the stored verb forms, or the tutor's own sentences. The AI writes one thing per lesson, the story. The story is checked for pointing, lexicon words, coverage, and length, and it is never graded. An outside AI on the MCP fills that story slot and nothing else, so it cannot change a lesson's shape.

## How many new words

RSCH-004 §1.2 found no tested count per lesson. Classroom courses average about four words per contact hour. An hour of pure card study has been reported near 35, from older work that was not read. A lesson that also has a story is the classroom kind of hour, so a word lesson introduces **4** new dictionary words. The eight-word cap already in the app is a ceiling, not a target. Under 100 known words the passage math already forces 3 or 4 (North Star §7.3). Past that tier a word lesson may go up to 8. It does not open at 35.

A conjugated form is not a new dictionary word. A word lesson teaches one form of a new verb. The other forms wait for a conjugation lesson.

## Word lesson

The learner hears the word, sees it, shows they know the meaning, meets it in a story, and then writes it.

1. **The grid.** All of this lesson's new words are on one screen, before any story. Each card shows the pointed form this lesson will use, a Listen button, the part of speech, and the English meaning. A verb also shows its binyan. One example sentence sits on the card. The other forms of a verb are on the card and start closed. Nothing on this screen is graded.
2. **The meaning check.** The meaning is hidden. They hear the word or read it, and pick the English. One right answer is enough to go on. A miss shows the meaning and asks once more. This check shows they can match the word right now. It does not mark the word known. A lesson `listen_pick` whose buttons are other Hebrew lines does not count: that can be passed without knowing the meaning.
3. **The story.** The new words appear in sentences where about 90–95% of the other words are already known (RSCH-004, Low for the coverage figure; staged for beginners in North Star §7.3). Tapping a word shows its meaning and plays it. They can hear the sentence and repeat it. The app does not score the repetition.
4. **Write it.** The Hebrew stays hidden. They type the word from the English meaning. RSCH-005 §1.3 (Moderate) says producing the spelling after it has been hidden is the spelling task, and that copying it while it is still on screen is a poor one.
5. **Use it in a sentence.** Fill a gap, put the words in order, answer a question while the passage stays on screen, or write a sentence. This step waits until they can produce the word. Writing a brand-new word into a sentence at the first meeting left a weaker memory of the spelling than studying the word (RSCH-005 §1.3, Moderate, Spanish words). Once they can spell it, the sentence is how they use it.

Hearing, saying, and writing all happen in this lesson. Saying is repeating the model. It is not a conversation, and it is not graded.

## Conjugation lesson

No new dictionary words. One new pattern, which is the rule the brief already follows (North Star §7.1, ADR-0026). The verbs are ones they have already met in a word lesson.

1. **The forms.** The screen opens the forms for that pattern, for example past tense. Each form has Listen and a short English line ("I wrote", "she wrote"). The binyan is named.
2. **Practice.** They pick the form that fits the person, or fill it into a sentence they can otherwise read.
3. **Sentences.** A short passage uses those forms and no other new language.

A new verb still arrives in a word lesson as one form. This lesson is where its other forms are taught, after the word itself is no longer new.

## Listening lesson

No new words. A short passage they hear. The questions are about what was heard, and the Hebrew text stays off until the answer. Use it when listening is the weak skill, on words a word lesson already taught. RSCH-004 treats listening as its own strand at a low grade, and only for short informal passages.

## Easy lesson

No new words and no new pattern. A longer passage made of words they are already learning, then the same kinds of questions as a word lesson. The brief already asks for this when too many words are still in learning or recent accuracy is low. Nation's fluency pass is defined as familiar language only (RSCH-004 §1.4).

## Spelling lesson

No new words. It waits until the letters check is passed, and until they already understand the words. The Hebrew stays hidden. They get the English meaning, or the audio, and type one word. The result marks a wrong letter and a missing letter. A fast answer and a slow one may be graded differently inside review. Speed is not the score of this lesson, and it is not shown as a typing test. RSCH-005 §1.4 found that keyboard practice has no shown effect on learning words.

A sentence of words they can already spell may follow the single-word typing. The sentence is not the activity that teaches the spelling.

## What stays outside a lesson

| Surface | Job |
| --- | --- |
| Review | A later day. Checked test: type it, dictation, listen and pick, read and pick, match. One ask per word per sitting. This is the retrieval the spacing research is about (RSCH-003 §1.1). |
| Flashcards | Optional, in My stuff. Forgot, Hard, Got it, Easy. |
| Tutor | Spoken conversation and correction of speech. |

Review, dictation, match boards, and flashcards are not lesson types. Fill-the-gap, word order, and a question on the passage are steps inside a word lesson or a conjugation lesson.
