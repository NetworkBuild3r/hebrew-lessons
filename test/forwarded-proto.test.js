/**
 * CSRF host scheme: X-Forwarded-Proto only from a TRUSTED_PROXIES peer.
 */
'use strict';

const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const { hostOrigin, originAllowed } = require('../server/http/security');
const { loadTrustedProxies } = require('../server/http/trusted-proxies');

const HOST = 'hebrew.example';

function request(remoteAddress, proto, opts) {
    const headers = { host: HOST };
    if (proto !== undefined) headers['x-forwarded-proto'] = proto;
    if (opts && opts.origin) headers.origin = opts.origin;
    return {
        headers,
        socket: {
            remoteAddress,
            encrypted: Boolean(opts && opts.encrypted)
        }
    };
}

describe('X-Forwarded-Proto for the CSRF host origin', () => {
    let previousOrigin;

    beforeEach(() => {
        previousOrigin = process.env.PUBLIC_ORIGIN;
        delete process.env.PUBLIC_ORIGIN;
        loadTrustedProxies('');
    });

    afterEach(() => {
        if (previousOrigin === undefined) delete process.env.PUBLIC_ORIGIN;
        else process.env.PUBLIC_ORIGIN = previousOrigin;
        loadTrustedProxies('');
    });

    it('an untrusted peer with a spoofed https or http scheme uses the socket', () => {
        const spoofedHttps = hostOrigin(request('203.0.113.10', 'https'));
        assert.equal(spoofedHttps, 'http://' + HOST);

        const spoofedHttp = hostOrigin(request('203.0.113.10', 'http', { encrypted: true }));
        assert.equal(spoofedHttp, 'https://' + HOST);

        assert.equal(originAllowed(request('203.0.113.10', 'https', {
            origin: 'https://' + HOST
        })), false);
        assert.equal(originAllowed(request('203.0.113.10', 'http', {
            origin: 'http://' + HOST
        })), true);
    });

    it('a trusted peer uses the proxy scheme', () => {
        loadTrustedProxies('10.0.0.0/8');
        assert.equal(
            hostOrigin(request('10.1.2.3', 'https')),
            'https://' + HOST
        );
        assert.equal(
            hostOrigin(request('::ffff:10.1.2.3', 'https')),
            'https://' + HOST
        );
        assert.equal(originAllowed(request('10.1.2.3', 'https', {
            origin: 'https://' + HOST
        })), true);
    });

    it('multiple values use the rightmost scheme, the one Traefik appends', () => {
        loadTrustedProxies('10.0.0.8');
        assert.equal(
            hostOrigin(request('10.0.0.8', 'http, https')),
            'https://' + HOST
        );
        assert.equal(
            hostOrigin(request('10.0.0.8', 'https,http')),
            'http://' + HOST
        );
        assert.equal(
            hostOrigin(request('10.0.0.8', ' HTTP ')),
            'http://' + HOST
        );
    });

    it('a garbage value falls back to the socket scheme', () => {
        loadTrustedProxies('10.0.0.8');
        assert.equal(hostOrigin(request('10.0.0.8', 'javascript')), 'http://' + HOST);
        assert.equal(hostOrigin(request('10.0.0.8', 'https, ftp')), 'http://' + HOST);
        assert.equal(
            hostOrigin(request('10.0.0.8', 'https://' + HOST, { encrypted: true })),
            'https://' + HOST
        );
        assert.equal(hostOrigin(request('10.0.0.8', ['https'])), 'http://' + HOST);
    });

    it('no header uses the socket scheme for a trusted or untrusted peer', () => {
        assert.equal(hostOrigin(request('203.0.113.10')), 'http://' + HOST);
        assert.equal(
            hostOrigin(request('203.0.113.10', undefined, { encrypted: true })),
            'https://' + HOST
        );
        loadTrustedProxies('10.0.0.8');
        assert.equal(hostOrigin(request('10.0.0.8', '')), 'http://' + HOST);
        assert.equal(hostOrigin(request('10.0.0.8', '   ', { encrypted: true })), 'https://' + HOST);
        assert.equal(hostOrigin({ headers: {}, socket: { remoteAddress: '10.0.0.8' } }), '');
    });
});
