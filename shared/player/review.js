/**
 * Cross-lesson review store and pure scheduler (ADR-0001).
 * Authored-by-spec: INIT-001/SPEC-003
 */
(function (root) {
    var STORAGE_KEY = 'hebrew-review:v1';
    var DAY_MS = 24 * 60 * 60 * 1000;
    var LADDER_DAYS = [1, 3, 7, 14, 30];
    var DEFAULT_CAPS = { due: 30, new: 10 };

    var memory = null;

    function keyFor(word) {
        return String(word || '').normalize('NFC').replace(/\s+/g, ' ').trim();
    }

    function toTime(now) {
        if (now instanceof Date) return now.getTime();
        if (typeof now === 'number' && Number.isFinite(now)) return now;
        if (typeof now === 'string' && now) {
            var parsed = Date.parse(now);
            if (Number.isFinite(parsed)) return parsed;
        }
        return Date.now();
    }

    function toIso(now) {
        return new Date(toTime(now)).toISOString();
    }

    function addDaysIso(now, days) {
        return new Date(toTime(now) + days * DAY_MS).toISOString();
    }

    function sameLocalDay(a, b) {
        if (a == null || b == null) return false;
        var da = new Date(a);
        var db = new Date(b);
        if (Number.isNaN(da.getTime()) || Number.isNaN(db.getTime())) return false;
        return da.getFullYear() === db.getFullYear()
            && da.getMonth() === db.getMonth()
            && da.getDate() === db.getDate();
    }

    function freshDirection(now) {
        return {
            due: toIso(now),
            step: 0,
            streak: 0,
            lapses: 0,
            lastResult: null,
            lastAt: null
        };
    }

    function cloneDirection(record, now) {
        if (!record || typeof record !== 'object') return freshDirection(now);
        return {
            due: record.due != null ? String(record.due) : toIso(now),
            step: Number.isInteger(record.step) ? record.step : 0,
            streak: Number.isInteger(record.streak) ? record.streak : 0,
            lapses: Number.isInteger(record.lapses) ? record.lapses : 0,
            lastResult: record.lastResult == null ? null : String(record.lastResult),
            lastAt: record.lastAt == null ? null : String(record.lastAt)
        };
    }

    /**
     * Pure ladder transition for one direction record.
     * @param {object|null} record
     * @param {'pass'|'miss'|'not_yet'} signal
     * @param {number|Date|string} now
     */
    function nextState(record, signal, now) {
        var t = toTime(now);
        var next = cloneDirection(record, t);
        var sig = String(signal || '');

        if (sig === 'miss') {
            next.step = 0;
            next.due = addDaysIso(t, LADDER_DAYS[0]);
            next.streak = 0;
            next.lapses = (next.lapses || 0) + 1;
            next.lastResult = 'miss';
            next.lastAt = toIso(t);
            return next;
        }

        if (sig === 'not_yet') {
            next.lastResult = 'not_yet';
            next.lastAt = toIso(t);
            if (record == null || record.lastAt == null) {
                next.due = toIso(t);
            }
            return next;
        }

        if (sig === 'pass') {
            if (sameLocalDay(next.lastAt, t)) {
                next.lastResult = 'pass';
                next.lastAt = toIso(t);
                return next;
            }
            var step = Number.isInteger(next.step) ? next.step : 0;
            step = Math.min(step + 1, 4);
            next.step = step;
            next.due = addDaysIso(t, LADDER_DAYS[step]);
            next.streak = (next.streak || 0) + 1;
            next.lastResult = 'pass';
            next.lastAt = toIso(t);
            return next;
        }

        return next;
    }

    function isDue(record, now) {
        if (!record || record.due == null) return false;
        var dueMs = Date.parse(String(record.due));
        if (!Number.isFinite(dueMs)) return false;
        return dueMs <= toTime(now);
    }

    function lessonOf(wordEntry) {
        var sources = wordEntry && Array.isArray(wordEntry.sources) ? wordEntry.sources : [];
        return sources.length ? String(sources[0]) : '';
    }

    function isNewWord(wordEntry) {
        if (!wordEntry) return true;
        var he = wordEntry.he_en || {};
        var en = wordEntry.en_he || {};
        return he.lastAt == null && en.lastAt == null;
    }

    /**
     * @param {object} records words map from the store (or { words })
     * @param {number|Date|string} now
     * @param {{due?:number,new?:number}} caps
     * @returns {Array<{key:string,direction:string,lesson:string,due:string,step:number,isNew:boolean}>}
     */
    function dueList(records, now, caps) {
        var words = records && records.words && typeof records.words === 'object'
            ? records.words
            : (records && typeof records === 'object' ? records : {});
        var dueCap = caps && Number.isFinite(caps.due) ? caps.due : DEFAULT_CAPS.due;
        var newCap = caps && Number.isFinite(caps.new) ? caps.new : DEFAULT_CAPS.new;
        var t = toTime(now);

        var items = [];
        Object.keys(words).forEach(function (key) {
            var entry = words[key];
            if (!entry || typeof entry !== 'object') return;
            var lesson = lessonOf(entry);
            var wordIsNew = isNewWord(entry);
            ['he_en', 'en_he'].forEach(function (direction) {
                var dir = entry[direction];
                if (!isDue(dir, t)) return;
                items.push({
                    key: key,
                    direction: direction,
                    lesson: lesson,
                    due: String(dir.due),
                    step: Number.isInteger(dir.step) ? dir.step : 0,
                    isNew: wordIsNew,
                    dueMs: Date.parse(String(dir.due))
                });
            });
        });

        items.sort(function (a, b) {
            if (a.dueMs !== b.dueMs) return a.dueMs - b.dueMs;
            if (a.key < b.key) return -1;
            if (a.key > b.key) return 1;
            return a.direction < b.direction ? -1 : 1;
        });

        var selected = [];
        var dueCount = 0;
        var newKeys = Object.create(null);
        var newKeyCount = 0;
        var pending = items.slice();

        while (pending.length && dueCount < dueCap) {
            var pickIndex = -1;
            var lastTwo = selected.length >= 2
                ? [selected[selected.length - 2].lesson, selected[selected.length - 1].lesson]
                : null;
            var otherLessonsWaiting = false;

            if (lastTwo && lastTwo[0] && lastTwo[0] === lastTwo[1]) {
                for (var i = 0; i < pending.length; i += 1) {
                    if (pending[i].lesson && pending[i].lesson !== lastTwo[0]) {
                        otherLessonsWaiting = true;
                        break;
                    }
                }
            }

            for (var j = 0; j < pending.length; j += 1) {
                var candidate = pending[j];
                if (candidate.isNew) {
                    if (!newKeys[candidate.key] && newKeyCount >= newCap) continue;
                }
                if (otherLessonsWaiting && lastTwo && candidate.lesson === lastTwo[0]) {
                    continue;
                }
                pickIndex = j;
                break;
            }

            if (pickIndex < 0) break;
            var chosen = pending.splice(pickIndex, 1)[0];
            if (chosen.isNew && !newKeys[chosen.key]) {
                newKeys[chosen.key] = true;
                newKeyCount += 1;
            }
            selected.push({
                key: chosen.key,
                direction: chosen.direction,
                lesson: chosen.lesson,
                due: chosen.due,
                step: chosen.step,
                isNew: chosen.isNew
            });
            dueCount += 1;
        }

        return selected;
    }

    function emptyStore() {
        return { v: 1, words: {}, migratedLessons: {} };
    }

    function normalizeStore(raw) {
        if (!raw || typeof raw !== 'object') return emptyStore();
        var store = {
            v: 1,
            words: raw.words && typeof raw.words === 'object' ? raw.words : {},
            migratedLessons: raw.migratedLessons && typeof raw.migratedLessons === 'object'
                ? raw.migratedLessons
                : {}
        };
        return store;
    }

    function syncActive() {
        return !!(root.HebrewSync && typeof root.HebrewSync.isActive === 'function'
            && root.HebrewSync.isActive());
    }

    function readStorage() {
        try {
            if (typeof localStorage === 'undefined' || !localStorage) return emptyStore();
            var raw = localStorage.getItem(STORAGE_KEY);
            if (!raw) return emptyStore();
            return normalizeStore(JSON.parse(raw));
        } catch (err) {
            return emptyStore();
        }
    }

    function writeStorage(store) {
        try {
            if (typeof localStorage === 'undefined' || !localStorage) return false;
            localStorage.setItem(STORAGE_KEY, JSON.stringify(store));
            return true;
        } catch (err) {
            return false;
        }
    }

    function load() {
        if (syncActive()) {
            var serverState = root.HebrewSync.getReviewState
                ? root.HebrewSync.getReviewState()
                : emptyStore();
            memory = normalizeStore(serverState);
            return memory;
        }
        memory = readStorage();
        return memory;
    }

    function ensureMemory() {
        if (!memory) {
            if (syncActive()) {
                memory = normalizeStore(
                    root.HebrewSync.getReviewState
                        ? root.HebrewSync.getReviewState()
                        : emptyStore()
                );
            } else {
                memory = readStorage();
            }
        }
        return memory;
    }

    function persist() {
        if (!memory) return;
        // While signed in, never touch hebrew-review:v1 (server is authoritative).
        if (syncActive()) return;
        writeStorage(memory);
    }

    function ensureWord(store, exactKey, lesson) {
        if (!store.words[exactKey]) {
            var now = Date.now();
            store.words[exactKey] = {
                he_en: freshDirection(now),
                en_he: freshDirection(now),
                sources: []
            };
        }
        var entry = store.words[exactKey];
        if (!Array.isArray(entry.sources)) entry.sources = [];
        if (lesson && entry.sources.indexOf(lesson) === -1) entry.sources.push(String(lesson));
        if (!entry.he_en) entry.he_en = freshDirection(Date.now());
        if (!entry.en_he) entry.en_he = freshDirection(Date.now());
        return entry;
    }

    /**
     * @param {string} word Hebrew text (or already-normalized key)
     * @param {'he_en'|'en_he'} direction
     * @param {'pass'|'miss'|'not_yet'} signal
     * @param {{lesson?:string,now?:number|Date|string}} meta
     */
    function record(word, direction, signal, meta) {
        var store = ensureMemory();
        var exactKey = keyFor(word);
        if (!exactKey) return null;
        var dir = direction === 'en_he' ? 'en_he' : 'he_en';
        var lesson = meta && meta.lesson ? String(meta.lesson) : '';
        var now = meta && meta.now != null ? meta.now : Date.now();
        var entry = ensureWord(store, exactKey, lesson);
        entry[dir] = nextState(entry[dir], signal, now);
        if (syncActive() && root.HebrewSync && typeof root.HebrewSync.queueSignal === 'function') {
            // Optimistic memory update only; server schedule replaces on flush.
            root.HebrewSync.queueSignal({
                word: exactKey,
                direction: dir,
                result: String(signal || ''),
                slug: lesson || undefined,
                activity: meta && meta.activity != null ? String(meta.activity) : undefined,
                at: toIso(now)
            });
            return entry[dir];
        }
        persist();
        return entry[dir];
    }

    function get(word) {
        var store = ensureMemory();
        var exactKey = keyFor(word);
        return store.words[exactKey] || null;
    }

    function all() {
        return ensureMemory();
    }

    /**
     * One-time migration of per-lesson wordMarks into hebrew-review:v1 (ADR-0001 §6).
     * @param {{wordMarks?:object}|null} lessonState
     * @param {{id?:string,vocab?:{items?:Array}}} ws
     * @param {number|Date|string} [now]
     */
    function migrateLessonMarks(lessonState, ws, now) {
        // Fresh accounts: never migrate local wordMarks while server sync is active.
        if (syncActive()) return;
        var slug = ws && ws.id ? String(ws.id) : '';
        if (!slug) return;
        var store = ensureMemory();
        if (store.migratedLessons[slug]) return;

        var marks = lessonState && lessonState.wordMarks && typeof lessonState.wordMarks === 'object'
            ? lessonState.wordMarks
            : {};
        var items = ws && ws.vocab && Array.isArray(ws.vocab.items) ? ws.vocab.items : [];
        var byId = Object.create(null);
        items.forEach(function (item) {
            if (!item) return;
            byId[String(item.id)] = item;
        });
        var t = toTime(now != null ? now : Date.now());

        Object.keys(marks).forEach(function (wordId) {
            var mark = marks[wordId];
            var item = byId[wordId];
            var exactKey = item && item.word ? keyFor(item.word) : keyFor(wordId);
            if (!exactKey) return;
            var entry = ensureWord(store, exactKey, slug);
            var dir = entry.he_en;
            if (mark === 'missed') {
                dir.step = 0;
                dir.due = toIso(t);
                dir.streak = 0;
                dir.lapses = (dir.lapses || 0) + 1;
                dir.lastResult = 'miss';
                dir.lastAt = toIso(t);
            } else if (mark === 'knew') {
                dir.step = 0;
                dir.due = addDaysIso(t, LADDER_DAYS[0]);
                dir.lastResult = 'pass';
                dir.lastAt = toIso(t);
            }
            entry.he_en = dir;
        });

        store.migratedLessons[slug] = true;
        persist();
    }

    var Review = {
        STORAGE_KEY: STORAGE_KEY,
        LADDER_DAYS: LADDER_DAYS,
        DEFAULT_CAPS: DEFAULT_CAPS,
        keyFor: keyFor,
        nextState: nextState,
        isDue: isDue,
        dueList: dueList,
        load: load,
        record: record,
        get: get,
        all: all,
        migrateLessonMarks: migrateLessonMarks,
        /** Test helper: replace in-memory store without touching localStorage. */
        _resetMemory: function (store) {
            memory = store ? normalizeStore(store) : null;
        }
    };

    root.HebrewReview = Review;
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = Review;
    }
})(typeof window !== 'undefined' ? window : globalThis);
