# INIT-039 diagrams

How a class becomes a lesson. Nothing here is a production deploy.

## Class load and the queue

```mermaid
flowchart LR
  sheet[Dated class sheet] --> check[Per-sheet check]
  check -->|notes, zip, html, no date| heldOut[Stays out]
  check -->|tutor lines exist| store[class_sheets and class_lines]
  store --> match[Match token, then one prefix]
  match -->|two hits| miss[True miss]
  match -->|one hit and entry checks pass| t0[T0 queue]
```

## Assemble, freeze, and the story

Kept in the code. Today does not call it (ADR-0034 D1).

```mermaid
flowchart TD
  pool[Ranked T0 words] --> word[Word lesson: 2 to 4 words]
  pool --> easy[Easy lesson: no new words]
  word --> pre[Pre-flight]
  easy --> pre
  pre -->|no graded key| none[Review only]
  pre -->|passes| today[Today offer]
  story[put_story] -->|fails| drop[Drop the story step]
  story -->|passes| today
```

## Today

ADR-0034 D1 and D5. The class-sheet assembler stays in the code and is not what Today offers.

```mermaid
flowchart TD
  due{Review due?}
  due -->|yes| review[Review sitting, about 20 minutes]
  due -->|no, course words waiting| sitting[Sitting that adds the next common words]
  due -->|nothing waiting| quiet[Today says there is nothing due]
```

## Five sections

ADR-0035. One player, the review player.

```mermaid
flowchart LR
  lessons[Lessons: five common words] --> review[Review]
  tutor[Tutor: a class sheet] --> review
  review --> progress[Progress: the mix]
  learn[Learn: alphabet and vowels]
```

## Flag

```mermaid
flowchart LR
  miss[My answer was also right] --> take[Void and flag]
  right[This question is wrong] --> take
  take --> skip[Build skips that word]
  undo[Undo] --> back[The answer counts and the word can be asked again]
```

## Retirement

The old player, the old validator, and units 1 to 34 stay. A later initiative archives units 12 to 34 after the new lessons have been on Today. Nothing in this picture deletes a learning record.
