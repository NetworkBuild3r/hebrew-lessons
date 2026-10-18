/**
 * Typed entry for /shared/dist/player.js.
 * Nothing loads this bundle yet. Later slices move the player modules here.
 *
 * The empty export keeps this file an ES module. The repo package.json is
 * CommonJS, and without it esbuild wraps the bundle in a CommonJS loader.
 */
type PageRoot = Document;

function startPlayer(): void {
  const root: PageRoot | undefined = undefined;
  void root;
}

startPlayer();

export {};
