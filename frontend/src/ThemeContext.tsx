import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

export type Theme = "dark" | "light";

interface ThemeCtx {
  theme: Theme;
  /** What the user actually chose. "system" means "keep following the OS". */
  preference: Theme | "system";
  setPreference: (p: Theme | "system") => void;
  toggle: () => void;
}

const Ctx = createContext<ThemeCtx | null>(null);

const STORAGE_KEY = "openimg.theme";

function systemTheme(): Theme {
  return window.matchMedia?.("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

function storedPreference(): Theme | "system" {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "dark" || v === "light" || v === "system") return v;
  } catch {
    // Private browsing and some embedded webviews throw on localStorage.
    // Falling through to "system" is fine — the theme just won't persist.
  }
  return "system";
}

/**
 * Theme state. The actual colours live in CSS: index.css defines the dark ramp
 * on :root and overrides it under [data-theme="light"], so switching themes is
 * one attribute write rather than a re-render of anything that draws colour.
 *
 * The initial value is applied by an inline script in index.html, before React
 * mounts — doing it here instead would paint the dark page first and then flash
 * to light.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreferenceState] = useState<Theme | "system">(storedPreference);
  const [systemPref, setSystemPref] = useState<Theme>(systemTheme);

  const theme: Theme = preference === "system" ? systemPref : preference;

  // Keep following the OS while the preference is "system". The listener is
  // registered regardless so switching back to "system" needs no re-subscribe.
  useEffect(() => {
    const mq = window.matchMedia?.("(prefers-color-scheme: light)");
    if (!mq) return;
    const onChange = (e: MediaQueryListEvent) => setSystemPref(e.matches ? "light" : "dark");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    root.setAttribute("data-theme", theme);
    // color-scheme drives the native bits CSS can't reach: form controls,
    // scrollbars, and the default canvas colour behind the page.
    root.style.colorScheme = theme;
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute("content", theme === "light" ? "#ffffff" : "#0a0a0a");
  }, [theme]);

  function setPreference(p: Theme | "system") {
    setPreferenceState(p);
    try {
      localStorage.setItem(STORAGE_KEY, p);
    } catch {
      // Non-persistent is acceptable; the in-memory switch still works.
    }
  }

  return (
    <Ctx.Provider
      value={{
        theme,
        preference,
        setPreference,
        // Toggling picks the opposite of what is on screen and pins it, rather
        // than cycling through "system" — a toggle that sometimes doesn't
        // change anything is a broken toggle.
        toggle: () => setPreference(theme === "dark" ? "light" : "dark"),
      }}
    >
      {children}
    </Ctx.Provider>
  );
}

export function useTheme() {
  const c = useContext(Ctx);
  if (!c) throw new Error("useTheme 必须在 ThemeProvider 内使用");
  return c;
}
