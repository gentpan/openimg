import { useEffect, useSyncExternalStore } from "react";
import { aiApi } from "./api";
import type { AIStatus } from "./types";

/**
 * One answer to "is AI generation on, and how much is left", shared by the nav
 * and the page.
 *
 * A module-level store rather than a context because the nav renders on every
 * route: a hook that fetched per mount would ask the server again on every
 * navigation for a value that decides whether one link is drawn. The cache is
 * filled once per session and refreshed explicitly — after a submission, and
 * after a generation settles, since a failure refunds the credit.
 *
 * Read through `useSyncExternalStore`, which is what this is: state that lives
 * outside React and changes without a render. That also keeps the subscribers
 * in agreement — the nav and the page can never show a different count.
 */
let cache: AIStatus | null = null;
let inflight: Promise<AIStatus> | null = null;
const listeners = new Set<() => void>();

function subscribe(fn: () => void) {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

/** Must stay referentially stable between publishes, hence the plain read. */
function snapshot(): AIStatus | null {
  return cache;
}

function publish(s: AIStatus) {
  cache = s;
  listeners.forEach((fn) => fn());
}

function fetchOnce(): Promise<AIStatus> {
  if (!inflight) {
    inflight = aiApi
      .status()
      .then((s) => {
        publish(s);
        return s;
      })
      .finally(() => {
        inflight = null;
      });
  }
  return inflight;
}

/** Refetches and notifies every subscriber. Returns null if the call failed. */
export async function refreshAIStatus(): Promise<AIStatus | null> {
  try {
    const s = await aiApi.status();
    publish(s);
    return s;
  } catch {
    return null;
  }
}

/** Drops the cached answer — call when the session changes, since credits are
 *  per account and this cache outlives a sign-out otherwise. */
export function resetAIStatus() {
  cache = null;
  inflight = null;
  listeners.forEach((fn) => fn());
}

/**
 * `active` gates the request: the endpoint requires a session, so asking while
 * signed out earns a 401 on every page load for nothing.
 *
 * A failed request resolves to "off" rather than staying unknown. The entry
 * point is meant to be absent when the feature is unavailable, and a link that
 * leads to an error page is worse than no link.
 */
export function useAIStatus(active = true): { status: AIStatus | null; loading: boolean } {
  const status = useSyncExternalStore(subscribe, snapshot, snapshot);

  useEffect(() => {
    if (!active || cache) return;
    fetchOnce().catch(() => publish({ enabled: false }));
  }, [active]);

  return { status, loading: active && status === null };
}
