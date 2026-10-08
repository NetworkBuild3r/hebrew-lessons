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

    root.HebrewUtils = utils;
})(typeof window !== 'undefined' ? window : globalThis);
