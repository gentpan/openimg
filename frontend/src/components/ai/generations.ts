import { useCallback, useEffect, useRef, useState } from "react";
import { aiApi } from "../../api";
import { refreshAIStatus } from "../../aiStatus";
import type { AIGenStatus, AIGeneration, AIStatusOn, Image } from "../../types";

/**
 * The half of the AI pages that is not a component.
 *
 * Text-to-image and retouching are the same machine with a different intake:
 * one credit pool, one submission-then-poll shape, one history endpoint. Only
 * the composer differs — a prompt alone versus a prompt plus source pictures —
 * so that is the only thing the pages own themselves. The visual half lives
 * next door in parts.tsx; it is split from this file only because Fast Refresh
 * wants a module to export components or values, not both.
 *
 * All of it was lifted out of pages/Generate.tsx with its behaviour unchanged,
 * comments included — those reasons are what keep the next edit from undoing
 * them.
 */

/** Mirrors the server's own cap. Enforced here so the counter can turn red
 *  before a submission is rejected for a reason nothing on screen showed. */
export const MAX_PROMPT = 1000;

/** Generation takes tens of seconds; three seconds is frequent enough to feel
 *  live without turning one picture into twenty requests. */
export const POLL_MS = 3000;

/** Upstream accepts sixteen source images; the product offers four. Past that
 *  the model stops treating them as one scene, and the picker stops being a
 *  choice and becomes a file manager. */
export const MAX_SOURCES = 4;

/**
 * Still working.
 *
 * Asked as "not settled" rather than by listing the working states, because the
 * two sides of that question fail differently: a status this build has not
 * heard of should keep the spinner up and the polling going until the server
 * calls it done, not look finished and stop the one thing that would have
 * corrected it.
 */
export const inFlight = (s: AIGenStatus) => s !== "completed" && s !== "failed";

/**
 * The pic.bi balance this account can actually spend, as a number.
 *
 * Two fields have to agree before that balance is real: the link has to exist,
 * and the lookup has to have answered. `picbi_credits` is absent both when the
 * account is unlinked and when the upstream call failed, and reading it alone
 * would turn a failed lookup into a confident zero.
 */
export function picbiRemaining(status: AIStatusOn): number {
  return status.picbi_linked ? (status.picbi_credits ?? 0) : 0;
}

/**
 * Whether a submission can go through at all.
 *
 * 本地额度见底不等于不能生成:关联了 pic.bi 且那边还有钱时,后端会自动接管。
 * 只看 `remaining` 会把提交按钮封死在"pic.bi 正该出场"的那一刻,整条付费路径
 * 永远走不到——所以这个问题只有一个答案来源,两页和额度提示条都问这里。
 */
export function canSpend(status: AIStatusOn): boolean {
  return status.remaining > 0 || picbiRemaining(status) > 0;
}

export type GenKind = "generate" | "edit";

/** Records written before retouching existed carry no `kind` at all, and the
 *  server does not backfill them. They are generations — read every record
 *  through here rather than comparing the raw field. */
export function genKind(g: AIGeneration): GenKind {
  return g.kind === "edit" ? "edit" : "generate";
}

/** The source pictures an edit started from. Stored as one comma-separated
 *  column, so unpacking it is the client's job. */
export function genSources(g: AIGeneration): string[] {
  return (g.source_ids ?? "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

/**
 * The polled history, shared by both pages.
 *
 * `kind` filters what comes back: each page lists its work, so the count in its
 * header means something and "still working" refers to a row you can see. The
 * fetch itself is unfiltered — one endpoint serves both — and the split happens
 * here.
 *
 * It takes a list because one page is no longer one kind: the generate page
 * submits an *edit* the moment reference pictures are attached, and a filter of
 * `"generate"` alone would swallow the record the user just watched appear.
 *
 * `onSettled` fires once, on the edge where the last in-flight job finishes: a
 * completed picture consumed storage and a failed one handed a credit back, and
 * both of those numbers live outside this hook.
 */
export function useGenerations(
  active: boolean,
  kind: GenKind | GenKind[],
  onSettled: () => void,
) {
  const [gens, setGens] = useState<AIGeneration[]>([]);
  const [images, setImages] = useState<Record<string, Image>>({});

  // Every list fetch takes a ticket, and only the newest one may write. Without
  // it a poll issued three seconds ago can land *after* a submission and
  // overwrite the record that submission just put on screen — and with nothing
  // pending any more, the polling that was about to correct the list stops too,
  // so the generation stays invisible until a reload.
  const ticket = useRef(0);

  // Held in a ref because callers build it from `refresh()` off the auth
  // context, which is a fresh function on every render — in a dependency array
  // it would re-arm the effects below on every keystroke.
  const settled = useRef(onSettled);
  useEffect(() => {
    settled.current = onSettled;
  });

  const load = useCallback(async () => {
    const mine = ++ticket.current;
    const r = await aiApi.generations();
    if (mine !== ticket.current) return;
    setGens(r.generations);
    setImages(r.images);
  }, []);

  useEffect(() => {
    if (!active) return;
    load().catch(() => {});
    // The cached status can be stale in either direction — a check-in grants
    // credits, a failed generation refunds one — so arriving here resyncs it.
    refreshAIStatus();
  }, [active, load]);

  const kinds = Array.isArray(kind) ? kind : [kind];
  const mine = gens.filter((g) => kinds.includes(genKind(g)));
  const working = mine.some((g) => inFlight(g.status));

  useEffect(() => {
    if (!working) return;
    const h = window.setInterval(() => {
      // A backgrounded tab is nobody watching. The next visible tick catches up.
      if (document.hidden) return;
      load().catch(() => {});
    }, POLL_MS);
    return () => window.clearInterval(h);
  }, [working, load]);

  const wasWorking = useRef(false);
  useEffect(() => {
    if (wasWorking.current && !working) settled.current();
    wasWorking.current = working;
  }, [working]);

  /** Puts a just-submitted record on screen without a round trip. Any list
   *  fetch still in flight was issued before this record existed, so it is now
   *  stale — retire its ticket rather than let it erase what was just added. */
  const prepend = useCallback((g: AIGeneration) => {
    ticket.current++;
    setGens((prev) => [g, ...prev]);
  }, []);

  return { gens: mine, images, setImages, working, prepend, reload: load };
}

/**
 * The source pictures this session has seen.
 *
 * A history row wants to show what an edit started from, but the generations
 * endpoint only promises the *result* image in its map — the sources are
 * ordinary gallery images this page may never have fetched. So what the user
 * picked or uploaded is remembered here rather than asked for again, and
 * `resolve` falls back to the server's map for anything it does not hold.
 *
 * Both AI pages need it now that either of them can submit an edit.
 */
export function useKnownImages(images: Record<string, Image>) {
  const [known, setKnown] = useState<Record<string, Image>>({});

  const remember = useCallback((list: Image[]) => {
    setKnown((prev) => {
      const next = { ...prev };
      for (const img of list) next[img.id] = img;
      return next;
    });
  }, []);

  /** The record outlives its picture: once one is deleted, stop offering a
   *  thumbnail that now 404s. */
  const forget = useCallback((id: string) => {
    setKnown((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
  }, []);

  const resolve = useCallback(
    (id: string): Image | undefined => known[id] ?? images[id],
    [known, images],
  );

  return { remember, forget, resolve };
}
