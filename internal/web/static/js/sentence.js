/* Sentence listen_pick. No dependency. Loaded with the shell.
   After an answer, the sticky sheet can cover the last choice. Continue's
   autofocus scrolls the button, which leaves the choice underneath, and under
   reduced motion that scroll is instant so the cover stays. This scrolls the
   end of the choice list to just above the sheet. Reduced motion assigns
   scrollTop. Anything else uses a smooth scroll. Nothing here is inline. */
(function () {
  "use strict";
  if (window.__sentenceListen) return;
  window.__sentenceListen = 1;

  function clear() {
    var last = document.querySelector(".choices--sentence .c-answer:last-child");
    var sheet = document.querySelector(".lf-result");
    if (!last || !sheet) return;
    var lr = last.getBoundingClientRect();
    var sr = sheet.getBoundingClientRect();
    var overlap = lr.bottom - sr.top;
    if (overlap <= 1 || lr.height < 20) return;
    var root = document.scrollingElement || document.documentElement;
    var top = root.scrollTop + overlap + 1;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      root.scrollTop = top;
      return;
    }
    root.scrollTo({ top: top, behavior: "smooth" });
  }

  function schedule() {
    clear();
    window.requestAnimationFrame(function () {
      window.requestAnimationFrame(clear);
    });
  }

  document.addEventListener("DOMContentLoaded", schedule);
  document.addEventListener("htmx:afterSettle", schedule);
})();
