# b — warm illustrated

Open `today.html` or `lesson-step.html` from the file system. No server and no build.

This option uses a warm paper colour, large rounded corners, and drawn placeholders (a sun, a hill, a filled cup). The drawings are marked מציין מקום. They are not photographs. Rubik is loaded with `@font-face` from `../../../shared/fonts/` (SIL OFL, `shared/fonts/OFL.txt`).

The lesson page is the same ADR-0003 `introduction` as the other looks: קָפֶה with its English gloss, the picture, and השמעה in one cluster, then `#miss` for בֹּקֶר. The attempt לילה stays. Today still has one learning action and a count of words due.

Scheme radios follow the system or force light or dark. The phone query is `max-width: 599px` (a 360px window). The desktop query is `min-width: 1280px`.

## RSCH-006 rules

| # | Rule | This mockup | Where to look |
| --- | --- | --- | --- |
| 1 | Pointed Hebrew at least 24px and larger than the English, in a nikud font | Follows | The hero word is 40–52px in Rubik. |
| 2 | The pointed word is live text | Follows | HTML text, including the vowel marks. |
| 3 | Default line height on a pointed line is 1.5 | Follows | `--line-height-word: 1.5` |
| 4 | Points stay visible at line height 1.5 and at the 1.4.12 spacing | Follows | Block padding and `overflow: visible` on the word. |
| 5 | Text contrast at least 4.5:1 | Follows | Both schemes, measured by `node ../check.js`. |
| 6 | Controls and meaningful picture parts at least 3:1 | Follows | The terracotta action and the cup stroke are in the UI pairs. |
| 7 | Pointer targets at least 24px | Follows | `--size-target` on radios and the back link. |
| 8 | Play, check, and next at least 44px | Follows | Primary controls use 48px. |
| 9 | Word, meaning, and picture in one cluster; audio while the word is visible | Follows | `.cluster` on `#meet`, with השמעה inside it. |
| 10 | One item, then the learner advances | Follows | One word on `#meet`. The advance link goes to `#miss`. |
| 11 | No decoration beyond the word, the meaning, the audio, and the picture | Trades off | Today opens with a sun-and-hill banner. The cup is a filled drawing inside a very round frame. The example sentence is also still there. |
| 12 | A miss says more than "wrong" and shows the correct form | Follows | `#miss` explains and shows בֹּקֶר. |
| 13 | One retry, then the answer | Follows | The note says the extra try was used. |
| 14 | Progress is words due, not points, a streak, or a badge | Follows | "4 מילים שמחכות לחזרה היום." |
| 15 | `dir="rtl"` and `lang="he"`; each English gloss is `dir="ltr"`; direction is not set in CSS | Follows | On `<html>` and on each gloss. |
| 16 | The home screen has one next action | Follows | One button, להתחיל את השיעור, inside the card. |
| 17 | The main phone action sits in the lower reachable area | Trades off | That button sits in the card, under the banner, not in a bottom dock. On a phone it lands in the middle of the screen. |
| 18 | Nothing moves on its own; a tap's motion honors reduced motion | Follows | Same reduced-motion split as the other looks. The banner does not animate. |

## Trade-offs a reader can see

Rule 11. The top of Today is a drawing: a circle, a hill, and a small cup. None of that is the word, the gloss, the audio, or the lesson picture. The lesson picture itself is a filled placeholder with steam, not a plain frame.

Rule 17. להתחיל את השיעור is centered in the card. It does not stick to the bottom edge the way it does in the calm look.
