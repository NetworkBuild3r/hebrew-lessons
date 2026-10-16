'use strict';

const { describe, it, afterEach } = require('node:test');
const assert = require('node:assert/strict');

const google = require('../server/auth/google');

const NAMES = [
    'GOOGLE_OAUTH_CLIENT_ID',
    'GOOGLE_OAUTH_CLIENT_SECRET',
    'web_client_id',
    'web_client_secret',
    'client_id',
    'client_secret',
    'GOOGLE_OAUTH_AUTH_URL'
];

describe('Google client env names', () => {
    const saved = {};
    for (const name of NAMES) saved[name] = process.env[name];

    afterEach(() => {
        for (const name of NAMES) {
            if (saved[name] === undefined) delete process.env[name];
            else process.env[name] = saved[name];
        }
    });

    it('uses the web client when both pairs are set', () => {
        delete process.env.GOOGLE_OAUTH_CLIENT_ID;
        delete process.env.GOOGLE_OAUTH_CLIENT_SECRET;
        process.env.web_client_id = 'web-id';
        process.env.web_client_secret = 'web-secret';
        process.env.client_id = 'other-id';
        process.env.client_secret = 'other-secret';
        process.env.GOOGLE_OAUTH_AUTH_URL = 'http://127.0.0.1:9/auth';
        const url = new URL(google.authorizationUrl({
            redirectUri: 'http://127.0.0.1/cb',
            state: 's',
            codeChallenge: 'c'.repeat(43)
        }));
        assert.equal(url.searchParams.get('client_id'), 'web-id');
        assert.equal(google.configured(), true);
    });
});
