/**
 * The audio file of a word in the plan editor: a list of sounds that exist, a Hear
 * button, and Make sound for this word. Fake DOM and a fake API: no ElevenLabs.
 */
'use strict';

const { describe, it, afterEach } = require('node:test');
const assert = require('node:assert/strict');

const Form = require('../shared/admin/plan-form.js');
const dom = require('./helpers/fake-dom');
const { LESSON, samplePlan, makeEditor } = require('./helpers/plan-editor-fixtures');

const HOUSE = 'בַּיִת';
const HOUSE_PATH = '../_audio/words/1111111111111111111111aa.mp3';
const OTHER_PATH = '../_audio/words/2222222222222222222222bb.mp3';
const NEW_PATH = '../_audio/words/3333333333333333333333cc.mp3';
const CLIPS = [
    { word: 'אָב', audioPath: OTHER_PATH, url: '/worksheets/_audio/words/2222222222222222222222bb.mp3' },
    { word: HOUSE, audioPath: HOUSE_PATH, url: '/worksheets/_audio/words/1111111111111111111111aa.mp3' }
];

const attr = (root, name) => dom.collect(root, (n) => n.hasAttribute && n.hasAttribute(name));
const settle = () => new Promise((resolve) => setTimeout(resolve, 20));

function setup(audioPath) {
    const plan = samplePlan();
    const block = plan.units[0].lessons[0].blocks.find((b) => b.type === 'introduction');
    if (audioPath) block.words[0].audioPath = audioPath;
    else delete block.words[0].audioPath;
    const env = makeEditor({ getPlan: () => plan });
    const form = env.editor.renderNode({ path: LESSON + '/blocks/' + block.id, node: block, kind: 'block' });
    env.host.appendChild(form.el);
    const box = attr(form.el, 'data-plan-audio-clip')[0];
    return Object.assign({ plan, block, form, box }, env, {
        select: attr(box, 'data-plan-audio-select')[0],
        hear: attr(box, 'data-plan-audio-hear')[0],
        make: attr(box, 'data-plan-audio-make')[0],
        note: attr(box, 'data-plan-audio-note')[0]
    });
}

/** Press the Save button on the audio file's row. */
async function save(env) {
    const field = attr(env.form.el, 'data-plan-field').find((n) => n.getAttribute('data-plan-field') === 'audioPath');
    attr(field.parentNode, 'data-plan-save')[0].click();
    await settle();
}

const optionValues = (select) => dom.collect(select, (n) => n.tagName === 'OPTION').map((n) => n.value);

describe('word sound (audio file of a word)', () => {
    let gets;
    let posts;
    let played;

    function install(options) {
        const opts = options || {};
        gets = 0;
        posts = [];
        played = [];
        globalThis.HebrewPlanForm = Form;
        globalThis.HebrewApi = {
            get() {
                gets += 1;
                return Promise.resolve({ ok: true, status: 200, data: { clips: opts.clips || CLIPS } });
            },
            post(url, body) {
                posts.push({ url, body });
                return Promise.resolve(opts.post || { ok: true, status: 200, data: { word: HOUSE, audioPath: NEW_PATH, created: true } });
            }
        };
        globalThis.Audio = function Audio(url) {
            played.push(url);
            this.play = () => Promise.resolve();
            this.pause = () => {};
        };
    }

    afterEach(() => {
        delete globalThis.HebrewApi;
        delete globalThis.HebrewPlanForm;
        delete globalThis.Audio;
    });

    it('is a list of sounds, not a typed path, and keeps what is saved', () => {
        install();
        const env = setup(OTHER_PATH);
        assert.ok(env.box, 'the sound picker is drawn');
        const field = attr(env.form.el, 'data-plan-field').find((n) => n.getAttribute('data-plan-field') === 'audioPath');
        assert.equal(field.getAttribute('data-plan-widget'), 'audio-clip');
        assert.equal(env.select.tagName, 'SELECT');
        assert.equal(env.select.value, OTHER_PATH);
        assert.deepEqual(optionValues(env.select), ['', OTHER_PATH]);
        assert.ok(env.hear && env.make);
    });

    it('lists every word that has sound, with this word first, and offers it when none is chosen', async () => {
        install();
        const env = setup(null);
        await settle();
        assert.deepEqual(optionValues(env.select), ['', HOUSE_PATH, OTHER_PATH]);
        assert.equal(env.select.value, '', 'nothing is chosen for the teacher');
        assert.equal(env.note.hidden, false);
        assert.match(env.note.textContent, /already has a sound|כבר יש צליל/);
    });

    it('shows the word, not the path, for the saved choice once the list arrives', async () => {
        install();
        const env = setup(OTHER_PATH);
        await settle();
        const labels = dom.collect(env.select, (n) => n.tagName === 'OPTION').map((n) => n.textContent);
        assert.ok(labels.includes('אָב'));
        assert.equal(labels.includes(OTHER_PATH), false);
        assert.equal(env.select.value, OTHER_PATH);
    });

    it('Hear plays the chosen clip from its page address, and asks for a choice when there is none', async () => {
        install();
        const env = setup(OTHER_PATH);
        await settle();
        env.hear.click();
        await settle();
        assert.deepEqual(played, ['/worksheets/_audio/words/2222222222222222222222bb.mp3']);

        env.select.value = '';
        env.hear.click();
        await settle();
        assert.equal(played.length, 1);
        assert.match(env.note.textContent, /Choose a sound|בחרו צליל/);
    });

    it('Make sound asks for this word, chooses the clip it gets, plays it, and marks the word as changed', async () => {
        install();
        const env = setup(null);
        await settle();
        env.make.click();
        await settle();
        assert.equal(posts.length, 1);
        assert.equal(posts[0].url, '/api/admin/word-audio');
        assert.deepEqual(posts[0].body, { text: HOUSE });
        assert.equal(env.select.value, NEW_PATH);
        assert.ok(optionValues(env.select).includes(NEW_PATH));
        assert.deepEqual(played, ['/worksheets/_audio/words/3333333333333333333333cc.mp3']);
        assert.match(env.note.textContent, /Sound made|הצליל נוצר/);
        await save(env);
        assert.equal(env.calls.length, 1);
        assert.deepEqual(env.calls[0], {
            op: 'set',
            path: LESSON + '/blocks/introduction/words/1',
            field: 'audioPath',
            value: NEW_PATH
        });
    });

    it('says what went wrong when the sound cannot be made, and changes nothing', async () => {
        install({ post: { ok: false, status: 503, data: { code: 'audio_not_configured' } } });
        const env = setup(null);
        await settle();
        env.make.click();
        await settle();
        assert.equal(env.select.value, '');
        assert.deepEqual(played, []);
        assert.match(env.note.textContent, /not set up|לא מוגדר/);
    });

    it('clearing the choice saves as no audio file', async () => {
        install();
        const env = setup(OTHER_PATH);
        await settle();
        env.select.value = '';
        await save(env);
        assert.equal(env.calls.length, 1);
        assert.equal(env.calls[0].op, 'put');
        assert.equal(env.calls[0].path, LESSON + '/blocks/introduction/words/1');
        assert.equal('audioPath' in env.calls[0].value, false, 'the field is taken off the word');
        assert.equal(env.calls[0].value.word, HOUSE, 'the rest of the word is kept');
    });
});
