/**
 * Local flashcard answer check. Compares a typed string to a key and accept[]
 * after stripping Hebrew vowels. Suggests a grade; it does not schedule, and
 * it does not put the typed text on the signal the server receives.
 *
 * Authored-by-spec: INIT-006/SPEC-012
 */

const VOWELS = /[\u0591-\u05C7]/g;

export interface HebrewUtilsLike {
  stripVowels?: (value: string) => string;
}

export interface FlashcardCard {
  word?: unknown;
  direction?: unknown;
}

export interface GradeSignal {
  word: string;
  direction: "en_he" | "he_en";
  result: "miss" | "pass";
  grade: 1 | 2 | 3 | 4;
  activity: "flashcard";
  slug?: string;
}

export interface FlashcardAnswerApi {
  stripVowels: (value: unknown) => string;
  matches: (typed: unknown, key: unknown, accept?: unknown) => boolean;
  suggestGrade: (typed: unknown, key: unknown, accept?: unknown) => 1 | 3 | null;
  keyToGrade: (key: unknown) => 1 | 2 | 3 | 4 | null;
  gradeSignal: (
    card: FlashcardCard | null | undefined,
    grade: unknown,
    slug?: unknown,
  ) => GradeSignal | null;
}

interface FlashcardHost {
  HebrewUtils?: HebrewUtilsLike;
  HebrewFlashcardAnswer?: FlashcardAnswerApi;
}

function host(): FlashcardHost {
  if (typeof window !== "undefined") return window as unknown as FlashcardHost;
  return globalThis as unknown as FlashcardHost;
}

export function stripVowels(value: unknown): string {
  const utils = host().HebrewUtils;
  const raw = String(value == null ? "" : value);
  const stripped =
    utils && typeof utils.stripVowels === "function"
      ? utils.stripVowels(raw)
      : raw.replace(VOWELS, "");
  return stripped.normalize("NFC").replace(/\s+/g, " ").trim();
}

/**
 * True when the vowel-stripped typing equals the stripped key or one accept[] entry.
 */
export function matches(typed: unknown, key: unknown, accept?: unknown): boolean {
  const got = stripVowels(typed);
  if (!got) return false;
  if (got === stripVowels(key)) return true;
  const list = Array.isArray(accept) ? accept : [];
  for (let i = 0; i < list.length; i += 1) {
    const entry: unknown = list[i];
    if (typeof entry === "string" && got === stripVowels(entry)) return true;
  }
  return false;
}

/**
 * A grade to highlight. Empty typing suggests nothing. A match suggests Good (3).
 * Any other typing suggests Again (1). The learner still chooses.
 */
export function suggestGrade(typed: unknown, key: unknown, accept?: unknown): 1 | 3 | null {
  if (!stripVowels(typed)) return null;
  return matches(typed, key, accept) ? 3 : 1;
}

export function keyToGrade(key: unknown): 1 | 2 | 3 | 4 | null {
  if (key === "1" || key === "2" || key === "3" || key === "4") {
    return Number(key) as 1 | 2 | 3 | 4;
  }
  return null;
}

/**
 * Signal fields the server accepts. Typed text, the prompt, and the answer
 * are never copied onto it.
 */
export function gradeSignal(
  card: FlashcardCard | null | undefined,
  grade: unknown,
  slug?: unknown,
): GradeSignal | null {
  const g = Number(grade);
  if (g !== 1 && g !== 2 && g !== 3 && g !== 4) return null;
  const signal: GradeSignal = {
    word: card && card.word ? String(card.word) : "",
    direction: card && card.direction === "en_he" ? "en_he" : "he_en",
    result: g === 1 ? "miss" : "pass",
    grade: g,
    activity: "flashcard",
  };
  if (slug) signal.slug = String(slug);
  return signal;
}

const api: FlashcardAnswerApi = {
  stripVowels,
  matches,
  suggestGrade,
  keyToGrade,
  gradeSignal,
};

export function installFlashcardAnswer(root: FlashcardHost = host()): FlashcardAnswerApi {
  root.HebrewFlashcardAnswer = api;
  return api;
}

installFlashcardAnswer();
