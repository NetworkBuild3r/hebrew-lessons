// Package speech holds the list of Hebrew lemmas that cover most of the
// words in a large subtitle corpus, and the one lesson built from the top
// of that list.
//
// The list is Juan D. Pinto's Frequency Dictionary of Spoken Hebrew (2018).
// It ranks 5,000 lemmas from OpenSubtitles2018, film and television
// subtitles used because a large corpus of recorded Hebrew conversation
// was never finished. The Corpus of Spoken Israeli Hebrew is real speech
// and does not publish a ranked list of this size. Subtitles are not
// conversation. They are the published stand-in.
//
// Coverage is the share of subtitle tokens, from the list's own
// per-million counts. The first 30 lemmas cover about 48 percent. All
// 5,000 cover about 91 percent, which is short of the 95 percent at which
// a line becomes one a learner can follow. The list is lemmas. A form such as יודע
// sits far down the list because the parser folded it into ידע. אני is
// on the list and ranked near the end, which is a limit of this file,
// not a claim that "I" is rare in speech.
//
// course.tsv is the teachable form of that list. Pointed spellings and
// English come from English Wiktionary through kaikki.org, and
// course_fixes.tsv records every split, prefix, correction, and drop.
// A row is ready only after it passes CourseCheck and a second reviewer
// has signed it. The Wiktionary terms are in LICENSE-wiktionary.txt.
package speech
