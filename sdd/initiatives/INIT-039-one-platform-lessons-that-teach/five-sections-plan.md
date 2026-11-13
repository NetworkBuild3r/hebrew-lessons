# Here is what I plan to do

## What I'll build

Brian asked on 2026-11-12 for five sections that each do one job: Lessons teaches the most common words first and reaches rarer words as he goes, Tutor holds what his tutor taught, Review brings back words he missed from either, Progress mixes the whole picture, and Learn widens out to vowels, the alphabet, and the background he needs for Isaiah.

**A plan of lessons from the word list.** The 475 signed common words become numbered lessons of five words each, in order of how often the words are used. Lessons 1 to 5 are the most used words in Hebrew, then the next five. The Lessons page lists them all: finished, next, later. Opening one shows its five words, pointed and English, with how often each appears. A button studies just those five in the player that already exists. Finishing puts them into review, which brings each back on later days.

**A tutor section.** The class sheets the tutor tool already loads become the second source. The Tutor page shows each class, its lines, and which of its words are already in the course, which are new, and which are not on the list at all. A button turns that class's words into a lesson, so the tutor's words enter review the same way the course words do. With no class loaded the page says how to load one and what to copy, and says plainly that it is empty.

**Review mixing both.** Review stays one sitting of about 20 minutes. It now names where each card came from in one small line, so a tutor word and a course word are told apart, and it keeps the new-word allowance that already follows his recent accuracy. Missed words come back from either source, and struggle words and letter drills stay as they are.

**A Learn section with the alphabet and vowels.** A page for the 22 letters, their names, sounds, and final forms, built from the letter table already in the app, and a page for the vowels with the dot-and-line marks and what each one sounds like. The five notes already written stay as guides. Isaiah appears here once its verses are loaded.

**Progress mixing everything.** One screen that says what he can now do, then three short lines: his place in the common words, what his tutor classes added, and how many words came back from review right. Detail stays behind links. No charts, percents, streaks, or points on that screen.

## What I'll reuse

The 475 signed words in `internal/speech/course.tsv` and the course list already in the database. The review player, its question kinds, its grading, its new-word tiers, and its struggle words, which already study any set of words the app names. The band-review button, which is the pattern for "study these five". The class-sheet tables and the tool that loads them. The letter table in `internal/web/views/alefbet.go`, with all 27 forms and their tell-apart notes. The five starter notes already on Learn. The course page, the weekly report's course block, and the Progress summary sentence.

## How I'll check it works

The Lessons list shows 95 lessons of five words in rank order, and the first lesson holds the seven pronouns. Opening a lesson shows its five words with their frequency, and studying it adds exactly those words to review. A lesson whose words are all learned shows as done. The Tutor page with no class loaded says so and shows no fake lines; with one loaded it lists the class's words and which are new. Review shows the source line on each card, and a sitting with a tutor word and a course word shows both. Review still cannot exceed its 20-minute card count. Learn shows all 22 letters, the five final forms, and each vowel mark with a name and a sound. Progress shows the three lines and no percent on the page. Each check is a local test on a temporary database or a local page.

## What I won't do

I will not open a separate project. I will not delete the old class-sheet units, the learning record, or the letters. I will not put a second lesson player in the app; the lessons use the player review already has. I will not build a tutor section that pretends to hold a class before one is loaded, and I will not invent tutor lines. I will not grade speech. I will not add streaks, points, or leaderboards, and I will not put charts or percents on Progress. I will not build a conjugation section or a listening lesson, because both still wait on other work. I will not promise the whole 5,000 words soon: at five a day, the first 1,000 take about 200 study days and cover about 80 percent of subtitle words.

## What you'll have at the end

Five sections that each do one job. Lessons walks the most common words outward, five at a time, and you can see how far you have come. Tutor holds what your tutor taught and turns it into the same kind of lesson. Review brings back what you missed and says where each word came from. Learn gives you the alphabet, the vowels, and the guides behind Isaiah. Progress says what you can do and how the pieces fit, without burying you in numbers.

---

Shall I go ahead and build this?

1. Yes, build it
2. Change something first
3. Not now
