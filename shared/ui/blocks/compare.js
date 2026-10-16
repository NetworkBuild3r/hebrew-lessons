/**
 * Shared Hebrew answer compare and retry-before-reveal step helper for plan exercises.
 * Provenance: INIT-012/SPEC-004, INIT-012/SPEC-005
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewBlockCompare = api;
})(typeof window !== 'undefined' ? window : globalThis, function () {
    'use strict';

    var FINAL_TO_REG = {
        '\u05DA': '\u05DB',
        '\u05DD': '\u05DE',
        '\u05DF': '\u05E0',
        '\u05E3': '\u05E4',
        '\u05E5': '\u05E6'
    };

    var TRAILING_PUNCT = /[.,!?;:]+$/;

    /**
     * @param {*} str
     * @returns {string}
     */
    function stripPoints(str) {
        return String(str == null ? '' : str).replace(/[\u0591-\u05C7]/g, '');
    }

    /**
     * Normalize gershayim / geresh variants for consistent compare.
     * @param {string} s
     * @returns {string}
     */
    function normalizeGeresh(s) {
        return s
            .replace(/\u05F4/g, '"')
            .replace(/\u05F3/g, "'")
            .replace(/[\u201C\u201D]/g, '"')
            .replace(/[\u2018\u2019]/g, "'");
    }

    /**
     * @param {*} str
     * @param {{ finalLetters?: boolean }} [opts]
     * @returns {string}
     */
    function normalizeHebrew(str, opts) {
        var s = normalizeGeresh(stripPoints(str))
            .replace(/\s+/g, ' ')
            .trim()
            .replace(TRAILING_PUNCT, '');
        if (!opts || opts.finalLetters !== false) {
            s = s.replace(/[\u05DA\u05DD\u05DF\u05E3\u05E5]/g, function (ch) {
                return FINAL_TO_REG[ch] || ch;
            });
        }
        return s.normalize('NFC');
    }

    /**
     * @param {*} typed
     * @param {*} answer
     * @param {string[]} [acceptList]
     * @returns {boolean}
     */
    function sameHebrew(typed, answer, acceptList) {
        var left = normalizeHebrew(typed);
        if (!left) return false;
        if (left === normalizeHebrew(answer)) return true;
        if (!Array.isArray(acceptList)) return false;
        for (var i = 0; i < acceptList.length; i += 1) {
            if (typeof acceptList[i] === 'string' && normalizeHebrew(acceptList[i]) === left) return true;
        }
        return false;
    }

    /**
     * @param {number} attempts Wrong-answer count for this item (1 = first miss).
     * @param {boolean} isCorrect
     * @param {number} [maxTries]
     * @returns {'correct'|'retry'|'reveal'}
     */
    function stepAttempt(attempts, isCorrect, maxTries) {
        if (isCorrect) return 'correct';
        var n = Number(attempts) || 0;
        var max = Number(maxTries) || 2;
        if (n >= max) return 'reveal';
        if (n === 1) return 'retry';
        return 'reveal';
    }

    /**
     * Deterministic FNV-1a style hash for shuffle seeds.
     * @param {string} seedString
     * @returns {number}
     */
    function hashSeed(seedString) {
        var h = 2166136261;
        var s = String(seedString == null ? '' : seedString);
        for (var i = 0; i < s.length; i += 1) {
            h ^= s.charCodeAt(i);
            h = Math.imul(h, 16777619);
        }
        return h >>> 0;
    }

    /**
     * @param {string[]} words Correct word order.
     * @param {string} seedString Stable seed (e.g. item id).
     * @param {string[][]} [altOrders] Alternate correct orders to avoid when possible.
     * @returns {string[]}
     */
    function shuffleWords(words, seedString, altOrders) {
        var source = Array.isArray(words) ? words.slice() : [];
        if (source.length < 2) return source.slice();
        var seed = hashSeed(seedString + '|' + source.join('\u0001'));
        var arr = source.slice();

        function nextRand() {
            seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
            return seed;
        }

        for (var i = arr.length - 1; i > 0; i -= 1) {
            var j = nextRand() % (i + 1);
            var tmp = arr[i];
            arr[i] = arr[j];
            arr[j] = tmp;
        }

        function sameOrder(a, b) {
            if (!a || !b || a.length !== b.length) return false;
            for (var k = 0; k < a.length; k += 1) {
                if (a[k] !== b[k]) return false;
            }
            return true;
        }

        function forbidden(order) {
            if (sameOrder(order, source)) return true;
            var alts = Array.isArray(altOrders) ? altOrders : [];
            for (var a = 0; a < alts.length; a += 1) {
                if (sameOrder(order, alts[a])) return true;
            }
            return false;
        }

        if (!forbidden(arr)) return arr;

        function distinctWordCount(list) {
            var seen = Object.create(null);
            var n = 0;
            for (var d = 0; d < list.length; d += 1) {
                var key = list[d];
                if (!seen[key]) {
                    seen[key] = true;
                    n += 1;
                }
            }
            return n;
        }

        function tryRotations(base) {
            for (var r = 1; r < base.length; r += 1) {
                var rotated = base.slice(r).concat(base.slice(0, r));
                if (!forbidden(rotated)) return rotated;
            }
            return null;
        }

        /** Swap two indices only when strings differ so duplicate slots cannot no-op. */
        function trySwapDistinct(base) {
            for (var si = 0; si < base.length; si += 1) {
                for (var sj = si + 1; sj < base.length; sj += 1) {
                    if (base[si] === base[sj]) continue;
                    var swapped = base.slice();
                    var tmpSw = swapped[si];
                    swapped[si] = swapped[sj];
                    swapped[sj] = tmpSw;
                    if (!forbidden(swapped)) return swapped;
                }
            }
            return null;
        }

        var fixed = tryRotations(arr);
        if (fixed) return fixed;

        fixed = trySwapDistinct(arr);
        if (fixed) return fixed;

        var reversed = arr.slice().reverse();
        if (!forbidden(reversed)) return reversed;

        fixed = tryRotations(source);
        if (fixed) return fixed;

        fixed = trySwapDistinct(source);
        if (fixed) return fixed;

        if (distinctWordCount(source) >= 2) {
            for (var fi = 0; fi < source.length; fi += 1) {
                for (var fj = fi + 1; fj < source.length; fj += 1) {
                    if (source[fi] === source[fj]) continue;
                    var forced = source.slice();
                    var tmpF = forced[fi];
                    forced[fi] = forced[fj];
                    forced[fj] = tmpF;
                    if (!sameOrder(forced, source)) return forced;
                }
            }
        }

        return arr;
    }

    return {
        hashSeed: hashSeed,
        normalizeHebrew: normalizeHebrew,
        sameHebrew: sameHebrew,
        shuffleWords: shuffleWords,
        stepAttempt: stepAttempt,
        stripPoints: stripPoints
    };
});
