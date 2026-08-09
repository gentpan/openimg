/** LiteZoom — the vanilla lightbox served from /static/litezoom.min.js.
 *
 * Loaded as a plain script in index.html rather than bundled: it is shared
 * with other sites, framework-free, and its CSS is embedded — bundling would
 * only re-minify it and tie its cache lifetime to the app bundle's.
 */
interface LiteZoomItem {
  src: string;
  thumb?: string;
  caption?: string;
}

interface LiteZoomApi {
  open(items: LiteZoomItem[], index?: number, opts?: { mode?: "simple" | "full" }): void;
  close(): void;
  bind(selector: string, opts?: Record<string, unknown>): void;
  enhance(selector: string, opts?: Record<string, unknown>): void;
  refresh(root?: Element): void;
  labels(map: Partial<Record<"viewer" | "prev" | "next" | "zoomIn" | "zoomOut" | "close", string> & { thumb: (i: number) => string }>): void;
}

interface Window {
  LiteZoom?: LiteZoomApi;
}
