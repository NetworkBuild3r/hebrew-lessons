package units

// Product thresholds. The numbers are the authoring rules from ADR-R1 and
// the QA plan: 8 new words in a unit, 2–3 in a lesson, each new word
// recycled at least 4 times across at least 3 later lessons, story and
// listening coverage blocking under 90% and warning from 90% up to but
// not including 95%.
const (
	MaxNewWordsPerUnit   = 8
	MinNewWordsPerLesson = 2
	MaxNewWordsPerLesson = 3
	RecycleMinCount      = 4
	RecycleMinLessons    = 3
	CoverageBlockBelow   = 90
	CoverageWarnBelow    = 95
)

// Rule ids. Severity on a coverage issue depends on the measured percent;
// the catalog row records the blocking severity and the description states
// the warn band.
const (
	RuleAudioMissing      = "audio.missing"
	RuleBudgetLesson      = "budget.lesson"
	RuleBudgetUnit        = "budget.unit"
	RuleCoverageListening = "coverage.listening"
	RuleCoverageStory     = "coverage.story"
	RuleLexemeUnknown     = "lexeme.unknown"
	RuleNounGender        = "noun.gender"
	RuleNounPlural        = "noun.plural"
	RuleRecycleCount      = "recycle.count"
	RuleRecycleSpread     = "recycle.spread"
	RuleSchemaActivity    = "schema.activity"
	RuleSchemaCheckpoint  = "schema.checkpoint"
	RuleSchemaKind        = "schema.kind"
	RuleSchemaLookup      = "schema.lookup"
	RuleSchemaRef         = "schema.ref"
	RuleSchemaRequired    = "schema.required"
	RuleStoryUnpointed    = "story.unpointed"
)

// Rule is one authoring check. The MCP rules resource can serve Rules.
type Rule struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// Rules is the validator catalog, sorted by id. Coverage ids fire as
// block or warn; Severity here is the block, and Description gives the band.
var Rules = []Rule{
	{ID: RuleAudioMissing, Severity: SeverityBlock, Description: "A Hebrew activity has no audio clip for its exact pointed text."},
	{ID: RuleBudgetLesson, Severity: SeverityBlock, Description: "A lesson introduces fewer than 2 or more than 3 new words."},
	{ID: RuleBudgetUnit, Severity: SeverityBlock, Description: "The unit lists more than 8 new words."},
	{ID: RuleCoverageListening, Severity: SeverityBlock, Description: "Listening-text coverage is below 95%. Block under 90%; warn from 90% up to but not including 95%."},
	{ID: RuleCoverageStory, Severity: SeverityBlock, Description: "Story coverage is below 95%. Block under 90%; warn from 90% up to but not including 95%."},
	{ID: RuleLexemeUnknown, Severity: SeverityBlock, Description: "An activity or new word references a lexeme that is not in the lexicon."},
	{ID: RuleNounGender, Severity: SeverityBlock, Description: "A new noun has no gender (m, f, or both)."},
	{ID: RuleNounPlural, Severity: SeverityWarn, Description: "A new noun with an irregular plural does not show that plural in the unit."},
	{ID: RuleRecycleCount, Severity: SeverityWarn, Description: "A new word appears fewer than 4 times in later lessons."},
	{ID: RuleRecycleSpread, Severity: SeverityWarn, Description: "A new word appears in fewer than 3 later lessons."},
	{ID: RuleSchemaActivity, Severity: SeverityBlock, Description: "An activity type is not one of the seven lesson types."},
	{ID: RuleSchemaCheckpoint, Severity: SeverityBlock, Description: "A checkpoint introduces a new word."},
	{ID: RuleSchemaKind, Severity: SeverityBlock, Description: "A lesson kind is not lesson or checkpoint."},
	{ID: RuleSchemaLookup, Severity: SeverityBlock, Description: "The lexicon or audio lookup failed."},
	{ID: RuleSchemaRef, Severity: SeverityBlock, Description: "A new word is duplicated or never appears in a lesson."},
	{ID: RuleSchemaRequired, Severity: SeverityBlock, Description: "A required field is missing or empty."},
	{ID: RuleStoryUnpointed, Severity: SeverityBlock, Description: "A story contains a Hebrew word with no niqqud. Names, numbers, and punctuation are ignored. Homographs are not exempt."},
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
