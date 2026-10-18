/**
 * Typed entry for /shared/dist/player.js.
 * The review route loads this bundle so HebrewFlashcardAnswer is set
 * before review-page.js. Later slices move the rest of the player here.
 *
 * The empty export keeps this file an ES module. The repo package.json is
 * CommonJS, and without it esbuild wraps the bundle in a CommonJS loader.
 */
import "./flashcard-answer";

export {};
