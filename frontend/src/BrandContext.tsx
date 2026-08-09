import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { MARK_PATH } from "./Logo";

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

const HUE: Record<Brand, string> = { green: "#5DE31D", violet: "#8E47FF" };

/**
 * Repaint the tab icon in the current brand, as a data URI.
 *
 * Pointing the link at a different *file* is the obvious move and it does not
 * work: browsers do not re-fetch a favicon just because its href changed, and
 * removing and re-inserting the element does not reliably force it either.
 * Both were tried. A data URI sidesteps the question entirely — there is no
 * resource to re-fetch, so the browser has nothing to decide, and a new URI is
 * a new icon.
 *
 * Built from the same path the wordmark draws, so the tab and the header can
 * never disagree. The generated files in public/ are still what the first
 * paint and the manifest use; this only takes over once JS runs.
 *
 * Only the SVG link is replaced. Every browser that supports switching an icon
 * at runtime also prefers image/svg+xml, so rewriting the PNG fallbacks would
 * be work nothing reads.
 */
function applyFavicon(brand: Brand) {
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">` +
    `<path fill="${HUE[brand]}" d="${MARK_PATH}"/></svg>`;

  document
    .querySelectorAll('link[rel~="icon"][type="image/svg+xml"]')
    .forEach((l) => l.remove());

  const link = document.createElement("link");
  link.rel = "icon";
  link.type = "image/svg+xml";
  // encodeURIComponent rather than base64: the payload is one path, and the
  // percent-encoded form stays greppable in devtools.
  link.href = `data:image/svg+xml,${encodeURIComponent(svg)}`;
  document.head.appendChild(link);
}

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
}

const Ctx = createContext<BrandCtx | null>(null);

export function BrandProvider({ children }: { children: ReactNode }) {
  const [brand, setBrandState] = useState<Brand>(storedBrand);

  useEffect(() => {
    const root = document.documentElement;
    // Absent for green so the default costs nothing.
    if (brand === "violet") root.setAttribute("data-brand", "violet");
    else root.removeAttribute("data-brand");
    applyFavicon(brand);
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
      value={{ brand, setBrand }}
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

/**
 * The brand the API should render server-side artifacts (emails) in.
 *
 * Read from localStorage rather than the context for the same reason as
 * currentLang: api.ts is a plain module with no hooks. Kept beside the
 * provider so the two can never disagree about the storage key.
 */
export function currentBrand(): "green" | "violet" {
  try {
    return localStorage.getItem(BRAND_KEY) === "violet" ? "violet" : "green";
  } catch {
    return "green";
  }
}
