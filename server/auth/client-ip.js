/**
 * Client address for the login throttle (ADR-0016 §7).
 *
 * Trust CF-Connecting-IP only when the immediate socket peer is inside
 * TRUSTED_PROXIES. Otherwise use the socket address. X-Forwarded-For is
 * never read: Cloudflare appends to a client-supplied list, so the leftmost
 * value is attacker-controlled.
 *
 * TRUSTED_PROXIES is a comma-separated list of IPv4 or IPv6 addresses or
 * CIDRs, read once when this module loads. The default is empty, which trusts
 * nobody. An invalid entry is skipped. Skipping is fail-closed (that entry
 * cannot grant trust) and matches SRS_SCHEDULER: a bad optional value warns
 * once and the process still starts. One warning lists every invalid entry.
 * Provenance: login throttle, trusted proxies.
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
            '[client-ip] ignoring invalid TRUSTED_PROXIES entries: '
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
 * @param {unknown} remoteAddress
 * @returns {boolean}
 */
function peerTrusted(remoteAddress) {
    const peer = peerAddress(remoteAddress);
    if (!peer) return false;
    return trusted.check(peer.address, peer.family);
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

loadTrustedProxies(process.env.TRUSTED_PROXIES);

module.exports = {
    clientIp,
    loadTrustedProxies
};
