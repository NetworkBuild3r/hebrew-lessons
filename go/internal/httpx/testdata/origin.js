'use strict';

// Prints Node originAllowed / requestOrigin for cases with no forwarded proto.
// Go's trusted-proxy scheme rule is not exercised here.

const fs = require('fs');
const path = require('path');

const security = require(path.join(__dirname, '../../../../server/http/security'));
const cases = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const out = [];
for (const item of cases) {
    if (item.publicOrigin) process.env.PUBLIC_ORIGIN = item.publicOrigin;
    else delete process.env.PUBLIC_ORIGIN;
    process.env.COOKIE_SECURE = item.cookieSecure ? '1' : '';
    const req = { headers: item.headers || {} };
    out.push({
        id: item.id,
        allowed: security.originAllowed(req),
        origin: security.requestOrigin(req)
    });
}
process.stdout.write(JSON.stringify(out));
