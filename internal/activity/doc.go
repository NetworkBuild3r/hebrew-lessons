// Package activity is the checked activities of the v3 learner and the engine that builds and
// grades them. It implements INIT-015/SPEC-015 and docs/north-star.md section 6.
//
// # Activities
//
//	kind         skill   shows                          answered by             wrong pick / typo is logged as
//	listen_pick  listen  audio only                     one of four glosses     a confusion with the word picked
//	read_pick    read    pointed Hebrew, no audio       one of four glosses     a confusion with the word picked
//	word_match   read    five Hebrew cues, five glosses pairing each cue        a confusion per wrong pair
//	audio_match  listen  four clips, four glosses       pairing each clip       a confusion per wrong pair
//	type_it      recall  the English gloss              typed Hebrew            letter pairs (and a word confusion)
//	dictation    spell   audio only                     typed Hebrew            letter pairs (and a word confusion)
//	flashcard    -       self-graded                    Forgot/Hard/Got it/Easy session.Record, not this package
//
// A flashcard has no item: it is what a visit becomes when no checked activity fits.
//
// # Build and Check
//
// Engine.Build makes an Item for a learner and a word. The item has an opaque token and no
// answer. The engine keeps the answer in memory under the token, so the answer is never sent to
// the client before Check. The token is HMAC-SHA256 signed with a key derived from
// HEBREW_SESSION_SECRET (KeyFromSecret), binds the learner, the word, the activity and a
// 30-minute expiry, and can be used once. Engine.Check refuses a token from another learner
// without using the item up. A restart drops the pending items, as it drops the study queues.
//
// Check grades the response and writes through evidence.Recorder, one attempt per word asked:
// the attempt carries the item's skill and activity, its source, what was shown and answered,
// correct, "I don't know", the hint flag, and the latency from Build to Check on the engine
// clock. The rating is the recorder's: wrong is Again, right after a hint is Hard, right is
// Good, right and faster than the learner's own median for the activity is Easy. A typed answer
// is graded with fsrs.GradeTyped: exact is Good, one letter off is Hard (stored as correct, with
// the near miss flag), anything else is Again. The learner's card for the word and skill is
// created by the first attempt, so a word gets its listen or spell card the first time it is
// asked through that skill. A lesson source stores the attempt and leaves every card alone. A
// placement item gives no feedback, and only a right answer seeds a card.
//
// # Distractors
//
// Pick chooses options from the lexicon. An option has a gloss, a different spelling from the
// answer, and a gloss that shares no content word with the answer's (GlossesOverlap), and no two
// options in one item overlap. Candidates come from the answer's placement band and part of
// speech first. When that group is too small the rest are taken from the same part of speech,
// then the same band, then any word. A word the learner confused with the answer in the last 30
// days is preferred inside its group.
//
// # Confusions
//
// A wrong pick is logged by the recorder with the word picked: a letter pair when the two
// spellings differ by one letter (final letters counted as the ordinary letter), otherwise kind
// other. A typed answer within two letters of the right one is aligned with Align, and each
// substituted letter is logged as a letter pair, keyed by the two letters in code point order
// (typing ם for ן is ם/ן). Final letters stay distinct here, as they do in GradeTyped.
//
// # Planning
//
// The planner is pure. WeakestSkills ranks a word's skills from the learner model, Choose picks
// the skill and the activity for a visit, and Facts says which activities the lexicon can
// support for the word. The session mix calls them: see session.CardRef.
//
// Time comes from internal/clock. Randomness comes from a seeded generator, so a test that
// gives WithSeed gets the same items. Tokens use crypto/rand.
//
// Provenance: INIT-015/SPEC-015.
package activity
