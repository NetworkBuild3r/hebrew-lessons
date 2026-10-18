/**
 * Login throttle client address: TRUSTED_PROXIES and CF-Connecting-IP.
 * X-Forwarded-For is not a throttle key.
 */
'use strict';

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const { clientIp, loadTrustedProxies } = require('../server/auth/client-ip');
const throttle = require('../server/auth/throttle');

function request(remoteAddress, headers) {
    return {
        headers: headers || {},
        socket: { remoteAddress }
    };
}

describe('client IP for the login throttle', () => {
    beforeEach(() => {
        loadTrustedProxies('');
        throttle.reset();
    });

    it('with no TRUSTED_PROXIES uses the socket address and ignores both headers', () => {
        const headers = {
            'x-forwarded-for': '203.0.113.5, 10.0.0.8',
            'cf-connecting-ip': '198.51.100.20'
        };
        for (const raw of [undefined, null, '', '   ', ',,,', ' , ']) {
            loadTrustedProxies(raw);
            assert.equal(clientIp(request('10.0.0.8', headers)), '10.0.0.8');
            assert.equal(clientIp(request('::ffff:10.0.0.8', headers)), '::ffff:10.0.0.8');
        }
    });

    it('a trusted peer with a valid CF-Connecting-IP uses the header', () => {
        loadTrustedProxies('10.0.0.0/8');
        const ip = clientIp(request('10.1.2.3', {
            'cf-connecting-ip': '  203.0.113.9  ',
            'x-forwarded-for': '198.51.100.4, 203.0.113.9'
        }));
        assert.equal(ip, '203.0.113.9');
    });

    it('a trusted peer with a missing or malformed header uses the socket address', () => {
        loadTrustedProxies('10.0.0.1');
        const cases = [
            {},
            { 'cf-connecting-ip': '' },
            { 'cf-connecting-ip': '   ' },
            { 'cf-connecting-ip': 'not-an-ip' },
            { 'cf-connecting-ip': '1.2.3' },
            { 'cf-connecting-ip': '10.0.0.01' },
            { 'cf-connecting-ip': '203.0.113.5, 198.51.100.5' },
            { 'cf-connecting-ip': '203.0.113.5 extra' },
            { 'cf-connecting-ip': ['203.0.113.5'] },
            { 'x-forwarded-for': '203.0.113.5' }
        ];
        for (const headers of cases) {
            assert.equal(clientIp(request('10.0.0.1', headers)), '10.0.0.1');
        }
    });

    it('an untrusted peer with a spoofed CF-Connecting-IP and X-Forwarded-For uses the socket address', () => {
        loadTrustedProxies('10.0.0.0/8, 2001:db8:1::/48');
        const ip = clientIp(request('192.0.2.10', {
            'cf-connecting-ip': '198.51.100.20',
            'x-forwarded-for': '203.0.113.5, 10.0.0.1'
        }));
        assert.equal(ip, '192.0.2.10');
    });

    it('an IPv4-mapped IPv6 peer matches an IPv4 trusted proxy', () => {
        loadTrustedProxies('10.0.0.1');
        const trusted = clientIp(request('::ffff:10.0.0.1', {
            'cf-connecting-ip': '203.0.113.8',
            'x-forwarded-for': '198.51.100.8'
        }));
        assert.equal(trusted, '203.0.113.8');

        const otherHost = clientIp(request('::ffff:10.0.0.2', {
            'cf-connecting-ip': '203.0.113.8',
            'x-forwarded-for': '198.51.100.8'
        }));
        assert.equal(otherHost, '::ffff:10.0.0.2');

        loadTrustedProxies('10.0.0.0/8');
        const hexMapped = clientIp(request('::ffff:a00:1', {
            'cf-connecting-ip': '203.0.113.7'
        }));
        assert.equal(hexMapped, '203.0.113.7');

        const outside = clientIp(request('::ffff:192.0.2.1', {
            'cf-connecting-ip': '203.0.113.7',
            'x-forwarded-for': '198.51.100.1'
        }));
        assert.equal(outside, '::ffff:192.0.2.1');
    });

    it('an IPv6 CIDR trusts only peers inside that prefix', () => {
        loadTrustedProxies(' 2001:db8:1::/48 , 10.0.0.0/8 ');
        const inside = clientIp(request('2001:db8:1:2::5', {
            'cf-connecting-ip': '198.51.100.8',
            'x-forwarded-for': '203.0.113.1'
        }));
        assert.equal(inside, '198.51.100.8');

        const outside = clientIp(request('2001:db8:2::1', {
            'cf-connecting-ip': '198.51.100.8',
            'x-forwarded-for': '203.0.113.1'
        }));
        assert.equal(outside, '2001:db8:2::1');
    });

    it('invalid TRUSTED_PROXIES entries warn once and the valid entries still apply', () => {
        const warnings = [];
        const realWarn = console.warn;
        console.warn = (message) => { warnings.push(String(message)); };
        try {
            const invalid = loadTrustedProxies('10.0.0.0/8, not-a-cidr, 2001:db8::/129');
            assert.deepEqual(invalid, ['not-a-cidr', '2001:db8::/129']);
            clientIp(request('10.1.2.3', { 'cf-connecting-ip': '203.0.113.9' }));
        } finally {
            console.warn = realWarn;
        }
        assert.equal(warnings.length, 1);
        assert.match(warnings[0], /not-a-cidr/);
        assert.match(warnings[0], /2001:db8::\/129/);
        assert.equal(
            clientIp(request('10.1.2.3', {
                'cf-connecting-ip': '203.0.113.9',
                'x-forwarded-for': '198.51.100.1'
            })),
            '203.0.113.9'
        );
        assert.equal(
            clientIp(request('192.0.2.5', { 'cf-connecting-ip': '203.0.113.9' })),
            '192.0.2.5'
        );
    });

    it('rotating X-Forwarded-For does not reset the per-IP throttle', () => {
        loadTrustedProxies('');
        const t0 = Date.parse('2026-04-01T12:00:00.000Z');
        for (let i = 0; i < throttle.IP_CAP; i++) {
            const ip = clientIp(request('10.0.0.8', {
                'x-forwarded-for': '203.0.113.' + ((i % 200) + 1) + ', 10.0.0.8',
                'cf-connecting-ip': '198.51.100.' + ((i % 200) + 1)
            }));
            assert.equal(ip, '10.0.0.8');
            assert.equal(throttle.check('user' + i, ip, t0 + i).ok, true);
            throttle.recordFailure('user' + i, ip, t0 + i);
        }

        const rotated = clientIp(request('10.0.0.8', {
            'x-forwarded-for': '198.51.100.50, 203.0.113.50',
            'cf-connecting-ip': '203.0.113.77'
        }));
        assert.equal(rotated, '10.0.0.8');
        const blocked = throttle.check('fresh-user', rotated, t0 + throttle.IP_CAP);
        assert.equal(blocked.ok, false);
        assert.match(blocked.reason, /try again later/i);

        const otherSocket = clientIp(request('10.0.0.9', {
            'x-forwarded-for': '203.0.113.1',
            'cf-connecting-ip': '198.51.100.1'
        }));
        assert.equal(otherSocket, '10.0.0.9');
        assert.equal(throttle.check('fresh-user', otherSocket, t0 + throttle.IP_CAP).ok, true);
    });
});
