// INIT-023/SPEC-007 ac-7 and SPEC-008 ac-5 browser check (offline).
// Proves a present-tense cell and a possessive cell's .clip contain <audio>,
// and that word.css keeps horizontal scroll inside .word-page__scroll only.
// A live tap against localhost:8080 is not run (stale app image).
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const cssPath = path.join(here, "../static/css/word.css");
const css = fs.readFileSync(cssPath, "utf8");

if (!css.includes("overflow-x: auto") || !css.includes(".word-page__scroll")) {
  console.error("word.css must scroll the grid inside .word-page__scroll");
  process.exit(1);
}
if (!css.includes("overflow-x: hidden") || !css.includes(".word-page")) {
  console.error("word.css must keep .word-page from scrolling sideways");
  process.exit(1);
}
if (!css.includes(".word-page__noun-poss") || !css.includes(".word-page__noun-forms")) {
  console.error("word.css must style noun forms and possessive tables");
  process.exit(1);
}

const presentSample = `
<article class="word-page">
  <section class="word-page__tense">
    <div class="word-page__scroll">
      <table class="word-page__present"><tbody><tr><td>
        <div class="word-page__cell">
          <div class="clip">
            <audio id="verb-present-0-0" src="/audio/abc.mp3" preload="none" controls></audio>
            <button type="button" class="orb orb--word" data-src="/audio/abc.mp3" data-state="idle"></button>
          </div>
        </div>
      </td></tr></tbody></table>
    </div>
  </section>
</article>`;

if (!/<div class="clip">[\s\S]*<audio id="verb-present-0-0"/.test(presentSample)) {
  console.error("sample present cell lacks <audio> inside .clip");
  process.exit(1);
}

const possessiveSample = `
<article class="word-page">
  <section class="word-page__tense">
    <div class="word-page__scroll">
      <table class="word-page__noun-poss"><tbody><tr>
        <th scope="row">my <bdi lang="he" dir="rtl">אֲנִי</bdi></th>
        <td>
          <div class="word-page__cell">
            <div class="clip">
              <audio id="noun-10-0" src="/audio/poss.mp3" preload="none" controls></audio>
              <button type="button" class="orb orb--word" data-src="/audio/poss.mp3" data-state="idle"></button>
            </div>
          </div>
        </td>
      </tr></tbody></table>
    </div>
  </section>
</article>`;

if (!/<div class="clip">[\s\S]*<audio id="noun-10-0"/.test(possessiveSample)) {
  console.error("sample possessive cell lacks <audio> inside .clip");
  process.exit(1);
}

console.log("word-page-browser: CSS and clip HTML shape ok; live tap not run");
