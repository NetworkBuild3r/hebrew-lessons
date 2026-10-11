/**
 * Recall and the edit form. Plan text is assigned with textContent.
 * Provenance: INIT-003/SPEC-009
 */
(function (factory) {
    var api = factory();
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    if (typeof document !== 'undefined') api.bind(document);
})(function () {
    'use strict';

    function normalizeAnswer(str) {
        return String(str || '')
            .replace(/[\u0591-\u05C7]/g, '')
            .replace(/[.,!?;:"'״׳\-–—()]/g, '')
            .replace(/\s+/g, ' ')
            .trim();
    }

    function decideRecall(attempt, expected) {
        var keep = String(attempt == null ? '' : attempt);
        if (!keep.trim()) {
            return { attempted: false, reveal: false, correct: false, keep: keep };
        }
        var correct = normalizeAnswer(keep) === normalizeAnswer(expected);
        return { attempted: true, reveal: !correct, correct: correct, keep: keep };
    }

    function bind(doc) {
        var check = doc.getElementById('recall-check');
        if (check) {
            check.addEventListener('click', function () {
                var input = doc.getElementById('recall-input');
                var section = doc.getElementById('recall');
                var decision = decideRecall(input.value, section.getAttribute('data-answer') || '');
                if (!decision.attempted) return;
                doc.getElementById('recall-kept').textContent = decision.keep;
                doc.getElementById('recall-attempt').hidden = false;
                doc.getElementById('recall-correction').hidden = !decision.reveal;
                doc.getElementById('recall-ok').hidden = !decision.correct;
            });
        }

        var audio = doc.querySelector('[data-audio="placeholder"]');
        if (audio) {
            audio.addEventListener('click', function () {
                var note = doc.getElementById('audio-note');
                if (note) note.textContent = 'אין הקלטה בתוכנית. זה מציין מקום.';
            });
        }

        var form = doc.getElementById('edit-form');
        if (form) {
            form.addEventListener('submit', function (event) {
                event.preventDefault();
                var body = {
                    op: 'set_field',
                    path: form.elements.path.value,
                    field: 'image',
                    value: {
                        mediaId: form.elements.mediaId.value,
                        alt: form.elements.alt.value
                    },
                    expectedRevision: Number(form.elements.expectedRevision.value)
                };
                var msg = doc.getElementById('edit-result');
                fetch('/apply', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                }).then(function (res) {
                    return res.json().then(function (data) {
                        return { status: res.status, data: data };
                    });
                }).then(function (out) {
                    if (out.data && out.data.conflict) {
                        msg.textContent = 'conflict ' + out.data.conflict.error + ' ' + String(out.data.conflict.revision);
                        return;
                    }
                    if (out.data && out.data.plan) {
                        msg.textContent = 'saved revision ' + String(out.data.plan.revision);
                        form.elements.expectedRevision.value = String(out.data.plan.revision);
                        return;
                    }
                    msg.textContent = 'edit failed';
                }).catch(function () {
                    msg.textContent = 'edit failed';
                });
            });
        }
    }

    return {
        bind: bind,
        decideRecall: decideRecall,
        normalizeAnswer: normalizeAnswer
    };
});
