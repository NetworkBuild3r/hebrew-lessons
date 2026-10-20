// Unit tests for static/js/hekbd.js. No dependency: node:assert and a small
// fake DOM. Run by hand with `node internal/web/testdata/keyboard/hekbd.test.js`,
// or by TestKeyboardNode in keyboard_test.go when node is installed.
"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const here = __dirname;
const jsPath = path.join(here, "..", "..", "static", "js", "hekbd.js");
const vectors = JSON.parse(fs.readFileSync(path.join(here, "vectors.json"), "utf8"));
const html = fs.readFileSync(path.join(here, "hebrewkeyboard-en.golden.html"), "utf8");

let failed = 0;
function test(name, fn) {
  try {
    fn();
    console.log("ok   " + name);
  } catch (err) {
    failed++;
    console.log("FAIL " + name + "\n     " + String(err.stack || err).split("\n").join("\n     "));
  }
}

function load() {
  delete require.cache[require.resolve(jsPath)];
  delete globalThis.__hekbd;
  return require(jsPath);
}

// ---- pure functions (no document) ----

const K = load();
const rendered = {};
for (const m of html.matchAll(/data-ch="([^"]+)" data-code="([^"]+)"/g)) rendered[m[2]] = m[1];

test("key map: the rendered keys are the Israeli layout, one table", () => {
  assert.deepEqual(rendered, vectors.keys);
  assert.equal(Object.keys(rendered).length, 27);
});

test("key map: 22 letters and 5 final forms, each once", () => {
  const letters = Object.values(rendered);
  assert.equal(new Set(letters).size, 27);
  for (const f of "\u05da\u05dd\u05df\u05e3\u05e5") assert.ok(letters.includes(f), "final " + f);
  const base = "\u05d0\u05d1\u05d2\u05d3\u05d4\u05d5\u05d6\u05d7\u05d8\u05d9\u05db\u05dc\u05de\u05e0\u05e1\u05e2\u05e4\u05e6\u05e7\u05e8\u05e9\u05ea";
  for (const b of base) assert.ok(letters.includes(b), "letter " + b);
  assert.equal(base.length, 22);
});

test("final letters are typed from their own keys and stay final", () => {
  assert.equal(rendered.KeyL, "\u05da");
  assert.equal(rendered.KeyO, "\u05dd");
  assert.equal(rendered.KeyI, "\u05df");
  assert.equal(rendered.Semicolon, "\u05e3");
  assert.equal(rendered.Period, "\u05e5");
  assert.equal(K.norm("\u05e9\u05dc\u05d5\u05dd"), "\u05e9\u05dc\u05d5\u05dd");
  assert.equal(K.norm("\u05da\u05db"), "\u05da\u05db");
});

for (const v of vectors.normalize) {
  test("norm: " + v.name, () => assert.equal(K.norm(v.in), v.out));
}

for (const v of vectors.edit) {
  test("edit: " + v.name, () => assert.deepEqual(K.edit(...v.args), v.want));
}

function keyEvent(code, key, extra) {
  return Object.assign({ code, key, ctrlKey: false, metaKey: false, altKey: false, isComposing: false }, extra);
}

test("pick: Latin layout t, c, comma give alef, bet, tav (ac-1 logic)", () => {
  assert.equal(K.pick(keyEvent("KeyT", "t"), rendered), "\u05d0");
  assert.equal(K.pick(keyEvent("KeyC", "c"), rendered), "\u05d1");
  assert.equal(K.pick(keyEvent("Comma", ","), rendered), "\u05ea");
  assert.equal(K.pick(keyEvent("KeyT", "T", { shiftKey: true }), rendered), "\u05d0");
});

test("pick: leaves a Hebrew OS layout, shortcuts, IME, and other keys alone", () => {
  assert.equal(K.pick(keyEvent("KeyT", "\u05d0"), rendered), "");
  assert.equal(K.pick(keyEvent("KeyT", "t", { ctrlKey: true }), rendered), "");
  assert.equal(K.pick(keyEvent("KeyT", "t", { metaKey: true }), rendered), "");
  assert.equal(K.pick(keyEvent("KeyT", "t", { altKey: true }), rendered), "");
  assert.equal(K.pick(keyEvent("KeyT", "t", { isComposing: true }), rendered), "");
  assert.equal(K.pick(keyEvent("KeyT", "Process"), rendered), "");
  assert.equal(K.pick(keyEvent("KeyT", "Dead"), rendered), "");
  assert.equal(K.pick(keyEvent("Backspace", "Backspace"), rendered), "");
  assert.equal(K.pick(keyEvent("Digit1", "1"), rendered), "");
  assert.equal(K.pick(keyEvent("Quote", "'"), rendered), "");
  assert.equal(K.pick(keyEvent("Slash", "/"), rendered), "");
});

test("hints: on for the first three sessions, off after, unless chosen", () => {
  assert.equal(K.hints(1, ""), true);
  assert.equal(K.hints(2, ""), true);
  assert.equal(K.hints(3, ""), true);
  assert.equal(K.hints(4, ""), false);
  assert.equal(K.hints(40, ""), false);
  assert.equal(K.hints(40, "1"), true);
  assert.equal(K.hints(1, "0"), false);
});

test("jar: reads the keyboard cookie", () => {
  assert.deepEqual(K.jar(""), [0, ""]);
  assert.deepEqual(K.jar(undefined), [0, ""]);
  assert.deepEqual(K.jar("a=b"), [0, ""]);
  assert.deepEqual(K.jar("hekbd=2:"), [2, ""]);
  assert.deepEqual(K.jar("a=b; hekbd=3:0; c=d"), [3, "0"]);
  assert.deepEqual(K.jar("hekbd=12:1"), [12, "1"]);
  assert.deepEqual(K.jar("xhekbd=9:1"), [0, ""]);
});

// ---- the wiring, against a small fake DOM ----

class El {
  constructor(tag, attrs) {
    this.tag = tag;
    this.attrs = Object.assign({}, attrs || {});
    this.children = [];
    this.parent = null;
    this.fired = [];
    this.value = "";
    this.selectionStart = 0;
    this.selectionEnd = 0;
    this.focused = 0;
  }
  add(...kids) { for (const k of kids) { k.parent = this; this.children.push(k); } return this; }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return n in this.attrs ? this.attrs[n] : null; }
  hasAttribute(n) { return n in this.attrs; }
  removeAttribute(n) { delete this.attrs[n]; }
  setSelectionRange(a, b) { this.selectionStart = a; this.selectionEnd = b; }
  dispatchEvent(ev) { this.fired.push(ev.type); (this.hooks || []).forEach((h) => h(ev)); return !ev.defaultPrevented; }
  focus() { this.focused++; doc.activeElement = this; }
  requestSubmit() { this.submitted = (this.submitted || 0) + 1; }
  get id() { return this.attrs.id; }
  get form() { for (let n = this.parent; n; n = n.parent) if (n.tag === "form") return n; return null; }
  simple(sel) {
    const m = /^([a-z]+)?((?:\[[^\]]+\])*)$/.exec(sel);
    if (!m) throw new Error("fake DOM cannot match " + sel);
    if (m[1] && m[1] !== this.tag) return false;
    for (const a of m[2].match(/\[[^\]]+\]/g) || []) {
      const p = /^\[([\w-]+)(?:="([^"]*)")?\]$/.exec(a);
      if (!this.hasAttribute(p[1]) || (p[2] !== undefined && this.attrs[p[1]] !== p[2])) return false;
    }
    return true;
  }
  matches(sel) {
    const parts = sel.split(" ");
    if (!this.simple(parts.pop())) return false;
    let n = this.parent;
    while (parts.length) {
      const want = parts.pop();
      while (n && !n.simple(want)) n = n.parent;
      if (!n) return false;
      n = n.parent;
    }
    return true;
  }
  closest(sel) { for (let n = this; n; n = n.parent) if (n.matches && n.matches(sel)) return n; return null; }
  all() { return this.children.flatMap((c) => [c, ...c.all()]); }
  querySelectorAll(sel) { return this.all().filter((e) => e.matches(sel)); }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
}
class Doc extends El {
  constructor() { super("#document"); this.listeners = {}; this.cookie = ""; this.readyState = "complete"; this.activeElement = null; }
  addEventListener(t, f) { (this.listeners[t] = this.listeners[t] || []).push(f); }
  getElementById(id) { return this.all().find((e) => e.attrs.id === id) || null; }
  fire(target, type, props) {
    const ev = Object.assign({ type, target, defaultPrevented: false, detail: 0, preventDefault() { this.defaultPrevented = true; } }, props);
    (this.listeners[type] || []).forEach((f) => f(ev));
    return ev;
  }
}

let doc;
function page(opts) {
  opts = opts || {};
  doc = new Doc();
  globalThis.document = doc;
  globalThis.location = { protocol: "https:" };
  globalThis.matchMedia = () => ({ matches: !!opts.coarse });
  const store = Object.assign({}, opts.session);
  const session = { getItem: (k) => (k in store ? store[k] : null), setItem: (k, v) => { store[k] = String(v); } };
  Object.defineProperty(globalThis, "sessionStorage", { value: session, configurable: true, writable: true });
  doc.cookie = opts.cookie || "";
  const keyBtn = (ch, code) => new El("button", { type: "button", "data-ch": ch, "data-code": code }).add(new El("span"));
  const act = (a) => new El("button", { type: "button", "data-act": a });
  const hintBtn = new El("button", { "data-hk-hints": "", "aria-pressed": "false" });
  const bet = keyBtn("\u05d1", "KeyC");
  const root = new El("div", { "data-hk": "", "data-target": "answer", "data-hints": "off" }).add(
    hintBtn,
    new El("div").add(keyBtn("\u05d0", "KeyT"), bet, keyBtn("\u05ea", "Comma"), act("back")),
    new El("div").add(act("space"), act("enter")),
  );
  const input = new El("input", { id: "answer", name: "answer" });
  const form = new El("form").add(input);
  doc.add(form, root);
  load();
  return { root, input, form, hintBtn, bet };
}
const find = (root, sel) => root.querySelector(sel);
const tap = (root, sel, detail) => {
  const b = find(root, sel);
  doc.fire(b, "mousedown");
  doc.fire(b, "click", { detail: detail === undefined ? 1 : detail });
  return b;
};

test("wiring: init sets rtl, lang he, and marks the input", () => {
  const p = page();
  assert.equal(p.input.getAttribute("dir"), "rtl");
  assert.equal(p.input.getAttribute("lang"), "he");
  assert.ok(p.input.hasAttribute("data-hk-input"));
  assert.ok(p.root.hasAttribute("data-ready"));
});

test("wiring: inputmode none only on a touch screen", () => {
  assert.equal(page({ coarse: true }).input.getAttribute("inputmode"), "none");
  assert.equal(page({ coarse: false }).input.getAttribute("inputmode"), null);
});

test("wiring: first session shows hints and counts the session (ac-4)", () => {
  const p = page();
  assert.equal(p.root.getAttribute("data-hints"), "on");
  assert.equal(p.hintBtn.getAttribute("aria-pressed"), "true");
  assert.match(doc.cookie, /^hekbd=1:;/);
});

test("wiring: third session shows hints, fourth hides them (ac-4)", () => {
  assert.equal(page({ cookie: "hekbd=2:" }).root.getAttribute("data-hints"), "on");
  const p = page({ cookie: "hekbd=3:" });
  assert.equal(p.root.getAttribute("data-hints"), "off");
  assert.equal(p.hintBtn.getAttribute("aria-pressed"), "false");
  assert.match(doc.cookie, /^hekbd=4:;/);
});

test("wiring: a second page load in the same tab session does not count again (ac-4)", () => {
  const p = page({ cookie: "hekbd=3:", session: { hekbd: "1" } });
  assert.equal(p.root.getAttribute("data-hints"), "on");
  assert.equal(doc.cookie, "hekbd=3:");
});

test("wiring: an explicit choice beats the session count (ac-4)", () => {
  assert.equal(page({ cookie: "hekbd=9:1" }).root.getAttribute("data-hints"), "on");
  assert.equal(page({ cookie: "hekbd=1:0" }).root.getAttribute("data-hints"), "off");
});

test("wiring: the toggle flips hints and stores the choice (ac-4)", () => {
  const p = page({ cookie: "hekbd=9:" });
  assert.equal(p.root.getAttribute("data-hints"), "off");
  tap(p.root, "[data-hk-hints]");
  assert.equal(p.root.getAttribute("data-hints"), "on");
  assert.equal(p.hintBtn.getAttribute("aria-pressed"), "true");
  assert.match(doc.cookie, /^hekbd=10:1;/);
  tap(p.root, "[data-hk-hints]");
  assert.equal(p.root.getAttribute("data-hints"), "off");
  assert.match(doc.cookie, /^hekbd=10:0;/);
  assert.match(doc.cookie, /samesite=lax/);
  assert.match(doc.cookie, /secure/);
});

test("wiring: init is idempotent after an htmx swap", () => {
  const p = page({ cookie: "hekbd=2:" });
  const before = doc.cookie;
  doc.fire(doc, "htmx:afterSwap");
  doc.fire(doc, "htmx:afterSwap");
  assert.equal(doc.cookie, before);
  assert.equal(p.root.getAttribute("data-hints"), "on");
});

test("wiring: physical t, c, comma type alef bet tav, and flash their keys (ac-1)", () => {
  const p = page();
  for (const [code, key] of [["KeyT", "t"], ["KeyC", "c"], ["Comma", ","]]) {
    const ev = doc.fire(p.input, "keydown", keyEvent(code, key));
    assert.equal(ev.defaultPrevented, true, code + " is handled");
  }
  assert.equal(p.input.value, "\u05d0\u05d1\u05ea");
  assert.equal(p.input.selectionStart, 3);
  assert.deepEqual(p.input.fired, ["input", "input", "input"]);
  assert.ok(p.bet.hasAttribute("data-pressed"), "bet key flashes");
});

test("wiring: a Hebrew OS layout types natively but still flashes the key", () => {
  const p = page();
  const ev = doc.fire(p.input, "keydown", keyEvent("KeyC", "\u05d1"));
  assert.equal(ev.defaultPrevented, false);
  assert.equal(p.input.value, "");
  assert.ok(p.bet.hasAttribute("data-pressed"));
});

test("wiring: keys outside the keyboard's input are left alone", () => {
  const p = page();
  const other = new El("input", { id: "other" });
  doc.add(other);
  const ev = doc.fire(other, "keydown", keyEvent("KeyT", "t"));
  assert.equal(ev.defaultPrevented, false);
  assert.equal(p.input.value, "");
});

test("wiring: tapping keys types in order and backspace removes one letter (ac-2)", () => {
  const p = page();
  tap(p.root, '[data-code="KeyT"]');
  tap(p.root, '[data-code="KeyC"]');
  tap(p.root, '[data-code="Comma"]');
  assert.equal(p.input.value, "\u05d0\u05d1\u05ea");
  tap(p.root, '[data-act="back"]');
  assert.equal(p.input.value, "\u05d0\u05d1");
  tap(p.root, '[data-act="space"]');
  tap(p.root, '[data-code="KeyT"]');
  assert.equal(p.input.value, "\u05d0\u05d1 \u05d0");
  assert.deepEqual(p.input.fired, Array(6).fill("input"));
});

test("wiring: taps use the caret, not just the end", () => {
  const p = page();
  p.input.value = "\u05d0\u05ea";
  p.input.setSelectionRange(1, 1);
  tap(p.root, '[data-code="KeyC"]');
  assert.equal(p.input.value, "\u05d0\u05d1\u05ea");
  assert.equal(p.input.selectionStart, 2);
});

test("wiring: a pointer press keeps focus in the input and never scrolls", () => {
  const p = page();
  const b = find(p.root, '[data-code="KeyT"]');
  const down = doc.fire(b, "mousedown");
  assert.equal(down.defaultPrevented, true, "mousedown is cancelled so focus stays put");
  tap(p.root, '[data-code="KeyT"]');
  assert.equal(p.input.focused, 1);
});

test("wiring: a keyboard activation does not pull focus off the key", () => {
  const p = page();
  tap(p.root, '[data-code="KeyT"]', 0);
  assert.equal(p.input.focused, 0);
  assert.equal(p.input.value, "\u05d0");
});

test("wiring: a tap on something outside the keyboard does nothing", () => {
  const p = page();
  const stray = new El("button");
  doc.add(stray);
  const down = doc.fire(stray, "mousedown");
  doc.fire(stray, "click", { detail: 1 });
  assert.equal(down.defaultPrevented, false);
  assert.equal(p.input.value, "");
});

test("wiring: enter submits the form unless the page cancels hekbd:enter", () => {
  const p = page();
  tap(p.root, '[data-act="enter"]');
  assert.equal(p.form.submitted, 1);
  assert.ok(p.input.fired.includes("hekbd:enter"));
  const q = page();
  q.input.hooks = [(ev) => ev.preventDefault()];
  tap(q.root, '[data-act="enter"]');
  assert.equal(q.form.submitted, undefined);
});

test("wiring: submit sends the NFC form that the audio package keys on", () => {
  const p = page();
  p.input.value = "  \u05d1\u200f\u05bc\u05b8 \u05da  ";
  doc.fire(p.form, "submit");
  assert.equal(p.input.value, "\u05d1\u05b8\u05bc \u05da");
});

test("wiring: an htmx request sends the same normalised answer", () => {
  const p = page();
  const params = { answer: " \u05d0\u05d1\u200f ", _csrf: "tok" };
  doc.fire(p.form, "htmx:configRequest", { detail: { parameters: params } });
  assert.equal(params.answer, "\u05d0\u05d1");
  assert.equal(params._csrf, "tok");
});

test("hekbd.js is idempotent: a second load adds no second set of listeners", () => {
  const p = page();
  const n = doc.listeners.click.length;
  delete require.cache[require.resolve(jsPath)];
  require(jsPath);
  assert.equal(doc.listeners.click.length, n);
  assert.ok(p.root);
});

if (failed) {
  console.log(failed + " failed");
  process.exit(1);
}
console.log("all passed");
