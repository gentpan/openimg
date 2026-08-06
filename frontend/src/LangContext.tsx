import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { dicts, type Dict, type Lang } from "./i18n";

/**
 * Which language the interface is in.
 *
 * Same shape as BrandContext: the choice lives in localStorage, is applied to
 * the document before React mounts (see index.html), and switching is a state
 * change rather than a reload — nothing here talks to the server.
 *
 * `document.documentElement.lang` is set alongside, because it is not
 * decoration: screen readers pick a voice from it, Chrome offers to translate
 * pages whose lang disagrees with their content, and CJK line breaking differs
 * from Latin.
 */
const KEY = "openimg-lang";

function stored(): Lang {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "zh" || v === "en") return v;
  } catch {
    // Private browsing throws; fall through to the browser's own preference.
  }
  // First visit: follow the browser rather than defaulting to Chinese. Someone
  // arriving from an English-language link is more likely to want English than
  // to go looking for a switch.
  return typeof navigator !== "undefined" && /^zh\b/i.test(navigator.language) ? "zh" : "en";
}

interface LangCtx {
  lang: Lang;
  setLang: (l: Lang) => void;
  /** The active dictionary. Access it directly: `t.nav.upload`. */
  t: Dict;
}

const Ctx = createContext<LangCtx | null>(null);

export function LangProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(stored);

  useEffect(() => {
    document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
  }, [lang]);

  function setLang(l: Lang) {
    setLangState(l);
    try {
      localStorage.setItem(KEY, l);
    } catch {
      // Non-persistent is acceptable; the in-memory switch still works.
    }
  }

  return <Ctx.Provider value={{ lang, setLang, t: dicts[lang] }}>{children}</Ctx.Provider>;
}

export function useLang() {
  const c = useContext(Ctx);
  if (!c) throw new Error("useLang 必须在 LangProvider 内使用");
  return c;
}

/**
 * The language the API should answer in.
 *
 * Read from localStorage rather than the context because `api.ts` is a plain
 * module — it has no hooks and is imported by things that render outside the
 * provider. Kept next to the provider so the two cannot disagree about the key.
 */
export function currentLang(): Lang {
  return stored();
}
