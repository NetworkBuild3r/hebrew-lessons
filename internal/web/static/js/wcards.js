/* Show all meanings. The control is a plain link. This module gives it
   role=button, toggles every teach-step card in place, and keeps the click
   from navigating. The label follows the cards: closing one after Show all
   reads "Show all meanings" again. There is no aria-pressed. One document
   listener covers cards swapped in later. */
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

  function upgrade(root) {
    var scope = root && root.querySelectorAll ? root : D;
    var nodes = [];
    if (scope.matches && scope.matches("[data-wcards-all]")) nodes.push(scope);
    if (scope.querySelectorAll) {
      var found = scope.querySelectorAll("[data-wcards-all]");
      for (var i = 0; i < found.length; i++) nodes.push(found[i]);
    }
    for (var j = 0; j < nodes.length; j++) nodes[j].setAttribute("role", "button");
  }

  var batch = false;

  function toggle(btn) {
    var root = rootOf(btn);
    var on = !allOpen(root);
    var cards = cardsIn(root);
    batch = true;
    for (var i = 0; i < cards.length; i++) cards[i].open = on;
    batch = false;
    sync(root);
  }

  function control(ev) {
    var t = ev.target;
    if (!t || !t.closest) return null;
    return t.closest("[data-wcards-all]");
  }

  upgrade(D);

  D.addEventListener("htmx:load", function (ev) {
    var elt = ev.detail && ev.detail.elt ? ev.detail.elt : ev.target;
    upgrade(elt || D);
  });

  D.addEventListener("click", function (ev) {
    var btn = control(ev);
    if (!btn) return;
    ev.preventDefault();
    ev.stopPropagation();
    toggle(btn);
  }, true);

  D.addEventListener("keydown", function (ev) {
    if (ev.key !== " " && ev.key !== "Spacebar") return;
    if (ev.repeat) return;
    var btn = control(ev);
    if (!btn) return;
    ev.preventDefault();
    ev.stopPropagation();
    toggle(btn);
  }, true);

  D.addEventListener("toggle", function (ev) {
    var d = ev.target;
    if (!d || !d.matches || !d.matches("details.wcard")) return;
    if (!batch && d.open) {
      var cards = cardsIn(rootOf(d));
      for (var i = 0; i < cards.length; i++) {
        if (cards[i] !== d) cards[i].open = false;
      }
    }
    sync(rootOf(d));
  }, true);
})(globalThis);
