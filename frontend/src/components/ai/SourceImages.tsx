import Icon from "../../Icon";
import { useCallback, useEffect, useRef, useState, type DragEvent } from "react";
import { imageApi } from "../../api";
import { useLang } from "../../LangContext";
import { useToast } from "../../ToastContext";
import { RingSpinner } from "../Spinner";
import ImagePicker from "./ImagePicker";
import { MAX_SOURCES } from "./generations";
import type { Image } from "../../types";

/** 「最近上传」快捷条取多少张。它是捷径不是图库,一屏放得下就够。 */
const RECENT_LIMIT = 12;

/** 一张正在上传的本地文件。上传完成后它就不存在了——变成 `value` 里的一张图。 */
interface Pending {
  id: string;
  file: File;
  progress: number;
  error?: string;
}

/**
 * 给 AI 页选图的那一行:从图库挑,或者直接把本地文件拖进来。
 *
 * 两页共用一份:修图页的「源图」和生成页的「参考图」是同一件事——凑齐一到四张
 * 图库图片交给上游——差别只在必填与否和空态的分量,所以那两点用 `variant` 分,
 * 其余的选图弹层、上传、拖放、最近上传快捷条都只有这一份实现。
 *
 * 上传走的是普通上传接口 `imageApi.upload`,和上传页一模一样的那一条:文件先成为
 * 图库里一张普通图片,拿到 id 之后再被选为源图。这不是绕远路,是这里唯一说得通的
 * 路径——上游只需要一个公网地址,把字节 base64 塞进请求体等于让同样的数据在我们
 * 这里过两遍;而且生成记录存的是图库图片 id,没有 id 的图在历史里就说不出「拿哪张
 * 改的」,也点不回去。配额、去重、格式限制因此也全部照旧,一条都没有绕过。
 */
export default function SourceImages({
  value,
  onChange,
  max = MAX_SOURCES,
  variant = "required",
  onUploaded,
}: {
  value: Image[];
  onChange: (next: Image[]) => void;
  max?: number;
  /** `optional` 用于生成页的参考图:空态收成一行,不跟提示词框抢版面。 */
  variant?: "required" | "optional";
  /** 上传成功一张。页面拿它去刷新可用空间——上传是真的花掉了容量。 */
  onUploaded?: (img: Image) => void;
}) {
  const { t } = useLang();
  const toast = useToast();

  const [picking, setPicking] = useState(false);
  const [pending, setPending] = useState<Pending[]>([]);
  const [recent, setRecent] = useState<Image[]>([]);
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  // 上传是异步的,回调里读到的 props 可能已经是几秒前那一版了。所以最新的一份
  // 放在 ref 里:两张图几乎同时传完时,第二个回调必须看到第一个刚加进去的那张,
  // 否则它会拿旧数组覆盖掉。
  const valueRef = useRef(value);
  // 开弹层那一刻 value 里有哪些。用来区分"用户在弹层里故意取消的"与"弹层
  // 开着期间上传完成新加进来的"——前者该消失,后者该留下。
  const pickerBaseRef = useRef<Set<string>>(new Set());

  const openPicker = () => {
    pickerBaseRef.current = new Set(valueRef.current.map((v) => v.id));
    setPicking(true);
  };
  const onChangeRef = useRef(onChange);
  const onUploadedRef = useRef(onUploaded);
  useEffect(() => {
    valueRef.current = value;
    onChangeRef.current = onChange;
    onUploadedRef.current = onUploaded;
  });

  const pendingRef = useRef<Pending[]>([]);
  useEffect(() => {
    pendingRef.current = pending;
  }, [pending]);

  // 只在进页面时拉一次:多数改图针对的就是刚传上去那张,为它开一次选图窗再从头
  // 找一遍,步骤全花在「找回自己五秒前传的图」上。它是捷径不是图库,不必实时。
  useEffect(() => {
    let alive = true;
    imageApi
      .list({ limit: RECENT_LIMIT, offset: 0, sort: "newest" })
      .then((r) => alive && setRecent(r.images ?? []))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  const startUpload = useCallback(
    (file: File, reuseId?: string) => {
      const id =
        reuseId ??
        `${file.name}-${file.size}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

      setPending((prev) =>
        reuseId
          ? prev.map((p) => (p.id === id ? { ...p, progress: 0, error: undefined } : p))
          : [...prev, { id, file, progress: 0 }],
      );

      imageApi
        .upload(file, {
          onProgress: (pct) =>
            setPending((prev) => prev.map((p) => (p.id === id ? { ...p, progress: pct } : p))),
        })
        .then((res) => {
          setPending((prev) => prev.filter((p) => p.id !== id));
          const img = res.image;
          const cur = valueRef.current;
          // 去重命中时服务端会把已有的那张还回来,可能正是已经选中的一张。
          if (!cur.some((s) => s.id === img.id) && cur.length < max) {
            const next = [...cur, img];
            valueRef.current = next;
            onChangeRef.current(next);
          }
          setRecent((prev) => [img, ...prev.filter((r) => r.id !== img.id)].slice(0, RECENT_LIMIT));
          onUploadedRef.current?.(img);
        })
        .catch((e) => {
          const msg = e instanceof Error ? e.message : String(e);
          // 失败的那一张留在原地,带着文件名和原因——一次拖进四张时,「哪张失败了」
          // 才是唯一有用的信息。文件也留着,所以「重试」不用重新去找。
          setPending((prev) => prev.map((p) => (p.id === id ? { ...p, error: msg } : p)));
          toast.error(t.aiSource.uploadFailedTitle, `${file.name} · ${msg}`);
        });
    },
    [max, t, toast],
  );

  const accept = useCallback(
    (list: FileList | File[]) => {
      const all = Array.from(list);
      const imgs = all.filter((f) => f.type.startsWith("image/"));
      if (imgs.length < all.length) toast.error(t.aiSource.uploadFailedTitle, t.aiSource.notImage);
      if (imgs.length === 0) return;

      const busy = pendingRef.current.filter((p) => !p.error).length;
      const room = max - valueRef.current.length - busy;
      if (room <= 0) {
        toast.error(t.aiSource.uploadFailedTitle, t.aiSource.full(max));
        return;
      }
      const take = imgs.slice(0, room);
      if (take.length < imgs.length) toast.error(t.aiSource.uploadFailedTitle, t.aiSource.full(max));
      for (const f of take) startUpload(f);
    },
    [max, startUpload, t, toast],
  );

  const optional = variant === "optional";
  const busyCount = pending.filter((p) => !p.error).length;
  const room = max - value.length - busyCount;
  const empty = value.length === 0 && pending.length === 0;
  // 已经选中的排除掉——留着只会让人点了没反应,而它在上面本来就看得见。
  const recentPickable = recent.filter((r) => !value.some((s) => s.id === r.id));

  function browse() {
    inputRef.current?.click();
  }

  function remove(id: string) {
    onChange(value.filter((v) => v.id !== id));
  }

  const dropProps = {
    onDragOver: (e: DragEvent) => {
      e.preventDefault();
      setDragging(true);
    },
    onDragLeave: () => setDragging(false),
    onDrop: (e: DragEvent) => {
      e.preventDefault();
      setDragging(false);
      accept(e.dataTransfer.files);
    },
  };

  /** 两个入口并排,同样大小:从图库选和上传是两条路,不是主次。 */
  const chip =
    "inline-flex items-center gap-1.5 rounded-lg border border-neutral-800 bg-neutral-900 px-2.5 py-1.5 text-[11px] text-neutral-300 transition hover:border-brand-500 hover:text-brand-300";
  const actions = (
    <div className="flex flex-wrap items-center gap-1.5">
      <button type="button" onClick={() => openPicker()} className={chip}>
        <Icon name="images" className="text-[10px] text-neutral-500"  />
        {t.aiSource.fromGallery}
      </button>
      <button type="button" onClick={browse} className={chip}>
        <Icon name="cloud-arrow-up" className="text-[10px] text-neutral-500"  />
        {t.aiSource.uploadNew}
      </button>
    </div>
  );

  return (
    <div>
      <div className="mb-1.5 flex items-baseline justify-between gap-2">
        <span className="text-xs text-neutral-300">
          {optional ? t.aiSource.refLabel : t.aiSource.sourceLabel}
          {optional && (
            <span className="ml-1.5 text-[10px] text-neutral-600">{t.aiSource.optionalTag}</span>
          )}
        </span>
        {(!optional || value.length > 0 || pending.length > 0) && (
          <span className="text-[10px] tabular-nums text-neutral-600">
            {t.aiSource.count(value.length, max)}
          </span>
        )}
      </div>

      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        multiple
        className="hidden"
        onChange={(e) => {
          if (e.target.files) accept(e.target.files);
          // 同一个文件连传两次也要触发 change。
          e.target.value = "";
        }}
      />

      <div {...dropProps}>
        {empty ? (
          optional ? (
            /* 生成页的空态:参考图多数时候是不填的,所以它收成一行,把版面留给
               提示词框。想用的人看得见两个入口,不想用的人几乎不会注意到它。 */
            <div
              className={`flex flex-wrap items-center gap-2 rounded-xl border border-dashed px-3 py-2.5 transition ${
                dragging ? "border-brand-500 bg-brand-950/20" : "border-neutral-800 bg-neutral-950"
              }`}
            >
              {actions}
              <span className="text-[10px] text-neutral-600">
                {dragging ? t.aiSource.dropHere : t.aiSource.refHint(max)}
              </span>
            </div>
          ) : (
            /* 修图页的空态:没有源图,下面写什么都没有意义,所以它占得起这个块。 */
            <div
              className={`flex flex-col items-center justify-center gap-2.5 rounded-xl border border-dashed px-4 py-8 transition ${
                dragging ? "border-brand-500 bg-brand-950/20" : "border-neutral-800 bg-neutral-950"
              }`}
            >
              <Icon name="images" className="text-lg text-neutral-600"  />
              {actions}
              <span className="text-[10px] text-neutral-600">
                {dragging ? t.aiSource.dropHere : t.aiSource.sourceHint(max)}
              </span>
            </div>
          )
        ) : (
          <div
            className={`flex flex-wrap items-center gap-2 rounded-xl transition ${
              dragging ? "border border-dashed border-brand-500 bg-brand-950/20 p-2" : ""
            }`}
          >
            {value.map((img, i) => (
              <div
                key={img.id}
                className="group relative h-20 w-20 overflow-hidden rounded-xl border border-neutral-800"
              >
                <img
                  src={img.thumb_url}
                  alt={img.orig_name}
                  title={img.orig_name}
                  loading="lazy"
                  className="h-full w-full object-cover"
                />
                <span className="absolute left-1 top-1 flex h-4 w-4 items-center justify-center rounded-full bg-black/60 text-[9px] tabular-nums text-white">
                  {i + 1}
                </span>
                <button
                  type="button"
                  onClick={() => remove(img.id)}
                  title={t.common.remove}
                  aria-label={t.common.remove}
                  className="absolute right-1 top-1 flex h-4 w-4 items-center justify-center rounded-full bg-black/60 text-[9px] text-white/70 transition hover:bg-red-600 hover:text-white"
                >
                  <Icon name="xmark"  />
                </button>
              </div>
            ))}

            {pending.map((p) => (
              <PendingTile
                key={p.id}
                item={p}
                onRetry={() => startUpload(p.file, p.id)}
                onDismiss={() => setPending((prev) => prev.filter((q) => q.id !== p.id))}
              />
            ))}

            {room > 0 && (
              <>
                <button
                  type="button"
                  onClick={() => openPicker()}
                  title={t.aiSource.fromGallery}
                  className="flex h-20 w-20 flex-col items-center justify-center gap-1 rounded-xl border border-dashed border-neutral-800 text-neutral-600 transition hover:border-brand-500 hover:text-brand-300"
                >
                  <Icon name="images" className="text-sm"  />
                  <span className="text-[10px]">{t.aiSource.fromGallery}</span>
                </button>
                <button
                  type="button"
                  onClick={browse}
                  title={t.aiSource.uploadNew}
                  className="flex h-20 w-20 flex-col items-center justify-center gap-1 rounded-xl border border-dashed border-neutral-800 text-neutral-600 transition hover:border-brand-500 hover:text-brand-300"
                >
                  <Icon name="cloud-arrow-up" className="text-sm"  />
                  <span className="text-[10px]">{t.aiSource.uploadNew}</span>
                </button>
              </>
            )}
          </div>
        )}
      </div>

      {/* 失败的那几张单独说一遍:方块上只放得下一个图标,而「哪个文件、为什么」
          才是能拿去做点什么的信息。 */}
      {pending.some((p) => p.error) && (
        <div className="mt-2 space-y-1">
          {pending
            .filter((p) => p.error)
            .map((p) => (
              <div key={p.id} className="text-[10px] text-red-400">
                <Icon name="triangle-exclamation" className="mr-1.5"  />
                <span className="text-red-300">{p.file.name}</span>
                <span className="ml-1 text-red-400/80">· {p.error}</span>
              </div>
            ))}
        </div>
      )}

      {/* 最近上传的快捷条。方块比源图小一半:它是捷径不是主角,尺寸上就该分得
          出来。刚上传成功的那张也会落在这里,所以取消选中之后还找得回来。 */}
      {room > 0 && recentPickable.length > 0 && (
        <div className="mt-3">
          <div className="mb-1.5 text-[10px] uppercase tracking-wider text-neutral-600">
            {t.aiSource.recentLabel}
          </div>
          <div className="flex flex-wrap gap-1.5">
            {recentPickable.map((img) => (
              <button
                key={img.id}
                type="button"
                onClick={() => onChange([...value, img])}
                title={img.orig_name}
                className="h-11 w-11 overflow-hidden rounded-lg border border-neutral-800 transition hover:border-brand-500"
              >
                <img
                  src={img.thumb_url}
                  alt={img.orig_name}
                  loading="lazy"
                  className="h-full w-full object-cover"
                />
              </button>
            ))}
          </div>
        </div>
      )}

      {picking && (
        <ImagePicker
          initial={value}
          max={max}
          onCancel={() => setPicking(false)}
          onConfirm={(picked) => {
            // 弹层里的选中态是它打开那一刻的快照。这期间如果有上传完成,新图
            // 被加进了 value 而弹层并不知道——直接用 picked 覆盖会把它挤掉,
            // 而那张图的字节已经传上去、配额已经扣了。所以取并集:弹层里选的
            // 算数,弹层开着期间新到的也算数。
            const known = new Set(picked.map((p) => p.id));
            const arrived = valueRef.current.filter(
              (v) => !known.has(v.id) && !pickerBaseRef.current.has(v.id),
            );
            onChange([...picked, ...arrived].slice(0, max));
            setPicking(false);
          }}
        />
      )}
    </div>
  );
}

/**
 * 一张正在上传的图,占着它将来会占的那个位置。
 *
 * 进度是可见的而不是一个转圈:上传一张手机原图要好几秒,只转圈的话没人分得清
 * 「在传」和「卡住了」。失败之后方块留在原地并且可以重试,因为文件还在手上。
 */
function PendingTile({
  item,
  onRetry,
  onDismiss,
}: {
  item: Pending;
  onRetry: () => void;
  onDismiss: () => void;
}) {
  const { t } = useLang();

  if (item.error) {
    return (
      <div
        title={`${item.file.name} · ${item.error}`}
        className="relative flex h-20 w-20 flex-col items-center justify-center gap-1 rounded-xl border border-red-500/40 bg-red-950/20"
      >
        <Icon name="triangle-exclamation" className="text-sm text-red-400"  />
        <button
          type="button"
          onClick={onRetry}
          className="text-[10px] text-red-300 transition hover:text-red-200"
        >
          <Icon name="rotate-right" className="mr-1 text-[9px]"  />
          {t.aiSource.retry}
        </button>
        <button
          type="button"
          onClick={onDismiss}
          title={t.common.remove}
          aria-label={t.common.remove}
          className="absolute right-1 top-1 flex h-4 w-4 items-center justify-center rounded-full bg-black/60 text-[9px] text-white/70 transition hover:bg-red-600 hover:text-white"
        >
          <Icon name="xmark"  />
        </button>
      </div>
    );
  }

  return (
    <div
      title={item.file.name}
      className="flex h-20 w-20 flex-col items-center justify-center gap-1.5 rounded-xl border border-neutral-800 bg-neutral-950"
    >
      <RingSpinner className="h-4 w-4 text-brand-500" />
      <span className="text-[10px] tabular-nums text-neutral-500">{item.progress}%</span>
      <div className="h-0.5 w-12 overflow-hidden rounded-full bg-neutral-800">
        <div
          className="h-full bg-brand-600 transition-all"
          style={{ width: `${Math.max(4, item.progress)}%` }}
        />
      </div>
    </div>
  );
}
