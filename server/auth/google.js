// Shared Google authorization-code client for MCP consent and app sign-in.
// Same client id and secret as oauth2-proxy (Tautulli). Each flow passes its own redirect URI.
// GOOGLE_OAUTH_AUTH_URL, GOOGLE_OAUTH_TOKEN_URL, and GOOGLE_OAUTH_USERINFO_URL override the
// Google endpoints. Tests point them at a local mock. Leave them unset in real deployments.
'use strict';

function firstSet(names) {
    for (let i = 0; i < names.length; i += 1) {
        const value = String(process.env[names[i]] || '').trim();
        if (value) return value;
    }
    return '';
}

// GOOGLE_OAUTH_* wins. Otherwise the web client (redirect URIs), then the
// oauth2-proxy / Tautulli names client_id and client_secret.
function clientId() {
    return firstSet(['GOOGLE_OAUTH_CLIENT_ID', 'web_client_id', 'client_id']);
}

function clientSecret() {
    return firstSet(['GOOGLE_OAUTH_CLIENT_SECRET', 'web_client_secret', 'client_secret']);
}

function configured() {
    return Boolean(clientId() && clientSecret());
}

function authEndpoint() {
    return process.env.GOOGLE_OAUTH_AUTH_URL || 'https://accounts.google.com/o/oauth2/v2/auth';
}

function tokenEndpoint() {
    return process.env.GOOGLE_OAUTH_TOKEN_URL || 'https://oauth2.googleapis.com/token';
}

function userinfoEndpoint() {
    return process.env.GOOGLE_OAUTH_USERINFO_URL || 'https://www.googleapis.com/oauth2/v2/userinfo';
}

/**
 * @param {{ redirectUri: string, state: string, codeChallenge: string }} input
 * @returns {string}
 */
function authorizationUrl(input) {
    const auth = new URL(authEndpoint());
    auth.searchParams.set('client_id', clientId());
    auth.searchParams.set('redirect_uri', input.redirectUri);
    auth.searchParams.set('response_type', 'code');
    // email + profile only. This is a confidential web client (client secret), the same
    // shape oauth2-proxy uses. openid, PKCE, and prompt=select_account make Google send
    // the browser back with an error, which Cursor shows as a token-exchange server_error.
    auth.searchParams.set('scope', 'email profile');
    auth.searchParams.set('state', input.state);
    if (input.codeChallenge) {
        auth.searchParams.set('code_challenge', input.codeChallenge);
        auth.searchParams.set('code_challenge_method', 'S256');
    }
    return auth.toString();
}

/**
 * Exchange a Google code for a verified email and subject.
 * Returns null when Google did not verify the email. Throws on a transport or token failure.
 * The thrown error carries no response body, code, or secret.
 * @param {{ code: string, verifier: string, redirectUri: string }} input
 * @returns {Promise<{ email: string, sub: string, name: string }|null>}
 */
async function identityFromCode(input) {
    const body = new URLSearchParams({
        code: input.code,
        client_id: clientId(),
        client_secret: clientSecret(),
        redirect_uri: input.redirectUri,
        grant_type: 'authorization_code'
    });
    if (input.verifier) body.set('code_verifier', input.verifier);
    const tokenRes = await fetch(tokenEndpoint(), {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json' },
        body: body
    });
    if (!tokenRes.ok) {
        const err = new Error('google');
        err.statusCode = tokenRes.status;
        throw err;
    }
    const tokenJson = await tokenRes.json();
    if (!tokenJson || typeof tokenJson.access_token !== 'string' || !tokenJson.access_token) {
        throw new Error('google');
    }
    const infoRes = await fetch(userinfoEndpoint(), {
        headers: { Authorization: 'Bearer ' + tokenJson.access_token, Accept: 'application/json' }
    });
    if (!infoRes.ok) throw new Error('google');
    const info = await infoRes.json();
    // OpenID userinfo says email_verified. The OAuth2 v2 userinfo this client returns says verified_email.
    const verified = info && (info.email_verified === true || info.email_verified === 'true'
        || info.verified_email === true || info.verified_email === 'true');
    if (!verified) return null;
    const email = String(info.email || '').trim().toLowerCase();
    if (!email) return null;
    const combined = [info.given_name, info.family_name].filter(Boolean).join(' ');
    const name = String(info.name || combined || '').replace(/[\u0000-\u001f\u007f]/g, '').replace(/\s+/g, ' ').trim().slice(0, 80);
    return { email: email, sub: String(info.sub || info.id || ''), name: name };
}

module.exports = {
    configured,
    clientId,
    authorizationUrl,
    identityFromCode
};
