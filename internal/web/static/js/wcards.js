/* Show all meanings. The control is a submit button in a GET form, so
   without script Space and Enter load ?meanings=all. This module toggles
   every teach-step card in place, so the click does not navigate. The label
   follows the cards: closing one after Show all reads "Show all meanings"
   again. One document listener covers cards swapped in later. */
(function (g) {
  "use strict";
  if (g.__wcards) return;
  g.__wcards = 1;
  var D = g.document;
  if (!D) return;

  function label(on) {
    return on ? "Hide meanings" : "Show all meanings";
  }

  function rootOf(node) {
    return (node && node.closest && node.closest("#activity")) || D;
  }

  function cardsIn(root) {
    return root.querySelectorAll ? root.querySelectorAll("details.wcard") : [];
  }

  function allOpen(root) {
    var cards = cardsIn(root);
    if (!cards.length) return false;
    for (var i = 0; i < cards.length; i++) if (!cards[i].open) return false;
    return true;
  }

  function sync(root) {
    var btn = root.querySelector && root.querySelector("[data-wcards-all]");
    var node = btn && btn.querySelector("[data-wcards-label]");
    if (node) node.textContent = label(allOpen(root));
  }

  function toggle(btn) {
    var root = rootOf(btn);
    var on = !allOpen(root);
    var cards = cardsIn(root);
    for (var i = 0; i < cards.length; i++) cards[i].open = on;
    sync(root);
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

  D.addEventListener("submit", function (ev) {
    var form = ev.target;
    if (!form || !form.querySelector || !form.querySelector("[data-wcards-all]")) return;
    ev.preventDefault();
    ev.stopPropagation();
  }, true);

  D.addEventListener("toggle", function (ev) {
    var d = ev.target;
    if (!d || !d.matches || !d.matches("details.wcard")) return;
    sync(rootOf(d));
  }, true);
})(globalThis);
