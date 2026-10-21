// Browser checks for the lesson clip orb (go test -tags browser).
// Prints one JSON object: { checks: [{name, ok, detail}], errors }.
//
// The lead's ruling on a clip:
//   JS on (html.js): every native bar's computed display is "none".
//   The hidden attribute is not an alternative: clips.js always sets it.
//   mode-js-on-hidden-mutation replaces that rule with opacity:0 and a 1px
//   box and requires the strict check to fail, then removes the override.
//   JS off: each bar is at least 200px wide at 390 and at 1440. A real click
//   on the native play control starts playback, and that check also requires
//   the 390px width floor, so a 39px bar cannot pass on playback alone.
//   Tab reaches a visible bar,
//   which is the positive control that the Tab walk can see a focusable bar.
//   Each clip has exactly one way to play in that mode. Never both a bar and an orb.
// Orbs (hero, word, dialogue .c-orb) are buttons with a name and a 44px hit area.
// One clip plays at a time. States run idle -> loading -> playing.
// A failed clip goes to error, is not painted with the stop/danger red, and a
// second press returns to loading.
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const cookie = process.env.SESSION_COOKIE;
const unit = process.env.UNIT;
const failSrc = process.env.FAIL_SRC;
const errors = [];
const checks = [];

function check(name, ok, detail) {
  checks.push({ name, ok: !!ok, detail: detail || "" });
}

const browser = await chromium.launch({ args: ["--autoplay-policy=no-user-gesture-required"] });

const tap = async (page, sel) => {
  const loc = page.locator(sel).first();
  await loc.evaluate((el) => el.scrollIntoView({ block: "center" }));
  const box = await loc.boundingBox();
  if (!box) throw new Error("no box for " + sel);
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
};

const probeOrbs = () => {
  const nameOf = (el) => {
    const by = el.getAttribute("aria-labelledby");
    if (by) {
      const text = by.split(/\s+/).map((id) => document.getElementById(id)?.innerText || "").join(" ").trim();
      if (text) return text;
    }
    return (el.getAttribute("aria-label") || "").trim();
  };
  const box = (el) => {
    const r = el.getBoundingClientRect();
    return { w: r.width, h: r.height };
  };
  const rows = [];
  for (const el of document.querySelectorAll(".orb--hero, .orb--word, .c-orb")) {
    const kind = el.classList.contains("orb--hero") ? "hero" : el.classList.contains("orb--word") ? "word" : "dialogue";
    rows.push({
      kind,
      tag: el.tagName,
      type: el.getAttribute("type") || "",
      name: nameOf(el),
      ...box(el),
    });
  }
  return rows;
};

const clipFacts = () => {
  const visible = (el) => {
    if (!el) return false;
    const s = getComputedStyle(el);
    const r = el.getBoundingClientRect();
    return s.display !== "none" && s.visibility !== "hidden" && Number(s.opacity) > 0 && r.width > 1 && r.height > 1;
  };
  // A recall word's bar lives in .clip-rows and its orb stays under the word,
  // joined by data-audio. The line clip still holds both.
  const orbFor = (audio) => {
    if (audio.id) {
      const named = document.querySelector('.orb[data-audio="' + audio.id + '"]');
      if (named) return named;
    }
    const clip = audio.closest(".clip");
    return clip ? clip.querySelector("button.orb, .orb") : null;
  };
  return [...document.querySelectorAll(".clip audio")].map((audio) => {
    const orb = orbFor(audio);
    const s = getComputedStyle(audio);
    const r = audio.getBoundingClientRect();
    return {
      id: audio.id,
      controls: !!audio.controls,
      hiddenAttr: audio.hasAttribute("hidden"),
      display: s.display,
      opacity: s.opacity,
      position: s.position,
      w: r.width,
      h: r.height,
      audioVisible: visible(audio),
      orbVisible: visible(orb),
      orbName: orb ? (orb.getAttribute("aria-label") || "").trim() : "",
    };
  });
};

const redsOf = (selector) => {
  const token = (name) => {
    const i = document.createElement("i");
    i.style.color = "var(" + name + ")";
    document.body.appendChild(i);
    const c = getComputedStyle(i).color;
    i.remove();
    return c;
  };
  const stops = new Set([token("--stop"), token("--stop-soft"), "rgb(179, 54, 63)", "rgb(255, 147, 139)", "rgb(248, 225, 224)", "rgb(58, 26, 29)"]);
  const props = ["color", "backgroundColor", "borderTopColor", "borderRightColor", "borderBottomColor", "borderLeftColor", "outlineColor"];
  const hits = [];
  for (const el of document.querySelectorAll(selector)) {
    for (const src of [getComputedStyle(el), getComputedStyle(el, "::before"), getComputedStyle(el, "::after")]) {
      for (const p of props) {
        const c = src[p];
        if (c && stops.has(c)) hits.push(el.getAttribute("data-src") || el.className + " " + p + "=" + c);
      }
    }
  }
  return hits;
};

const axAudio = async (page) => {
  const session = await page.context().newCDPSession(page);
  const tree = await session.send("Accessibility.getFullAXTree");
  const exposed = [];
  for (const n of tree.nodes) {
    if (n.ignored) continue;
    const role = (n.role && n.role.value) || "";
    if (/audio/i.test(role)) exposed.push(role);
  }
  await session.detach();
  return exposed;
};

// activateNativeBar starts one native control the way a person would.
// It never calls audio.play(). The play glyph sits at the left of Chromium's
// media controls. If that click misses, CDP clicks the play button inside the
// closed shadow root, then Space and Enter are the keyboard fallback.
const startedPlaying = (page, id) => page.waitForFunction((id) => {
  const a = document.getElementById(id);
  return !!(a && (!a.paused || a.currentTime > 0));
}, id, { timeout: 1500 }).then(() => true).catch(() => false);

const playbackState = (page, id) => page.evaluate((id) => {
  const a = document.getElementById(id);
  return a ? { paused: a.paused, t: a.currentTime } : { paused: true, t: 0 };
}, id);

const clickPlayInShadow = async (page, id) => {
  const cdp = await page.context().newCDPSession(page);
  try {
    const doc = await cdp.send("DOM.getDocument", { depth: -1, pierce: true });
    const button = findPlayButton(doc.root, id);
    if (!button) return false;
    const model = await cdp.send("DOM.getBoxModel", { nodeId: button.nodeId });
    const q = model.model.content;
    const x = (q[0] + q[2] + q[4] + q[6]) / 4;
    const y = (q[1] + q[3] + q[5] + q[7]) / 4;
    await page.mouse.click(x, y);
    return true;
  } catch {
    return false;
  } finally {
    await cdp.detach().catch(() => {});
  }
};

const nodeAttr = (node, name) => {
  const attrs = node.attributes || [];
  for (let i = 0; i < attrs.length; i += 2) {
    if (attrs[i] === name) return attrs[i + 1] || "";
  }
  return "";
};

const findPlayButton = (root, audioId) => {
  let host = null;
  const walk = (node, fn) => {
    fn(node);
    for (const child of node.children || []) walk(child, fn);
    for (const shadow of node.shadowRoots || []) walk(shadow, fn);
  };
  walk(root, (node) => {
    if (!host && nodeAttr(node, "id") === audioId) host = node;
  });
  if (!host) return null;
  let play = null;
  walk(host, (node) => {
    if (play) return;
    const label = (nodeAttr(node, "aria-label") + " " + (node.nodeName || "")).toLowerCase();
    const role = nodeAttr(node, "role").toLowerCase();
    const looksButton = node.nodeName === "BUTTON" || role === "button";
    if (looksButton && label.includes("play")) play = node;
  });
  return play;
};

const activateNativeBar = async (page, id) => {
  const loc = page.locator("#" + id);
  await loc.scrollIntoViewIfNeeded();
  const box = await loc.boundingBox();
  if (!box) return { ok: false, how: "no box", paused: true, t: 0, w: 0 };
  const finish = async (how) => {
    const state = await playbackState(page, id);
    return { ok: true, how, paused: state.paused, t: state.t, w: box.width };
  };
  // A real pointer click on the native play glyph, not audio.play().
  await page.mouse.click(box.x + 18, box.y + box.height / 2);
  if (await startedPlaying(page, id)) return finish("click");
  if (await clickPlayInShadow(page, id) && await startedPlaying(page, id)) return finish("cdp-play");
  await loc.focus();
  await page.keyboard.press("Space");
  if (await startedPlaying(page, id)) return finish("space");
  await page.keyboard.press("Enter");
  if (await startedPlaying(page, id)) return finish("enter");
  const state = await playbackState(page, id);
  return { ok: false, how: "no-gesture", paused: state.paused, t: state.t, w: box.width };
};

const tabTags = async (page) => {
  await page.evaluate(() => {
    document.body.setAttribute("tabindex", "-1");
    document.body.focus();
  });
  const seen = [];
  for (let i = 0; i < 30; i++) {
    await page.keyboard.press("Tab");
    const info = await page.evaluate(() => {
      const el = document.activeElement;
      if (!el) return { tag: "", id: "", cls: "", display: "", w: 0, h: 0 };
      const r = el.getBoundingClientRect();
      return {
        tag: el.tagName,
        id: el.id || "",
        cls: String(el.className || ""),
        display: getComputedStyle(el).display,
        w: r.width,
        h: r.height,
      };
    });
    seen.push(info);
    if (info.tag === "BODY") break;
  }
  return seen;
};

try {
  const ctx = await browser.newContext({
    bypassCSP: true,
    viewport: { width: 390, height: 844 },
  });
  await ctx.addCookies([{ name: "hebrew_session", value: cookie, url: origin }]);
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.route("**/audio/*.mp3", async (route) => {
    await new Promise((r) => setTimeout(r, 350));
    if (route.request().url().includes(failSrc)) {
      await route.fulfill({ status: 404, body: "", contentType: "audio/mpeg" });
      return;
    }
    await route.continue();
  });

  await page.goto(origin + "/lesson/" + unit + "/0", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("[data-dialogue] .c-orb");
  const dialogueOrbs = await page.evaluate(probeOrbs);
  const dialogueOnly = dialogueOrbs.filter((o) => o.kind === "dialogue");
  check(
    "orb-dialogue",
    dialogueOnly.length >= 1 && dialogueOnly.every((o) => o.tag === "BUTTON" && o.name && o.w >= 44 && o.h >= 44),
    dialogueOnly.map((o) => `${o.tag} ${o.w.toFixed(1)}x${o.h.toFixed(1)} "${o.name}"`).join("; ") || "no .c-orb",
  );

  await Promise.all([
    page.waitForResponse((r) => r.request().method() === "POST" && r.ok()),
    tap(page, 'button[name="answer"][value="continue"]'),
  ]);
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(() => document.documentElement.classList.contains("js") && document.querySelector(".orb--hero"));

  const lessonOrbs = await page.evaluate(probeOrbs);
  const hero = lessonOrbs.filter((o) => o.kind === "hero");
  const words = lessonOrbs.filter((o) => o.kind === "word");
  check(
    "orb-hero",
    hero.length === 1 && hero[0].tag === "BUTTON" && hero[0].name && hero[0].w >= 44 && hero[0].h >= 44,
    hero.map((o) => `${o.tag} type=${o.type} ${o.w.toFixed(1)}x${o.h.toFixed(1)} "${o.name}"`).join("; ") || "no hero orb",
  );
  check(
    "orb-word",
    words.length >= 1 && words.every((o) => o.tag === "BUTTON" && o.name && o.w >= 44 && o.h >= 44),
    words.map((o) => `${o.tag} ${o.w.toFixed(1)}x${o.h.toFixed(1)} "${o.name}"`).join("; ") || "no word orb",
  );

  const onFacts = await page.evaluate(clipFacts);
  const displayNone = onFacts.length > 0 && onFacts.every((c) => c.display === "none");
  check(
    "mode-js-on-hidden",
    displayNone,
    displayNone
      ? onFacts.map((c) => c.id + " display=" + c.display).join("; ")
      : "native audio computed display is not none. The hidden attribute does not count. " +
        onFacts.map((c) => `${c.id} display=${c.display} opacity=${c.opacity} hiddenAttr=${c.hiddenAttr} ${c.w.toFixed(1)}x${c.h.toFixed(1)}`).join("; "),
  );
  // Mutation: the ruled-out hide (opacity 0 and a 1px box) must make the
  // strict display check fail. clips.js still sets hidden; that must not save it.
  await page.evaluate(() => {
    const style = document.createElement("style");
    style.id = "qa-hide-mutation";
    style.textContent = [
      "html.js .clip > audio {",
      "  display: block !important;",
      "  position: absolute !important;",
      "  inline-size: 1px !important;",
      "  block-size: 1px !important;",
      "  margin: 0 !important;",
      "  padding: 0 !important;",
      "  border: 0 !important;",
      "  opacity: 0 !important;",
      "}",
      "html.js .clip-rows {",
      "  display: flex !important;",
      "  position: absolute !important;",
      "  inline-size: 1px !important;",
      "  block-size: 1px !important;",
      "  opacity: 0 !important;",
      "  overflow: hidden !important;",
      "}",
    ].join("\n");
    document.head.appendChild(style);
  });
  const mutated = await page.evaluate(clipFacts);
  const mutationFailsStrict = mutated.length > 0 && mutated.some((c) => c.display !== "none");
  check(
    "mode-js-on-hidden-mutation",
    mutationFailsStrict,
    mutationFailsStrict
      ? "replacing display:none with opacity:0 and a 1px box makes display===none fail. " +
        mutated.map((c) => `${c.id} display=${c.display} opacity=${c.opacity} hiddenAttr=${c.hiddenAttr} ${c.w.toFixed(1)}x${c.h.toFixed(1)}`).join("; ")
      : "the strict check still passed under opacity:0 and a 1px box. " +
        mutated.map((c) => `${c.id} display=${c.display} hiddenAttr=${c.hiddenAttr}`).join("; "),
  );
  await page.evaluate(() => {
    const style = document.getElementById("qa-hide-mutation");
    if (style) style.remove();
  });
  const exposed = await axAudio(page);
  check("mode-js-on-a11y", exposed.length === 0, exposed.length ? "accessibility tree still has " + exposed.join(",") : "no audio role in the accessibility tree");
  const tabsOn = await tabTags(page);
  const audioTab = tabsOn.filter((t) => t.tag === "AUDIO" || t.id === "line-clip" || t.id.startsWith("word-clip"));
  check("mode-js-on-tab", audioTab.length === 0, audioTab.length ? "Tab reached " + JSON.stringify(audioTab) : "Tab did not reach a native audio element");
  const bothOn = onFacts.filter((c) => c.audioVisible && c.orbVisible);
  const oneWayOn = onFacts.length > 0 && onFacts.every((c) => c.orbVisible && !c.audioVisible);
  check(
    "mode-js-on-one-way",
    oneWayOn && bothOn.length === 0,
    onFacts.map((c) => `${c.id} bar=${c.audioVisible} orb=${c.orbVisible}`).join("; ") || "no clips",
  );

  const srcOf = async (sel) => page.locator(sel).first().getAttribute("data-src");
  const heroSrc = await srcOf(".orb--hero");
  const wordSrcs = await page.locator(".orb--word").evaluateAll((els) => els.map((el) => el.getAttribute("data-src")));
  const playSrc = wordSrcs.find((s) => s && !s.includes(failSrc));
  const distinct = async (page, src) => page.evaluate((want) => {
    const orb = [...document.querySelectorAll(".orb")].find((o) => o.getAttribute("data-src") === want);
    if (!orb || !orb.__seq) return [];
    return orb.__seq;
  }, src);

  await page.evaluate(() => {
    window.__overlap = false;
    const note = () => {
      const playing = [...document.querySelectorAll(".orb")].filter((o) => o.getAttribute("data-state") === "playing");
      const audible = [...document.querySelectorAll("audio")].filter((a) => !a.paused && !a.ended);
      if (playing.length > 1 || audible.length > 1) window.__overlap = true;
    };
    new MutationObserver(note).observe(document, { attributes: true, subtree: true, attributeFilter: ["data-state"] });
    for (const orb of document.querySelectorAll(".orb")) {
      orb.__seq = [orb.getAttribute("data-state")];
      new MutationObserver(() => {
        const s = orb.getAttribute("data-state");
        if (orb.__seq[orb.__seq.length - 1] !== s) orb.__seq.push(s);
      }).observe(orb, { attributes: true, attributeFilter: ["data-state"] });
    }
  });

  await tap(page, `.orb[data-src="${heroSrc}"]`);
  await page.waitForFunction((src) => {
    const orb = [...document.querySelectorAll(".orb")].find((o) => o.getAttribute("data-src") === src);
    return orb && orb.getAttribute("data-state") === "playing";
  }, heroSrc, { timeout: 5000 });
  const heroSeq = await distinct(page, heroSrc);
  const heroPaused = await page.evaluate((src) => {
    const audio = [...document.querySelectorAll("audio")].find((a) => (a.getAttribute("src") || "").endsWith(src));
    return audio ? audio.paused : true;
  }, heroSrc);

  await tap(page, `.orb[data-src="${playSrc}"]`);
  await page.waitForFunction((src) => {
    const orb = [...document.querySelectorAll(".orb")].find((o) => o.getAttribute("data-src") === src);
    return orb && orb.getAttribute("data-state") === "playing";
  }, playSrc, { timeout: 5000 });
  const secondSeq = await distinct(page, playSrc);
  const after = await page.evaluate((first) => {
    const a = [...document.querySelectorAll(".orb")].find((o) => o.getAttribute("data-src") === first);
    const audio = [...document.querySelectorAll("audio")].find((el) => (el.getAttribute("src") || "").endsWith(first));
    return {
      firstState: a ? a.getAttribute("data-state") : "",
      firstPaused: audio ? audio.paused : false,
      overlap: window.__overlap,
    };
  }, heroSrc);
  const idleLoadingPlaying = (seq) => seq.length >= 3 && seq[0] === "idle" && seq.includes("loading") && seq[seq.length - 1] === "playing" && seq.indexOf("loading") < seq.lastIndexOf("playing");
  check(
    "play-states",
    idleLoadingPlaying(heroSeq) && idleLoadingPlaying(secondSeq) && !heroPaused,
    `hero ${heroSeq.join("->")} (paused while playing=${heroPaused}); second ${secondSeq.join("->")}`,
  );
  check(
    "play-single",
    after.firstPaused && after.firstState !== "playing" && !after.overlap,
    `first orb state=${after.firstState} paused=${after.firstPaused} overlap=${after.overlap}`,
  );

  await page.evaluate((src) => {
    const orb = [...document.querySelectorAll(".orb")].find((o) => o.getAttribute("data-src") === src);
    orb.__retry = [];
    window.__retryOrb = orb;
  }, failSrc);
  await tap(page, `.orb[data-src="${failSrc}"]`);
  await page.waitForFunction((src) => {
    const orb = [...document.querySelectorAll(".orb")].find((o) => o.getAttribute("data-src") === src);
    return orb && orb.getAttribute("data-state") === "error";
  }, failSrc, { timeout: 5000 });
  const errSeq = await distinct(page, failSrc);
  const reds = await page.evaluate(redsOf, `.orb[data-state="error"]`);
  check(
    "play-error-color",
    reds.length === 0 && errSeq.includes("error"),
    reds.length ? "error orb uses a stop/danger colour: " + reds.join("; ") : "error state is not --stop / --stop-soft",
  );
  await page.evaluate((src) => {
    const orb = [...document.querySelectorAll(".orb")].find((o) => o.getAttribute("data-src") === src);
    window.__retrySawLoading = false;
    window.__retrySeq = [orb.getAttribute("data-state")];
    new MutationObserver(() => {
      const s = orb.getAttribute("data-state");
      if (s === "loading") window.__retrySawLoading = true;
      if (window.__retrySeq[window.__retrySeq.length - 1] !== s) window.__retrySeq.push(s);
    }).observe(orb, { attributes: true, attributeFilter: ["data-state"] });
  }, failSrc);
  await tap(page, `.orb[data-src="${failSrc}"]`);
  await page.waitForFunction((src) => {
    const orb = [...document.querySelectorAll(".orb")].find((o) => o.getAttribute("data-src") === src);
    return window.__retrySawLoading && orb && orb.getAttribute("data-state") === "error";
  }, failSrc, { timeout: 5000 }).catch(() => {});
  const retry = await page.evaluate(() => ({ saw: window.__retrySawLoading, seq: window.__retrySeq || [] }));
  check(
    "play-error-retry",
    retry.saw === true,
    "second press sequence " + (retry.seq || []).join("->") + (retry.saw ? " (saw loading)" : " (never returned to loading)"),
  );

  await ctx.close();

  const off = await browser.newContext({
    bypassCSP: true,
    viewport: { width: 390, height: 844 },
  });
  await off.addCookies([{ name: "hebrew_session", value: cookie, url: origin }]);
  const bare = await off.newPage();
  bare.on("pageerror", (e) => errors.push("js-off " + String(e)));
  await bare.route("**/static/js/clips.js", (route) => route.abort());
  await bare.goto(origin + "/lesson/" + unit + "/0", { waitUntil: "domcontentloaded" });
  await bare.waitForSelector(".clip audio");
  const jsClass = await bare.evaluate(() => document.documentElement.classList.contains("js"));
  const usable = (facts) => facts.length > 0 && facts.every((c) => c.controls && c.audioVisible && c.w >= 200 && c.h >= 16 && !c.orbVisible);
  const describe = (facts) => facts.map((c) => `${c.id} controls=${c.controls} bar=${c.audioVisible} ${c.w.toFixed(0)}x${c.h.toFixed(0)} orb=${c.orbVisible} display=${c.display}`).join("; ");
  const offFacts = await bare.evaluate(clipFacts);
  const bars390 = usable(offFacts);
  check("mode-js-off-bar-390", !jsClass && bars390, `viewport=390 html.js=${jsClass}; ` + describe(offFacts));
  await bare.setViewportSize({ width: 1440, height: 900 });
  const wideFacts = await bare.evaluate(clipFacts);
  const bars1440 = usable(wideFacts);
  check("mode-js-off-bar-1440", !jsClass && bars1440, `viewport=1440 html.js=${jsClass}; ` + describe(wideFacts));
  await bare.setViewportSize({ width: 390, height: 844 });
  const played = [];
  for (const fact of offFacts) {
    const result = await activateNativeBar(bare, fact.id);
    played.push({ id: fact.id, ...result });
  }
  // Width is measured at 390, where the click happens. Playback alone is not
  // enough: a 39px bar that still plays fails this check.
  const playsAtFloor = played.length > 0 && played.every((p) => p.ok && p.w >= 200);
  check(
    "mode-js-off-plays",
    playsAtFloor,
    played.map((p) => `${p.id} ok=${p.ok} via=${p.how} paused=${p.paused} t=${Number(p.t).toFixed(2)} w390=${Number(p.w).toFixed(0)}`).join("; ") || "no audio",
  );
  const tabsOff = await tabTags(bare);
  const orbTab = tabsOff.filter((t) => String(t.cls).includes("orb"));
  const visibleAudio = tabsOff.filter((t) => t.tag === "AUDIO" && t.display !== "none" && t.w >= 200 && t.h >= 16);
  const bothOff = offFacts.filter((c) => c.audioVisible && c.orbVisible);
  check(
    "mode-js-off-tab-reaches-bar",
    visibleAudio.length >= 1,
    visibleAudio.length
      ? "positive control: Tab reached a visible bar " + visibleAudio.map((t) => `${t.id} ${t.w.toFixed(0)}x${t.h.toFixed(0)} display=${t.display}`).join("; ")
      : "Tab did not reach a visible native bar. Walk: " + JSON.stringify(tabsOff.filter((t) => t.tag === "AUDIO" || t.tag === "BUTTON").slice(0, 8)),
  );
  check(
    "mode-js-off-one-way",
    bars390 && bothOff.length === 0 && orbTab.length === 0 && visibleAudio.length >= 1,
    `orb focused=${orbTab.length} visible audio focused=${visibleAudio.length}; ` + offFacts.map((c) => `${c.id} bar=${c.audioVisible} orb=${c.orbVisible}`).join("; "),
  );
  await off.close();
} catch (e) {
  errors.push(String(e && e.stack || e));
} finally {
  await browser.close();
}

console.log(JSON.stringify({ checks, errors }));
