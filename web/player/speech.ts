/**
 * Shared clip player and Hebrew TTS fallback.
 * One Audio element. Clip lookup is NFC exact text, then a vowel-stripped
 * key only when that stripped form is unique. Relative clip URLs resolve
 * against the current page (a lesson is /worksheets/<slug>/).
 *
 * Authored-by-spec: INIT-001, INIT-007/SPEC-002
 */

interface HebrewUtilsLike {
  stripVowels?: (value: string) => string;
}

interface FeedbackLike {
  setSpeechActive?: (active: boolean) => void;
}

interface VoiceLike {
  name: string;
  lang: string;
}

interface UtteranceLike {
  text?: string;
  lang: string;
  rate: number;
  voice: VoiceLike | null;
  onend: (() => void) | null;
  onerror: (() => void) | null;
}

interface SpeechSynthesisLike {
  getVoices: () => VoiceLike[] | undefined;
  cancel: () => void;
  speak: (utterance: UtteranceLike) => void;
  addEventListener: (type: string, listener: () => void, options?: { once?: boolean }) => void;
}

interface AudioLike {
  preload: string;
  src: string;
  onended: (() => void) | null;
  onerror: (() => void) | null;
  pause: () => void;
  load: () => void;
  removeAttribute: (name: string) => void;
  play: () => Promise<void> | void;
}

interface ClassListLike {
  remove: (...names: string[]) => void;
  toggle: (name: string, force?: boolean) => boolean;
}

interface PlayEl {
  nodeType: number;
  classList?: ClassListLike;
}

interface PlayOpts {
  nodeType?: number;
  el?: PlayEl;
  button?: PlayEl;
}

interface ClipEntry {
  text?: unknown;
  url?: unknown;
}

interface StoryState {
  run: number;
  status: string;
  index: number;
  utterance: UtteranceLike | null;
  clips: unknown[];
}

interface PlaybackState {
  status: string;
  el: PlayEl | null;
}

interface LastClip {
  text: string;
  url: string;
  el: PlayEl | null;
}

interface SpeechHost {
  HebrewUtils?: HebrewUtilsLike;
  HebrewFeedback?: FeedbackLike;
  HebrewSpeech?: HebrewSpeechApi;
  Audio?: new () => AudioLike;
  location?: { href: string };
  speechSynthesis?: SpeechSynthesisLike;
  SpeechSynthesisUtterance?: new (text: string) => UtteranceLike;
  CustomEvent?: new (type: string, init?: { detail?: unknown }) => Event;
  dispatchEvent?: (event: Event) => boolean;
  addEventListener?: (type: string, listener: () => void) => void;
  setTimeout?: (fn: () => void, ms: number) => unknown;
}

export interface HebrewSpeechApi {
  hebrewVoice: VoiceLike | null;
  story: StoryState;
  clipsExact: Record<string, string>;
  clipsStripped: Record<string, string>;
  audioEl: AudioLike | null;
  playback: PlaybackState;
  _playingEl: PlayEl | null;
  _lastClip: LastClip | null;
  onStoryPaint?: () => void;
  resolveAudioUrl: (audioPath: unknown) => string;
  setClips: (entries: ClipEntry[] | null | undefined) => void;
  clipFor: (text: unknown) => string;
  play: (text: unknown, url?: unknown, elOrOpts?: unknown) => void;
  playClip: (text: unknown, url?: unknown, el?: unknown) => void;
  replayLast: () => boolean;
  speak: (text: unknown, elOrOpts?: unknown) => void;
  silence: () => void;
  stopAll: () => void;
  stopStory: () => void;
  pauseStory: () => void;
  startStory: (lines: unknown, from?: unknown, clips?: unknown) => void;
}

const VOWELS = /[\u0591-\u05C7]/g;

function host(): SpeechHost {
  if (typeof window !== "undefined") return window as unknown as SpeechHost;
  return globalThis as unknown as SpeechHost;
}

/** Read once when the script runs, the same moment the old IIFE captured HebrewUtils. */
const hebrewUtils = host().HebrewUtils;

function synthesis(root: SpeechHost): SpeechSynthesisLike | null {
  if (!("speechSynthesis" in root)) return null;
  return root.speechSynthesis || null;
}

const speech: HebrewSpeechApi = {
  hebrewVoice: null,
  story: { run: 0, status: "idle", index: 0, utterance: null, clips: [] },
  clipsExact: Object.create(null) as Record<string, string>,
  clipsStripped: Object.create(null) as Record<string, string>,
  audioEl: null,
  playback: { status: "idle", el: null },
  _playingEl: null,
  _lastClip: null,
  resolveAudioUrl: () => "",
  setClips: () => {},
  clipFor: () => "",
  play: () => {},
  playClip: () => {},
  replayLast: () => false,
  speak: () => {},
  silence: () => {},
  stopAll: () => {},
  stopStory: () => {},
  pauseStory: () => {},
  startStory: () => {},
};

function resolvePlayEl(elOrOpts: unknown): PlayEl | null {
  if (!elOrOpts || typeof elOrOpts !== "object") return null;
  const value = elOrOpts as PlayOpts;
  if (value.nodeType === 1) return value as PlayEl;
  if (value.el && value.el.nodeType === 1) return value.el;
  if (value.button && value.button.nodeType === 1) return value.button;
  return null;
}

function notifyFeedback(active: boolean): void {
  const fb = host().HebrewFeedback;
  if (fb && typeof fb.setSpeechActive === "function") fb.setSpeechActive(!!active);
}

function dispatchPlayback(status: string, el: PlayEl | null): void {
  const root = host();
  speech.playback = { status: status || "idle", el: el || null };
  notifyFeedback(status === "loading" || status === "playing");
  if (typeof root.CustomEvent === "function" && typeof root.dispatchEvent === "function") {
    try {
      root.dispatchEvent(new root.CustomEvent("hebrew-speech-playback", {
        detail: { status: speech.playback.status, el: speech.playback.el },
      }));
    } catch (err) { /* ignore */ }
  }
}

function clearPlayingClasses(el?: PlayEl | null): void {
  const target = el || speech._playingEl;
  if (target && target.classList) {
    target.classList.remove("is-loading", "is-playing");
  }
  if (!el || el === speech._playingEl) speech._playingEl = null;
}

function setPlaybackUi(status: string, el: PlayEl | null): void {
  if (speech._playingEl && speech._playingEl !== el) clearPlayingClasses(speech._playingEl);
  if (el && el.classList) {
    speech._playingEl = el;
    el.classList.toggle("is-loading", status === "loading");
    el.classList.toggle("is-playing", status === "playing");
  } else if (status === "idle" || status === "error") {
    clearPlayingClasses();
  }
  dispatchPlayback(status, el || null);
}

function endPlayback(status?: string): void {
  const el = speech._playingEl;
  clearPlayingClasses(el);
  dispatchPlayback(status || "idle", null);
}

function normalizeKey(text: unknown): string {
  return String(text || "").normalize("NFC").replace(/\s+/g, " ").trim();
}

function stripKey(text: unknown): string {
  if (hebrewUtils && typeof hebrewUtils.stripVowels === "function") {
    return normalizeKey(hebrewUtils.stripVowels(text as string));
  }
  return normalizeKey(String(text || "").replace(VOWELS, ""));
}

function resolveAudioUrl(audioPath: unknown): string {
  if (!audioPath) return "";
  const root = host();
  try {
    const url = new URL(String(audioPath), root.location ? root.location.href : "http://localhost/");
    url.pathname = url.pathname
      .split("/")
      .map((part) => {
        if (!part) return part;
        try { return encodeURIComponent(decodeURIComponent(part)); }
        catch (err) { return encodeURIComponent(part); }
      })
      .join("/");
    return url.href;
  } catch (err) {
    return String(audioPath);
  }
}

function sharedAudio(): AudioLike | null {
  const root = host();
  if (!speech.audioEl && typeof root.Audio === "function") {
    speech.audioEl = new root.Audio();
    speech.audioEl.preload = "auto";
  }
  return speech.audioEl;
}

function stopSharedAudio(): void {
  const audio = speech.audioEl;
  if (!audio) return;
  try {
    audio.onended = null;
    audio.onerror = null;
    audio.pause();
    audio.removeAttribute("src");
    audio.load();
  } catch (err) { /* ignore */ }
  endPlayback("idle");
}

function pickHebrewVoice(): VoiceLike | null {
  const synth = synthesis(host());
  if (!synth) return null;
  const voices = synth.getVoices() || [];
  const prefer = voices.find((v) => /carmit|yael|female/i.test(v.name) && /he/i.test(v.lang))
    || voices.find((v) => /he/i.test(v.lang));
  speech.hebrewVoice = prefer || null;
  return speech.hebrewVoice;
}

function buildUtterance(text: unknown): UtteranceLike {
  const root = host();
  const Ctor = root.SpeechSynthesisUtterance as (new (value: unknown) => UtteranceLike) | undefined;
  const utterance = new (Ctor as new (value: unknown) => UtteranceLike)(text);
  utterance.lang = "he-IL";
  utterance.rate = 0.85;
  const voice = speech.hebrewVoice || pickHebrewVoice();
  if (voice) utterance.voice = voice;
  return utterance;
}

function whenVoicesReady(start: () => void): void {
  const root = host();
  const synth = synthesis(root);
  if (!synth) return;
  const voices = synth.getVoices();
  if (voices && voices.length) {
    pickHebrewVoice();
    start();
    return;
  }
  let done = false;
  const finish = () => {
    if (done) return;
    done = true;
    pickHebrewVoice();
    start();
  };
  synth.addEventListener("voiceschanged", finish, { once: true });
  root.setTimeout!(finish, 500);
}

function speakTts(text: unknown, el: PlayEl | null): void {
  const root = host();
  const synth = synthesis(root);
  if (!synth || !text) {
    endPlayback("error");
    return;
  }
  whenVoicesReady(() => {
    const utterance = buildUtterance(text);
    utterance.onend = () => endPlayback("idle");
    utterance.onerror = () => endPlayback("error");
    setPlaybackUi("playing", el || speech._playingEl);
    synth.cancel();
    root.setTimeout!(() => synth.speak(utterance), 60);
  });
}

function setClips(entries: ClipEntry[] | null | undefined): void {
  speech.clipsExact = Object.create(null) as Record<string, string>;
  speech.clipsStripped = Object.create(null) as Record<string, string>;
  const strippedCounts: Record<string, number> = Object.create(null) as Record<string, number>;
  const strippedFirst: Record<string, string> = Object.create(null) as Record<string, string>;
  (entries || []).forEach((entry) => {
    if (!entry || !entry.text || !entry.url) return;
    const exact = normalizeKey(entry.text);
    const url = String(entry.url);
    if (!exact) return;
    speech.clipsExact[exact] = url;
    const stripped = stripKey(exact);
    if (!stripped) return;
    strippedCounts[stripped] = (strippedCounts[stripped] || 0) + 1;
    if (!strippedFirst[stripped]) strippedFirst[stripped] = url;
  });
  Object.keys(strippedCounts).forEach((key) => {
    if (strippedCounts[key] === 1) speech.clipsStripped[key] = strippedFirst[key];
  });
}

function clipFor(text: unknown): string {
  const exact = normalizeKey(text);
  if (!exact) return "";
  if (speech.clipsExact[exact]) return speech.clipsExact[exact];
  const stripped = stripKey(exact);
  return speech.clipsStripped[stripped] || "";
}

function play(text: unknown, url?: unknown, elOrOpts?: unknown): void {
  const root = host();
  const el = resolvePlayEl(elOrOpts);
  const clip = url || clipFor(text);
  speech.stopStory();
  const synth = synthesis(root);
  if (synth) synth.cancel();
  speech._lastClip = { text: (text || "") as string, url: (clip || "") as string, el: el || null };
  if (!clip) {
    speakTts(text, el);
    return;
  }
  const audio = sharedAudio();
  if (!audio) {
    speakTts(text, el);
    return;
  }
  const resolved = resolveAudioUrl(clip);
  let settled = false;
  const fallback = () => {
    if (settled) return;
    settled = true;
    speakTts(text, el);
  };
  setPlaybackUi("loading", el);
  audio.onended = () => endPlayback("idle");
  audio.onerror = () => {
    endPlayback("error");
    fallback();
  };
  try {
    audio.pause();
  } catch (err) { /* ignore */ }
  audio.src = resolved;
  const started = audio.play();
  if (started && typeof started.then === "function") {
    started.then(() => {
      settled = true;
      setPlaybackUi("playing", el);
    }).catch(() => {
      endPlayback("error");
      fallback();
    });
  } else {
    settled = true;
    setPlaybackUi("playing", el);
  }
}

function replayLast(): boolean {
  const last = speech._lastClip;
  if (!last || (!last.text && !last.url)) return false;
  speech.play(last.text, last.url || null, last.el || null);
  return true;
}

function speak(text: unknown, elOrOpts?: unknown): void {
  if (!text) return;
  const el = resolvePlayEl(elOrOpts);
  const clip = clipFor(text);
  if (clip) {
    speech.play(text, clip, el);
    return;
  }
  speech.stopStory();
  speech._lastClip = { text: (text || "") as string, url: "", el: el || null };
  speakTts(text, el);
}

function stopStory(): void {
  speech.story.run += 1;
  speech.story.status = "idle";
  speech.story.index = 0;
  speech.story.utterance = null;
  speech.story.clips = [];
  stopSharedAudio();
  const synth = synthesis(host());
  if (synth) synth.cancel();
  if (typeof speech.onStoryPaint === "function") speech.onStoryPaint();
}

function silence(): void {
  speech.stopStory();
  const synth = synthesis(host());
  if (synth) synth.cancel();
  endPlayback("idle");
}

function stopAll(): void {
  speech.silence();
  stopSharedAudio();
}

function pauseStory(): void {
  speech.story.run += 1;
  speech.story.status = "paused";
  speech.story.utterance = null;
  stopSharedAudio();
  const synth = synthesis(host());
  if (synth) synth.cancel();
  if (typeof speech.onStoryPaint === "function") speech.onStoryPaint();
}

function readLine(run: number, list: unknown[]): void {
  if (run !== speech.story.run) return;
  if (speech.story.index >= list.length) {
    speech.story.status = "idle";
    speech.story.index = 0;
    speech.story.utterance = null;
    stopSharedAudio();
    notifyFeedback(false);
    if (typeof speech.onStoryPaint === "function") speech.onStoryPaint();
    return;
  }
  const text = list[speech.story.index];
  const clip = speech.story.clips[speech.story.index];
  if (typeof speech.onStoryPaint === "function") speech.onStoryPaint();

  if (clip) {
    const audio = sharedAudio();
    if (!audio) {
      readLineTts(run, list, text);
      return;
    }
    const resolved = resolveAudioUrl(clip);
    audio.onended = () => {
      if (run !== speech.story.run) return;
      speech.story.index += 1;
      if (typeof speech.onStoryPaint === "function") speech.onStoryPaint();
      readLine(run, list);
    };
    audio.onerror = () => {
      if (run !== speech.story.run) return;
      readLineTts(run, list, text);
    };
    try { audio.pause(); } catch (err) { /* ignore */ }
    audio.src = resolved;
    const started = audio.play();
    if (started && typeof started.then === "function") {
      started.catch(() => {
        if (run !== speech.story.run) return;
        readLineTts(run, list, text);
      });
    }
    return;
  }
  readLineTts(run, list, text);
}

function readLineTts(run: number, list: unknown[], text: unknown): void {
  if (run !== speech.story.run) return;
  const synth = synthesis(host());
  if (!synth) {
    speech.story.index += 1;
    readLine(run, list);
    return;
  }
  whenVoicesReady(() => {
    if (run !== speech.story.run) return;
    const live = synthesis(host());
    if (!live) return;
    const utterance = buildUtterance(text);
    speech.story.utterance = utterance;
    utterance.onend = () => {
      if (run !== speech.story.run) return;
      speech.story.index += 1;
      if (typeof speech.onStoryPaint === "function") speech.onStoryPaint();
      readLine(run, list);
    };
    utterance.onerror = () => {
      if (run !== speech.story.run) return;
      speech.stopStory();
    };
    live.speak(utterance);
    if (typeof speech.onStoryPaint === "function") speech.onStoryPaint();
  });
}

function startStory(lines: unknown, from?: unknown, clips?: unknown): void {
  const list = (lines || []) as unknown[];
  if (!list.length) return;
  speech.story.run += 1;
  const run = speech.story.run;
  speech.story.status = "playing";
  speech.story.index = Math.max(0, ((from as number) || 0) as number);
  speech.story.clips = Array.isArray(clips) ? clips.slice() : [];
  stopSharedAudio();
  const synth = synthesis(host());
  if (synth) synth.cancel();
  notifyFeedback(true);
  if (typeof speech.onStoryPaint === "function") speech.onStoryPaint();
  readLine(run, list);
}

speech.resolveAudioUrl = resolveAudioUrl;
speech.setClips = setClips;
speech.clipFor = clipFor;
speech.play = play;
speech.playClip = function (text, url, el) {
  speech.play(text, url, el);
};
speech.replayLast = replayLast;
speech.speak = speak;
speech.silence = silence;
speech.stopAll = stopAll;
speech.stopStory = stopStory;
speech.pauseStory = pauseStory;
speech.startStory = startStory;

export function installSpeech(root: SpeechHost = host()): HebrewSpeechApi {
  root.HebrewSpeech = speech;
  if (typeof root.addEventListener === "function") {
    root.addEventListener("pagehide", () => {
      stopSharedAudio();
      const synth = synthesis(root);
      if (synth) synth.cancel();
    });
  }
  return speech;
}

installSpeech();
