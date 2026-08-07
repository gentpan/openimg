import { startRegistration } from "@simplewebauthn/browser";
import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useAuth } from "../AuthContext";
import Footer from "../Footer";
import Nav from "../components/Nav";
import { accountApi, authApi } from "../api";
import ApiTokens from "../components/ApiTokens";
import OtpConfirm from "../components/OtpConfirm";
import { useToast } from "../ToastContext";
import Avatar from "../components/Avatar";
import { RingSpinner } from "../components/Spinner";
import PasswordField from "../components/PasswordField";
import ConvertSettings from "../components/ConvertSettings";
import DeleteAccount from "../components/DeleteAccount";
import StorageProfiles from "../components/StorageProfiles";
import { useLang } from "../LangContext";
import { useDialog } from "../DialogContext";

export default function SettingsPage() {
  const dialog = useDialog();
  const { t } = useLang();
  const [pwOpen, setPwOpen] = useState(false);
  const { user, refresh } = useAuth();
  const [params, setParams] = useSearchParams();
  const [busy, setBusy] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  // pick up callback flash messages from ?linked=google or ?error=...
  useEffect(() => {
    if (params.get("linked")) {
      setInfo(t.settings.oauth.linked(params.get("linked") ?? ""));
      params.delete("linked");
      setParams(params, { replace: true });
      refresh();
    }
    if (params.get("error")) {
      setErr(decodeURIComponent(params.get("error") || ""));
      params.delete("error");
      setParams(params, { replace: true });
    }
  }, [params, setParams, refresh]);

  if (!user) {
    return (
      <div className="min-h-screen flex items-center justify-center text-neutral-500">
        {t.common.authRequired(t.common.signIn)}
      </div>
    );
  }

  async function unlink(provider: "google" | "github") {
    const ok = await dialog.confirm({
      title: t.settings.oauth.unlinkConfirm(provider),
      danger: true,
    });
    if (!ok) return;
    setBusy(provider);
    setErr(null);
    setInfo(null);
    try {
      await authApi.unlinkOAuth(provider);
      await refresh();
      setInfo(t.settings.oauth.unlinked(provider));
    } catch (e) {
      setErr(String(e instanceof Error ? e.message : e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="min-h-screen flex flex-col bg-neutral-950">
      <Nav />
      <div className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-8">
        <h1 className="mb-5 flex items-center gap-2.5 text-lg font-brand text-neutral-100">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-neutral-900 text-brand-400">
            <i className="fa-solid fa-gear text-sm" />
          </span>
          {t.settings.title}
        </h1>

        {info && <div className="mb-4 rounded-lg border border-teal-900/60 bg-teal-900/15 px-4 py-2 text-sm text-teal-300">{info}</div>}
        {err && <div className="mb-4 rounded-lg border border-red-900/60 bg-red-900/15 px-4 py-2 text-sm text-red-300">{err}</div>}

        <Section icon="fa-id-badge" title={t.settings.profile.title} subtitle={t.settings.profile.subtitle}>
          <Profile />
        </Section>

        <Section
          icon="fa-hard-drive"
          title={t.settings.storage.title}
          subtitle={t.settings.storage.subtitle}
        >
          <StorageProfiles />
        </Section>

        <Section icon="fa-wand-magic-sparkles" title={t.settings.convert.title} subtitle={t.settings.convert.subtitle}>
          <ConvertSettings />
        </Section>

        <Section
          icon="fa-key"
          title="API Token"
          subtitle={
            <>
              {t.settings.apiTokens.subtitle} ·{" "}
              {/* The question "now what?" arrives exactly here, the moment a
                  token exists and its plaintext is on screen. */}
              <Link to="/docs" className="text-brand-400 hover:underline">
                {t.settings.apiTokens.docsLink}
              </Link>
            </>
          }
        >
          <ApiTokens />
        </Section>

        {/* Account info */}
        <Section icon="fa-circle-info" title={t.settings.accountInfo.title}>
          <Row label={t.common.email} value={
            <span>
              {user.email}{" "}
              {user.email_verified && <Tag color="teal">{t.common.verified}</Tag>}
            </span>
          } />
          <Row label={t.settings.accountInfo.role} value={<Tag color="brand">{user.role}</Tag>} />
          {user.group && <Row label={t.settings.accountInfo.tier} value={user.group} />}
        </Section>

        {/* Login methods */}
        <Section icon="fa-right-to-bracket" title={t.settings.loginMethods.title} subtitle={t.settings.loginMethods.subtitle}>
          {/* Password */}
          <ConnectionRow
            icon={<KeyIcon />}
            label={t.common.emailPassword}
            connected={user.has_password}
            connectedLabel={t.settings.loginMethods.passwordSet}
            note={user.has_password ? "" : t.settings.loginMethods.passwordNotSet}
            actionLabel={user.has_password ? t.settings.loginMethods.changePasswordAction : t.common.setPassword}
            onAction={() => setPwOpen(true)}
          />
          {/* Email OTP */}
          <ConnectionRow
            icon={<MailIcon />}
            label={t.common.emailCode}
            connected={user.email_verified}
            connectedLabel={t.common.verified}
            note={t.settings.loginMethods.emailOtpNote}
            actionLabel={user.email_verified ? "" : t.settings.loginMethods.verifyEmail}
            onAction={() => void dialog.alert({ title: t.settings.loginMethods.verifyEmailComingSoon })}
          />
          {/* Google */}
          <ConnectionRow
            icon={<GoogleIcon />}
            label="Google"
            connected={user.google_connected}
            actionBusy={busy === "google"}
            actionLabel={user.google_connected ? t.settings.oauth.unlink : t.settings.oauth.link}
            danger={user.google_connected}
            onAction={() => {
              if (user.google_connected) unlink("google");
              else window.location.href = "/auth/google/link-start";
            }}
          />
          {/* GitHub */}
          <ConnectionRow
            icon={<GithubIcon />}
            label="GitHub"
            connected={user.github_connected}
            actionBusy={busy === "github"}
            actionLabel={user.github_connected ? t.settings.oauth.unlink : t.settings.oauth.link}
            danger={user.github_connected}
            onAction={() => {
              if (user.github_connected) unlink("github");
              else window.location.href = "/auth/github/link-start";
            }}
          />
        </Section>

        <PasskeySection />

        <Section icon="fa-triangle-exclamation" danger title={t.settings.danger.title} subtitle={t.settings.danger.subtitle}>
          <DeleteAccount />
        </Section>
      </div>

      {pwOpen && (
        <ChangePassword onClose={() => setPwOpen(false)} />
      )}

      <Footer />
    </div>
  );
}

/**
 * Avatar + nickname.
 *
 * The picture is uploaded on selection rather than behind a save button: there
 * is exactly one field, the result is visible immediately, and a two-step
 * commit for "pick a file" is friction with nothing to protect.
 */
function Profile() {
  const { t } = useLang();
  const { user, refresh } = useAuth();
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  async function pick(file?: File | null) {
    if (!file) return;
    setBusy(true);
    try {
      await accountApi.uploadAvatar(file);
      await refresh();
      toast.success(t.settings.profile.avatarUpdated);
    } catch (e) {
      toast.error(t.settings.profile.avatarUploadFailed, e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function clear() {
    setBusy(true);
    try {
      await accountApi.removeAvatar();
      await refresh();
      toast.success(t.settings.profile.avatarRemoved);
    } catch (e) {
      toast.error(t.settings.profile.removeFailed, e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  if (!user) return null;

  return (
    <div className="flex items-start gap-4 py-1">
      <button
        onClick={() => fileRef.current?.click()}
        disabled={busy}
        title={t.settings.profile.avatarButtonTitle}
        className="group relative h-16 w-16 shrink-0 overflow-hidden rounded-full border border-neutral-800 bg-neutral-800 transition hover:border-brand-500 disabled:opacity-60"
      >
        <Avatar user={user} size={64} />
        <span className="absolute inset-0 flex items-center justify-center bg-black/55 text-[10px] text-white opacity-0 transition group-hover:opacity-100">
          {busy ? <RingSpinner className="h-4 w-4" /> : t.settings.profile.avatarChangeOverlay}
        </span>
      </button>
      <input
        ref={fileRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={(e) => pick(e.target.files?.[0])}
      />

      <div className="min-w-0 flex-1">
        <label className="block text-[10px] text-neutral-500 mb-1">{t.common.displayName}</label>
        <NicknameField />
        <div className="mt-2 flex items-center gap-2">
          <span className="text-[10px] text-faint">
            {t.settings.profile.avatarHint}
          </span>
          {user.avatar_url && (
            <button
              onClick={clear}
              disabled={busy}
              className="inline-flex h-8 items-center justify-center rounded-lg px-3 text-xs text-neutral-500 hover:text-red-400 disabled:opacity-60 transition"
            >
              {t.common.remove}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

/**
 * Nickname editor. Saves on blur or Enter rather than behind a button — it is
 * one free-text field with no validation to fail, so a save step would be
 * ceremony. Blank is allowed and falls back to the email in the UI.
 */
function NicknameField() {
  const { t } = useLang();
  const { user, refresh } = useAuth();
  const [value, setValue] = useState(user?.name ?? "");
  const [state, setState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [err, setErr] = useState<string | null>(null);
  const toast = useToast();

  useEffect(() => setValue(user?.name ?? ""), [user?.name]);

  async function save() {
    const next = value.trim();
    if (next === (user?.name ?? "")) return;
    setState("saving");
    setErr(null);
    try {
      await accountApi.setNickname(next);
      await refresh();
      setState("saved");
      toast.success(next ? t.settings.profile.displayNameChanged(next) : t.settings.profile.displayNameCleared, t.settings.profile.displayNameToastDetail);
      setTimeout(() => setState("idle"), 1800);
    } catch (e) {
      setState("error");
      setErr(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <span className="inline-flex items-center gap-2">
      <input
        value={value}
        maxLength={32}
        placeholder={t.settings.profile.displayNamePlaceholder}
        onChange={(e) => setValue(e.target.value)}
        onBlur={save}
        onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()}
        className="w-44 h-8 rounded-lg bg-neutral-950 border border-neutral-800 px-2.5 text-xs outline-none focus:border-brand-500 placeholder-faint transition"
      />
      {state === "saving" && <RingSpinner className="h-3 w-3 text-brand-400" />}
      {state === "saved" && <i className="fa-solid fa-check text-[10px] text-teal-400" />}
      {state === "error" && <span className="text-[10px] text-red-400">{err}</span>}
      {state === "idle" && (
        <span className="text-[10px] text-faint">{t.settings.profile.displayNameHint}</span>
      )}
    </span>
  );
}

/**
 * Change password.
 *
 * One form, not two steps. The code field sits alongside the password fields
 * so it reads as part of the same act — a separate confirmation dialog after
 * the fact invites the reading that the password was already changed and this
 * is just a formality. Nothing is submitted until all three are filled.
 *
 * The current password is not asked for: proving control of the mailbox is the
 * stronger check, and accounts created by the old magic-link flow have no old
 * password to give.
 */
function ChangePassword({ onClose }: { onClose: () => void }) {
  const { t } = useLang();
  const { user, refresh } = useAuth();
  const toast = useToast();
  const [code, setCode] = useState("");
  const [pw, setPw] = useState("");
  const [pw2, setPw2] = useState("");
  const [sending, setSending] = useState(false);
  const [sentTo, setSentTo] = useState("");
  const [cooldown, setCooldown] = useState(0);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const tooShort = pw.length > 0 && pw.length < 8;
  const mismatch = pw2.length > 0 && pw !== pw2;
  const ready = code.length === 6 && pw.length >= 8 && pw === pw2;

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown((n) => n - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && !busy && onClose();
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [busy, onClose]);

  async function sendCode() {
    setSending(true);
    setErr(null);
    try {
      const r = await accountApi.requestOtp("password");
      setSentTo(r.email);
      setCooldown(r.resend_in);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSending(false);
    }
  }

  async function submit() {
    if (!ready || busy) return;
    setBusy(true);
    setErr(null);
    try {
      await accountApi.changePassword(pw, code);
      await refresh();
      onClose();
      toast.success(t.settings.password.updated, t.settings.password.updatedDetail);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setCode("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-black/50" onClick={() => !busy && onClose()} />
      <div className="relative w-full max-w-sm rounded-2xl border border-neutral-800 bg-neutral-900 p-5 shadow-panel">
        <div className="text-sm text-neutral-100 mb-1">
          {user?.has_password ? t.common.changePassword : t.common.setPassword}
        </div>
        <p className="text-[11px] leading-relaxed text-neutral-500 mb-4">
          {t.settings.password.dialogIntro}
          {sentTo && <> {t.common.otpSentTo(sentTo)}</>}
        </p>

        <label className="block text-[10px] text-neutral-500 mb-1">{t.common.emailCode}</label>
        <div className="flex gap-1.5 mb-3">
          <input
            value={code}
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            placeholder={t.common.otpPlaceholder}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
            className="flex-1 min-w-0 h-8 rounded-lg bg-neutral-950 border border-neutral-800 px-2.5 text-xs tabular-nums tracking-[0.2em] outline-none focus:border-brand-500 placeholder-faint placeholder:tracking-normal"
          />
          <button
            onClick={sendCode}
            disabled={sending || cooldown > 0 || busy}
            className="inline-flex h-8 shrink-0 items-center justify-center rounded-lg bg-neutral-800 px-3 text-xs text-neutral-300 hover:bg-neutral-700 disabled:text-neutral-500 disabled:hover:bg-neutral-800 transition"
          >
            {sending ? (
              <RingSpinner className="h-3.5 w-3.5" />
            ) : cooldown > 0 ? (
              `${cooldown}s`
            ) : sentTo ? (
              t.common.resend
            ) : (
              t.common.sendCode
            )}
          </button>
        </div>

        <label className="block text-[10px] text-neutral-500 mb-1">{t.common.newPassword}</label>
        <PasswordField value={pw} onChange={setPw} className="mb-2" />
        <PasswordField
          value={pw2}
          onChange={setPw2}
          placeholder={t.common.enterAgain}
          showStrength={false}
          showGenerate={false}
        />

        {tooShort && <div className="mt-2 text-[11px] text-red-400">{t.settings.password.tooShort}</div>}
        {mismatch && <div className="mt-2 text-[11px] text-red-400">{t.common.passwordMismatch}</div>}
        {err && <div className="mt-2 text-[11px] text-red-400">{err}</div>}

        <div className="mt-4 flex items-center gap-2">
          <div className="flex-1" />
          <button
            onClick={onClose}
            disabled={busy}
            className="inline-flex h-8 items-center justify-center rounded-lg px-3 text-xs text-neutral-400 hover:text-neutral-100 transition"
          >
            {t.common.cancel}
          </button>
          <button
            onClick={submit}
            disabled={!ready || busy}
            className="inline-flex h-8 items-center justify-center rounded-lg bg-brand-600 px-3 text-xs font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-500 transition"
          >
            {busy ? <RingSpinner className="h-3.5 w-3.5" /> : t.common.confirmChange}
          </button>
        </div>
      </div>
    </div>
  );
}

function Section({
  title,
  subtitle,
  icon,
  danger,
  children,
}: {
  title: string;
  subtitle?: React.ReactNode;
  icon?: string;
  danger?: boolean;
  children: React.ReactNode;
}) {
  return (
    <section className="mb-8 rounded-xl border border-neutral-800 bg-neutral-900/40 p-5">
      {/* items-start, not items-center: with a subtitle the block is two lines
          tall and a centred glyph drifts down past the title it belongs to. */}
      <div className="mb-4 flex items-start gap-2.5">
        {icon && (
          <span
            className={`mt-px flex h-7 w-7 shrink-0 items-center justify-center rounded-lg ${
              danger ? "bg-red-950/40 text-red-400" : "bg-neutral-800/70 text-brand-400"
            }`}
          >
            <i className={`fa-solid ${icon} text-xs`} />
          </span>
        )}
        <div className="min-w-0">
          <h2 className="text-sm font-medium text-neutral-100">{title}</h2>
          {subtitle && <p className="text-xs text-neutral-500 mt-0.5">{subtitle}</p>}
        </div>
      </div>
      <div className="space-y-3">{children}</div>
    </section>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 text-sm">
      <span className="w-20 text-neutral-500 text-xs">{label}</span>
      <span className="text-neutral-200">{value}</span>
    </div>
  );
}

function ConnectionRow({
  icon,
  label,
  connected,
  connectedLabel,
  note,
  actionLabel,
  onAction,
  actionBusy,
  danger,
}: {
  icon: React.ReactNode;
  label: string;
  connected: boolean;
  connectedLabel?: string;
  note?: string;
  actionLabel: string;
  onAction: () => void;
  actionBusy?: boolean;
  danger?: boolean;
}) {
  const { t } = useLang();
  return (
    <div className="flex items-center gap-3 py-2">
      <div className="text-neutral-300">{icon}</div>
      <div className="flex-1">
        <div className="text-sm text-neutral-200">{label}</div>
        {note && <div className="text-xs text-neutral-500">{note}</div>}
      </div>
      {actionLabel && (
        <button
          disabled={actionBusy}
          onClick={onAction}
          className={`inline-flex h-8 items-center justify-center text-xs rounded-lg px-3 transition ${
            danger
              ? "text-red-400 hover:bg-red-900/30 border border-red-900/40"
              : connected
              ? "text-neutral-400 hover:text-neutral-200"
              : "bg-brand-600 text-brand-ink hover:bg-brand-500"
          } ${actionBusy ? "opacity-60" : ""}`}
        >
          {actionBusy ? "…" : actionLabel}
        </button>
      )}
      {connected && <Tag color="teal">{connectedLabel ?? t.settings.loginMethods.connected}</Tag>}
    </div>
  );
}

function Tag({ color, children }: { color: "teal" | "brand"; children: React.ReactNode }) {
  const cls =
    color === "teal"
      ? "bg-teal-900/60 text-teal-300"
      : "bg-brand-900/60 text-brand-300";
  return <span className={`text-[10px] rounded-full px-2 py-0.5 ${cls}`}>{children}</span>;
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 18 18" className="h-5 w-5" aria-hidden>
      <path fill="#4285F4" d="M17.64 9.2c0-.64-.06-1.25-.16-1.84H9v3.49h4.84a4.14 4.14 0 0 1-1.79 2.71v2.26h2.9c1.7-1.57 2.69-3.88 2.69-6.62z" />
      <path fill="#34A853" d="M9 18c2.43 0 4.47-.8 5.96-2.18l-2.9-2.26c-.8.54-1.83.86-3.06.86-2.35 0-4.34-1.59-5.05-3.72H.96v2.33A9 9 0 0 0 9 18z" />
      <path fill="#FBBC05" d="M3.95 10.7A5.41 5.41 0 0 1 3.66 9c0-.59.1-1.17.29-1.7V4.97H.96A9 9 0 0 0 0 9c0 1.45.35 2.83.96 4.03l2.99-2.33z" />
      <path fill="#EA4335" d="M9 3.58c1.32 0 2.5.45 3.44 1.35l2.58-2.58A9 9 0 0 0 .96 4.97L3.95 7.3C4.66 5.17 6.65 3.58 9 3.58z" />
    </svg>
  );
}

function GithubIcon() {
  return (
    <svg viewBox="0 0 16 16" className="h-5 w-5 fill-current" aria-hidden>
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

function KeyIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="1.8">
      <path d="M15.5 7.5a4 4 0 1 1-3.95 4.7l-7.05 7.05v2.25h2.25l1.5-1.5v-1.5h1.5v-1.5h1.5l1.45-1.45A4 4 0 0 1 15.5 7.5z" />
      <circle cx="15.5" cy="11.5" r=".75" fill="currentColor" />
    </svg>
  );
}

function PasskeySection() {
  const dialog = useDialog();
  const { t, locale } = useLang();
  const [list, setList] = useState<{ id: string; name: string; created_at: string; last_used_at?: string }[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [enroll, setEnroll] = useState(false);
  const toast = useToast();

  async function load() {
    try {
      setList(await authApi.passkeyList());
    } catch (e) {
      setErr(String(e));
    }
  }
  useEffect(() => {
    load();
  }, []);

  // The emailed code is collected first, then the browser is asked for a
  // biometric. Doing it the other way round would fingerprint-prompt the user
  // and only afterwards tell them they also need to go read their mail.
  async function add(code: string) {
    setBusy(true);
    setErr(null);
    setInfo(null);
    try {
      const { flow, options } = await authApi.passkeyEnrollBegin(code);
      const credential = await startRegistration({ optionsJSON: options.publicKey });
      const name =
        (await dialog.prompt({
          title: t.settings.passkey.namePrompt,
          initial: defaultPasskeyName(),
        })) || "Passkey";
      await authApi.passkeyEnrollFinish(flow, name, credential);
      setInfo(null);
      toast.success(t.settings.passkey.added, t.settings.passkey.addedDetail);
      load();
    } catch (e: any) {
      if (e?.name === "NotAllowedError") {
        setErr(t.common.cancelled);
      } else {
        setErr(String(e?.message || e));
      }
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string, name: string) {
    const ok = await dialog.confirm({
      title: t.settings.passkey.deleteConfirm(name),
      danger: true,
      confirmLabel: t.common.delete,
    });
    if (!ok) return;
    try {
      await authApi.passkeyDelete(id);
      load();
    } catch (e) {
      setErr(String(e));
    }
  }

  return (
    <Section icon="fa-fingerprint" title="Passkey" subtitle={t.settings.passkey.subtitle}>
      {enroll && (
        <OtpConfirm
          purpose="passkey"
          detail={t.settings.passkey.otpDetail}
          onCancel={() => setEnroll(false)}
          onVerified={async (code) => {
            setEnroll(false);
            await add(code);
          }}
        />
      )}
      {info && <div className="text-xs text-teal-400">{info}</div>}
      {err && <div className="text-xs text-red-400">{err}</div>}
      <div className="space-y-2">
        {list.map((pk) => (
          <div key={pk.id} className="flex items-center gap-3 py-2 border-b border-neutral-900 last:border-b-0">
            <div className="text-brand-400">
              <i className="fa-solid fa-fingerprint" aria-hidden></i>
            </div>
            <div className="flex-1">
              <div className="text-sm text-neutral-200">{pk.name}</div>
              <div className="text-xs text-neutral-500">
                {t.settings.passkey.addedOn(new Date(pk.created_at).toLocaleDateString(locale))}
                {pk.last_used_at && ` · ${t.common.lastUsed(new Date(pk.last_used_at).toLocaleString(locale))}`}
              </div>
            </div>
            <button
              onClick={() => remove(pk.id, pk.name)}
              className="text-xs text-red-400 hover:text-red-300"
            >
              {t.common.delete}
            </button>
          </div>
        ))}
        {list.length === 0 && <div className="text-xs text-neutral-500 py-2">{t.settings.passkey.empty}</div>}
      </div>
      <button
        onClick={() => setEnroll(true)}
        disabled={busy}
        className="mt-3 inline-flex h-8 items-center gap-1.5 rounded-lg bg-brand-600 px-3 text-xs font-medium text-brand-ink hover:bg-brand-500 disabled:bg-neutral-800 disabled:text-neutral-500 transition"
      >
        <i className="fa-solid fa-plus" aria-hidden></i>
        {busy ? t.settings.passkey.waiting : t.common.addPasskey}
      </button>
    </Section>
  );
}

function defaultPasskeyName(): string {
  const { t } = useLang();
  const ua = navigator.userAgent;
  if (/iPhone/.test(ua)) return "iPhone";
  if (/iPad/.test(ua)) return "iPad";
  if (/Mac/.test(ua)) return "Mac";
  if (/Android/.test(ua)) return "Android";
  if (/Windows/.test(ua)) return "Windows";
  return t.settings.passkey.defaultDeviceName;
}

function MailIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="1.8">
      <rect x="3" y="5" width="18" height="14" rx="2" />
      <path d="m3 7 9 6 9-6" />
    </svg>
  );
}
