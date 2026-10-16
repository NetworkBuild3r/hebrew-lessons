#!/usr/bin/env node
/**
 * Headless Chrome: the interface-language switch shows one language and remembers it.
 * Ephemeral server. Never calls ElevenLabs.
 */
'use strict';

const harness = require('./helpers/signed-in-server');
const cdpHelp = require('./helpers/chrome-cdp');

function fail(msg) {
    console.error('chrome-ui-lang-smoke: ' + msg);
    process.exitCode = 1;
}

(async () => {
    let chrome;
    try {
        await harness.start({ lessons: 'fixture' });
        chrome = await cdpHelp.launch({ tag: 'ui-lang', width: 1280, height: 900 });
        const cdp = chrome.cdp;
        const base = harness.baseUrl;

        await cdp.call('Page.navigate', { url: base + '/login/' });
        await cdpHelp.waitFor(cdp, `document.querySelector('#login-submit')`, 'sign-in');
        const login = await cdp.evaluate(`(() => {
            const en = document.querySelector('#login-submit .ui-en');
            const he = document.querySelector('#login-submit .ui-he');
            return {
                lang: document.documentElement.getAttribute('data-ui-lang'),
                en: en && getComputedStyle(en).display,
                he: he && getComputedStyle(he).display,
                enText: en && en.textContent
            };
        })()`);
        if (login.lang !== 'en' || login.en === 'none' || login.he !== 'none' || login.enText !== 'Sign in') {
            fail('login default: ' + JSON.stringify(login));
        }

        await cdpHelp.signInAs(cdp, harness, base, 'learner');
        await cdp.call('Page.navigate', { url: base + '/' });
        await cdpHelp.waitFor(cdp, `document.querySelector('[data-nav="today"] .ui-en')`, 'today label');
        const before = await cdp.evaluate(`(() => {
            const link = document.querySelector('[data-nav="today"]');
            const shell = document.querySelector('.hebrew-shell');
            return {
                lang: document.documentElement.getAttribute('data-ui-lang'),
                en: getComputedStyle(link.querySelector('.ui-en')).display,
                he: getComputedStyle(link.querySelector('.ui-he')).display,
                dir: shell.getAttribute('dir'),
                title: document.title
            };
        })()`);
        if (before.lang !== 'en' || before.en === 'none' || before.he !== 'none' || before.dir !== 'ltr') {
            fail('today default: ' + JSON.stringify(before));
        }
        if (!/Today/.test(before.title)) fail('title: ' + before.title);

        await cdp.evaluate(`document.querySelector('[data-ui-lang-set="he"]').click()`);
        const he = await cdp.evaluate(`(() => {
            const link = document.querySelector('[data-nav="today"]');
            return {
                lang: document.documentElement.getAttribute('data-ui-lang'),
                en: getComputedStyle(link.querySelector('.ui-en')).display,
                he: getComputedStyle(link.querySelector('.ui-he')).display,
                dir: document.querySelector('.hebrew-shell').getAttribute('dir'),
                stored: localStorage.getItem('hebrew-ui-lang'),
                pressed: document.querySelector('[data-ui-lang-set="he"]').getAttribute('aria-pressed'),
                title: document.title
            };
        })()`);
        if (he.lang !== 'he' || he.he === 'none' || he.en !== 'none' || he.dir !== 'rtl' || he.stored !== 'he' || he.pressed !== 'true') {
            fail('after Hebrew: ' + JSON.stringify(he));
        }
        if (!/היום/.test(he.title)) fail('Hebrew title: ' + he.title);

        await cdp.call('Page.reload', { ignoreCache: true });
        await cdpHelp.waitFor(cdp, `document.documentElement.getAttribute('data-ui-lang') === 'he' && document.querySelector('[data-nav="today"] .ui-he')`, 'remembered Hebrew');
        const again = await cdp.evaluate(`getComputedStyle(document.querySelector('[data-nav="today"] .ui-en')).display`);
        if (again !== 'none') fail('Hebrew did not survive reload: ' + again);

        await cdp.evaluate(`document.querySelector('[data-ui-lang-set="en"]').click()`);
        const back = await cdp.evaluate(`document.documentElement.getAttribute('data-ui-lang')`);
        if (back !== 'en') fail('did not return to English');

        if (!process.exitCode) console.log('chrome-ui-lang-smoke: PASS');
    } catch (err) {
        fail(err && err.stack ? err.stack : String(err));
    } finally {
        if (chrome) await chrome.close();
        await harness.stop();
    }
})();
