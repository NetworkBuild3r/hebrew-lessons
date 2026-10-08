#!/usr/bin/env node
/**
 * Batch-generate ElevenLabs audio for a lesson (or all catalog lessons).
 *
 * Words are permanent: LESSONS_DIR/_audio/words/ is never overwritten.
 * Sentences, story lines, grammar, and questions regenerate when text changes
 * (or with --force-sentences).
 *
 * Usage:
 *   node scripts/generate-audio.js <slug>
 *   node scripts/generate-audio.js <slug> --force-sentences
 *   node scripts/generate-audio.js <slug> --dry-run
 *   node scripts/generate-audio.js <slug> --words-only
 *   node scripts/generate-audio.js              # all catalog lessons
 */
const { generateLessonAudio, listLessonSlugs, formatAudioSummary, loadEnvFile } = require('../server/audio');

function parseArgs(argv) {
    const flags = new Set(argv.filter(a => a.startsWith('--')));
    const positional = argv.filter(a => !a.startsWith('--'));
    return {
        slug: positional[0],
        dryRun: flags.has('--dry-run'),
        forceSentences: flags.has('--force-sentences'),
        wordsOnly: flags.has('--words-only'),
        sentencesOnly: flags.has('--sentences-only'),
        linkOnly: flags.has('--link-only')
    };
}

async function main() {
    loadEnvFile();
    const args = parseArgs(process.argv.slice(2));
    const slugs = args.slug ? [args.slug] : listLessonSlugs();
    if (!slugs.length) {
        console.error('Usage: node scripts/generate-audio.js [<slug>] [--force-sentences] [--words-only] [--sentences-only] [--dry-run] [--link-only]');
        process.exit(1);
    }

    const opts = {
        dryRun: args.dryRun,
        forceSentences: args.forceSentences,
        wordsOnly: args.wordsOnly,
        sentencesOnly: args.sentencesOnly,
        linkOnly: args.linkOnly
    };

    for (const slug of slugs) {
        const result = await generateLessonAudio(slug, opts);
        console.log(JSON.stringify({
            slug: result.slug,
            voiceId: result.voiceId,
            model: result.model,
            dryRun: result.dryRun,
            linkOnly: result.linkOnly,
            forceSentences: result.forceSentences,
            words: {
                reused: result.wordsReused,
                created: result.wordsCreated,
                missing: result.wordsMissing.length,
                wouldCreate: result.dryRun ? result.wordsMissing.length : 0
            },
            clips: {
                reused: result.clipsReused,
                created: result.clipsCreated,
                missing: result.clipsMissing.length
            },
            errors: result.errors
        }, null, 2));
        console.log(formatAudioSummary(result));
    }
}

main().catch(err => {
    console.error(err.message || err);
    process.exit(1);
});
