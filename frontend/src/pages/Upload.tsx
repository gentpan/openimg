import { useEffect, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import Uploader from "../components/Uploader";
import { useUpload } from "../UploadContext";
import { formatBytes, imageApi, quotaApi } from "../api";
import ImageDetail from "../components/ImageDetail";
import type { Image, QuotaInfo } from "../types";
import { useLang } from "../LangContext";

export default function UploadPage() {
  const { t } = useLang();
  const { user, loading } = useAuth();
  const [quota, setQuota] = useState<QuotaInfo | null>(null);
  const [recent, setRecent] = useState<Image[]>([]);
  const [detail, setDetail] = useState<Image | null>(null);

  // Refetch after every finished upload so the strip below stays current
  // without the user having to reload.
  const { items } = useUpload();
  const doneCount = items.filter((i) => i.state === "done").length;

  useEffect(() => {
    if (!user) return;
    quotaApi.info().then(setQuota).catch(() => {});
    imageApi
      .list({ limit: 18 })
      .then((r) => setRecent(r.images))
      .catch(() => {});
  }, [user, doneCount]);

  if (loading) return <Center>{t.common.loading}</Center>;
  if (!user) return <Navigate to="/login" replace />;

  const pct = quota && quota.quota_bytes > 0 ? (quota.used_bytes / quota.quota_bytes) * 100 : 0;

  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        {!user.email_verified && (
          <div className="mb-5 rounded-xl border border-amber-500/30 bg-amber-950/20 px-4 py-3 text-xs text-amber-200">
            <i className="fa-solid fa-triangle-exclamation mr-1.5" />
            {t.upload.emailUnverified}
            <Link to="/settings" className="ml-1.5 underline hover:text-amber-100">
              {t.upload.goVerify}
            </Link>
          </div>
        )}

        <Uploader />

        {quota && (
          <div className="mt-6 grid sm:grid-cols-2 gap-3">
            <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
              <div className="flex items-center justify-between text-xs mb-2">
                <span className="text-neutral-400">{t.common.availableStorage}</span>
                <Link to="/space" className="text-brand-400 hover:underline text-[11px]">
                  {quota.checkin.checked_in_today ? t.upload.storageDetails : t.upload.checkInForStorage}
                </Link>
              </div>
              <div className="text-xl font-brand text-neutral-100">
                {formatBytes(quota.available_bytes)}
                <span className="text-xs text-neutral-600 ml-1.5">/ {formatBytes(quota.quota_bytes)}</span>
              </div>
              <div className="mt-2.5 h-1 rounded-full bg-neutral-800 overflow-hidden">
                <div
                  className={`h-full transition-all ${pct >= 90 ? "bg-amber-500" : "bg-brand-600"}`}
                  style={{ width: `${Math.min(100, pct)}%` }}
                />
              </div>
            </div>

            <div className="rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
              <div className="text-xs text-neutral-400 mb-2">{t.upload.currentLimits(quota.tier.name)}</div>
              <dl className="space-y-1 text-[11px]">
                <Row label={t.upload.maxFileSize} value={formatBytes(quota.tier.max_file_size, 0)} />
                <Row
                  label={t.upload.uploadedToday}
                  value={t.upload.uploadedTodayValue(quota.uploads_today, quota.tier.daily_upload_count)}
                />
                <Row label={t.upload.supportedFormats} value={quota.tier.allowed_formats.join(" · ")} />
              </dl>
            </div>
          </div>
        )}

        {recent.length > 0 && (
          <div className="mt-6 rounded-2xl border border-neutral-800 bg-neutral-900/40 p-4">
            <div className="flex items-center gap-3 mb-3">
              <div>
                <div className="text-xs text-neutral-300">{t.common.recentUploads}</div>
                <div className="text-[10px] text-neutral-600 mt-0.5">{t.upload.recentHint}</div>
              </div>
              <div className="flex-1" />
              <Link to="/gallery" className="text-[11px] text-brand-400 hover:underline">
                {t.upload.viewGallery}
              </Link>
            </div>
            <div className="grid grid-cols-3 sm:grid-cols-6 lg:grid-cols-9 gap-2">
              {recent.map((img) => (
                <button
                  key={img.id}
                  onClick={() => setDetail(img)}
                  title={`${img.orig_name} · ${formatBytes(img.size_stored, 0)}`}
                  className="group relative rounded-lg overflow-hidden border border-neutral-800 hover:border-brand-500 transition"
                >
                  <img
                    src={img.thumb_url}
                    alt={img.orig_name}
                    loading="lazy"
                    className="w-full aspect-square object-cover"
                  />
                  <span className="absolute inset-x-0 bottom-0 scrim px-1 py-0.5 text-[9px] opacity-0 group-hover:opacity-100 transition truncate">
                    {formatBytes(img.size_stored, 0)}
                  </span>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>

      {detail && (
        <ImageDetail
          img={detail}
          onClose={() => setDetail(null)}
          onDeleted={() => {
            setRecent((prev) => prev.filter((i) => i.id !== detail.id));
            setDetail(null);
          }}
        />
      )}
      <Footer />
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <dt className="text-neutral-600">{label}</dt>
      <dd className="text-neutral-300">{value}</dd>
    </div>
  );
}

function Center({ children }: { children: React.ReactNode }) {
  return <div className="min-h-screen flex items-center justify-center text-neutral-500">{children}</div>;
}
