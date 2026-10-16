# c — dense tool

Open `today.html` or `lesson-step.html` from the file system. No server and no build.

This option is a workbench: a top bar with more than one destination, a word index, tighter gaps, and a smaller (still legible) pointed word. Rubik is loaded with `@font-face` from `../../../shared/fonts/` (SIL OFL, `shared/fonts/OFL.txt`).

The lesson body is still an ADR-0003 `introduction`. The current word קָפֶה, the English, the placeholder, and השמעה stay in one cluster. `#miss` keeps לילה and shows בֹּקֶר. The index beside that cluster also lists שָׁלוֹם and בֹּקֶר, which is the density.

Scheme radios follow the system or force light or dark. `max-width: 599px` is the phone layout (360px). `min-width: 1280px` turns Today into two panels and the lesson into an index plus a column.

## RSCH-006 rules

| # | Rule | This mockup | Where to look |
| --- | --- | --- | --- |
| 1 | Pointed Hebrew at least 24px and larger than the English, in a nikud font | Follows | The hero word is 32–36px. The English gloss is 16px. Rubik draws the points. |
| 2 | The pointed word is live text | Follows | Index and hero are HTML text. |
| 3 | Default line height on a pointed line is 1.5 | Follows | `--line-height-word: 1.5`. Body text is slightly tighter (1.45). |
| 4 | Points stay visible at line height 1.5 and at the 1.4.12 spacing | Follows | The hero word is not in a fixed-height box. |
| 5 | Text contrast at least 4.5:1 | Follows | Measured in both schemes by `node ../check.js`. |
| 6 | Controls and meaningful picture parts at least 3:1 | Follows | Navy (or pale blue) actions, and the corner-mark picture stroke. |
| 7 | Pointer targets at least 24px | Follows | Bar links, index links, and radios. |
| 8 | Play, check, and next at least 44px | Follows | השמעה, אפשר להמשיך, and השיעור use the primary size. |
| 9 | Word, meaning, and picture in one cluster; audio while the word is visible | Follows | `.cluster` still groups them, including on the wide row. |
| 10 | One item, then the learner advances | Trades off | The index shows שָׁלוֹם, קָפֶה, and בֹּקֶר at the same time as the coffee card. |
| 11 | No decoration beyond the word, the meaning, the audio, and the picture | Trades off | The bar, the panel borders, the row labels ("מילה", "משמעות", "צורה נכונה"), and the example sentence are extra chrome. |
| 12 | A miss says more than "wrong" and shows the correct form | Follows | `#miss` states the miss and shows בֹּקֶר. |
| 13 | One retry, then the answer | Follows | The sentence says the extra try was used and the form stays. |
| 14 | Progress is words due, not points, a streak, or a badge | Follows | The due panel says "4 מילים שמחכות לחזרה היום." The 4 is a word count. |
| 15 | `dir="rtl"` and `lang="he"`; each English gloss is `dir="ltr"`; direction is not set in CSS | Follows | On `<html>` and on each gloss. The index sits on the start side. |
| 16 | The home screen has one next action | Trades off | The bar offers היום, השיעור, and חזרה as three top-level destinations. |
| 17 | The main phone action sits in the lower reachable area | Trades off | השיעור is in the top bar, not along the bottom edge. |
| 18 | Nothing moves on its own; a tap's motion honors reduced motion | Follows | No looping motion. The reduce query clears animation and transition. |

## Trade-offs a reader can see

Rule 10 and rule 16. On the lesson, three pointed words are listed at once. On Today, the bar is three links, not one action.

Rule 17. The filled השיעור control is the first row on a phone, where a thumb does not rest.

Rule 11. Labels, rules, and panels are the tool chrome around the cluster.
