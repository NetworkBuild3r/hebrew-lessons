'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const { loadPlan } = require('../ops');
const { TEXT_LIMIT, escapeHtml, renderPage } = require('../render');
const { validatePlan } = require('../validate');

const PAYLOAD = '<script>alert(1)</script><img src=x onerror=alert(1)>';
const SKELETON = path.join(__dirname, '..');

function assertCleanTags(html) {
    const tags = html.match(/<\/?[a-zA-Z][^>]*>/g) || [];
    tags.forEach((tag) => {
        const body = tag.replace(/^<\/?[a-zA-Z][\w:-]*/, '').replace(/>$/, '');
        const names = [];
        const re = /([:@\w-]+)(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+))?/g;
        let match;
        while ((match = re.exec(body))) names.push(match[1].toLowerCase());
        names.forEach((name) => {
            assert.equal(/^on/.test(name), false, tag);
        });
    });
}

function coffee(plan) {
    return plan.units[0].lessons[0].blocks[0].words[1];
}

test('hostile text is escaped, truncated, and does not become markup', () => {
    const plan = loadPlan();
    plan.title = PAYLOAD;
    plan.subtitle = PAYLOAD;
    plan.units[0].title = PAYLOAD;
    plan.units[0].lessons[0].title = PAYLOAD;
    const word = coffee(plan);
    word.word = PAYLOAD;
    word.meaning = PAYLOAD;
    word.sentence = PAYLOAD;
    word.image.alt = PAYLOAD;
    plan.units[0].lessons[2].recycles[0].word = PAYLOAD;
    plan.units[0].theme = 'א'.repeat(TEXT_LIMIT + 1000);

    const html = renderPage(plan);
    assert.equal(html.includes(PAYLOAD), false);
    assert.ok(html.includes(escapeHtml(PAYLOAD)));
    assert.equal((html.match(/<script/gi) || []).length, 1);
    assert.match(html, /<script src="\/app.js"><\/script>/);
    assertCleanTags(html);
    assert.equal(/javascript:/i.test(html), false);
    assert.ok(html.includes('א'.repeat(TEXT_LIMIT)));
    assert.equal(html.includes('א'.repeat(TEXT_LIMIT + 1)), false);
    const imgs = html.match(/<img\b[^>]*>/gi) || [];
    assert.equal(imgs.length, 1);
    assert.match(imgs[0], /src="\/media\/placeholder-coffee\.png"/);
    assert.match(imgs[0], /alt="&lt;script&gt;/);
});

test('missing alt and a broken media id are refused at render and by validation', () => {
    const missing = loadPlan();
    delete coffee(missing).image.alt;
    const missingHtml = renderPage(missing);
    assert.match(missingHtml, /data-image-refused="missing-alt"/);
    assert.equal(/<img\b/.test(missingHtml), false);
    const missingCheck = validatePlan(missing);
    assert.equal(missingCheck.ok, false);
    assert.ok(missingCheck.errors.some((error) => error.message === 'missing alt text'));

    const broken = loadPlan();
    coffee(broken).image.mediaId = 'http://evil.example/a.png';
    const brokenHtml = renderPage(broken);
    assert.match(brokenHtml, /data-image-refused="broken-media"/);
    assert.equal(/<img\b/.test(brokenHtml), false);
    assert.equal(brokenHtml.includes('http://evil.example'), false);
    const brokenCheck = validatePlan(broken);
    assert.equal(brokenCheck.ok, false);
    assert.ok(brokenCheck.errors.some((error) => error.message === 'bad media reference'));

    const unknown = loadPlan();
    coffee(unknown).image.mediaId = 'not-a-real-picture';
    const unknownHtml = renderPage(unknown);
    assert.match(unknownHtml, /data-image-refused="broken-media"/);
    assert.equal(/<img\b/.test(unknownHtml), false);
    const unknownCheck = validatePlan(unknown);
    assert.equal(unknownCheck.ok, false);
    assert.ok(unknownCheck.errors.some((error) => error.message === 'broken media reference'));
});

test('the page script does not write plan text as HTML', () => {
    const src = fs.readFileSync(path.join(SKELETON, 'public', 'app.js'), 'utf8');
    assert.equal(/innerHTML|outerHTML|insertAdjacentHTML|document\.write|\beval\s*\(/.test(src), false);
});

test('spike scripts do not call the network', () => {
    const files = [
        'validate.js',
        'ops.js',
        'cli.js',
        'render.js',
        'server.js',
        'placeholder-png.js',
        path.join('public', 'app.js')
    ].map((name) => path.join(SKELETON, name));
    files.forEach((file) => {
        const text = fs.readFileSync(file, 'utf8');
        assert.equal(/\bhttps?\.(request|get)\s*\(/.test(text), false, path.basename(file));
        assert.equal(/\bnet\.(connect|createConnection)\s*\(/.test(text), false, path.basename(file));
        assert.equal(/\bdns\.(lookup|resolve)\s*\(/.test(text), false, path.basename(file));
        assert.equal(/\bfetch\s*\(\s*['"`]https?:/i.test(text), false, path.basename(file));
        assert.equal(/XMLHttpRequest|WebSocket/.test(text), false, path.basename(file));
    });
});
