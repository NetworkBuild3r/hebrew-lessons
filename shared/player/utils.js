(function (root) {
    const utils = {};

    utils.escapeHtml = function (str) {
        return String(str == null ? '' : str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    };

    utils.bi = function (he, en) {
        const h = utils.escapeHtml(he || '');
        const e = utils.escapeHtml(en || '');
        if (!e) return h;
        if (!h) return '<span class="en" dir="ltr">' + e + '</span>';
        return h + ' <span class="en" dir="ltr">(' + e + ')</span>';
    };

    utils.sid = function (id) { return String(id); };

    utils.stripVowels = function (str) {
        return String(str || '').replace(/[\u0591-\u05C7]/g, '');
    };

    utils.normalizeHebrew = function (str) {
        return utils.stripVowels(str)
            .replace(/[.,!?;:"'״׳\-–—()]/g, '')
            .replace(/\s+/g, ' ')
            .trim();
    };

    utils.hasVowels = function (str) {
        return /[\u0591-\u05C7]/.test(String(str || ''));
    };

    utils.shuffle = function (list) {
        const next = list.slice();
        for (let i = next.length - 1; i > 0; i -= 1) {
            const j = Math.floor(Math.random() * (i + 1));
            const tmp = next[i];
            next[i] = next[j];
            next[j] = tmp;
        }
        return next;
    };

    utils.partText = function (part) {
        if (typeof part === 'string') return part;
        if (Array.isArray(part)) return part.map(utils.partText).join('');
        if (!part || typeof part !== 'object') return '';
        const value = part.reported || part.direct || part.strong || part.word || '';
        return Array.isArray(value) ? value.map(utils.partText).join('') : String(value);
    };

    utils.renderParts = function (parts) {
        if (typeof parts === 'string') return utils.escapeHtml(parts);
        if (!Array.isArray(parts)) return utils.escapeHtml(utils.partText(parts));
        return parts.map(part => {
            if (typeof part === 'string') return utils.escapeHtml(part);
            if (Array.isArray(part)) return utils.renderParts(part);
            if (!part || typeof part !== 'object') return '';
            if (part.word) {
                const inner = Array.isArray(part.word) ? utils.renderParts(part.word) : utils.escapeHtml(part.word);
                return '<mark class="new-word">' + inner + '</mark>';
            }
            if (part.reported != null) {
                return '<span class="reported">' + utils.renderParts(Array.isArray(part.reported) ? part.reported : [part.reported]) + '</span>';
            }
            if (part.direct != null) {
                return '<span class="direct">' + utils.renderParts(Array.isArray(part.direct) ? part.direct : [part.direct]) + '</span>';
            }
            if (part.strong != null) {
                return '<strong>' + utils.renderParts(Array.isArray(part.strong) ? part.strong : [part.strong]) + '</strong>';
            }
            return utils.escapeHtml(utils.partText(part));
        }).join('');
    };

    utils.storyPlain = function (paragraphs) {
        return (paragraphs || []).map(p => utils.partText(Array.isArray(p) ? p : [p]).trim()).filter(Boolean);
    };

    utils.levelOf = function (ws) {
        const raw = String(ws && ws.level || 'intermediate').toLowerCase();
        if (raw === 'basic' || raw === 'a1' || raw === 'beginner') return 'basic';
        if (raw === 'advanced' || raw === 'b1' || raw === 'b2') return 'advanced';
        return 'intermediate';
    };

    // Inline SVGs so icons work without Font Awesome CDN / webfonts.
    const ICONS = {
        'volume-high': '<path d="M11 5L6 9H2v6h4l5 4V5z"/><path d="M15.54 8.46a5 5 0 010 7.07"/><path d="M19.07 4.93a10 10 0 010 14.14"/>',
        microphone: '<path d="M12 1a3 3 0 00-3 3v8a3 3 0 006 0V4a3 3 0 00-3-3z"/><path d="M19 10v2a7 7 0 01-14 0v-2"/><line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/>',
        play: '<polygon points="6 3 20 12 6 21 6 3"/>',
        pause: '<rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/>',
        stop: '<rect x="5" y="5" width="14" height="14" rx="1"/>',
        house: '<path d="M3 10.5L12 3l9 7.5"/><path d="M5 9.5V21h14V9.5"/>',
        'rotate-right': '<path d="M21 12a9 9 0 11-3.2-6.8"/><polyline points="21 3 21 9 15 9"/>',
        check: '<polyline points="20 6 9 17 4 12"/>',
        list: '<line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>',
        circle: '<circle cx="12" cy="12" r="8"/>',
        'book-bookmark': '<path d="M4 19.5A2.5 2.5 0 016.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z"/><path d="M12 2v8l2-1.5L16 10V2"/>',
        'table-cells-large': '<rect x="3" y="3" width="8" height="8" rx="1"/><rect x="13" y="3" width="8" height="8" rx="1"/><rect x="3" y="13" width="8" height="8" rx="1"/><rect x="13" y="13" width="8" height="8" rx="1"/>',
        'file-lines': '<path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="8" y1="13" x2="16" y2="13"/><line x1="8" y1="17" x2="16" y2="17"/>',
        'pen-to-square': '<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 013 3L7 19l-4 1 1-4 12.5-12.5z"/>',
        'puzzle-piece': '<path d="M19.4 8.5a2 2 0 00-2.8-2.8l-.6.6V4a2 2 0 00-2-2h-2a2 2 0 00-2 2v2.3l-.6-.6a2 2 0 10-2.8 2.8l.6.6H4a2 2 0 00-2 2v2a2 2 0 002 2h2.3l-.6.6a2 2 0 102.8 2.8l.6-.6V20a2 2 0 002 2h2a2 2 0 002-2v-2.3l.6.6a2 2 0 102.8-2.8l-.6-.6H20a2 2 0 002-2v-2a2 2 0 00-2-2h-2.3z"/>',
        clock: '<circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 15"/>',
        comments: '<path d="M21 11.5a8.4 8.4 0 01-.9 3.8 8.5 8.5 0 01-7.6 4.7 8.4 8.4 0 01-3.8-.9L3 21l1.9-5.7a8.4 8.4 0 01-.9-3.8 8.5 8.5 0 014.7-7.6 8.4 8.4 0 013.8-.9h.5a8.5 8.5 0 018 8v.5z"/>',
        bolt: '<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/>',
        cubes: '<path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/>',
        'ear-listen': '<path d="M6 15a6 6 0 1112 0c0 2.5-1.5 3.5-3 5"/><path d="M12 9v3"/><circle cx="12" cy="15.5" r="0.8"/>',
        keyboard: '<rect x="2" y="6" width="20" height="12" rx="2"/><line x1="6" y1="10" x2="6.01" y2="10"/><line x1="10" y1="10" x2="10.01" y2="10"/><line x1="14" y1="10" x2="14.01" y2="10"/><line x1="18" y1="10" x2="18.01" y2="10"/><line x1="8" y1="14" x2="16" y2="14"/>'
    };

    utils.icon = function (name, className) {
        let key = String(name || 'circle');
        if (key.indexOf('fa-') === 0) key = key.slice(3);
        const body = ICONS[key] || ICONS.circle;
        const cls = className ? ' class="' + utils.escapeHtml(className) + '"' : '';
        return '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"' + cls + '>' + body + '</svg>';
    };

    root.HebrewUtils = utils;
})(typeof window !== 'undefined' ? window : globalThis);
