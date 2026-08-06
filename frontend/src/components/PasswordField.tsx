import { useMemo, useState } from "react";
import { useLang } from "../LangContext";

/**
 * Generates a password from crypto/getRandomValues, never Math.random.
 *
 * Rejection sampling rather than modulo: 256 is not a multiple of the alphabet
 * length, so `byte % len` would make the first few symbols meaningfully more
 * likely and shrink the real keyspace below what the length suggests.
 */
const ALPHABET = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*-_=+";

function generatePassword(length = 20): string {
  const max = 256 - (256 % ALPHABET.length);
  const out: string[] = [];
  const buf = new Uint8Array(length * 2);
  while (out.length < length) {
    crypto.getRandomValues(buf);
    for (const b of buf) {
      if (b >= max) continue;
      out.push(ALPHABET[b % ALPHABET.length]);
      if (out.length === length) break;
    }
  }
  return out.join("");
}

/** Rough strength, for the bar under the field.
 *
 * Returns a level only. The label is looked up by the component, because this
 * runs outside React and has no access to the dictionary. */
function score(pw: string): 0 | 1 | 2 | 3 | 4 {
  if (!pw) return 0;
  let n = 0;
  if (pw.length >= 8) n++;
  if (pw.length >= 12) n++;
  if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) n++;
  if (/\d/.test(pw)) n++;
  if (/[^a-zA-Z0-9]/.test(pw)) n++;
  // A long passphrase beats a short jumble, so length alone can reach "strong".
  if (pw.length >= 20) n = Math.max(n, 4);
  return Math.min(4, n) as 0 | 1 | 2 | 3 | 4;
}

const BAR = ["bg-neutral-700", "bg-red-500", "bg-amber-500", "bg-brand-500", "bg-teal-500"];

/**
 * Password input with a visibility toggle, a generator, and a strength bar.
 *
 * The generator writes into the field rather than into a popup: a password the
 * user has to copy out of a dialog is one they will paste somewhere insecure,
 * and revealing it in place means they can actually save it to their manager.
 */
export default function PasswordField({
  value,
  onChange,
  placeholder,
  autoComplete = "new-password",
  showStrength = true,
  showGenerate = true,
  className = "",
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  autoComplete?: string;
  showStrength?: boolean;
  showGenerate?: boolean;
  className?: string;
}) {
  const { t } = useLang();
  const [visible, setVisible] = useState(false);
  const [copied, setCopied] = useState(false);
  const level = useMemo(() => score(value), [value]);
  const strengthLabel = [
    "",
    t.passwordField.strength.veryWeak,
    t.passwordField.strength.weak,
    t.passwordField.strength.fair,
    t.passwordField.strength.strong,
  ][level];

  function generate() {
    const pw = generatePassword(20);
    onChange(pw);
    // Shown automatically: a generated password the user cannot see is one
    // they cannot record, and they are about to need it again.
    setVisible(true);
    navigator.clipboard?.writeText(pw).then(
      () => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      },
      () => {},
    );
  }

  return (
    <div className={className}>
      <div className="relative">
        <input
          type={visible ? "text" : "password"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder ?? t.passwordField.placeholder}
          autoComplete={autoComplete}
          className="w-full rounded-lg border border-neutral-800 bg-neutral-950 px-3 py-2.5 pr-[4.5rem] text-sm outline-none transition placeholder-faint focus:border-brand-500"
        />
        <div className="absolute inset-y-0 right-1.5 flex items-center gap-0.5">
          {showGenerate && (
            <button
              type="button"
              onClick={generate}
              title={t.passwordField.generateTitle}
              aria-label={t.passwordField.generateAria}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-neutral-500 transition hover:bg-neutral-800 hover:text-brand-300"
            >
              <i className="fa-solid fa-dice text-xs" />
            </button>
          )}
          <button
            type="button"
            onClick={() => setVisible((v) => !v)}
            title={visible ? t.passwordField.hide : t.passwordField.show}
            aria-label={visible ? t.passwordField.hide : t.passwordField.show}
            className="inline-flex h-7 w-7 items-center justify-center rounded-md text-neutral-500 transition hover:bg-neutral-800 hover:text-brand-300"
          >
            <i className={`fa-solid ${visible ? "fa-eye-slash" : "fa-eye"} text-xs`} />
          </button>
        </div>
      </div>

      {showStrength && value && (
        <div className="mt-1.5 flex items-center gap-2">
          <div className="flex flex-1 gap-1">
            {[1, 2, 3, 4].map((i) => (
              <span
                key={i}
                className={`h-1 flex-1 rounded-full transition-colors ${
                  i <= level ? BAR[level] : "bg-neutral-800"
                }`}
              />
            ))}
          </div>
          <span className="shrink-0 text-[10px] text-neutral-500">{strengthLabel}</span>
        </div>
      )}

      {copied && (
        <div className="mt-1.5 text-[10px] text-teal-400">
          <i className="fa-solid fa-check mr-1" />
          {t.passwordField.generatedCopied}
        </div>
      )}
    </div>
  );
}
