import { useEffect, useState } from "react";
import { authApi } from "../api";

/**
 * Third-party sign-in as a single row of round buttons.
 *
 * Passkey sits alongside Google and GitHub rather than only in a separate tab:
 * to the user it's the same decision ("how do I get in?"), and burying the
 * passwordless option makes it invisible. Clicking it switches the form's mode
 * rather than navigating, since passkey auth happens in-page.
 */
export default function OAuthButtons({ onPasskey }: { onPasskey?: () => void }) {
  const [providers, setProviders] = useState<string[]>([]);

  useEffect(() => {
    authApi
      .providers()
      .then(setProviders)
      .catch(() => setProviders([]));
  }, []);

  const hasGoogle = providers.includes("google");
  const hasGithub = providers.includes("github");
  const hasPasskey = providers.includes("passkey") && !!onPasskey;

  if (!hasGoogle && !hasGithub && !hasPasskey) return null;

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        <div className="flex-1 h-px bg-neutral-800" />
        <span className="text-[11px] text-neutral-600">或使用</span>
        <div className="flex-1 h-px bg-neutral-800" />
      </div>

      <div className="flex items-center justify-center gap-3 pb-1">
        {hasGoogle && (
          <RoundButton href="/auth/google/start" label="用 Google 账号登录">
            <GoogleIcon />
          </RoundButton>
        )}
        {hasGithub && (
          <RoundButton href="/auth/github/start" label="用 GitHub 账号登录">
            <GithubIcon />
          </RoundButton>
        )}
        {hasPasskey && (
          <RoundButton onClick={onPasskey} label="用 Passkey / 指纹登录">
            <PasskeyIcon />
          </RoundButton>
        )}
      </div>
    </div>
  );
}

function RoundButton({
  href,
  onClick,
  label,
  children,
}: {
  href?: string;
  onClick?: () => void;
  label: string;
  children: React.ReactNode;
}) {
  const cls =
    "group relative flex items-center justify-center w-12 h-12 rounded-full border border-neutral-700 " +
    "bg-neutral-800/40 hover:bg-neutral-800 hover:border-violet-500/50 transition-all duration-200 " +
    "hover:-translate-y-0.5 active:translate-y-0";

  const inner = (
    <>
      {children}
      {/* The icons alone don't say what happens on click. */}
      <span
        className="pointer-events-none absolute -bottom-6 left-1/2 -translate-x-1/2 whitespace-nowrap rounded-md
                   bg-neutral-950 border border-neutral-800 px-2 py-0.5 text-[10px] text-neutral-300
                   opacity-0 group-hover:opacity-100 transition-opacity z-10"
      >
        {label}
      </span>
    </>
  );

  if (href) {
    return (
      <a href={href} aria-label={label} title={label} className={cls}>
        {inner}
      </a>
    );
  }
  return (
    <button type="button" onClick={onClick} aria-label={label} title={label} className={cls}>
      {inner}
    </button>
  );
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 18 18" className="h-[18px] w-[18px]" aria-hidden="true">
      <path
        fill="#4285F4"
        d="M17.64 9.2c0-.64-.06-1.25-.16-1.84H9v3.49h4.84a4.14 4.14 0 0 1-1.79 2.71v2.26h2.9c1.7-1.57 2.69-3.88 2.69-6.62z"
      />
      <path
        fill="#34A853"
        d="M9 18c2.43 0 4.47-.8 5.96-2.18l-2.9-2.26c-.8.54-1.83.86-3.06.86-2.35 0-4.34-1.59-5.05-3.72H.96v2.33A9 9 0 0 0 9 18z"
      />
      <path
        fill="#FBBC05"
        d="M3.95 10.7A5.41 5.41 0 0 1 3.66 9c0-.59.1-1.17.29-1.7V4.97H.96A9 9 0 0 0 0 9c0 1.45.35 2.83.96 4.03l2.99-2.33z"
      />
      <path
        fill="#EA4335"
        d="M9 3.58c1.32 0 2.5.45 3.44 1.35l2.58-2.58A9 9 0 0 0 .96 4.97L3.95 7.3C4.66 5.17 6.65 3.58 9 3.58z"
      />
    </svg>
  );
}

function GithubIcon() {
  return (
    <svg viewBox="0 0 16 16" className="h-[18px] w-[18px] fill-neutral-200" aria-hidden="true">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

function PasskeyIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-[19px] w-[19px] fill-violet-300" aria-hidden="true">
      <path d="M12 2a5 5 0 0 0-5 5c0 1.9 1.06 3.55 2.62 4.4A7.01 7.01 0 0 0 5 18v2a1 1 0 0 0 1 1h7.35a4.5 4.5 0 0 1-.3-1.5H7v-1a5 5 0 0 1 5-5c.36 0 .71.04 1.05.11A4.5 4.5 0 0 1 16.5 11c.17 0 .34.01.5.03V7a5 5 0 0 0-5-5zm0 2a3 3 0 1 1 0 6 3 3 0 0 1 0-6z" />
      <path d="M16.5 12a3.5 3.5 0 0 0-1.5 6.66V22l1.5-1.2 1.5 1.2v-3.34A3.5 3.5 0 0 0 16.5 12zm0 2a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3z" />
    </svg>
  );
}
