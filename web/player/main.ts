/**
 * Typed entry for /shared/dist/player.js.
 * Routes and the lesson loader pull this one bundle in place of the
 * browser modules that have moved here. It sets HebrewFlashcardAnswer
 * and HebrewSpeech.
 *
 * The empty export keeps this file an ES module. The repo package.json is
 * CommonJS, and without it esbuild wraps the bundle in a CommonJS loader.
 */
import "./flashcard-answer";
import "./speech";

export {};
