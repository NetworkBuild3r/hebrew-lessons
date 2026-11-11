/* Build-the-sentence: drag or tap chips into an RTL row, tap to send back.
   Posts order=N chip indices in visual right-to-left (DOM) order.
   INIT-029/SPEC-004. Does not touch cloze drag.js. */
(function (g) {
  "use strict";
  if (g.__order) return;
  g.__order = 1;
  var D = g.document;
  if (!D) return;

  var dragged = null;

  function rootOf(node) {
    return node && node.closest && node.closest("[data-order]");
  }

  function chipOf(node) {
    return node && node.closest && node.closest("[data-order-chip]");
  }

  function rowOf(root) {
    return root && root.querySelector("[data-order-row]");
  }

  function bankOf(root) {
    return root && root.querySelector("[data-order-bank]");
  }

  function fieldsOf(root) {
    return root && root.querySelector("[data-order-fields]");
  }

  function checkOf(root) {
    return root && root.querySelector("[data-order-check]");
  }

  function placed(root) {
    var row = rowOf(root);
    if (!row) return [];
    return Array.prototype.slice.call(row.querySelectorAll("[data-order-chip]"));
  }

  function bankChips(root) {
    var bank = bankOf(root);
    if (!bank) return [];
    return Array.prototype.slice.call(bank.querySelectorAll("[data-order-chip]"));
  }

  function allChips(root) {
    return bankChips(root).concat(placed(root));
  }

  function sync(root) {
    var fields = fieldsOf(root);
    var check = checkOf(root);
    var row = rowOf(root);
    if (!fields || !check || !row) return;
    fields.innerHTML = "";
    var chips = placed(root);
    for (var i = 0; i < chips.length; i++) {
      var input = D.createElement("input");
      input.type = "hidden";
      input.name = "order";
      input.value = chips[i].getAttribute("data-order-i") || "";
      fields.appendChild(input);
    }
    var total = allChips(root).length;
    var complete = total > 0 && chips.length === total;
    check.disabled = false;
    check.classList.toggle("is-waiting", !complete);
    row.classList.toggle("is-drop", false);
    row.setAttribute("aria-label", chips.length ? "Your sentence, " + chips.length + " words" : "Your sentence");
  }

  function place(chip, root, before) {
    if (!chip || !root) return;
    var row = rowOf(root);
    if (!row) return;
    if (before && before.parentNode === row) {
      row.insertBefore(chip, before);
    } else {
      row.appendChild(chip);
    }
    chip.setAttribute("aria-grabbed", "false");
    sync(root);
    chip.focus();
  }

  function unplace(chip, root) {
    if (!chip || !root) return;
    var bank = bankOf(root);
    if (!bank) return;
    bank.appendChild(chip);
    chip.setAttribute("aria-grabbed", "false");
    sync(root);
    chip.focus();
  }

  function focusable(root) {
    return allChips(root).filter(function (c) {
      return !c.disabled;
    });
  }

  function moveFocus(chip, dir) {
    var root = rootOf(chip);
    if (!root) return;
    var list = focusable(root);
    var i = list.indexOf(chip);
    if (i < 0) return;
    var next = list[i + dir];
    if (next) next.focus();
  }

  function arm(scope) {
    var roots = [];
    var base = scope && scope.querySelectorAll ? scope : D;
    if (base.matches && base.matches("[data-order]")) roots.push(base);
    if (base.querySelectorAll) {
      var found = base.querySelectorAll("[data-order]");
      for (var i = 0; i < found.length; i++) roots.push(found[i]);
    }
    for (var j = 0; j < roots.length; j++) sync(roots[j]);
  }

  D.addEventListener("dragstart", function (ev) {
    var chip = chipOf(ev.target);
    if (!chip || !rootOf(chip) || !ev.dataTransfer) return;
    dragged = chip;
    chip.setAttribute("aria-grabbed", "true");
    ev.dataTransfer.setData("text/plain", chip.getAttribute("data-order-i") || "");
    ev.dataTransfer.effectAllowed = "move";
  });

  D.addEventListener("dragend", function () {
    if (dragged) dragged.setAttribute("aria-grabbed", "false");
    dragged = null;
    var rows = D.querySelectorAll("[data-order-row].is-drop");
    for (var i = 0; i < rows.length; i++) rows[i].classList.remove("is-drop");
  });

  D.addEventListener("dragover", function (ev) {
    var t = ev.target;
    var row = t && t.closest && t.closest("[data-order-row]");
    var overChip = chipOf(t);
    if (!dragged) return;
    if (row) {
      ev.preventDefault();
      row.classList.add("is-drop");
      return;
    }
    if (overChip && overChip.parentNode && overChip.parentNode.hasAttribute("data-order-row")) {
      ev.preventDefault();
      overChip.parentNode.classList.add("is-drop");
    }
  });

  D.addEventListener("dragleave", function (ev) {
    var row = ev.target && ev.target.closest && ev.target.closest("[data-order-row]");
    if (row) row.classList.remove("is-drop");
  });

  D.addEventListener("drop", function (ev) {
    var t = ev.target;
    var root = rootOf(t);
    if (!dragged || !root) return;
    var row = t.closest && t.closest("[data-order-row]");
    var overChip = chipOf(t);
    if (overChip && overChip.parentNode && overChip.parentNode.hasAttribute("data-order-row")) {
      ev.preventDefault();
      place(dragged, root, overChip);
      dragged = null;
      return;
    }
    if (row) {
      ev.preventDefault();
      place(dragged, root, null);
      dragged = null;
    }
  });

  D.addEventListener("click", function (ev) {
    var chip = chipOf(ev.target);
    if (!chip) return;
    var root = rootOf(chip);
    if (!root) return;
    if (chip.parentNode && chip.parentNode.hasAttribute("data-order-row")) {
      unplace(chip, root);
      return;
    }
    if (chip.parentNode && chip.parentNode.hasAttribute("data-order-bank")) {
      place(chip, root, null);
    }
  });

  D.addEventListener("keydown", function (ev) {
    var chip = chipOf(ev.target);
    var root = rootOf(ev.target);
    if (!root) return;

    if (ev.key === "ArrowRight" || ev.key === "ArrowLeft") {
      if (!chip) return;
      // DOM order is visual RTL order; Right moves toward earlier DOM (right side).
      var dir = ev.key === "ArrowRight" ? -1 : 1;
      ev.preventDefault();
      moveFocus(chip, dir);
      return;
    }
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      if (!chip) return;
      ev.preventDefault();
      moveFocus(chip, ev.key === "ArrowDown" ? 1 : -1);
      return;
    }
    if (ev.key !== "Enter" && ev.key !== " ") return;
    if (!chip) return;
    ev.preventDefault();
    if (chip.parentNode && chip.parentNode.hasAttribute("data-order-row")) {
      unplace(chip, root);
    } else if (chip.parentNode && chip.parentNode.hasAttribute("data-order-bank")) {
      place(chip, root, null);
    }
  });

  // Write the hidden fields before htmx reads the form. A disabled Check
  // looked the same as a live one, so the click appeared to do nothing.
  D.addEventListener("submit", function (ev) {
    var form = ev.target;
    if (!form || !form.querySelector) return;
    var root = form.querySelector("[data-order]");
    if (!root) return;
    sync(root);
    var chips = placed(root);
    var total = allChips(root).length;
    if (total === 0 || chips.length !== total) {
      ev.preventDefault();
      ev.stopImmediatePropagation();
      var hint = root.querySelector(".order-hint");
      if (hint) hint.textContent = "Place every word in the row, then check.";
    }
  }, true);

  D.addEventListener("htmx:load", function (ev) {
    var elt = ev.detail && ev.detail.elt ? ev.detail.elt : ev.target;
    arm(elt || D);
  });

  arm(D);
})(globalThis);
