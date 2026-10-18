/**
 * Startup warning: empty TRUSTED_PROXIES and an https PUBLIC_ORIGIN.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const {
    httpsPublicOriginWarning,
    warnHttpsPublicOrigin
} = require('../server/http/trusted-proxies');

const HTTPS = 'https://hebrew.example';

function warningsFor(proxies, origin) {
    const warnings = [];
    const message = warnHttpsPublicOrigin(proxies, origin, (line) => {
        warnings.push(String(line));
    });
    return { warnings, message };
}

describe('https PUBLIC_ORIGIN with an empty TRUSTED_PROXIES', () => {
    it('warns when the proxy list is empty and PUBLIC_ORIGIN is https', () => {
        for (const proxies of [undefined, null, '', '   ', ',', ' , ']) {
            const { warnings, message } = warningsFor(proxies, HTTPS);
            assert.equal(warnings.length, 1, JSON.stringify(proxies));
            assert.equal(warnings[0], message);
            assert.equal(httpsPublicOriginWarning(proxies, HTTPS), message);
            assert.ok(message.includes('\n'));
            assert.match(message, /TRUSTED_PROXIES/);
            assert.match(message, /PUBLIC_ORIGIN/);
            assert.match(message, /https:\/\/hebrew\.example/);
            assert.match(message, /proxy CIDR/);
        }
        const padded = warningsFor('', '  https://hebrew.example/  ');
        assert.equal(padded.warnings.length, 1);
        assert.match(padded.message, /https:\/\/hebrew\.example\//);
    });

    it('is silent when the proxy list is empty and PUBLIC_ORIGIN is http or unset', () => {
        for (const origin of [undefined, null, '', '   ', 'http://localhost:8080', '  http://hebrew.example  ']) {
            const { warnings, message } = warningsFor(undefined, origin);
            assert.equal(message, '', JSON.stringify(origin));
            assert.deepEqual(warnings, []);
            assert.equal(httpsPublicOriginWarning('', origin), '');
            assert.equal(httpsPublicOriginWarning('   ', origin), '');
        }
    });

    it('is silent when TRUSTED_PROXIES is set and PUBLIC_ORIGIN is https', () => {
        for (const proxies of ['10.0.0.0/8', ' 10.0.0.1 ', '10.0.0.0/8, 2001:db8::/32']) {
            const { warnings, message } = warningsFor(proxies, HTTPS);
            assert.equal(message, '', proxies);
            assert.deepEqual(warnings, []);
            assert.equal(httpsPublicOriginWarning(proxies, HTTPS), '');
        }
    });
});
