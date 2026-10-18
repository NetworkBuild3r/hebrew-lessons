/**
 * Immediate peers allowed to supply proxy headers (TRUSTED_PROXIES).
 *
 * Comma-separated IPv4 or IPv6 addresses or CIDRs. Read once when this module
 * loads. The default is empty, which trusts nobody. An invalid entry is
 * skipped. Skipping is fail-closed (that entry cannot grant trust) and matches
 * SRS_SCHEDULER: a bad optional value warns once and the process still starts.
 * One warning lists every invalid entry.
 *
 * The login throttle and the CSRF host scheme both use this list. A peer
 * outside it cannot choose the client address or the scheme.
 *
 * An empty list plus an https PUBLIC_ORIGIN is warned at startup
 * (httpsPublicOriginWarning). The process still starts.
 */
'use strict';

const { BlockList, isIP } = require('node:net');

/** Dotted IPv4-mapped IPv6, e.g. ::ffff:10.0.0.1. Node's socket form. */
const MAPPED_V4 = /^::ffff:(\d{1,3}(?:\.\d{1,3}){3})$/i;

/** @type {BlockList} */
let trusted = new BlockList();

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
 * @param {string} entry trimmed TRUSTED_PROXIES item
 * @returns {{ kind: 'address', address: string, family: 'ipv4'|'ipv6' }
 *   | { kind: 'subnet', address: string, prefix: number, family: 'ipv4'|'ipv6' }
 *   | null}
 */
function parseEntry(entry) {
    const slash = entry.lastIndexOf('/');
    if (slash === -1) {
        const family = familyOf(entry);
        if (!family) return null;
        return { kind: 'address', address: entry, family };
    }
    const address = entry.slice(0, slash);
    const prefixText = entry.slice(slash + 1);
    if (!/^\d+$/.test(prefixText)) return null;
    const prefix = Number(prefixText);
    const family = familyOf(address);
    if (!family) return null;
    const max = family === 'ipv4' ? 32 : 128;
    if (prefix > max) return null;
    return { kind: 'subnet', address, prefix, family };
}

/**
 * Replace the trusted-proxy list from a TRUSTED_PROXIES value.
 * @param {string|undefined|null} raw
 * @returns {string[]} invalid entries that were ignored
 */
function loadTrustedProxies(raw) {
    const list = new BlockList();
    const invalid = [];
    const text = raw == null ? '' : String(raw);
    for (const part of text.split(',')) {
        const entry = part.trim();
        if (!entry) continue;
        const parsed = parseEntry(entry);
        if (!parsed) {
            invalid.push(entry);
            continue;
        }
        try {
            if (parsed.kind === 'address') list.addAddress(parsed.address, parsed.family);
            else list.addSubnet(parsed.address, parsed.prefix, parsed.family);
        } catch (_) {
            invalid.push(entry);
        }
    }
    trusted = list;
    if (invalid.length) {
        console.warn(
            '[trusted-proxies] ignoring invalid TRUSTED_PROXIES entries: '
            + invalid.map((entry) => JSON.stringify(entry)).join(', ')
        );
    }
    return invalid;
}

/**
 * Socket peer as a BlockList address. Dotted IPv4-mapped IPv6 is checked as
 * IPv4 so ::ffff:10.0.0.1 matches an IPv4 address or CIDR. Other spellings
 * (including hex-mapped ::ffff:a00:1) keep their family; BlockList matches
 * those against IPv4 rules when the family is ipv6.
 * @param {unknown} remoteAddress
 * @returns {{ address: string, family: 'ipv4'|'ipv6' }|null}
 */
function peerAddress(remoteAddress) {
    if (typeof remoteAddress !== 'string') return null;
    const trimmed = remoteAddress.trim();
    if (!trimmed) return null;
    const mapped = MAPPED_V4.exec(trimmed);
    if (mapped && isIP(mapped[1]) === 4) {
        return { address: mapped[1], family: 'ipv4' };
    }
    const family = familyOf(trimmed);
    if (!family) return null;
    return { address: trimmed, family };
}

/**
 * True when the immediate socket peer is inside TRUSTED_PROXIES.
 * @param {unknown} remoteAddress
 * @returns {boolean}
 */
function peerTrusted(remoteAddress) {
    const peer = peerAddress(remoteAddress);
    if (!peer) return false;
    return trusted.check(peer.address, peer.family);
}

/**
 * True when the list trusts nobody: unset, blank, or only commas.
 * A non-empty token counts as set, even if loadTrustedProxies later ignores it.
 * @param {string|undefined|null} raw
 * @returns {boolean}
 */
function trustedProxiesEmpty(raw) {
    if (raw == null) return true;
    for (const part of String(raw).split(',')) {
        if (part.trim()) return false;
    }
    return true;
}

/**
 * Startup warning when CSRF will see a plain-HTTP socket behind an https site.
 *
 * PUBLIC_ORIGIN is process.env.PUBLIC_ORIGIN (server/http/security.js
 * originAllowed, and the Google redirect base in server/auth/google-login.js).
 * originAllowed accepts an Origin that equals that value. It also accepts the
 * request host. With an empty TRUSTED_PROXIES list the host scheme is the
 * socket, which is plain HTTP behind Traefik, so an https Origin for any other
 * host returns 403.
 *
 * Returns a multi-line message, or '' when startup should stay quiet.
 * Does not read the environment and does not log.
 * @param {string|undefined|null} trustedProxies
 * @param {string|undefined|null} publicOrigin
 * @returns {string}
 */
function httpsPublicOriginWarning(trustedProxies, publicOrigin) {
    if (!trustedProxiesEmpty(trustedProxies)) return '';
    if (publicOrigin == null) return '';
    const origin = String(publicOrigin).trim();
    if (!origin.toLowerCase().startsWith('https://')) return '';
    return [
        '[trusted-proxies] TRUSTED_PROXIES is empty and PUBLIC_ORIGIN starts with https:// ('
            + JSON.stringify(origin) + ').',
        '[trusted-proxies] Behind Traefik the socket is plain HTTP, so the CSRF host origin',
        '[trusted-proxies] is http:// while the browser sends https://. A state-changing',
        '[trusted-proxies] request whose Origin is not exactly PUBLIC_ORIGIN returns 403.',
        '[trusted-proxies] Fix: set TRUSTED_PROXIES to the proxy CIDR.'
    ].join('\n');
}

/**
 * Log httpsPublicOriginWarning when it has something to say. Warn only.
 * @param {string|undefined|null} trustedProxies
 * @param {string|undefined|null} publicOrigin
 * @param {(message: string) => void} [warn]
 * @returns {string} the message, or ''
 */
function warnHttpsPublicOrigin(trustedProxies, publicOrigin, warn) {
    const message = httpsPublicOriginWarning(trustedProxies, publicOrigin);
    if (!message) return '';
    (warn || console.warn)(message);
    return message;
}

loadTrustedProxies(process.env.TRUSTED_PROXIES);

module.exports = {
    loadTrustedProxies,
    peerTrusted,
    httpsPublicOriginWarning,
    warnHttpsPublicOrigin
};
