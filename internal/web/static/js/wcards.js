/* Show all meanings, and a way to put a card away. The Show all control is a
   plain link. This module gives it role=button, toggles every teach-step card
   in place, and keeps the click from navigating. The label follows the cards:
   closing one after Show all reads "Show all meanings" again. There is no
   aria-pressed. A click that is not on an open card closes the open ones.
   One document listener covers cards swapped in later.

   One open card moves its panel onto the page, so the spotlight cannot
   squeeze the word into the tile. Show all leaves every panel in the line. */
(function (g) {
  "use strict";
  if (g.__wcards) return;
  g.__wcards = 1;
  var D = g.document;
  if (!D) return;
  if (D.documentElement) D.documentElement.classList.add("js");

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

  function panel(card) {
    if (card.__detail && card.__detail.isConnected !== false) return card.__detail;
    var found = card.querySelector ? card.querySelector(":scope > .wcard__detail") : null;
    if (found) card.__detail = found;
    return found;
  }

  function alone(card) {
    var line = card.closest && card.closest(".wline");
    if (!line) return card.open;
    var open = line.querySelectorAll("details.wcard[open]");
    return card.open && open.length < 2;
  }

  function dock(card) {
    var detail = panel(card);
    if (!detail || !D.body || !D.body.appendChild) return;
    detail.__card = card;
    card.__detail = detail;
    if (!detail.__home) detail.__home = card;
    if (detail.parentNode !== D.body) D.body.appendChild(detail);
    if (detail.classList) detail.classList.add("wcard__detail--pop");
    else if ((detail.className || "").indexOf("wcard__detail--pop") < 0) detail.className += " wcard__detail--pop";
  }

  function undock(card) {
    var detail = card.__detail;
    if (!detail) return;
    if (detail.classList) detail.classList.remove("wcard__detail--pop");
    else detail.className = (detail.className || "").replace(" wcard__detail--pop", "");
    if (detail.__home && detail.parentNode !== detail.__home) detail.__home.appendChild(detail);
  }

  function syncPanel(card) {
    if (alone(card)) dock(card);
    else undock(card);
  }

  function syncLine(root) {
    var cards = cardsIn(root);
    for (var i = 0; i < cards.length; i++) syncPanel(cards[i]);
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
    syncLine(root);
  }

  function control(ev) {
    var t = ev.target;
    if (!t || !t.closest) return null;
    return t.closest("[data-wcards-all]");
  }

  upgrade(D);
  syncLine(D);

  D.addEventListener("htmx:beforeSwap", function () {
    var pops = D.querySelectorAll ? D.querySelectorAll(".wcard__detail--pop") : [];
    for (var i = 0; i < pops.length; i++) {
      if (pops[i].__card) undock(pops[i].__card);
    }
  });

  D.addEventListener("htmx:load", function (ev) {
    var elt = ev.detail && ev.detail.elt ? ev.detail.elt : ev.target;
    upgrade(elt || D);
    syncLine(elt || D);
  });

  D.addEventListener("click", function (ev) {
    var t = ev.target;
    var close = t && t.closest && t.closest("[data-wcard-close]");
    if (close) {
      var held = close.closest(".wcard__detail");
      var card = (held && held.__card) || close.closest("details.wcard");
      if (!card) return;
      ev.preventDefault();
      ev.stopPropagation();
      card.open = false;
      syncPanel(card);
      return;
    }
    var btn = control(ev);
    if (btn) {
      ev.preventDefault();
      ev.stopPropagation();
      toggle(btn);
      return;
    }
    // A click on the lesson, or on another word, leaves the open card.
    if (!t || !t.closest || t.closest("details.wcard[open]") || t.closest(".wcard__detail--pop")) return;
    var open = D.querySelectorAll("details.wcard[open]");
    var seen = [];
    for (var i = 0; i < open.length; i++) {
      var root = rootOf(open[i]);
      open[i].open = false;
      if (seen.indexOf(root) < 0) seen.push(root);
    }
    for (var j = 0; j < seen.length; j++) {
      sync(seen[j]);
      syncLine(seen[j]);
    }
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
    syncLine(rootOf(d));
  }, true);
})(globalThis);
