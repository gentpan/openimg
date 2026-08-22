// Route chunk prefetching.
//
// The lazy() imports in App.tsx make each page a separate chunk that loads on
// navigation; this module starts that load a beat earlier — on hover or focus
// — so the click usually finds the chunk already in cache. The specifiers must
// stay identical to App.tsx's or the bundler emits a second copy of the page.
const loaders: Record<string, () => Promise<unknown>> = {
  "/dashboard": () => import("./pages/Dashboard"),
  "/upload": () => import("./pages/Upload"),
  "/gallery": () => import("./pages/Gallery"),
  "/generate": () => import("./pages/Generate"),
  "/retouch": () => import("./pages/Retouch"),
  "/space": () => import("./pages/Space"),
  "/settings": () => import("./pages/Settings"),
  "/refer": () => import("./pages/Refer"),
  "/docs": () => import("./pages/Docs"),
  "/login": () => import("./pages/Login"),
  "/register": () => import("./pages/Register"),
};

// prefetch kicks off the chunk load for a known route path. Unknown paths
// (short links, the admin console) are a no-op. Failures are silent by
// design: a prefetch miss just means the normal lazy load happens instead.
export function prefetch(path: string) {
  loaders[path]?.().catch(() => {});
}
