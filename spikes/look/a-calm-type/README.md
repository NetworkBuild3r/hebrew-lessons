# a — calm type

Open `today.html` or `lesson-step.html` from the file system. No server and no build.

This is the quiet option: a narrow column, a lot of empty paper, one filled action, and the pointed word set much larger than the English. Rubik is loaded with `@font-face` from `../../../shared/fonts/` (SIL OFL, `shared/fonts/OFL.txt`).

The lesson page is an ADR-0003 `introduction` (meet the words): קָפֶה with its English gloss, a picture placeholder, and an audio control in one cluster, then an example sentence. `#miss` is the recall state after one retry: the attempt לילה stays on screen, the note says more than "wrong", and the correct form בֹּקֶר is shown. There is no picture on the miss, which matches the picture limit in ADR-0003. Today names the next lesson (שָׁלוֹם, קָפֶה, בֹּקֶר) and how many words are due.

The scheme radios follow the system, or force light or dark, with `color-scheme` and `light-dark()`. A 360px-wide window uses the phone query (the start action sticks to the bottom). A 1280px-wide window uses the wider measure and a larger word.

## RSCH-006 rules

| # | Rule | This mockup | Where to look |
| --- | --- | --- | --- |
| 1 | Pointed Hebrew at least 24px and larger than the English, in a nikud font | Follows | קָפֶה and בֹּקֶר use `--size-word` (40–56px). Rubik is the face. |
| 2 | The pointed word is live text | Follows | The words are HTML text, not images. |
| 3 | Default line height on a pointed line is 1.5 | Follows | `--line-height-word: 1.5` |
| 4 | Points stay visible at line height 1.5 and at the 1.4.12 spacing | Follows | The word has block padding and `overflow: visible`. Letter spacing is not tightened. |
| 5 | Text contrast at least 4.5:1 | Follows | `node ../check.js` measures both schemes. |
| 6 | Controls and meaningful picture parts at least 3:1 | Follows | The filled action and the cup stroke use the UI pairs in `look.css`. |
| 7 | Pointer targets at least 24px | Follows | Radios, the back link, and labels use `--size-target`. |
| 8 | Play, check, and next at least 44px | Follows | השמעה, אפשר להמשיך, and להתחיל use `--size-target-primary`. |
| 9 | Word, meaning, and picture in one cluster; audio while the word is visible | Follows | `.cluster` on `#meet`. The button does not leave the word. |
| 10 | One item, then the learner advances | Follows | `#meet` shows קָפֶה only. אפשר להמשיך moves to `#miss`. |
| 11 | No decoration beyond the word, the meaning, the audio, and the picture | Trades off | The example אֲנִי שׁוֹתֶה קָפֶה sits under the cluster because an introduction in ADR-0003 asks the learner to read an example. There is no other ornament. |
| 12 | A miss says more than "wrong" and shows the correct form | Follows | `#miss` keeps לילה and shows בֹּקֶר with a sentence. |
| 13 | One retry, then the answer | Follows | The sentence says the extra try was used and the form stays. |
| 14 | Progress is words due, not points, a streak, or a badge | Follows | Today says "4 מילים שמחכות לחזרה היום." |
| 15 | `dir="rtl"` and `lang="he"`; each English gloss is `dir="ltr"`; direction is not set in CSS | Follows | On `<html>` and on each gloss. |
| 16 | The home screen has one next action | Follows | The only learning destination is להתחיל את השיעור. |
| 17 | The main phone action sits in the lower reachable area | Follows | On a narrow window the start link sticks to the bottom of Today. |
| 18 | Nothing moves on its own; a tap's motion honors reduced motion | Follows | The only transition is inside `prefers-reduced-motion: no-preference`. The reduce query turns animation and transition off. |

## Trade-off a reader can see

Rule 11. Under the coffee cluster, the line אֲנִי שׁוֹתֶה קָפֶה is an example sentence, not the picture, the audio, or the gloss. It is there so this screen matches the introduction block. The rest of the page stays empty on purpose.
