/* Drop a Hebrew choice onto the blank. A tap still submits the same button.
   The value is the lexeme id the button already posts. */
(function (g) {
  "use strict";
  if (g.__drag) return;
  g.__drag = 1;
  var D = g.document;
  if (!D) return;
  var dragged = null;
  var dropped = false;

  function gapIn(node) {
    var act = node && node.closest && node.closest("#activity");
    return act && act.querySelector(".word-block--gap");
  }

  function choice(node) {
    if (!node || !node.closest) return null;
    return node.closest(".choices button[name='answer']");
  }

  function arm(root) {
    var scope = root && root.querySelectorAll ? root : D;
    var nodes = [];
    if (scope.matches && scope.matches(".choices button[name='answer']")) nodes.push(scope);
    if (scope.querySelectorAll) {
      var found = scope.querySelectorAll(".choices button[name='answer']");
      for (var i = 0; i < found.length; i++) nodes.push(found[i]);
    }
    for (var j = 0; j < nodes.length; j++) {
      if (gapIn(nodes[j])) nodes[j].draggable = true;
    }
  }

  function clearGaps() {
    var gaps = D.querySelectorAll(".word-block--gap.is-drop");
    for (var i = 0; i < gaps.length; i++) gaps[i].classList.remove("is-drop");
  }

  D.addEventListener("dragstart", function (ev) {
    var btn = choice(ev.target);
    if (!btn || !gapIn(btn) || !ev.dataTransfer) return;
    dragged = btn;
    dropped = false;
    ev.dataTransfer.setData("text/plain", btn.value || "");
    ev.dataTransfer.effectAllowed = "copy";
  });

  D.addEventListener("dragover", function (ev) {
    var t = ev.target;
    var gap = t && t.closest && t.closest(".word-block--gap");
    if (!gap || !dragged) return;
    ev.preventDefault();
    gap.classList.add("is-drop");
  });

  D.addEventListener("dragleave", function (ev) {
    var t = ev.target;
    var gap = t && t.closest && t.closest(".word-block--gap");
    if (gap) gap.classList.remove("is-drop");
  });

  D.addEventListener("drop", function (ev) {
    var t = ev.target;
    var gap = t && t.closest && t.closest(".word-block--gap");
    if (!gap || !dragged) return;
    ev.preventDefault();
    dropped = true;
    var btn = dragged;
    clearGaps();
    var form = btn.form;
    if (form && form.requestSubmit) form.requestSubmit(btn);
    else btn.click();
  });

  D.addEventListener("dragend", function () {
    clearGaps();
    if (!dropped) dragged = null;
  });

  D.addEventListener("click", function (ev) {
    if (!dropped) return;
    var btn = choice(ev.target);
    if (btn && btn === dragged) {
      ev.preventDefault();
      ev.stopPropagation();
    }
    dropped = false;
    dragged = null;
  }, true);

  D.addEventListener("htmx:load", function (ev) {
    var elt = ev.detail && ev.detail.elt ? ev.detail.elt : ev.target;
    arm(elt || D);
  });

  arm(D);
})(globalThis);
