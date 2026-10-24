package units

import "fmt"

// Product thresholds. The numbers live only in the constants below.
// Descriptions and issue messages read those constants.
// MaxNewWordsPerUnit is the unit budget. Each new word is recycled at
// least RecycleMinCount times across at least RecycleMinLessons later
// lessons. Story and listening passages block below CoverageBlockBelow
// and warn from there up to but not including CoverageWarnBelow. A lesson
// that introduces more than MaxNewWordsPerLesson new words blocks. A
// lesson that introduces fewer than MinNewWordsPerLesson warns, because a
// review or story lesson may introduce fewer than the minimum. Checkpoints
// are exempt from both lesson budgets.
const (
	MaxNewWordsPerUnit   = 8
	MinNewWordsPerLesson = 2
	MaxNewWordsPerLesson = 3
	RecycleMinCount      = 4
	RecycleMinLessons    = 3
	CoverageBlockBelow   = 90
	CoverageWarnBelow    = 95

	// TranslationMaxLen is the longest English line translation that does not
	// warn. MinTranslatedLines is how many translated lines a unit needs once
	// it has a sentence listen_pick, so three other choices exist.
	TranslationMaxLen  = 120
	MinTranslatedLines = 4

	// Brief checks (INIT-015/SPEC-020). The brief's own numbers win when it
	// carries them. These apply when it carries none, so a malformed brief
	// fails closed instead of open.
	DefaultNewWordMinUses = 3
	DefaultStruggleUses   = 3
	// DefaultMinKnownShare is the coverage a passage needs when the brief
	// names no tier figure. It is the full-strength 0.95 of north-star 7.3.
	DefaultMinKnownShare = 0.95
)

// Rule ids. Severity on a coverage issue depends on the measured percent;
// the catalog row records the blocking severity and the description states
// the warn band.
const (
	RuleActivityTarget             = "activity.target"
	RuleAudioMissing               = "audio.missing"
	RuleBriefAudioAll              = "brief.audio_all"
	RuleBriefCoverage              = "brief.coverage"
	RuleBriefExpired               = "brief.expired"
	RuleBriefFixContrast           = "brief.fix_contrast"
	RuleBriefGloss                 = "brief.gloss_out_of_brief"
	RuleBriefIntro                 = "brief.new_intro_before_test"
	RuleBriefNewMinUses            = "brief.new_min_uses"
	RuleBriefNotFound              = "brief.not_found"
	RuleBriefStruggle              = "brief.struggle_reuse"
	RuleBriefTextTooLong           = "brief.text_too_long"
	RuleBriefUsed                  = "brief.used"
	RuleBudgetLesson               = "budget.lesson"
	RuleBudgetLessonFew            = "budget.lesson.few"
	RuleBudgetUnit                 = "budget.unit"
	RuleCoverageListening          = "coverage.listening"
	RuleCoverageStory              = "coverage.story"
	RuleLexemeUnknown              = "lexeme.unknown"
	RuleNounGender                 = "noun.gender"
	RuleNounPlural                 = "noun.plural"
	RuleRecycleCount               = "recycle.count"
	RuleRecycleSpread              = "recycle.spread"
	RuleSayAudio                   = "say.audio_required"
	RuleSayPrompt                  = "say.prompt_required"
	RuleSchemaActivity             = "schema.activity"
	RuleSchemaCheckpoint           = "schema.checkpoint"
	RuleSchemaKind                 = "schema.kind"
	RuleSchemaLookup               = "schema.lookup"
	RuleSchemaRef                  = "schema.ref"
	RuleSchemaRequired             = "schema.required"
	RuleStoryUnmapped              = "story.unmapped"
	RuleStoryUnpointed             = "story.unpointed"
	RuleUnitNoMarkup               = "unit.no_markup"
	RulePatternOrder               = "unit.pattern_order"
	RuleUnitTranslationDistractors = "unit.translation_distractors"
	RuleUnitTranslationLang        = "unit.translation_lang"
	RuleUnitTranslationLength      = "unit.translation_length"
	RuleUnitTranslationMissing     = "unit.translation_missing"
	RuleWorksheetShape             = "unit.worksheet_shape"

	// Dialogue rules (INIT-015/SPEC-026).
	RuleUnitDialogueLines    = "unit.dialogue_lines"
	RuleUnitDialogueSpeakers = "unit.dialogue_speakers"
	RuleDialoguePlay         = "dialogue.play"
)

// lexeme.unknown and story.unmapped are the same kind of miss, split by
// where the word sits. A story content word that does not map to a lexicon
// entry is story.unmapped only, so a story token is never also
// lexeme.unknown. lexeme.unknown stays for a new word, and for a Hebrew
// token in any activity other than story. Names and numbers are exempt
// on the activity token. The split keeps one finding per story token: story.unmapped
// is what coverage and glosses need, because a gloss has to land on a
// lexicon entry.
//
// A Hebrew numeral is gematria in rank order, written with geresh
// (U+05F3) or gershayim (U+05F4): zero or more ת, then at most one of
// ק/ר/ש, then at most one tens letter, then at most one units letter.
// ט״ו and ט״ז are 15 and 16. Acronyms such as צה״ל and ע״י are words and
// must map to a lexeme. A known limitation: spellings that are also
// valid numerals, such as ת״א and ש״ס, cannot be told apart from numbers,
// so they are treated as numerals. ASCII quotes are not numerals.
// Tokenize splits י"ב into י and ב, and ה' into ה, and each letter is
// flagged. Authors should use U+05F3 and U+05F4. ה׳תשפ״ו is a word;
// write תשפ״ו.

// Rule is one authoring check. The MCP rules resource can serve Rules.
type Rule struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// Rules is the validator catalog, sorted by id. Coverage ids fire as
// block or warn; Severity here is the block, and Description gives the band.
var Rules = []Rule{
	{ID: RuleActivityTarget, Severity: SeverityBlock, Description: "A recall, produce, cloze, or say whose line has two or more words that are in the lexicon must set target to the exact pointed token it grades. That token must be one of the line's tokens, and that token's exact pointed text must be a lexicon entry. The unpointed spelling does not count. A line with fewer lexicon words may omit target. On listen_pick, target is optional. If target is set, only that word gets credit. If not, a line with exactly one lexicon word credits that word. With no target on a line with two or more lexicon words, the player grades the first new word on the line, or the first lexicon word if there is no new word, and logs no exposure. Planned with #139 and migration 015: such a step credits nothing and logs a lesson_listen exposure. A target on any other activity type is the same rule. On a new save or publish a missing target is a block, and so is publishing an older revision. Checking the current published revision reports it as a warning."},
	{ID: RuleAudioMissing, Severity: SeverityBlock, Description: "A Hebrew activity has no audio clip for its exact pointed text. A dialogue line needs a clip in its own speaker's voice, and a clip in the other voice does not count."},
	{ID: RuleBriefAudioAll, Severity: SeverityBlock, Description: "Brief check. A new word, or a title, situation, or concept that contains Hebrew, has no audio clip for its exact pointed text. Activity text is covered by audio.missing."},
	{ID: RuleBriefCoverage, Severity: SeverityBlock, Description: "Brief check. In a story or listen_pick passage, the share of Hebrew tokens that are known or solid for the learner, in the brief, or one of this unit's newWords with an exact pointed lexicon entry, is below the brief's tier figure (0.60, 0.90, or 0.95). The intro and the minimum uses run on the brief's new words and on every newWord that coverage credited, including one outside the brief. Function words, proper names, and numerals are left out of the count, and a token that matches no lexeme counts as unknown. Replaces coverage.story and coverage.listening when a brief is used."},
	{ID: RuleBriefExpired, Severity: SeverityBlock, Description: "Brief check. The brief is older than its time to live and cannot be used."},
	{ID: RuleBriefFixContrast, Severity: SeverityBlock, Description: "Brief check. A right form from the brief's fixMistakes does not appear in the unit. Practise that form, and show the wrong form beside it once."},
	{ID: RuleBriefGloss, Severity: SeverityBlock, Description: "Brief check. A word that is outside the brief and not known or solid for the learner has no English gloss in the lexicon."},
	{ID: RuleBriefIntro, Severity: SeverityBlock, Description: "Brief check. A new word of the brief, or a unit newWord that coverage credited, reaches its first checked activity with no story, dialogue, or shadow step that shows it earlier."},
	{ID: RuleBriefNewMinUses, Severity: SeverityBlock, Description: fmt.Sprintf("Brief check. A new word of the brief appears fewer times in the unit than the brief asks, and at least %d. A unit newWord that coverage credited and that is not in the brief uses that same floor.", DefaultNewWordMinUses)},
	{ID: RuleBriefNotFound, Severity: SeverityBlock, Description: "Brief check. The brief id is missing, malformed, unknown, or belongs to another learner. The cases are not told apart."},
	{ID: RuleBriefStruggle, Severity: SeverityBlock, Description: "Brief check. A struggle word appears fewer times than the brief requires, or never in an activity of its failing skill."},
	{ID: RuleBriefTextTooLong, Severity: SeverityBlock, Description: "Brief check. A story or listen_pick passage has more Hebrew tokens than the tier's limit, when the tier sets one."},
	{ID: RuleBriefUsed, Severity: SeverityBlock, Description: "Brief check. The brief already built a unit. A brief can be used once."},
	{ID: RuleBudgetLesson, Severity: SeverityBlock, Description: fmt.Sprintf("A lesson introduces more than %d new words.", MaxNewWordsPerLesson)},
	{ID: RuleBudgetLessonFew, Severity: SeverityWarn, Description: fmt.Sprintf("A lesson introduces fewer than %d new words. Review and story lessons may introduce fewer than the minimum, so this warns. Checkpoints are exempt.", MinNewWordsPerLesson)},
	{ID: RuleBudgetUnit, Severity: SeverityBlock, Description: fmt.Sprintf("The unit lists more than %d new words.", MaxNewWordsPerUnit)},
	{ID: RuleCoverageListening, Severity: SeverityBlock, Description: fmt.Sprintf("A listening passage (listen_pick) is below %d%% known. Block under %d%%; warn from %d%% up to but not including %d%%. Same thresholds as a story. Other activity types are not passages.", CoverageWarnBelow, CoverageBlockBelow, CoverageBlockBelow, CoverageWarnBelow)},
	{ID: RuleCoverageStory, Severity: SeverityBlock, Description: fmt.Sprintf("A story passage is below %d%% known. Block under %d%%; warn from %d%% up to but not including %d%%.", CoverageWarnBelow, CoverageBlockBelow, CoverageBlockBelow, CoverageWarnBelow)},
	{ID: RuleDialoguePlay, Severity: SeverityBlock, Description: "play on a dialogue is a or b, naming the side the learner speaks. Each of that side's lines needs prompt_en: plain English, no Hebrew and no markup, at most 200 characters. play on any other activity type is the same rule."},
	{ID: RuleLexemeUnknown, Severity: SeverityBlock, Description: "A new word, or a Hebrew token in an activity other than story, is not in the lexicon. A story token is story.unmapped instead."},
	{ID: RuleNounGender, Severity: SeverityBlock, Description: "A new noun has no gender (m, f, or both)."},
	{ID: RuleNounPlural, Severity: SeverityWarn, Description: "A new noun with an irregular plural does not show that plural in the unit."},
	{ID: RuleRecycleCount, Severity: SeverityWarn, Description: fmt.Sprintf("A new word appears fewer than %d times in later lessons.", RecycleMinCount)},
	{ID: RuleRecycleSpread, Severity: SeverityWarn, Description: fmt.Sprintf("A new word appears in fewer than %d later lessons.", RecycleMinLessons)},
	{ID: RuleSayAudio, Severity: SeverityBlock, Description: "A say step has no audio clip for its exact pointed text. The learner hears that clip only after they say the line."},
	{ID: RuleSayPrompt, Severity: SeverityBlock, Description: fmt.Sprintf("A say step needs prompt_en: plain English, no Hebrew and no markup, at most %d characters. It is the cue the learner sees before the Hebrew.", PromptMaxRunes)},
	{ID: RuleSchemaActivity, Severity: SeverityBlock, Description: "An activity type is not one of the nine lesson types."},
	{ID: RuleSchemaCheckpoint, Severity: SeverityBlock, Description: "A checkpoint introduces a new word."},
	{ID: RuleSchemaKind, Severity: SeverityBlock, Description: "A lesson kind is not lesson or checkpoint."},
	{ID: RuleSchemaLookup, Severity: SeverityBlock, Description: "The lexicon or audio lookup failed."},
	{ID: RuleSchemaRef, Severity: SeverityBlock, Description: "A new word is duplicated or never appears in a lesson."},
	{ID: RuleSchemaRequired, Severity: SeverityBlock, Description: "A required field is missing or empty."},
	{ID: RuleStoryUnmapped, Severity: SeverityBlock, Description: "A story content word does not map to a lexicon entry. Names and numbers are exempt. A number is digits or gematria in rank order with U+05F3/U+05F4. Acronyms are words, except spellings that are also valid numerals (ת״א, ש״ס), which are treated as numerals. ASCII quotes are split into letters and flagged. The same token is not also lexeme.unknown."},
	{ID: RuleStoryUnpointed, Severity: SeverityBlock, Description: "A story contains a Hebrew word with no niqqud. Names, numbers, and punctuation are ignored. A number is digits or gematria in rank order with U+05F3/U+05F4. Acronyms are words, except spellings that are also valid numerals (ת״א, ש״ס), which are treated as numerals. ASCII quotes are split into letters and flagged. Homographs are not exempt."},
	{ID: RuleUnitDialogueLines, Severity: SeverityBlock, Description: fmt.Sprintf("A dialogue has fewer than %d or more than %d lines, a dialogue carries text instead of lines, or an activity that is not a dialogue carries lines.", MinDialogueLines, MaxDialogueLines)},
	{ID: RuleUnitDialogueSpeakers, Severity: SeverityBlock, Description: "A dialogue line has a speaker other than a (female voice) or b (male voice), or one of the two speakers has no line."},
	{ID: RuleUnitNoMarkup, Severity: SeverityBlock, Description: "Brief check. A text field of the unit (title, situation, concept, a new word, a lesson kind, an activity type, text, target, or translation, a dialogue line's speaker, text, or translation) contains <, >, or javascript: in any letter case. The unit is plain text and the templates escape it again on output."},
	{ID: RulePatternOrder, Severity: SeverityBlock, Description: "A shared unit teaches a pattern before a pattern it requires. The issue names the unit that teaches the prerequisite later. A learner-scoped unit reports the same finding as a warning."},
	{ID: RuleUnitTranslationDistractors, Severity: SeverityWarn, Description: fmt.Sprintf("A unit with a listen_pick of two or more words that are in the lexicon has fewer than %d different English lines. A line is an activity translation or a dialogue line translation. Lines that match after lowercasing, stripping punctuation, and collapsing spaces count once, and the sentence's own translation counts.", MinTranslatedLines)},
	{ID: RuleUnitTranslationLang, Severity: SeverityBlock, Description: "A translation contains a Hebrew character (U+0590 through U+05FF) or markup (<, >, or javascript:). A translation is English plain text. unit.no_markup also flags the markup."},
	{ID: RuleUnitTranslationLength, Severity: SeverityWarn, Description: fmt.Sprintf("A translation is longer than %d characters.", TranslationMaxLen)},
	{ID: RuleUnitTranslationMissing, Severity: SeverityBlock, Description: "A listen_pick whose text has two or more words that are in the lexicon has an empty translation. The count is the same exact pointed lexicon match as activity.target. The unpointed spelling does not count, and a number is not a word. On a new save or publish this is a block, and so is publishing an older revision. Checking the current published revision reports it as a warning."},
	{ID: RuleWorksheetShape, Severity: SeverityBlock, Description: "The unit's worksheet type does not match its shape. A short story needs a story step and at least three listen_pick items (the true/false questions). A topic conversation needs a dialogue. A verb workshop needs a pattern step. A review sheet introduces no new words. A foundations reading needs a story step."},
}

// Guidance is one player rule for the model that writes units.
// Basis is E (evidence), P (product choice), or E/P, as graded in the
// English-to-Hebrew research note. The validator does not enforce these.
type Guidance struct {
	ID        string `json:"id"`
	Basis     string `json:"basis"`
	Statement string `json:"statement"`
}

// PlayerGuidance is the research table the MCP rules resource will serve.
// It is guidance for authoring and for the player. Validate does not apply it,
// apart from the noun and story-pointing checks in Rules.
var PlayerGuidance = []Guidance{
	{ID: "N1", Basis: "E", Statement: "Point Hebrew by default in every activity, in Review, and in the story; the display string is the pointed lexeme."},
	{ID: "N2", Basis: "P", Statement: "Unpoint one word at a time on a recall card only after its he→en stability is at least 21 days and none of the last 3 reviews was Again; a lapse restores the points."},
	{ID: "N3", Basis: "P", Statement: "Fade points in cloze and story sentences before single-word recall, and keep a homograph whose bare form collides pointed when it is shown alone."},
	{ID: "N4", Basis: "P", Statement: "Keep a story pointed until at least 80% of its lexemes qualify under N2; tapping a word still shows the pointed form and the gloss."},
	{ID: "N5", Basis: "P", Statement: "Review offers a hold-to-peek for points; a peek is logged and caps that card's rating at Hard."},
	{ID: "T1", Basis: "E/P", Statement: "Do not put transliteration on any prompt; audio is the pronunciation model."},
	{ID: "T2", Basis: "P", Statement: "Show transliteration only on the feedback sheet, for a word's first 3 exposures or until its he→en stability is at least 3 days, whichever comes first."},
	{ID: "T3", Basis: "P", Statement: "Never show transliteration in produce or in a story, and never as a tappable gloss."},
	{ID: "T4", Basis: "P", Statement: "Use one fixed transliteration scheme stored on the lexeme."},
	{ID: "D1", Basis: "E/P", Statement: "Teach each final form as a pair with its base letter, and contrast the pair in the first unit that uses it."},
	{ID: "D2", Basis: "P", Statement: "In typed produce, a wrong final or base form at the end of an otherwise correct word is graded Hard, not Again, and the feedback names the rule."},
	{ID: "D3", Basis: "E/P", Statement: "Produce tiles are whole syllables or morphemes, never single shuffled root letters."},
	{ID: "D4", Basis: "P", Statement: "Units 0 and 1 open with a short letter-to-sound decoding block of about 5 items before the vocabulary."},
	{ID: "R1", Basis: "P", Statement: "Show the root chip and teach the root as a family only once 2 or more known words share that root, then ask which word shares it."},
	{ID: "R2", Basis: "P", Statement: "Teach a binyan as a pattern only after 3 or more known verbs are in that binyan."},
	{ID: "R3", Basis: "P", Statement: "On feedback for a verb or noun, show the root chip only once 2 or more known words share that root; show it, do not test it."},
	{ID: "R4", Basis: "P", Statement: "Progress may group words by root, and that grouping is off by default."},
	{ID: "A1", Basis: "E", Statement: "In listen_pick, audio autoplays and the Hebrew stays hidden until the learner answers."},
	{ID: "A2", Basis: "P", Statement: "On a new-word intro, pointed text fades in about 600 ms after audio onset on the same screen."},
	{ID: "A3", Basis: "P", Statement: "Shadow comes only after the same item has had a listen_pick or recall success in that lesson."},
	{ID: "A4", Basis: "P", Statement: "Every feedback sheet has a replay control and a 0.75× speed option."},
	{ID: "G1", Basis: "E", Statement: "Every noun shows a gender chip and, when the plural is irregular, that plural, on the intro and on every feedback sheet."},
	{ID: "G2", Basis: "E/P", Statement: "Produce tiles include the same adjective or verb in the wrong gender or number, and an agreement-only error is named in the feedback."},
	{ID: "G3", Basis: "P", Statement: "A unit that introduces an adjective or a present-tense verb has one pattern step on agreement, preferring irregular nouns."},
	{ID: "G4", Basis: "P", Statement: "At least one cloze per unit places another noun between the subject and the verb."},
	{ID: "G5", Basis: "P", Statement: "Hebrew addressed to the learner uses the gender stored on the learner profile."},
	{ID: "C1", Basis: "E/P", Statement: "Decoding drills are interleaved minimal pairs: the learner hears a word and picks between two written words that differ by one confusable letter."},
	{ID: "C2", Basis: "P", Statement: "After 2 confusable-letter misses within 7 days, the next lesson opens with a 4-item minimal-pair drill, which drops out after 4 correct in a row."},
	{ID: "C3", Basis: "P", Statement: "In typed produce, a sound-alike letter substitution with everything else correct is graded Hard, and the feedback shows the letter."},
	{ID: "C4", Basis: "P", Statement: "Do not group new vocabulary by look-alike spelling."},
}
