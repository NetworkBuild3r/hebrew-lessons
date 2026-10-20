/* Show all meanings. The control is a real link to ?meanings=all.
   This module toggles every teach-step card in place, so the click does not
   navigate. One document listener covers cards swapped in later. */
(function (g) {
  "use strict";
  if (g.__wcards) return;
  g.__wcards = 1;
  var D = g.document;
  if (!D) return;

  function label(on) {
    return on ? "Hide meanings" : "Show all meanings";
  }

  function toggle(btn) {
    var on = btn.getAttribute("aria-pressed") !== "true";
    var root = btn.closest("#activity") || D;
    var cards = root.querySelectorAll("details.wcard");
    for (var i = 0; i < cards.length; i++) cards[i].open = on;
    btn.setAttribute("aria-pressed", on ? "true" : "false");
    var node = btn.querySelector("[data-wcards-label]");
    if (node) node.textContent = label(on);
  }

  function control(ev) {
    var t = ev.target;
    if (!t || !t.closest) return null;
    return t.closest("[data-wcards-all]");
  }

  D.addEventListener("click", function (ev) {
    var btn = control(ev);
    if (!btn) return;
    ev.preventDefault();
    ev.stopPropagation();
    toggle(btn);
  }, true);

  D.addEventListener("keydown", function (ev) {
    if (ev.key !== " " && ev.key !== "Spacebar") return;
    var btn = control(ev);
    if (!btn) return;
    ev.preventDefault();
    ev.stopPropagation();
    toggle(btn);
  }, true);
})(globalThis);
