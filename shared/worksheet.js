/**
 * Entry point loaded by every lesson's index.html.
 * Loads the player core and activity modules in order, then boots.
 * Do not edit lesson index.html files — add activities under shared/activities/.
 * Provenance note: INIT-002/SPEC-013 waits for HebrewSync.init before boot.
 */
(function () {
    const script = document.currentScript;
    const base = script && script.src
        ? script.src.replace(/[^/]+$/, '')
        : '../../shared/';

    const files = [
        'player/utils.js',
        'player/review.js',
        'api.js',
        'word-lookup.js',
        'player/sync.js',
        'dist/player.js',
        'player/feedback.js',
        'player/core.js',
        'activities/catalog.js',
        'activities/words.js',
        'activities/match.js',
        'activities/story.js',
        'activities/fill.js',
        'activities/pattern.js',
        'activities/retell.js',
        'activities/ask-friend.js',
        'activities/speed-quiz.js',
        'activities/build-sentence.js',
        'activities/listen-pick.js',
        'activities/type-it.js'
    ];

    let index = 0;
    function next() {
        if (index >= files.length) {
            bootAfterSync();
            return;
        }
        const el = document.createElement('script');
        el.src = base + files[index];
        index += 1;
        el.onload = next;
        el.onerror = function () {
            console.error('Failed to load ' + el.src);
            next();
        };
        (document.head || document.documentElement).appendChild(el);
    }

    function bootPlayer() {
        if (window.HebrewPlayer && typeof window.HebrewPlayer.boot === 'function') {
            window.HebrewPlayer.boot();
        } else {
            const app = document.getElementById('app');
            if (app) app.innerHTML = '<main class="wrap"><div class="card"><p>Player failed to load.</p></div></main>';
        }
    }

    function bootAfterSync() {
        const Sync = window.HebrewSync;
        const slug = window.WORKSHEET && window.WORKSHEET.id ? String(window.WORKSHEET.id) : null;
        if (Sync && typeof Sync.init === 'function') {
            Promise.resolve(Sync.init({ lessonSlug: slug })).then(bootPlayer).catch(bootPlayer);
            return;
        }
        bootPlayer();
    }

    next();
})();
