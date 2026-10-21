/* Lesson clip orb. No dependency. Loaded by a lesson step that has a clip.
   The server renders button.orb beside a native audio control. This script
   adds class js on html, which shows the orb and sets the native bar to
   display:none. quiet() also sets the hidden attribute, so a swapped bar
   is out of the tab order and the accessibility tree.
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
    var id = orb.getAttribute("data-audio");
    if (id) {
      var named = D.getElementById(id);
      if (named) return named;
    }
    var clip = orb.closest(".clip");
    return clip ? clip.querySelector("audio") : null;
  }

  function orbOf(audio) {
    if (!audio) return null;
    if (audio.id) {
      var named = D.querySelector('.orb[data-audio="' + audio.id + '"]');
      if (named) return named;
    }
    if (!audio.closest) return null;
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
    audio.setAttribute("hidden", "");
    audio.setAttribute("aria-hidden", "true");
    audio.tabIndex = -1;
  }

  function upgrade() {
    Array.prototype.forEach.call(D.querySelectorAll(".clip > audio"), quiet);
  }

  function idleOthers(except) {
    Array.prototype.forEach.call(D.querySelectorAll(".orb"), function (orb) {
      if (orb === except) return;
      clearLoad(orb);
      var state = orb.getAttribute("data-state");
      if (state === "playing" || state === "loading") paint(orb, "idle");
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

  // pause() rejects a play() that has not started yet. Those two names are
  // that interruption, not a broken file, so the orb that was starting stays
  // idle instead of flipping to error.
  function playIgnored(err) {
    var name = err && err.name;
    return name === "AbortError" || name === "NotAllowedError";
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
    var fromError = state === "error";
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
      if (fromError && audio.load) audio.load();
      else {
        try { audio.currentTime = 0; } catch (seekErr) { /* not seekable yet */ }
      }
      pending = audio.play();
    } catch (e) {
      if (playIgnored(e)) {
        clearLoad(orb);
        return;
      }
      fail(orb);
      return;
    }
    if (pending && pending.then) {
      pending.then(function () {
        clearLoad(orb);
        paint(orb, "playing");
      }, function (err) {
        if (playIgnored(err)) {
          clearLoad(orb);
          if (orb.getAttribute("data-state") === "loading") paint(orb, "idle");
          return;
        }
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

  // Click handling is on document, so an orb swapped in later still plays.
  // afterSwap hides the new native bars. A second run of this file (the
  // swapped step includes the script again) returns at g.__clips above,
  // and these listeners from the first run stay in place.
  upgrade();
  D.addEventListener("htmx:afterSwap", upgrade);
  D.addEventListener("htmx:afterSettle", upgrade);
})(window);
