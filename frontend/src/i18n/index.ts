import { zh } from "./zh";
import { en } from "./en";

/**
 * Two languages, one shape.
 *
 * The dictionary is a nested object of strings and functions, not a flat map of
 * string keys, and that is the whole design. `t.upload.dropHere` is checked by
 * the compiler: a typo is a build error, a key removed from zh but still used
 * somewhere is a build error, and — the one that matters at this size — `en`
 * is typed as `Dict`, so **a missing translation fails the build** rather than
 * silently rendering `undefined` on a page nobody happened to open.
 *
 * Interpolation is a function in the dictionary rather than a `{0}` placeholder
 * scheme, which means the arguments are typed too, and the two languages are
 * free to put them in different places — English and Chinese frequently want a
 * different order, and a positional format string cannot express that.
 *
 * No library. react-i18next would bring namespace loading, plural rules for 40
 * languages and a runtime key lookup we do not need; what this project needs is
 * two objects and a context, which is what this is.
 */
export type Lang = "zh" | "en";

export type Dict = typeof zh;

export const dicts: Record<Lang, Dict> = { zh, en };

/**
 * English pluralisation, kept here because Chinese has none and every call site
 * that needs it would otherwise invent its own.
 *
 * Deliberately not a general plural-rules engine: these two languages need
 * exactly this much, and a `Intl.PluralRules` wrapper would be more code than
 * the thing it replaces.
 */
export function plural(n: number, one: string, many: string): string {
  return n === 1 ? one : many;
}
