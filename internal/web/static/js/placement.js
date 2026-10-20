/* Placement screens (INIT-015/SPEC-012). No dependency. Loaded by the placement page, after motion.js.
   The server renders every screen and decides every outcome. This file only takes input and plays audio:
   - the clip of a listening or letters screen plays when the screen arrives. If the browser blocks
     autoplay the orb says "Tap to listen" and pulses; the first tap plays it. If the clip cannot be
     loaded the screen asks the server for another word (the hidden replace form), and never stays
     silent without saying so;
   - Replay and Slower (0.75x) play the clip again;
   - keys: 1 to 4 pick, ? is "I don't know", Space replays. After a pick, any tap or key moves on at
     once; otherwise the screen moves on by itself after 0.8s (motion.js, data-advance);
   - the close control asks before it leaves (a native dialog). The run is saved either way;
   - the review's speaker buttons play their clip.
   Nothing here knows whether an answer was right: the server never says. */
(function (g) {
  "use strict";
  var D = g.document;
  if (!D || g.__pl) return;
  g.__pl = 1;

  function one(sel, from) { return (from || D).querySelector(sel); }
  function all(sel, from) { return Array.prototype.slice.call((from || D).querySelectorAll(sel)); }
  function root() { return one("#placement.pl"); }
  function clip() { return one("audio[data-pl-clip]"); }
  function disabled(el) { return !!el && el.getAttribute("aria-disabled") === "true"; }

  /* The clip's state is on the root (data-audio-state) and in the status line under the orb. */
  var SAY = {
    idle: "Listen, then pick.",
    playing: "Listening…",
    ended: "Tap the orb, or press Space, to hear it again.",
    blocked: "Tap to listen.",
    error: "We couldn’t play this clip."
  };
  function state(s, text) {
    var r = root();
    if (!r || !clip()) return;
    r.setAttribute("data-audio-state", s === "ended" ? "idle" : s);
    var el = one("[data-pl-hint-text]", r);
    if (el) el.textContent = text || SAY[s] || "";
  }

  function failed(a) {
    if (!a || a.hasAttribute("data-pl-failed")) return;
    a.setAttribute("data-pl-failed", "");
    var swap = one("form[data-pl-replace]");
    if (swap) {
      state("error", "We couldn’t play that clip. Finding another word…");
      if (swap.requestSubmit) swap.requestSubmit();
      return;
    }
    state("error", "We couldn’t play this clip. Tap “I don’t know” to move on.");
  }
  function refused(err) {
    var a = clip();
    if (!err) return;
    if (err.name === "NotAllowedError") state("blocked");
    else if (err.name !== "AbortError" && a) failed(a);
  }
  function play(a, rate) {
    if (!a || a.hasAttribute("data-pl-failed")) return;
    var p;
    try {
      a.playbackRate = rate || 1;
      a.currentTime = 0;
      p = a.play();
    } catch (e) { return; }
    if (p && p.then) p.then(null, refused);
  }

  /* Audio events do not bubble, so they are taken in the capture phase. */
  D.addEventListener("playing", function (e) {
    var a = e.target;
    if (!a || a.tagName !== "AUDIO") return;
    if (a === clip()) state("playing");
    var b = a.id && one('[data-pl-listen="' + CSS.escape(a.id) + '"]');
    if (b) b.setAttribute("data-playing", "");
  }, true);
  ["ended", "pause"].forEach(function (n) {
    D.addEventListener(n, function (e) {
      var a = e.target;
      if (!a || a.tagName !== "AUDIO") return;
      if (a === clip() && !a.hasAttribute("data-pl-failed")) state("ended");
      var b = a.id && one('[data-pl-listen="' + CSS.escape(a.id) + '"]');
      if (b) b.removeAttribute("data-playing");
    }, true);
  });
  D.addEventListener("error", function (e) {
    if (e.target && e.target === clip()) failed(e.target);
  }, true);

  /* A tap after the pick, or the timer, moves on. */
  function picked() { var r = root(); return !!r && r.getAttribute("data-phase") === "picked"; }
  function advance() {
    var f = one("form[data-pl-advance]");
    if (!f || f.hasAttribute("data-advancing")) return;
    f.setAttribute("data-advancing", "");
    if (f.requestSubmit) f.requestSubmit(); else f.submit();
  }

  /* Picking: show the choice at once. The server draws the same state a moment later. */
  function mark(btn) {
    all("[data-pl-option]").forEach(function (b) {
      if (b === btn) {
        b.setAttribute("data-state", "selected");
        b.setAttribute("aria-pressed", "true");
      } else {
        b.setAttribute("data-dim", "");
      }
    });
  }

  D.addEventListener("click", function (e) {
    var t = e.target;
    if (!t || !t.closest || !root()) return;
    if (picked() && !t.closest("[data-pl-close], dialog")) {
      e.preventDefault();
      advance();
      return;
    }
    if (t.closest("[data-pl-close]")) {
      var d = one("dialog[data-pl-confirm]");
      if (d && d.showModal && !d.open) {
        e.preventDefault();
        d.showModal();
      }
      return;
    }
    if (t.closest("[data-pl-keep]")) {
      var dlg = t.closest("dialog");
      if (dlg && dlg.close) dlg.close();
      return;
    }
    var orb = t.closest("[data-pl-play]");
    if (orb) {
      e.preventDefault();
      if (!disabled(orb)) play(clip(), 1);
      return;
    }
    var again = t.closest("[data-pl-replay]");
    if (again) {
      e.preventDefault();
      if (!disabled(again)) play(clip(), 1);
      return;
    }
    var slow = t.closest("[data-pl-slower]");
    if (slow) {
      e.preventDefault();
      if (!disabled(slow)) play(clip(), 0.75);
      return;
    }
    var listen = t.closest("[data-pl-listen]");
    if (listen) {
      e.preventDefault();
      var a = D.getElementById(listen.getAttribute("data-pl-listen"));
      all("audio").forEach(function (o) { if (o !== a) o.pause(); });
      if (a) { try { a.currentTime = 0; a.play(); } catch (x) {} }
      return;
    }
    var opt = t.closest("[data-pl-option]");
    if (opt && opt.getAttribute("type") === "submit" && !disabled(opt)) mark(opt);
  });

  /* When the dialog closes on a screen that was about to move on, move on. */
  D.addEventListener("close", function (e) {
    if (e.target && e.target.matches && e.target.matches("dialog[data-pl-confirm]") && picked()) advance();
  }, true);

  D.addEventListener("keydown", function (e) {
    if (e.ctrlKey || e.metaKey || e.altKey || e.isComposing) return;
    var r = root();
    if (!r || one("dialog[open]")) return;
    var k = e.key;
    if (picked()) {
      if (k === "Tab" || k === "Shift" || k === "Escape" || k === "Control" || k === "Alt" || k === "Meta") return;
      e.preventDefault();
      advance();
      return;
    }
    var t = e.target;
    if (t && t.closest && t.closest("input, textarea, select, [contenteditable]")) return;
    var screen = r.getAttribute("data-screen");
    if (screen !== "letters" && screen !== "listening" && screen !== "reading") return;
    if (k === " " || k === "Spacebar") {
      // Space on a button or a link keeps its own meaning. Anywhere else it replays.
      if (t && t.closest && t.closest("button, a, summary")) return;
      if (clip()) { e.preventDefault(); play(clip(), 1); }
      return;
    }
    if (k === "?") {
      var idk = one('button[name="dont_know"]', r);
      if (idk) { e.preventDefault(); idk.click(); }
      return;
    }
    if (/^[1-4]$/.test(k)) {
      var o = one('[data-hotkey="' + k + '"]', r);
      if (o && !disabled(o)) { e.preventDefault(); o.click(); }
    }
  });

  /* On arrival of a screen: start the clip, and put focus where the next key should land. */
  function arrive() {
    var r = root();
    if (!r || r.hasAttribute("data-pl-arrived")) return;
    r.setAttribute("data-pl-arrived", "");
    var a = clip();
    if (a && !picked()) {
      if (a.error) failed(a);
      else {
        var p;
        try { p = a.play(); } catch (e) { p = null; }
        if (p && p.then) p.then(null, refused);
      }
    }
    var input = one("[data-hk-input]", r);
    if (input) input.focus({ preventScroll: true });
  }
  D.addEventListener("htmx:afterSettle", arrive);
  D.addEventListener("htmx:load", arrive);
  if (D.readyState === "loading") D.addEventListener("DOMContentLoaded", arrive);
  else arrive();
})(typeof window !== "undefined" ? window : globalThis);
