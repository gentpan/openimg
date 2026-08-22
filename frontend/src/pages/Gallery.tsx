import Icon from "../Icon";
import { useCallback, useEffect, useRef, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import { SORTS, formatBytes, imageApi, type SortKey } from "../api";
import ImageDetail from "../components/ImageDetail";
import type { Image } from "../types";
import { RingSpinner } from "../components/Spinner";
import OtpConfirm from "../components/OtpConfirm";
import { useToast } from "../ToastContext";
import { useLang } from "../LangContext";
import PageSizeMenu from "../components/PageSizeMenu";
import { useDialog } from "../DialogContext";

// Every size is a multiple of the 5-column grid, so a full page never ends in
// a short row with holes where the missing cards would be.
const DEFAULT_PAGE = 25;

export default function GalleryPage() {
  const dialog = useDialog();
  const { t } = useLang();
  const { user, loading, refresh } = useAuth();
  const [images, setImages] = useState<Image[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("newest");
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE);
  const [busy, setBusy] = useState(false);
  const [wiping, setWiping] = useState<number | null>(null);
  const [wipeOpen, setWipeOpen] = useState(false);
  const toast = useToast();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [detail, setDetail] = useState<Image | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(
    async (nextOffset: number, q: string, s: SortKey, append: boolean, size: number) => {
    setBusy(true);
    try {
      const res = await imageApi.list({ limit: size, offset: nextOffset, q: q || undefined, sort: s });
      setImages((prev) => (append ? [...prev, ...res.images] : res.images));
      setTotal(res.total);
      setOffset(nextOffset);
      setErr(null);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  },
    [],
  );

  // Debounced search; also covers the initial load and any sort change.
  // Selection is dropped whenever the result set changes, since a checkbox on
  // a row you can no longer see is a good way to delete the wrong thing.
  useEffect(() => {
    if (!user) return;
    const timer = setTimeout(() => {
      setSelected(new Set());
      load(0, query, sort, false, pageSize);
    }, query ? 300 : 0);
    return () => clearTimeout(timer);
  }, [query, sort, pageSize, user, load]);

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  const allLoadedSelected = images.length > 0 && selected.size === images.length;
  function toggleAll() {
    setSelected(allLoadedSelected ? new Set() : new Set(images.map((i) => i.id)));
  }

  async function removeSelected() {
    if (selected.size === 0) return;
    const ok = await dialog.confirm({
      title: t.gallery.deleteSelectedConfirm(selected.size),
      danger: true,
      confirmLabel: t.common.delete,
    });
    if (!ok) return;
    setBusy(true);
    try {
      const ids = [...selected];
      const res = await imageApi.bulkRemove({ ids });
      setImages((prev) => prev.filter((i) => !selected.has(i.id)));
      setTotal((n) => Math.max(0, n - ids.length));
      setSelected(new Set());
      setErr(null);
      toast.success(t.gallery.deletedToast(res.deleted), t.gallery.deletedToastDetail);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      toast.error(t.common.deleteFailed, e instanceof Error ? e.message : undefined);
    } finally {
      setBusy(false);
      refresh();
    }
  }

  // "Clear everything" loops because the server caps each call — otherwise a
  // 5000-image library would silently stop at the first batch.
  async function wipeAll(code: string) {
    setWipeOpen(false);
    setWiping(0);
    let done = 0;
    try {
      for (;;) {
        // The code is single-use, so only the first batch carries it; the
        // server re-checks ownership on every call regardless.
        const res = await imageApi.bulkRemove({
          all: true,
          q: query || undefined,
          ...(done === 0 ? { code } : {}),
        });
        done += res.deleted;
        setWiping(done);
        if (res.remaining === 0 || res.deleted === 0) break;
      }
      setSelected(new Set());
      await load(0, query, sort, false, pageSize);
      setErr(null);
      toast.success(t.gallery.wipeDoneToast(done), t.gallery.wipeDoneToastDetail);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      await load(0, query, sort, false, pageSize);
      toast.error(t.gallery.wipeFailed, t.gallery.wipeFailedDetail(done));
    } finally {
      setWiping(null);
      refresh();
    }
  }

  if (loading) return <Center>{t.common.loading}</Center>;
  if (!user) return <Navigate to="/login" replace />;

  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        <div className="flex flex-wrap items-center gap-3 mb-4">
          <h1 className="text-lg font-brand text-neutral-100">
            {t.gallery.title}
            <span className="ml-2 text-xs text-neutral-600 font-normal">{t.common.imageCount(total)}</span>
          </h1>
          <div className="flex-1" />

          <PageSizeMenu value={pageSize} onChange={setPageSize} />

          <SortMenu value={sort} onChange={setSort} />

          <div className="relative">
            <Icon name="magnifying-glass" className="absolute left-3 top-1/2 -translate-y-1/2 text-[10px] text-neutral-600"  />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t.gallery.searchPlaceholder}
              className="rounded-lg bg-neutral-900 border border-neutral-800 pl-8 pr-3 py-1.5 text-xs outline-none focus:border-brand-500 transition w-44"
            />
          </div>

          <Link
            to="/upload"
            className="rounded-lg bg-brand-600 px-3 py-1.5 text-xs font-medium text-brand-ink hover:bg-brand-500 transition"
          >
            <Icon name="plus" className="mr-1.5"  />
            {t.common.upload}
          </Link>
        </div>

        {/* Selection bar. Always present once there's anything to act on, so
            the controls don't shift the grid down the moment you tick a box. */}
        {images.length > 0 && (
          <div className="flex flex-wrap items-center gap-2 mb-4 rounded-xl border border-neutral-800 bg-neutral-900/40 px-3 py-2">
            <button
              onClick={toggleAll}
              className="flex items-center gap-2 text-xs text-neutral-400 hover:text-neutral-100 transition"
            >
              <span
                className={`w-4 h-4 rounded border flex items-center justify-center text-[8px] transition ${
                  allLoadedSelected
                    ? "bg-brand-600 border-brand-600 text-brand-ink"
                    : selected.size > 0
                      ? "border-brand-500 text-brand-400"
                      : "border-neutral-600 text-transparent"
                }`}
              >
                <Icon name={allLoadedSelected ? "check" : "minus"}  />
              </span>
              {allLoadedSelected ? t.gallery.clearSelection : t.gallery.selectAllOnPage}
            </button>

            <span className="text-[11px] text-neutral-600">
              {selected.size > 0 ? t.gallery.selectedCount(selected.size) : t.gallery.loadedCount(images.length, total)}
            </span>

            <div className="flex-1" />

            {selected.size > 0 && (
              <button
                onClick={removeSelected}
                disabled={busy || wiping !== null}
                className="rounded-lg bg-red-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-700 disabled:opacity-50 transition"
              >
                <Icon name="trash-can" className="mr-1.5"  />
                {t.gallery.deleteSelected(selected.size)}
              </button>
            )}

            <button
              onClick={() => setWipeOpen(true)}
              disabled={busy || wiping !== null || total === 0}
              title={query ? t.gallery.wipeSearchTitle : t.gallery.wipeAllTitle}
              className="rounded-lg border border-red-900/60 px-3 py-1.5 text-xs text-red-400 hover:bg-red-950/40 hover:border-red-800 disabled:opacity-40 transition"
            >
              {wiping !== null ? (
                <>
                  <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px] mr-1.5" />
                  {t.gallery.wipeProgress(wiping)}
                </>
              ) : (
                <>
                  <Icon name="fire" className="mr-1.5"  />
                  {query ? t.gallery.wipeSearch : t.gallery.wipeAll}
                </>
              )}
            </button>
          </div>
        )}

        {err && <div className="mb-4 text-xs text-red-400">{err}</div>}

        {images.length === 0 && !busy ? (
          <div className="text-center py-24">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-neutral-900 mb-4">
              <Icon name="images" className="text-2xl text-brand-500"  />
            </div>
            <div className="text-sm text-neutral-400">{query ? t.gallery.emptySearch : t.common.noUploadsYet}</div>
            {!query && (
              <Link to="/upload" className="mt-3 inline-block text-xs text-brand-400 hover:underline">
                {t.common.uploadFirst}
              </Link>
            )}
          </div>
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
              {images.map((img) => (
                <Card
                  key={img.id}
                  img={img}
                  selected={selected.has(img.id)}
                  anySelected={selected.size > 0}
                  onToggle={() => toggle(img.id)}
                  onOpen={() => setDetail(img)}
                />
              ))}
            </div>

            {images.length < total && (
              <div className="mt-6 text-center">
                <button
                  onClick={() => load(offset + pageSize, query, sort, true, pageSize)}
                  disabled={busy}
                  className="rounded-xl bg-neutral-900 border border-neutral-800 px-5 py-2 text-xs text-neutral-300 hover:border-neutral-700 disabled:opacity-50 transition"
                >
                  {busy ? <RingSpinner className="h-3.5 w-3.5 inline-block align-[-2px]" /> : t.gallery.loadMore}
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {wipeOpen && (
        <OtpConfirm
          purpose="purge"
          danger
            detail={query ? t.gallery.wipeConfirmSearch(total, query) : t.gallery.wipeConfirmAll(total)}
          onCancel={() => setWipeOpen(false)}
          onVerified={wipeAll}
        />
      )}

      {detail && (
        <ImageDetail
          img={detail}
          onClose={() => setDetail(null)}
          onDeleted={() => {
            setImages((prev) => prev.filter((i) => i.id !== detail.id));
            setTotal((n) => Math.max(0, n - 1));
            setDetail(null);
            refresh();
          }}
        />
      )}
      <Footer />
    </div>
  );
}


function SortMenu({ value, onChange }: { value: SortKey; onChange: (v: SortKey) => void }) {
  const { t } = useLang();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const current = SORTS.find((s) => s.key === value) ?? SORTS[0];

  useEffect(() => {
    if (!open) return;
    function onDoc(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-1.5 rounded-lg bg-neutral-900 border border-neutral-800 px-3 py-1.5 text-xs text-neutral-300 hover:border-neutral-700 transition"
      >
        <Icon name={current.icon} className={`text-[10px] text-neutral-500`}  />
        {t.gallery.sort[current.key]}
        <Icon name="chevron-down" className="text-[8px] text-neutral-600"  />
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1.5 z-20 w-40 rounded-xl border border-neutral-800 bg-neutral-900 py-1 shadow-panel">
          {SORTS.map((s) => (
            <button
              key={s.key}
              onClick={() => {
                onChange(s.key);
                setOpen(false);
              }}
              className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs transition ${
                s.key === value ? "text-brand-300 bg-brand-950/30" : "text-neutral-400 hover:bg-neutral-800/60"
              }`}
            >
              <Icon name={s.icon} className={`text-[10px] w-3`}  />
              {t.gallery.sort[s.key]}
              {s.key === value && <Icon name="check" className="ml-auto text-[9px]"  />}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function Card({
  img,
  selected,
  anySelected,
  onToggle,
  onOpen,
}: {
  img: Image;
  selected: boolean;
  anySelected: boolean;
  onToggle: () => void;
  onOpen: () => void;
}) {
  const { t } = useLang();
  return (
    <div
      className={`group relative rounded-2xl overflow-hidden border transition ${
        selected ? "border-brand-500" : "border-neutral-800 hover:border-neutral-700"
      } bg-neutral-900/40`}
    >
      <button onClick={onOpen} className="block w-full aspect-square">
        <img src={img.thumb_url} alt={img.orig_name} loading="lazy" className="w-full h-full object-cover" />
      </button>

      {/* Once anything is selected the checkboxes stay visible: hunting for a
          hover target is a bad way to build a multi-select. */}
      <button
        onClick={onToggle}
        title={selected ? t.gallery.deselect : t.gallery.select}
        className={`absolute top-2 left-2 w-5 h-5 rounded-md border flex items-center justify-center text-[9px] transition ${
          selected
            ? "bg-brand-600 border-brand-600 text-brand-ink"
            : `bg-black/50 border-white/50 text-transparent hover:text-white ${
                anySelected ? "opacity-100" : "opacity-0 group-hover:opacity-100"
              }`
        }`}
      >
        <Icon name="check"  />
      </button>

      {img.status === "blocked" && (
        <span className="absolute top-2 right-2 rounded-full bg-red-900/80 px-1.5 py-0.5 text-[9px] text-red-200">
          {t.gallery.blocked}
        </span>
      )}

      <div className="px-2.5 py-2">
        <div className="text-[11px] text-neutral-400 truncate" title={img.orig_name}>
          {img.orig_name}
        </div>
        {/* 格式靠右单列:五列网格下卡片只有一百多像素宽,把格式接在尺寸后面会
            第一个被 truncate 掉。而它恰恰是最该看见的一项——文件名写着 .jpeg,
            存进来的其实是 webp。 */}
        <div className="text-[10px] text-neutral-600 flex items-baseline justify-between gap-1.5">
          <span className="truncate">
            {img.width}×{img.height} · {formatBytes(img.size_stored, 0)}
          </span>
          {img.ext && <span className="shrink-0 uppercase">{img.ext}</span>}
        </div>
      </div>
    </div>
  );
}

function Center({ children }: { children: React.ReactNode }) {
  return <div className="min-h-screen flex items-center justify-center text-neutral-500">{children}</div>;
}
