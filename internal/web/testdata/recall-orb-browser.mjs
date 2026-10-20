// Browser checks for the lesson clip orb (go test -tags browser).
// Prints one JSON object: { checks: [{name, ok, detail}], errors }.
//
// The lead's ruling on a clip:
//   JS on (html.js): the native audio is not visible, not focusable, and not in
//   the accessibility tree. It is hidden with display:none or the hidden
//   attribute. Opacity and a 1px off-screen box do not count.
//   JS off (clips.js not loaded): the native bar is visible and plays.
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
  return [...document.querySelectorAll(".clip")].map((clip) => {
    const audio = clip.querySelector("audio");
    const orb = clip.querySelector("button.orb, .orb");
    const s = audio ? getComputedStyle(audio) : null;
    const r = audio ? audio.getBoundingClientRect() : { width: 0, height: 0 };
    return {
      id: audio ? audio.id : "",
      controls: !!(audio && audio.controls),
      hiddenAttr: !!(audio && audio.hasAttribute("hidden")),
      display: s ? s.display : "",
      opacity: s ? s.opacity : "",
      position: s ? s.position : "",
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
      if (!el) return { tag: "", id: "", cls: "" };
      return { tag: el.tagName, id: el.id || "", cls: String(el.className || "") };
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
  const hiddenProperly = onFacts.every((c) => c.display === "none" || c.hiddenAttr);
  const opacityOrOffscreen = onFacts.filter((c) => c.display !== "none" && !c.hiddenAttr && (Number(c.opacity) === 0 || (c.position === "absolute" && c.w <= 1)));
  check(
    "mode-js-on-hidden",
    onFacts.length > 0 && hiddenProperly,
    hiddenProperly
      ? onFacts.map((c) => c.id + " display=" + c.display).join("; ")
      : "native audio is not display:none and has no hidden attribute. " +
        opacityOrOffscreen.map((c) => `${c.id} display=${c.display} opacity=${c.opacity} position=${c.position} ${c.w.toFixed(1)}x${c.h.toFixed(1)}`).join("; ") +
        ". html.js .clip > audio in internal/web/static/css/base.css hides with opacity:0 and a 1px box (the rule at .clip > audio / html.js .clip > audio), which the lead ruled out.",
  );
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
  const offFacts = await bare.evaluate(clipFacts);
  // A word bar fills its column, so a short word is narrower than 40px.
  // The native control is the controls attribute plus a bar taller than a
  // 1px hide (audioVisible already rejects display:none, opacity 0, and a
  // 1×1 box).
  const bars = offFacts.length > 0 && offFacts.every((c) => c.controls && c.audioVisible && c.h >= 16 && !c.orbVisible);
  check(
    "mode-js-off-bar",
    !jsClass && bars,
    `html.js=${jsClass}; ` + offFacts.map((c) => `${c.id} controls=${c.controls} bar=${c.audioVisible} ${c.w.toFixed(0)}x${c.h.toFixed(0)} orb=${c.orbVisible} display=${c.display}`).join("; "),
  );
  const played = await bare.evaluate(async () => {
    const out = [];
    for (const audio of document.querySelectorAll(".clip audio")) {
      try {
        await audio.play();
        out.push({ id: audio.id, ok: !audio.paused && !audio.error, paused: audio.paused, err: audio.error && String(audio.error.code) });
      } catch (e) {
        out.push({ id: audio.id, ok: false, err: String(e) });
      }
    }
    return out;
  });
  check(
    "mode-js-off-plays",
    played.length > 0 && played.every((p) => p.ok),
    played.map((p) => `${p.id} ok=${p.ok} ${p.err || ""}`).join("; ") || "no audio",
  );
  const tabsOff = await tabTags(bare);
  const orbTab = tabsOff.filter((t) => String(t.cls).includes("orb"));
  const audioReached = tabsOff.some((t) => t.tag === "AUDIO");
  const bothOff = offFacts.filter((c) => c.audioVisible && c.orbVisible);
  check(
    "mode-js-off-one-way",
    bars && bothOff.length === 0 && orbTab.length === 0 && audioReached,
    `orb focused=${orbTab.length} audio focused=${audioReached}; ` + offFacts.map((c) => `${c.id} bar=${c.audioVisible} orb=${c.orbVisible}`).join("; "),
  );
  await off.close();
} catch (e) {
  errors.push(String(e && e.stack || e));
} finally {
  await browser.close();
}

console.log(JSON.stringify({ checks, errors }));
