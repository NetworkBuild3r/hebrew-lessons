#!/usr/bin/env node
/**
 * One-off: rasterize the brand SVGs in shared/brand/ to the PNG sizes browsers and
 * phones ask for. Uses `sharp`, which is already an approved dependency. No network.
 *
 *   node scripts/build-brand-assets.js
 *
 * Inputs (SVG, committed):   shared/brand/favicon.svg, shared/brand/icon-maskable.svg
 * Outputs (PNG, committed):  favicon-16.png, favicon-32.png, apple-touch-icon.png (180),
 *                            icon-192.png, icon-512.png
 *
 * icon-192 / icon-512 / apple-touch-icon come from the full-bleed maskable source, so the
 * mark sits inside the maskable safe zone (inner 80% circle) and iOS can round the corners.
 * Provenance: INIT-005/SPEC-006
 */
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const sharp = require('sharp');

const BRAND = path.join(__dirname, '..', 'shared', 'brand');

const JOBS = [
    { src: 'favicon.svg', out: 'favicon-16.png', size: 16 },
    { src: 'favicon.svg', out: 'favicon-32.png', size: 32 },
    { src: 'icon-maskable.svg', out: 'apple-touch-icon.png', size: 180, flatten: true },
    { src: 'icon-maskable.svg', out: 'icon-192.png', size: 192 },
    { src: 'icon-maskable.svg', out: 'icon-512.png', size: 512 }
];

async function render(job) {
    const svg = fs.readFileSync(path.join(BRAND, job.src));
    // High density first, then resize down: keeps small sizes crisp.
    let img = sharp(svg, { density: 384 }).resize(job.size, job.size, { fit: 'contain' });
    if (job.flatten) img = img.flatten({ background: '#1d4ed8' });
    const file = path.join(BRAND, job.out);
    await img.png({ compressionLevel: 9 }).toFile(file);
    return job.out + ' ' + job.size + 'x' + job.size;
}

async function main() {
    for (const job of JOBS) {
        console.log('wrote', await render(job));
    }
}

main().catch((err) => {
    console.error(err && err.message ? err.message : err);
    process.exit(1);
});
