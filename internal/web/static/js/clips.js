/* Lesson clip playback. No dependency. Loaded by a lesson step that has a clip.
   An orb with data-clip plays the audio element named by data-audio.
   Starting one clip pauses every other audio on the page, so two clips never
   play together. Nothing here plays by itself.
   The playing orb takes data-playing from motion.js. That pulse lives in
   motion.css, inside prefers-reduced-motion: no-preference. This file does
   not animate, and it does not start a clip the learner did not ask for. */
(function (g) {
  "use strict";
  var D = g.document;
  if (!D || g.__clips) return;
  g.__clips = 1;

  function stopOthers(except) {
    Array.prototype.forEach.call(D.querySelectorAll("audio"), function (a) {
      if (a === except) return;
      a.pause();
      try { a.currentTime = 0; } catch (e) { /* not seekable yet */ }
    });
  }

  function play(a) {
    stopOthers(a);
    var p;
    try {
      a.currentTime = 0;
      p = a.play();
    } catch (e) {
      return;
    }
    if (p && p.catch) p.catch(function () {});
  }

  D.addEventListener("click", function (ev) {
    var t = ev.target;
    if (!t || !t.closest) return;
    var orb = t.closest(".c-orb[data-clip]");
    if (!orb) return;
    var id = orb.getAttribute("data-audio");
    var clip = id && D.getElementById(id);
    if (!clip) return;
    ev.preventDefault();
    play(clip);
  });
})(window);
