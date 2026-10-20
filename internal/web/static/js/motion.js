/* Motion hooks (INIT-015/SPEC-009). No dependency. Loaded after shell.js.
   1. View transitions run only when reduced motion is not set.
   2. data-playing on an orb while its audio plays (CSS pulses it).
      An orb names its audio with data-audio="<audio id>", or sits in the same
      [data-audio-scope] (default: the parent) as the audio element.
   3. [data-advance="800"] on a form or button: submit or click it after that
      many ms (default 800). Nothing waits on this timer. A tap, a key, or any
      htmx request cancels it, so input is taken at once. */
(function () {
  var mq = matchMedia("(prefers-reduced-motion: reduce)");
  function views() {
    if (window.htmx) htmx.config.globalViewTransitions = !mq.matches;
  }
  views();
  mq.addEventListener("change", views);

  /* A view transition makes the whole page untappable for its length (about
     300ms in Chromium: hit-testing returns <html>, and turning pointer events
     off on the overlay does not change it). So a swap that brings in answer options runs without one, and
     the first tap on an option always registers. data-no-transition on any
     element in the response does the same for other tap targets. */
  document.addEventListener("htmx:beforeTransition", function (e) {
    var d = e.detail || {};
    var html = d.serverResponse || (d.xhr && d.xhr.responseText) || "";
    if (/class="[^"]*\bc-answer\b|data-no-transition/.test(html)) e.preventDefault();
  });

  function orbs(a) {
    if (a.id) return document.querySelectorAll('.c-orb[data-audio="' + CSS.escape(a.id) + '"]');
    var p = a.closest("[data-audio-scope]") || a.parentElement;
    return p ? p.querySelectorAll(".c-orb") : [];
  }
  function playing(on) {
    return function (e) {
      if (!e.target || e.target.tagName !== "AUDIO") return;
      orbs(e.target).forEach(function (o) {
        if (on) o.setAttribute("data-playing", "");
        else o.removeAttribute("data-playing");
      });
    };
  }
  document.addEventListener("playing", playing(true), true);
  ["pause", "ended", "emptied", "abort", "error"].forEach(function (n) {
    document.addEventListener(n, playing(false), true);
  });

  var timers = [];
  function cancel() {
    timers.forEach(clearTimeout);
    timers = [];
  }
  function arm() {
    document.querySelectorAll("[data-advance]").forEach(function (el) {
      if (el.hasAttribute("data-advance-armed")) return;
      el.setAttribute("data-advance-armed", "");
      var ms = parseInt(el.getAttribute("data-advance"), 10) || 800;
      timers.push(setTimeout(function () {
        if (!el.isConnected) return;
        if (el.requestSubmit) el.requestSubmit();
        else el.click();
      }, ms));
    });
  }
  ["pointerdown", "keydown", "submit", "htmx:beforeRequest"].forEach(function (n) {
    document.addEventListener(n, cancel, true);
  });
  document.addEventListener("DOMContentLoaded", arm);
  document.addEventListener("htmx:afterSettle", arm);
})();
