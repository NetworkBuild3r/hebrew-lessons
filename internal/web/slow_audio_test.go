package web

// INIT-036/SPEC-007: shared 0.75× pace helper and Replay/Slower markup.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestSlowScriptServed(t *testing.T) {
	b := readStatic(t, "static/js/slow.js")
	if strings.TrimSpace(b) == "" {
		t.Fatal("slow.js is empty")
	}
	if body := get(t, New(nil, nil), "/static/js/slow.js"); body != b {
		t.Fatal("served slow.js does not match the file")
	}
	for _, bad := range []string{"eval(", "new Function", "innerHTML", "document.write", "onclick"} {
		if strings.Contains(b, bad) {
			t.Errorf("slow.js uses %s", bad)
		}
	}
	for _, need := range []string{
		"hebrew-slow", "0.75", "playbackRate", "preservesPitch",
		"localStorage", "data-slow", "data-replay", "aria-pressed",
		"HebrewSlow",
	} {
		if !strings.Contains(b, need) {
			t.Errorf("slow.js missing %s", need)
		}
	}
}

func TestPaceScriptsUseHebrewSlow(t *testing.T) {
	for _, name := range []string{
		"static/js/clips.js",
		"static/js/activity.js",
		"static/js/dialogue.js",
		"static/js/follow.js",
		"static/js/placement.js",
	} {
		body := readStatic(t, name)
		if !strings.Contains(body, "HebrewSlow") {
			t.Errorf("%s does not call HebrewSlow", name)
		}
	}
}

func TestPaceControlsMarkup(t *testing.T) {
	var buf strings.Builder
	if err := views.PaceControls("line-clip").Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	for _, need := range []string{
		`class="pace"`, `data-pace`, `data-pace-audio="line-clip"`,
		`data-replay`, `data-audio="line-clip"`, `aria-label="Replay"`,
		`data-slow`, `aria-pressed="false"`, `aria-label="Slower"`,
		`>Replay</span>`, `>Slower</span>`,
	} {
		if !strings.Contains(html, need) {
			t.Errorf("PaceControls missing %s\n%s", need, html)
		}
	}
}

func TestPaceChipCSS(t *testing.T) {
	css := readStatic(t, "static/css/components.css")
	for _, need := range []string{".pace-chip", "min-block-size: var(--size-tap)", `aria-pressed="true"`, "focus-visible"} {
		if !strings.Contains(css, need) {
			t.Errorf("components.css missing %s", need)
		}
	}
}

// TestSlowPlaybackRate runs slow.js under Node and checks the shared setting:
// default 1×, Slower → 0.75 with pressed state, Replay follows the setting,
// turning Slower off returns to 1×, and the choice survives a storage round-trip.
func TestSlowPlaybackRate(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	script := filepath.Join(t.TempDir(), "slow-check.js")
	if err := os.WriteFile(script, []byte(slowPlaybackCheckJS), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", script, filepath.Join("static", "js", "slow.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("slow.js behaviour: %v\n%s", err, out)
	}
}

const slowPlaybackCheckJS = `
const fs = require("fs");
const vm = require("vm");
const src = fs.readFileSync(process.argv[2], "utf8");
const fails = [];
function check(cond, msg) { if (!cond) fails.push(msg); }

const store = Object.create(null);
const listeners = [];
function makeEl(tag) {
  const el = {
    tagName: String(tag).toUpperCase(),
    attrs: {},
    kids: [],
    parent: null,
    playbackRate: 1,
    preservesPitch: false,
    currentTime: 0,
    paused: true,
    plays: 0,
  };
  el.setAttribute = (k, v) => { el.attrs[k] = String(v); };
  el.getAttribute = (k) => (Object.prototype.hasOwnProperty.call(el.attrs, k) ? el.attrs[k] : null);
  el.hasAttribute = (k) => Object.prototype.hasOwnProperty.call(el.attrs, k);
  el.removeAttribute = (k) => { delete el.attrs[k]; };
  el.append = (child) => { child.parent = el; el.kids.push(child); };
  el.closest = (sel) => {
    let n = el;
    while (n) {
      if (sel === "[data-dialogue]" && n.attrs && Object.prototype.hasOwnProperty.call(n.attrs, "data-dialogue")) return n;
      if (sel === "[data-slow]" && n.hasAttribute && n.hasAttribute("data-slow")) return n;
      if (sel === "[data-replay]" && n.hasAttribute && n.hasAttribute("data-replay")) return n;
      if (sel === "[data-pace]" && n.hasAttribute && n.hasAttribute("data-pace")) return n;
      n = n.parent;
    }
    return null;
  };
  el.querySelector = (sel) => {
    if (sel === "audio") {
      const walk = (n) => {
        if (n.tagName === "AUDIO") return n;
        for (const k of n.kids || []) { const f = walk(k); if (f) return f; }
        return null;
      };
      return walk(el);
    }
    return null;
  };
  el.querySelectorAll = (sel) => {
    const out = [];
    const walk = (n) => {
      if (sel === "[data-slow]" && n.hasAttribute && n.hasAttribute("data-slow")) out.push(n);
      (n.kids || []).forEach(walk);
    };
    walk(el);
    return out;
  };
  el.play = () => { el.plays++; el.paused = false; return Promise.resolve(); };
  return el;
}

const doc = makeEl("#document");
doc.readyState = "complete";
doc.documentElement = makeEl("html");
doc.addEventListener = (name, fn) => { listeners.push({ name, fn }); };
doc.querySelectorAll = (sel) => {
  const out = [];
  const walk = (n) => {
    if (sel === "[data-slow]" && n.hasAttribute && n.hasAttribute("data-slow")) out.push(n);
    (n.kids || []).forEach(walk);
  };
  walk(doc);
  return out;
};
doc.getElementById = (id) => {
  let found = null;
  const walk = (n) => {
    if (n.attrs && n.attrs.id === id) found = n;
    (n.kids || []).forEach(walk);
  };
  walk(doc);
  return found;
};

const wrap = makeEl("div");
wrap.setAttribute("data-pace", "");
wrap.setAttribute("data-pace-audio", "clip-1");
const audio = makeEl("audio");
audio.attrs.id = "clip-1";
const replay = makeEl("button");
replay.setAttribute("data-replay", "");
replay.setAttribute("data-audio", "clip-1");
const slower = makeEl("button");
slower.setAttribute("data-slow", "");
slower.setAttribute("aria-pressed", "false");
wrap.append(audio);
wrap.append(replay);
wrap.append(slower);
doc.append(wrap);

const g = {
  document: doc,
  localStorage: {
    getItem(k) { return Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null; },
    setItem(k, v) { store[k] = String(v); },
  },
};
g.window = g;
vm.runInNewContext(src, { window: g, globalThis: g });

check(!!g.HebrewSlow, "HebrewSlow missing");
g.HebrewSlow.apply(audio);
check(audio.playbackRate === 1, "default rate is not 1: " + audio.playbackRate);
check(audio.preservesPitch === true, "preservesPitch not set");

listeners.filter((l) => l.name === "click").forEach((l) => l.fn({
  target: slower, preventDefault() {},
}));
check(audio.playbackRate === 0.75, "Slower did not set 0.75: " + audio.playbackRate);
check(slower.getAttribute("aria-pressed") === "true", "Slower not pressed");
check(store["hebrew-slow"] === "1", "slow setting not stored");

audio.playbackRate = 1;
listeners.filter((l) => l.name === "click").forEach((l) => l.fn({
  target: replay, preventDefault() {},
}));
check(audio.playbackRate === 0.75, "Replay did not follow slow setting: " + audio.playbackRate);

listeners.filter((l) => l.name === "click").forEach((l) => l.fn({
  target: slower, preventDefault() {},
}));
check(audio.playbackRate === 1, "turning Slower off did not return to 1: " + audio.playbackRate);
check(slower.getAttribute("aria-pressed") === "false", "Slower still pressed");
check(store["hebrew-slow"] === "0", "slow off not stored");

if (fails.length) {
  console.error(fails.join("\n"));
  process.exit(1);
}
console.log("slow.js playbackRate ok");
`
