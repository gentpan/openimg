import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

/**
 * Which brand hue the page is painted in.
 *
 * This replaced a ThemeContext that also carried light/dark. The light theme is
 * gone — the product is dark-only — and what is left is one axis, so the module
 * is named after it rather than keeping a "theme" that no longer themes
 * anything.
 *
 * The colours themselves live in CSS: index.css defines the green on :root and
 * the violet under [data-brand="violet"], so switching is one attribute write
 * rather than a re-render of anything that draws colour.
 *
 * The initial value is applied by an inline script in index.html, before React
 * mounts — doing it here instead would paint green first and then flip.
 */
export type Brand = "green" | "violet";

const BRAND_KEY = "openimg-brand";

function storedBrand(): Brand {
  try {
    return localStorage.getItem(BRAND_KEY) === "violet" ? "violet" : "green";
  } catch {
    // Private browsing and some embedded webviews throw on localStorage.
    return "green";
  }
}

interface BrandCtx {
  brand: Brand;
  setBrand: (b: Brand) => void;
  toggle: () => void;
}

const Ctx = createContext<BrandCtx | null>(null);

export function BrandProvider({ children }: { children: ReactNode }) {
  const [brand, setBrandState] = useState<Brand>(storedBrand);

  useEffect(() => {
    const root = document.documentElement;
    // Absent for green so the default costs nothing.
    if (brand === "violet") root.setAttribute("data-brand", "violet");
    else root.removeAttribute("data-brand");
  }, [brand]);

  function setBrand(b: Brand) {
    setBrandState(b);
    try {
      localStorage.setItem(BRAND_KEY, b);
    } catch {
      // Non-persistent is acceptable; the in-memory switch still works.
    }
  }

  return (
    <Ctx.Provider
      value={{ brand, setBrand, toggle: () => setBrand(brand === "green" ? "violet" : "green") }}
    >
      {children}
    </Ctx.Provider>
  );
}

export function useBrand() {
  const c = useContext(Ctx);
  if (!c) throw new Error("useBrand 必须在 BrandProvider 内使用");
  return c;
}
