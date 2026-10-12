/**
 * Competence feedback helpers and optional Web Audio tactile chimes.
 * Sound effects are OFF by default (INIT-005 REQ-014 / RSCH-007) and never
 * overlap Hebrew speech. Provenance: INIT-007/SPEC-002.
 */
(function (root) {
    const U = root.HebrewUtils;
    const STORAGE_KEY = 'hebrew-sfx-enabled';

    const feedback = {
        speechActive: false,
        _ctx: null,
        _bound: false
    };

    function esc(str) {
        if (U && typeof U.escapeHtml === 'function') return U.escapeHtml(str);
        return String(str == null ? '' : str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function bi(he, en) {
        if (U && typeof U.bi === 'function') return U.bi(he, en);
        return esc(he || '') + (en ? ' (' + esc(en) + ')' : '');
    }

    function readEnabled() {
        try {
            if (!root.localStorage) return false;
            return root.localStorage.getItem(STORAGE_KEY) === '1';
        } catch (err) {
            return false;
        }
    }

    function writeEnabled(on) {
        try {
            if (!root.localStorage) return;
            root.localStorage.setItem(STORAGE_KEY, on ? '1' : '0');
        } catch (err) { /* ignore quota / private mode */ }
    }

    feedback.isSfxEnabled = function () {
        return readEnabled();
    };

    feedback.setSfxEnabled = function (on) {
        writeEnabled(!!on);
        feedback.paintToggles();
        return feedback.isSfxEnabled();
    };

    feedback.toggleSfx = function () {
        return feedback.setSfxEnabled(!feedback.isSfxEnabled());
    };

    /** Called by HebrewSpeech while clips/TTS are loading or playing. */
    feedback.setSpeechActive = function (active) {
        feedback.speechActive = !!active;
    };

    function audioContext() {
        const Ctx = root.AudioContext || root.webkitAudioContext;
        if (!Ctx) return null;
        if (!feedback._ctx) {
            try { feedback._ctx = new Ctx(); } catch (err) { return null; }
        }
        return feedback._ctx;
    }

    /**
     * Tiny sine chime. No-ops when SFX are off or Hebrew speech is active.
     * @param {'correct'|'miss'|'tap'} [kind]
     */
    feedback.chime = function (kind) {
        if (!feedback.isSfxEnabled()) return false;
        if (feedback.speechActive) return false;
        const ctx = audioContext();
        if (!ctx) return false;
        if (ctx.state === 'suspended' && typeof ctx.resume === 'function') {
            try { ctx.resume(); } catch (err) { /* ignore */ }
        }
        const now = ctx.currentTime;
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        const type = kind === 'miss' ? 'miss' : kind === 'tap' ? 'tap' : 'correct';
        osc.type = 'sine';
        osc.frequency.value = type === 'miss' ? 220 : type === 'tap' ? 520 : 660;
        const peak = type === 'miss' ? 0.04 : 0.055;
        const dur = type === 'tap' ? 0.045 : 0.09;
        gain.gain.setValueAtTime(0.0001, now);
        gain.gain.exponentialRampToValueAtTime(peak, now + 0.012);
        gain.gain.exponentialRampToValueAtTime(0.0001, now + dur);
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start(now);
        osc.stop(now + dur + 0.02);
        return true;
    };

    feedback.clear = function (container) {
        if (!container) return null;
        container.classList.add('hidden');
        container.classList.remove('feedback-panel', 'feedback-ok', 'feedback-miss');
        container.innerHTML = '';
        return container;
    };

    /**
     * Emerald check affirmation — no confetti, no points.
     * @param {Element} container
     * @param {{ messageHe?: string, messageEn?: string }} [opts]
     */
    feedback.showCorrect = function (container, opts) {
        opts = opts || {};
        if (!container) return null;
        container.classList.remove('hidden');
        container.classList.add('feedback-panel', 'feedback-ok');
        container.classList.remove('feedback-miss');
        const msg = bi(opts.messageHe || 'נכון', opts.messageEn || 'Correct');
        container.innerHTML =
            '<div class="feedback-mark" aria-hidden="true">✓</div>' +
            '<p class="feedback-msg">' + msg + '</p>';
        feedback.chime('correct');
        return container;
    };

    /**
     * Miss feedback: immediate pointed Hebrew in `.correction`, one retry control.
     * @param {Element} container
     * @param {{
     *   pointed?: string,
     *   correct?: string,
     *   highlight?: string,
     *   retryAction?: string,
     *   retryLabelHe?: string,
     *   retryLabelEn?: string,
     *   showRetry?: boolean
     * }} [opts]
     */
    feedback.showMiss = function (container, opts) {
        opts = opts || {};
        if (!container) return null;
        const pointed = String(opts.pointed || opts.correct || '');
        const highlight = String(opts.highlight || '');
        const retryAction = String(opts.retryAction || 'retry');
        const showRetry = opts.showRetry !== false;

        let wordHtml = esc(pointed);
        if (highlight && pointed) {
            const idx = pointed.indexOf(highlight);
            if (idx >= 0) {
                wordHtml =
                    esc(pointed.slice(0, idx)) +
                    '<mark class="correction-highlight">' + esc(highlight) + '</mark>' +
                    esc(pointed.slice(idx + highlight.length));
            }
        }

        container.classList.remove('hidden');
        container.classList.add('feedback-panel', 'feedback-miss');
        container.classList.remove('feedback-ok');

        const retryBtn = showRetry
            ? '<button type="button" class="btn btn-ghost feedback-retry" data-action="' +
              esc(retryAction) + '">' +
              bi(opts.retryLabelHe || 'נסו שוב', opts.retryLabelEn || 'Try again') +
              '</button>'
            : '';

        container.innerHTML =
            '<div class="correction">' +
            '<p class="correction-label">' + bi('הצורה הנכונה', 'The correct form') + '</p>' +
            '<p class="correction-word he-word" lang="he" dir="rtl">' + wordHtml + '</p>' +
            retryBtn +
            '</div>';
        feedback.chime('miss');
        return container;
    };

    feedback.paintToggles = function (rootEl) {
        const scope = rootEl || (root.document ? root.document : null);
        if (!scope || typeof scope.querySelectorAll !== 'function') return;
        const on = feedback.isSfxEnabled();
        scope.querySelectorAll('[data-sfx-toggle]').forEach(btn => {
            btn.setAttribute('aria-pressed', on ? 'true' : 'false');
            btn.classList.toggle('is-on', on);
            if (!btn.getAttribute('title')) {
                btn.setAttribute('title', on ? 'Sound effects on' : 'Sound effects off');
            }
        });
    };

    function isTypingTarget(el) {
        if (!el || !el.closest) return false;
        return !!el.closest('input, textarea, select, [contenteditable="true"], [contenteditable=""]');
    }

    function replayLastClip() {
        const Speech = root.HebrewSpeech;
        if (!Speech || typeof Speech.replayLast !== 'function') return false;
        return Speech.replayLast();
    }

    function onKeydown(event) {
        if (event.defaultPrevented) return;
        if (event.altKey || event.ctrlKey || event.metaKey) return;
        if (isTypingTarget(event.target)) return;
        const key = event.key;
        if (key !== ' ' && key !== 'r' && key !== 'R') return;
        // Space already activates focused buttons; only hijack when not on a control.
        if (key === ' ' && event.target && event.target.closest &&
            event.target.closest('button, a, [role="button"]')) {
            return;
        }
        if (replayLastClip()) {
            event.preventDefault();
        }
    }

    function onClick(event) {
        const toggle = event.target && event.target.closest
            ? event.target.closest('[data-sfx-toggle]')
            : null;
        if (!toggle) return;
        event.preventDefault();
        feedback.toggleSfx();
        feedback.chime('tap');
    }

    feedback.bind = function (doc) {
        const d = doc || root.document;
        if (!d || feedback._bound) {
            feedback.paintToggles(d);
            return;
        }
        feedback._bound = true;
        d.addEventListener('click', onClick);
        d.addEventListener('keydown', onKeydown);
        feedback.paintToggles(d);
    };

    if (typeof root.document !== 'undefined') {
        if (root.document.readyState === 'loading') {
            root.document.addEventListener('DOMContentLoaded', () => feedback.bind());
        } else {
            feedback.bind();
        }
    }

    root.HebrewFeedback = feedback;
})(typeof window !== 'undefined' ? window : globalThis);
