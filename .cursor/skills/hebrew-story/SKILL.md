---
name: hebrew-story
description: Write one optional Hebrew story for a lesson that code already built. Use when a class lesson needs a story. Do not write questions, keys, or a whole unit.
---

# Hebrew story

Code builds the lesson. You may write one story, and only a story.

1. Call `get_story_request` for the learner. Read the known words, the coverage target, and the length.
2. Write one pointed Hebrew story that uses only those words, plus any due word the request names.
3. Call `put_story` with the text, the known words, and the due words. Read every failure in that one reply.
4. If it fails, rewrite against that list and call `put_story` again. Stop when it still fails. Do not save the story any other way.
5. Do not call `put_unit`. Do not grade the story. Do not add a step for a word the passage left out. Code adds that step.

The author skill and the reviser skill stay paused. They are not how a story is saved.
