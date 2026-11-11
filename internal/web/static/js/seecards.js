// The See step of a lesson. On a phone it shows one new word at a time and the button reads
// "Next word" until the last card, where it is the lesson's own Continue. On a wide screen
// every card shows and the button is Continue. Without this script every card shows and the
// button is Continue. Safe to load again after an htmx swap.
(() => {
  'use strict';
  if (window.__seeCards) {
    window.__seeCards.scan();
    return;
  }
  const narrow = window.matchMedia('(max-width: 719px)');

  function setup(root) {
    const cards = Array.from(root.querySelectorAll('[data-see-card]'));
    const form = root.closest('#activity');
    const btn = form && form.querySelector('[data-see-next]');
    if (!btn) return;
    const next = btn.dataset.nextLabel || 'Next word';
    const done = btn.dataset.doneLabel || 'Continue';
    let at = 0;

    function paint() {
      if (!narrow.matches || cards.length < 2) {
        delete root.dataset.seeOne;
        cards.forEach((c) => c.removeAttribute('data-current'));
        btn.textContent = done;
        return;
      }
      root.dataset.seeOne = '';
      cards.forEach((c, i) => {
        if (i === at) c.setAttribute('data-current', '');
        else c.removeAttribute('data-current');
      });
      btn.textContent = at < cards.length - 1 ? next : done;
    }

    root.__seePaint = paint;
    if (!btn.dataset.seeBound) {
      btn.dataset.seeBound = '1';
      btn.addEventListener('click', (e) => {
        if (narrow.matches && cards.length > 1 && at < cards.length - 1) {
          e.preventDefault();
          at += 1;
          paint();
        }
      });
    }
    paint();
  }

  function scan() {
    document.querySelectorAll('[data-see]').forEach(setup);
  }

  narrow.addEventListener('change', () => {
    document.querySelectorAll('[data-see]').forEach((r) => r.__seePaint && r.__seePaint());
  });
  document.addEventListener('htmx:afterSwap', scan);
  window.__seeCards = { scan };
  scan();
})();
