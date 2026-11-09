/* Follow along a story. The line clip is one recording of the whole text.
   While it plays, the word the recording has reached is marked, and the page
   keeps that word in view. The control remembers the choice on this browser. */
(function (g) {
  "use strict";
  if (g.__follow) return;
  g.__follow = 1;
  var D = g.document;
  if (!D) return;
  var key = "hebrew-follow";

  function on() {
    try { return g.localStorage.getItem(key) !== "0"; }
    catch (e) { return true; }
  }

  function paint() {
    var pressed = on() ? "true" : "false";
    var btns = D.querySelectorAll("[data-follow]");
    for (var i = 0; i < btns.length; i++) btns[i].setAttribute("aria-pressed", pressed);
    if (pressed === "false") clearAll();
  }

  function wordsOf(audio) {
    var act = audio.closest && (audio.closest("#activity") || audio.closest(".activity"));
    if (!act) return [];
    return act.querySelectorAll(".wline > details.wcard");
  }

  function clearAll() {
    var marked = D.querySelectorAll(".wcard.follow-on");
    for (var i = 0; i < marked.length; i++) marked[i].classList.remove("follow-on");
  }

  function mark(audio) {
    if (!on()) return;
    var words = wordsOf(audio);
    var n = words.length;
    if (!n || !audio.duration || !isFinite(audio.duration)) return;
    var i = Math.min(n - 1, Math.floor((audio.currentTime / audio.duration) * n));
    if (audio.__at === i) return;
    audio.__at = i;
    for (var k = 0; k < n; k++) words[k].classList.toggle("follow-on", k === i);
    if (words[i] && words[i].scrollIntoView) {
      words[i].scrollIntoView({ block: "nearest", inline: "nearest" });
    }
  }

  function bind(root) {
    var scope = root && root.querySelectorAll ? root : D;
    var nodes = [];
    if (scope.matches && scope.matches("#line-clip")) nodes.push(scope);
    if (scope.querySelectorAll) {
      var found = scope.querySelectorAll("#line-clip");
      for (var i = 0; i < found.length; i++) nodes.push(found[i]);
    }
    for (var j = 0; j < nodes.length; j++) {
      var audio = nodes[j];
      // INIT-036/SPEC-007: line clip follows the shared slow setting.
      if (g.HebrewSlow && g.HebrewSlow.apply) g.HebrewSlow.apply(audio);
      if (audio.__follow) continue;
      audio.__follow = 1;
      audio.addEventListener("timeupdate", function (ev) { mark(ev.currentTarget); });
      audio.addEventListener("seeked", function (ev) {
        ev.currentTarget.__at = -1;
        mark(ev.currentTarget);
      });
      audio.addEventListener("ended", function (ev) {
        ev.currentTarget.__at = -1;
        clearAll();
      });
    }
    paint();
  }

  D.addEventListener("click", function (ev) {
    var t = ev.target;
    var btn = t && t.closest && t.closest("[data-follow]");
    if (!btn) return;
    var next = !on();
    try { g.localStorage.setItem(key, next ? "1" : "0"); }
    catch (e) {}
    paint();
  });

  D.addEventListener("htmx:load", function (ev) {
    var elt = ev.detail && ev.detail.elt ? ev.detail.elt : ev.target;
    bind(elt || D);
  });

  bind(D);
})(globalThis);
