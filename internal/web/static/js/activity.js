/* Activity screens (INIT-015/SPEC-016). No dependency. Loaded after motion.js.
   The server renders every screen. This file only takes input:
   - a pick option or a match tile is chosen by tap, by its key, or by Tab and Enter;
   - Check turns on once there is an answer to check;
   - a clip plays from its orb. Playing the prompt clip a second time before the
     answer sets hinted=1, which is what the engine reads as a hint;
   - keys: 1 to 4 pick (and grade on the flashcard), 1 to 5 and A to E on a match
     board, Enter takes the gold action, Space replays the prompt clip.
   Nothing here decides right or wrong. The answer is posted and the server checks. */
(function (g) {
  "use strict";
  var D = g.document;
  if (!D || g.__act) return;
  g.__act = 1;

  function one(sel, from) { return (from || D).querySelector(sel); }
  function all(sel, from) { return Array.prototype.slice.call((from || D).querySelectorAll(sel)); }
  function frame() { return one(".act[data-phase]"); }
  function asking(f) { return !!f && f.getAttribute("data-phase") === "ask"; }
  function interactive(el) {
    return !!el && !!el.closest && !!el.closest("button, a, input, textarea, select, summary, [contenteditable]");
  }

  /* The gold action. */
  function primary(f) { return one("[data-act-primary]", f); }
  function enable(f, on) { var b = primary(f); if (b) b.disabled = !on; }

  /* Audio. */
  function hint(f) { var h = one('input[name="hinted"]', f); if (h) h.value = "1"; }
  function play(a) {
    if (!a) return;
    var f = frame();
    var replay = a.hasAttribute("data-played");
    var p;
    // Setting currentTime while readyState is HAVE_NOTHING throws (HTML spec).
    // A throw must not skip play(): that is what starts the load.
    try { a.currentTime = 0; } catch (e) { /* not seekable yet */ }
    try { p = a.play(); } catch (e) { return; }
    if (p && p.then) {
      p.then(function () {
        if (!a.hasAttribute("data-prompt")) return;
        if (replay && asking(f)) hint(f);
        a.setAttribute("data-played", "");
      }, function () {});
    }
  }
  function clipOf(el) {
    var id = el.getAttribute("data-audio");
    return (id && D.getElementById(id)) || one("audio", el);
  }

  /* Pick. */
  function pick(btn) {
    var f = frame();
    if (!asking(f) || btn.getAttribute("aria-disabled") === "true") return;
    all("[data-pick]", f).forEach(function (b) {
      var on = b === btn;
      b.setAttribute("data-state", on ? "selected" : "default");
      if (on) b.setAttribute("aria-pressed", "true"); else b.removeAttribute("aria-pressed");
    });
    var c = one('input[name="choice"]', f);
    if (c) c.value = btn.getAttribute("data-pick");
    enable(f, true);
  }

  /* Match. A tap selects a tile. When a left and a right tile are both selected they
     lock as a pair. A locked tile does not move again: Start over clears the board. */
  var selL = null, selR = null;
  function mark(t, on) {
    if (!t) return;
    t.setAttribute("data-state", on ? "selected" : "default");
    if (on) t.setAttribute("aria-pressed", "true"); else t.removeAttribute("aria-pressed");
  }
  function name(t, extra) {
    var base = t.getAttribute("data-label") || "";
    t.setAttribute("aria-label", extra ? base + ", " + extra : base);
  }
  function lock(f) {
    var i = selL.getAttribute("data-index");
    var n = all('[data-tile="l"][data-locked]', f).length + 1;
    var field = one('input[name="pair_' + i + '"]', f);
    if (field) field.value = selR.getAttribute("data-index");
    var l = selL, r = selR;
    [l, r].forEach(function (t) {
      mark(t, false);
      t.setAttribute("data-locked", "");
      t.setAttribute("aria-disabled", "true");
      var p = one("[data-pair]", t);
      if (p) p.textContent = String(n);
    });
    name(l, "paired with " + r.getAttribute("aria-keyshortcuts") + ", pair " + n);
    name(r, "paired with " + l.getAttribute("aria-keyshortcuts") + ", pair " + n);
    selL = selR = null;
    var total = all('[data-tile="l"]', f).length;
    enable(f, all('[data-tile="l"][data-locked]', f).length === total);
  }
  function tile(t) {
    var f = frame();
    if (!asking(f) || t.hasAttribute("data-locked")) return;
    var left = t.getAttribute("data-tile") === "l";
    var cur = left ? selL : selR;
    if (cur) mark(cur, false);
    if (cur === t) {
      if (left) selL = null; else selR = null;
      return;
    }
    mark(t, true);
    if (left) selL = t; else selR = t;
    var clip = one("audio", t);
    if (clip) play(clip);
    if (selL && selR) lock(f);
  }
  function reset() {
    var f = frame();
    if (!f) return;
    selL = selR = null;
    all("[data-tile]", f).forEach(function (t) {
      t.removeAttribute("data-locked");
      t.removeAttribute("aria-disabled");
      mark(t, false);
      name(t, "");
      var p = one("[data-pair]", t);
      if (p) p.textContent = "";
    });
    all('input[name^="pair_"]', f).forEach(function (i) { i.value = ""; });
    enable(f, false);
  }

  D.addEventListener("click", function (e) {
    var t = e.target;
    if (!t || !t.closest) return;
    var orb = t.closest(".c-orb[data-src]");
    if (orb) {
      e.preventDefault();
      play(clipOf(orb));
      return;
    }
    var opt = t.closest("[data-pick]");
    if (opt) { pick(opt); return; }
    var tl = t.closest("[data-tile]");
    if (tl) { tile(tl); return; }
    if (t.closest("[data-act-reset]")) reset();
  });

  D.addEventListener("keydown", function (e) {
    if (e.ctrlKey || e.metaKey || e.altKey || e.isComposing) return;
    var f = frame();
    if (!f) return;
    var target = e.target;
    var typing = !!target && !!target.closest && !!target.closest("input, textarea, select, [contenteditable]");
    if (typing) return;
    if (e.key === "Enter") {
      // Enter on a focused option takes it. On an option that is already taken, or a
      // locked tile, it takes the gold action, so a keyboard learner can press Enter twice.
      var taken = !!target && !!target.closest && !!target.closest('[data-pick][aria-pressed="true"], [data-tile][data-locked]');
      if (interactive(target) && !taken) return;
      var go = primary(f);
      if (go && !go.disabled) { e.preventDefault(); go.click(); }
      return;
    }
    if (e.key === " " || e.key === "Spacebar") {
      if (interactive(target)) return;
      var orb = one(".c-orb[data-hint]", f) || one(".c-orb[data-src]", f);
      if (orb) { e.preventDefault(); play(clipOf(orb)); }
      return;
    }
    if (e.key.length !== 1) return;
    var el = one('[data-hotkey="' + e.key.toLowerCase() + '"]', f);
    if (el && el.getAttribute("aria-disabled") !== "true") {
      e.preventDefault();
      // Focus follows the key, so Enter then takes the gold action from the option.
      if (el.focus) el.focus({ preventScroll: true });
      el.click();
    }
  });

  /* The Hebrew input turns Check on once there is something to check. */
  D.addEventListener("input", function (e) {
    var t = e.target;
    if (!t || !t.matches || !t.matches("[data-hk-input]")) return;
    var f = frame();
    if (asking(f)) enable(f, t.value.trim() !== "");
  });

  /* On arrival of a screen: start the prompt clip, and put focus where the next key
     should land. The feedback sheet takes focus so it is announced, and so Enter and
     Space still reach the document handler. */
  function arrive() {
    var f = frame();
    selL = selR = null;
    if (!f || f.hasAttribute("data-arrived")) return;
    f.setAttribute("data-arrived", "");
    all("audio[data-autoplay]", f).forEach(play);
    if (asking(f)) {
      var input = one("[data-hk-input]", f);
      if (input) input.focus({ preventScroll: true });
    } else {
      var sheet = one(".c-sheet", f);
      if (sheet) {
        sheet.setAttribute("tabindex", "-1");
        sheet.focus({ preventScroll: true });
      }
    }
  }
  D.addEventListener("htmx:afterSettle", arrive);
  if (D.readyState === "loading") D.addEventListener("DOMContentLoaded", arrive);
  else arrive();
})(typeof window !== "undefined" ? window : globalThis);
