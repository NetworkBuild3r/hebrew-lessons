/**
 * Player TypeScript toolchain. The bundle is built, not committed.
 * Lesson pages load it as a relative dist/player.js URL from worksheet.js,
 * which resolves under /shared/. app.html and app-boot.js do not name the
 * absolute /shared/dist/player.js path. App routes that need speech do.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const harness = require('./helpers/signed-in-server');
const bundle = require('./helpers/player-bundle');

function read(rel) {
    return fs.readFileSync(path.join(root, rel), 'utf8');
}

function runNpm(script) {
    const result = spawnSync('npm', ['run', script], {
        cwd: root,
        encoding: 'utf8'
    });
    assert.equal(
        result.status,
        0,
        script + ' failed\n' + (result.stdout || '') + (result.stderr || '')
    );
}

describe('player toolchain', () => {
    it('keeps esbuild and typescript as the only devDependencies', () => {
        const pkg = JSON.parse(read('package.json'));
        assert.deepEqual(Object.keys(pkg.dependencies).sort(), ['@modelcontextprotocol/sdk', 'sharp', 'zod']);
        assert.deepEqual(Object.keys(pkg.devDependencies).sort(), ['esbuild', 'typescript']);
        assert.equal(pkg.scripts.start, 'node server/start.js');
        assert.equal(pkg.scripts.web, 'node server/web.js');
        assert.equal(pkg.scripts.build, 'npm run typecheck && npm run build:player');
        assert.equal(pkg.scripts.typecheck, 'tsc --noEmit -p web/tsconfig.json');
        assert.match(pkg.scripts['build:player'], /esbuild web\/player\/main\.ts/);
        assert.match(pkg.scripts['build:player'], /--bundle/);
        assert.match(pkg.scripts['build:player'], /--format=iife/);
        assert.match(pkg.scripts['build:player'], /--target=es2020/);
        assert.match(pkg.scripts['build:player'], /--minify/);
        assert.match(pkg.scripts['build:player'], /--sourcemap/);
        assert.match(pkg.scripts['build:player'], /--outfile=shared\/dist\/player\.js/);
    });

    it('typechecks and writes a single gated bundle', async () => {
        const tsconfig = JSON.parse(read('web/tsconfig.json'));
        assert.equal(tsconfig.compilerOptions.strict, true);
        assert.equal(tsconfig.compilerOptions.noEmit, true);
        assert.equal(tsconfig.compilerOptions.allowJs, false);
        assert.equal(tsconfig.compilerOptions.target, 'ES2020');
        assert.deepEqual(tsconfig.compilerOptions.lib, ['DOM', 'ES2020']);

        runNpm('typecheck');

        const outfile = path.join(root, 'shared/dist/player.js');
        const mapfile = path.join(root, 'shared/dist/player.js.map');
        let source = '';
        bundle.withBundleLock(() => {
            fs.rmSync(path.dirname(outfile), { recursive: true, force: true });
            runNpm('build:player');
            source = fs.readFileSync(outfile, 'utf8');
            assert.ok(source.length > 0);
            assert.match(source, /sourceMappingURL=player\.js\.map/);
            assert.equal(fs.existsSync(mapfile), true);
            const names = fs.readdirSync(path.dirname(outfile)).sort();
            assert.deepEqual(names, ['player.js', 'player.js.map']);
        });
        new vm.Script(source);

        await harness.start();
        try {
            const signedOut = await fetch(harness.baseUrl + '/shared/dist/player.js');
            assert.equal(signedOut.status, 401);
            const signedIn = await fetch(harness.baseUrl + '/shared/dist/player.js', {
                headers: { Cookie: harness.cookieFor('learner') }
            });
            assert.equal(signedIn.status, 200);
            assert.match(signedIn.headers.get('content-type') || '', /javascript/);
            assert.equal(signedIn.headers.get('cache-control'), 'no-cache');
            assert.equal(await signedIn.text(), source);
        } finally {
            await harness.stop();
        }

        const ignore = spawnSync('git', ['check-ignore', '-q', 'shared/dist/player.js'], { cwd: root });
        assert.equal(ignore.status, 0, 'shared/dist/player.js must be gitignored');

        const dockerignore = read('.dockerignore');
        assert.match(dockerignore, /^shared\/dist$/m);
        const dockerfile = read('Dockerfile');
        assert.match(dockerfile, /npm run typecheck && npm run build:player/);
        assert.match(dockerfile, /npm ci --omit=dev/);
        assert.match(dockerfile, /COPY --from=player \/app\/shared\/dist/);
        assert.match(dockerfile, /^USER node$/m);
        assert.match(dockerfile, /CMD \["node", "server\/start\.js"\]/);

        const check = read('.github/workflows/check.yml');
        assert.match(check, /runs-on:\s*k8-build/);
        assert.doesNotMatch(check, /ubuntu-latest/);
        assert.match(check, /docker build --target player/);

        for (const rel of ['shared/app.html', 'shared/worksheet.js', 'shared/app-boot.js']) {
            assert.equal(read(rel).includes('/shared/dist/player.js'), false, rel);
        }
        assert.match(read('shared/worksheet.js'), /dist\/player\.js/);
        assert.equal((read('shared/app-routes.js').match(/\/shared\/dist\/player\.js/g) || []).length, 2);
    });
});
