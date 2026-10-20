/* Lesson clip orb. No dependency. Loaded by a lesson step that has a clip.
   The server renders button.orb beside a native audio control. This script
   adds class js on html, which shows the orb and hides the native bar.
   Without this script the bar stays. One clip plays at a time.
   Loading is shown only after 150ms, so a clip that starts at once does not flicker.
   Reduced motion is entirely CSS (no spin, no pulse). This file does not animate,
   and it does not start a clip the learner did not ask for. */
(function (g) {
  "use strict";
  var D = g.document;
  if (!D || !D.documentElement) return;
  D.documentElement.classList.add("js");
  if (g.__clips) return;
  g.__clips = 1;

  var LOAD_MS = 150;
  var RETRY = "Audio unavailable, tap to retry";
  var timers = new WeakMap();

  function audioOf(orb) {
    var clip = orb.closest(".clip");
    return clip ? clip.querySelector("audio") : null;
  }

  function orbOf(audio) {
    if (!audio || !audio.closest) return null;
    var clip = audio.closest(".clip");
    return clip ? clip.querySelector(".orb") : null;
  }

  function clearLoad(orb) {
    var t = timers.get(orb);
    if (t) g.clearTimeout(t);
    timers.delete(orb);
  }

  function paint(orb, state) {
    orb.setAttribute("data-state", state);
    if (state === "playing") orb.setAttribute("aria-pressed", "true");
    else orb.removeAttribute("aria-pressed");
    if (state === "loading") orb.setAttribute("aria-busy", "true");
    else orb.removeAttribute("aria-busy");
    var label = orb.getAttribute("data-label");
    if (state === "error") orb.setAttribute("aria-label", RETRY);
    else if (label) orb.setAttribute("aria-label", label);
  }

  function quiet(audio) {
    audio.setAttribute("aria-hidden", "true");
    audio.tabIndex = -1;
  }

  function upgrade() {
    Array.prototype.forEach.call(D.querySelectorAll(".clip > audio"), quiet);
  }

  function idleOthers(except) {
    Array.prototype.forEach.call(D.querySelectorAll(".orb"), function (orb) {
      if (orb === except) return;
      var state = orb.getAttribute("data-state");
      if (state === "playing" || state === "loading") {
        clearLoad(orb);
        paint(orb, "idle");
      }
    });
  }

  function stopOthers(except) {
    Array.prototype.forEach.call(D.querySelectorAll("audio"), function (a) {
      if (a === except) return;
      a.pause();
      try { a.currentTime = 0; } catch (e) { /* not seekable yet */ }
    });
    idleOthers(except ? orbOf(except) : null);
  }

  function fail(orb) {
    clearLoad(orb);
    paint(orb, "error");
  }

  function play(orb) {
    var audio = audioOf(orb);
    if (!audio) {
      fail(orb);
      return;
    }
    var state = orb.getAttribute("data-state");
    if (state === "loading") return;
    if (state === "playing") {
      audio.pause();
      paint(orb, "idle");
      return;
    }
    stopOthers(audio);
    clearLoad(orb);
    paint(orb, "idle");
    var timer = g.setTimeout(function () {
      if (!orb.isConnected) return;
      if (orb.getAttribute("data-state") === "idle") paint(orb, "loading");
    }, LOAD_MS);
    timers.set(orb, timer);
    var pending;
    try {
      audio.currentTime = 0;
      pending = audio.play();
    } catch (e) {
      fail(orb);
      return;
    }
    if (pending && pending.then) {
      pending.then(function () {
        clearLoad(orb);
        paint(orb, "playing");
      }, function () {
        fail(orb);
      });
    }
  }

  D.addEventListener("click", function (ev) {
    var t = ev.target;
    if (!t || !t.closest) return;
    var orb = t.closest(".orb");
    if (!orb || !orb.hasAttribute("data-src")) return;
    ev.preventDefault();
    play(orb);
  });

  function onMedia(name, fn) {
    D.addEventListener(name, function (ev) {
      var a = ev.target;
      if (!a || a.tagName !== "AUDIO") return;
      var orb = orbOf(a);
      if (!orb) return;
      fn(orb, a);
    }, true);
  }

  onMedia("playing", function (orb) {
    clearLoad(orb);
    paint(orb, "playing");
  });
  onMedia("pause", function (orb, a) {
    if (!a.ended && orb.getAttribute("data-state") === "playing") paint(orb, "idle");
  });
  onMedia("ended", function (orb) {
    clearLoad(orb);
    paint(orb, "idle");
  });
  onMedia("error", function (orb) {
    fail(orb);
  });

  upgrade();
  D.addEventListener("htmx:afterSwap", upgrade);
})(window);
