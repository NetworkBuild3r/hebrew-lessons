/**
 * Client address for the login throttle.
 *
 * Trust CF-Connecting-IP only when the immediate socket peer is inside
 * TRUSTED_PROXIES (server/http/trusted-proxies.js). Otherwise use the socket
 * address. X-Forwarded-For is never read: Cloudflare appends to a
 * client-supplied list, so the leftmost value is attacker-controlled.
 */
'use strict';

const { isIP } = require('node:net');
const { peerTrusted, loadTrustedProxies } = require('../http/trusted-proxies');

/**
 * @param {string} address
 * @returns {'ipv4'|'ipv6'|''}
 */
function familyOf(address) {
    const kind = isIP(address);
    if (kind === 4) return 'ipv4';
    if (kind === 6) return 'ipv6';
    return '';
}

/**
 * A single IP from CF-Connecting-IP, or '' when the header is missing or not an IP.
 * A list is not an IP: do not take the leftmost value.
 * @param {import('http').IncomingMessage|object|null|undefined} req
 * @returns {string}
 */
function connectingIp(req) {
    const headers = req && req.headers;
    if (!headers) return '';
    const raw = headers['cf-connecting-ip'];
    if (typeof raw !== 'string') return '';
    const value = raw.trim();
    if (!familyOf(value)) return '';
    return value;
}

/**
 * @param {import('http').IncomingMessage|object|null|undefined} req
 * @returns {string}
 */
function socketAddress(req) {
    const socket = req && req.socket;
    const address = socket && socket.remoteAddress;
    return typeof address === 'string' ? address : '';
}

/**
 * Address passed to the login throttle.
 * @param {import('http').IncomingMessage|object|null|undefined} req
 * @returns {string}
 */
function clientIp(req) {
    const socket = socketAddress(req);
    if (peerTrusted(socket)) {
        const header = connectingIp(req);
        if (header) return header;
    }
    return socket;
}

module.exports = {
    clientIp,
    loadTrustedProxies
};
