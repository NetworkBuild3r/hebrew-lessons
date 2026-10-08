#!/usr/bin/env node
/**
 * Validate ElevenLabs env wiring without printing the secret.
 * Usage: node scripts/check-elevenlabs-env.js
 */
const https = require('https');
const { loadEnvFile } = require('../server/audio');

loadEnvFile();

const key = process.env.ELEVENLABS_API_KEY
    || process.env['11_labs_api_key']
    || process.env.ELEVEN_LABS_API_KEY
    || '';

const report = {
    hasKey: !!key,
    keyLen: key.length,
    keyLooksLikeElevenLabs: /^sk_/.test(key),
    voiceId: process.env.ELEVENLABS_VOICE_ID || '(default)',
    model: process.env.ELEVENLABS_MODEL || '(default)',
    wouldCallApi: !!key
};

function get(pathName) {
    return new Promise((resolve, reject) => {
        const req = https.request({
            hostname: 'api.elevenlabs.io',
            path: pathName,
            method: 'GET',
            headers: {
                'xi-api-key': key,
                Accept: 'application/json'
            }
        }, res => {
            const chunks = [];
            res.on('data', c => chunks.push(c));
            res.on('end', () => {
                const body = Buffer.concat(chunks).toString('utf8');
                resolve({ status: res.statusCode, body });
            });
        });
        req.on('error', reject);
        req.end();
    });
}

function postTtsProbe(voiceId, model) {
    const body = JSON.stringify({
        text: 'שָׁלוֹם',
        model_id: model,
        language_code: 'he'
    });
    const pathName = '/v1/text-to-speech/' + encodeURIComponent(voiceId) + '?output_format=mp3_44100_128';
    return new Promise((resolve, reject) => {
        const req = https.request({
            hostname: 'api.elevenlabs.io',
            path: pathName,
            method: 'POST',
            headers: {
                'xi-api-key': key,
                'Content-Type': 'application/json',
                Accept: 'audio/mpeg',
                'Content-Length': Buffer.byteLength(body)
            }
        }, res => {
            const chunks = [];
            res.on('data', c => chunks.push(c));
            res.on('end', () => {
                const buf = Buffer.concat(chunks);
                resolve({ status: res.statusCode, bytes: buf.length, body: buf });
            });
        });
        req.on('error', reject);
        req.write(body);
        req.end();
    });
}

function redact(text) {
    return String(text || '').slice(0, 200).replace(/sk_[a-zA-Z0-9]+/g, 'sk_REDACTED');
}

async function main() {
    if (!key) {
        console.log(JSON.stringify({ ...report, apiOk: false, reason: 'No ELEVENLABS_API_KEY in env or .env' }, null, 2));
        process.exit(1);
    }

    const voiceId = process.env.ELEVENLABS_VOICE_ID || '6WRlPmU1fqPwYbSxeP16';
    const model = process.env.ELEVENLABS_MODEL || 'eleven_v3';

    try {
        const voices = await get('/v1/voices');
        let voicesDetail = '';
        try {
            const parsed = JSON.parse(voices.body);
            voicesDetail = parsed.detail && (parsed.detail.message || JSON.stringify(parsed.detail)) || '';
        } catch (err) {
            voicesDetail = redact(voices.body);
        }

        // What the app actually needs: text-to-speech for the configured voice.
        const tts = await postTtsProbe(voiceId, model);
        let ttsDetail = '';
        if (tts.status < 200 || tts.status >= 300) {
            ttsDetail = redact(tts.body.toString('utf8'));
            try {
                const parsed = JSON.parse(tts.body.toString('utf8'));
                const d = parsed.detail || parsed;
                ttsDetail = d.message || d.code || ttsDetail;
            } catch (err) { /* keep */ }
        }

        const out = {
            ...report,
            voiceId,
            model,
            voicesListStatus: voices.status,
            voicesListOk: voices.status === 200,
            voicesDetail: voicesDetail || undefined,
            ttsStatus: tts.status,
            ttsOk: tts.status >= 200 && tts.status < 300,
            ttsBytes: tts.status >= 200 && tts.status < 300 ? tts.bytes : 0,
            ttsDetail: ttsDetail || undefined,
            readyForK8s: (tts.status >= 200 && tts.status < 300),
            k8sHint: 'Inject pod env ELEVENLABS_API_KEY from vault via secretKeyRef key ELEVENLABS_API_KEY. Never bake .env into the image.'
        };
        console.log(JSON.stringify(out, null, 2));
        process.exit(out.readyForK8s ? 0 : 2);
    } catch (err) {
        console.log(JSON.stringify({ ...report, apiOk: false, error: err.message }, null, 2));
        process.exit(2);
    }
}

main();
