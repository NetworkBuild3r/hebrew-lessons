/**
 * Entry point loaded by every lesson's index.html.
 * Loads the player core and activity modules in order, then boots.
 * Do not edit lesson index.html files — add activities under shared/activities/.
 */
(function () {
    const script = document.currentScript;
    const base = script && script.src
        ? script.src.replace(/[^/]+$/, '')
        : '../../shared/';

    const files = [
        'player/utils.js',
        'player/speech.js',
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
            if (window.HebrewPlayer && typeof window.HebrewPlayer.boot === 'function') {
                window.HebrewPlayer.boot();
            } else {
                const app = document.getElementById('app');
                if (app) app.innerHTML = '<main class="wrap"><div class="card"><p>Player failed to load.</p></div></main>';
            }
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
    next();
})();
