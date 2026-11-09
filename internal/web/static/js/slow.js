/* Shared 0.75× clip pace (INIT-036/SPEC-007). No dependency.
   clips.js, dialogue.js, activity.js, follow.js, and placement.js call apply
   before they play. [data-replay] and [data-slow] are the learner controls:
   Replay follows the stored setting; Slower toggles it and plays at the new rate.
   The choice lives in localStorage only. Autoplay uses apply with no override,
   so it stays 1× until the learner turns slow on. */
(function (g) {
  "use strict";
  if (g.HebrewSlow) return;
  var KEY = "hebrew-slow";
  var SLOW = 0.75;
  var D = g.document;

  function on() {
    try { return g.localStorage.getItem(KEY) === "1"; }
    catch (e) { return false; }
  }

  function setOn(v) {
    try { g.localStorage.setItem(KEY, v ? "1" : "0"); }
    catch (e) { /* private mode */ }
    paint();
  }

  function paint() {
    if (!D) return;
    var pressed = on() ? "true" : "false";
    var nodes = D.querySelectorAll("[data-slow]");
    for (var i = 0; i < nodes.length; i++) {
      nodes[i].setAttribute("aria-pressed", pressed);
    }
  }

  function apply(audio, rate) {
    if (!audio) return;
    var r = rate != null ? rate : (on() ? SLOW : 1);
    try {
      audio.playbackRate = r;
      if ("preservesPitch" in audio) audio.preservesPitch = true;
      else if ("mozPreservesPitch" in audio) audio.mozPreservesPitch = true;
      else if ("webkitPreservesPitch" in audio) audio.webkitPreservesPitch = true;
    } catch (e) { /* some engines reject a rate change while loading */ }
  }

  function audioOf(btn) {
    if (!btn || !D) return null;
    var id = btn.getAttribute("data-audio");
    if (id) {
      var named = D.getElementById(id);
      if (named) return named;
    }
    var scope = btn.closest("[data-pace]") ||
      btn.closest("[data-line]") ||
      btn.closest(".clip") ||
      btn.closest(".act") ||
      btn.closest("#activity") ||
      btn.closest(".word-page") ||
      D;
    var aid = scope.getAttribute && scope.getAttribute("data-pace-audio");
    if (aid) {
      var a = D.getElementById(aid);
      if (a) return a;
    }
    return scope.querySelector ? scope.querySelector("audio") : null;
  }

  function play(audio, rate) {
    if (!audio) return;
    apply(audio, rate);
    try { audio.currentTime = 0; } catch (e) { /* not seekable yet */ }
    var p;
    try { p = audio.play(); } catch (e) { return; }
    if (p && p.catch) p.catch(function () { /* same silence as before: no new error state */ });
  }

  if (D) {
    D.addEventListener("click", function (ev) {
      var t = ev.target;
      if (!t || !t.closest) return;
      // Dialogue owns its own Replay/Slower so playLine can silence other clips first.
      if (t.closest("[data-dialogue]")) return;
      var slow = t.closest("[data-slow]");
      if (slow) {
        ev.preventDefault();
        var next = !on();
        setOn(next);
        play(audioOf(slow), next ? SLOW : 1);
        return;
      }
      var replay = t.closest("[data-replay]");
      if (replay) {
        ev.preventDefault();
        play(audioOf(replay));
      }
    });
    function boot() { paint(); }
    if (D.readyState === "loading") D.addEventListener("DOMContentLoaded", boot);
    else boot();
    D.addEventListener("htmx:afterSettle", paint);
    D.addEventListener("htmx:load", paint);
  }

  g.HebrewSlow = {
    on: on,
    setOn: setOn,
    apply: apply,
    paint: paint,
    play: play,
    rate: function () { return on() ? SLOW : 1; }
  };
})(typeof window !== "undefined" ? window : globalThis);
