/* Dialogue playback (INIT-015/SPEC-026). No dependency. Loaded by the lesson page.
   The server renders the lines and picks each clip by speaker. This file only plays them:
   - a line's orb ([data-dlg-line]) plays that one line, and stops any other audio first;
   - "Play the conversation" ([data-dlg-all]) plays every line in order, each one after
     the one before has ended. Press it again to stop;
   - a clip that cannot play is skipped, so one bad clip does not stop the conversation;
   - nothing plays by itself: a browser refuses audio the learner did not ask for.
   The playing orb pulses through motion.js (data-playing). The line being heard carries
   data-current, which the stylesheet marks without moving anything. */
(function (g) {
  "use strict";
  var D = g.document;
  if (!D || g.__dlg) return;
  g.__dlg = 1;

  /* Counts up each time playback is stopped or restarted. A sequence only goes on
     while the number it started with is still the current one. */
  var run = 0;

  function clipsIn(root) {
    return Array.prototype.slice.call(root.querySelectorAll("audio[data-dlg-clip]"));
  }
  function mark(root, clip) {
    Array.prototype.forEach.call(D.querySelectorAll("[data-dialogue] [data-current]"), function (li) {
      li.removeAttribute("data-current");
    });
    var li = clip && clip.closest ? clip.closest("[data-line]") : null;
    if (li) li.setAttribute("data-current", "");
  }
  function busy(root, on) {
    var b = root.querySelector("[data-dlg-all]");
    if (!b) return;
    if (on) {
      root.setAttribute("data-dlg-busy", "");
      b.setAttribute("aria-pressed", "true");
    } else {
      root.removeAttribute("data-dlg-busy");
      b.setAttribute("aria-pressed", "false");
    }
  }
  function silence(root) {
    run++;
    Array.prototype.forEach.call(D.querySelectorAll("audio"), function (a) {
      a.pause();
      try { a.currentTime = 0; } catch (e) { /* not seekable yet */ }
    });
    mark(root, null);
    Array.prototype.forEach.call(D.querySelectorAll("[data-dialogue]"), function (r) { busy(r, false); });
  }
  function start(a, onFail) {
    var p;
    try { a.currentTime = 0; p = a.play(); } catch (e) { if (onFail) onFail(); return; }
    if (p && p.catch) p.catch(function () { if (onFail) onFail(); });
  }

  function playLine(root, clip) {
    silence(root);
    mark(root, clip);
    var token = run;
    var clear = function () {
      clip.removeEventListener("ended", clear);
      if (token === run) mark(root, null);
    };
    clip.addEventListener("ended", clear);
    start(clip, clear);
  }

  function playAll(root) {
    var list = clipsIn(root);
    silence(root);
    if (!list.length) return;
    var token = run;
    busy(root, true);
    var i = 0;
    var next = function () {
      if (token !== run) return;
      if (i >= list.length) {
        mark(root, null);
        busy(root, false);
        return;
      }
      var a = list[i++];
      var fired = false;
      var done = function () {
        if (fired) return; /* a failed play() and an error event can both arrive */
        fired = true;
        a.removeEventListener("ended", done);
        a.removeEventListener("error", done);
        next();
      };
      mark(root, a);
      a.addEventListener("ended", done);
      a.addEventListener("error", done);
      start(a, done);
    };
    next();
  }

  D.addEventListener("click", function (ev) {
    var n = ev.target && ev.target.closest ? ev.target : ev.target && ev.target.parentElement;
    if (!n || !n.closest) return;
    var line = n.closest("[data-dlg-line]");
    if (line) {
      var root = line.closest("[data-dialogue]");
      var clip = D.getElementById(line.getAttribute("data-audio"));
      if (root && clip) playLine(root, clip);
      return;
    }
    var all = n.closest("[data-dlg-all]");
    if (all) {
      var r = all.closest("[data-dialogue]");
      if (!r) return;
      if (r.hasAttribute("data-dlg-busy")) silence(r);
      else playAll(r);
    }
  });
})(window);
