import { useEffect, useState } from "react";
import type { User } from "../types";

/** Deterministic hue from the identity, so the same account always gets the
 *  same colour and two people side by side rarely collide. */
function hueFor(seed: string): number {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) % 360;
  return h;
}

function initial(user: Pick<User, "name" | "email">): string {
  const src = (user.name || user.email || "?").trim();
  // Take a whole code point: [...str][0] keeps an emoji or a CJK character
  // intact where str[0] would split a surrogate pair into a replacement box.
  return ([...src][0] || "?").toUpperCase();
}

/**
 * Profile picture, with a generated fallback.
 *
 * The fallback is a coloured initial rather than a generic silhouette: in a
 * list every silhouette looks the same, which defeats the point of showing a
 * picture at all.
 *
 * It also covers a browser that cannot decode the stored AVIF. Avatars are
 * encoded AVIF-only — no WebP twin, no <picture> element — so a viewer on
 * Safari below 16.4 would otherwise get the broken-image glyph on every page,
 * since the avatar sits in the nav bar. Falling back on the load error turns
 * that into an initial, which is what an account with no picture already
 * looks like.
 */
export default function Avatar({
  user,
  size = 24,
  className = "",
}: {
  user: Pick<User, "name" | "email" | "avatar_url">;
  size?: number;
  className?: string;
}) {
  const common = `shrink-0 rounded-full object-cover ${className}`;
  const [failed, setFailed] = useState(false);

  // A new URL deserves a fresh attempt: without this, one failure would keep
  // the initial showing even after the user uploads a different picture.
  useEffect(() => setFailed(false), [user.avatar_url]);

  if (user.avatar_url && !failed) {
    return (
      <img
        src={user.avatar_url}
        alt=""
        width={size}
        height={size}
        onError={() => setFailed(true)}
        className={`${common} block h-full w-full`}
        style={{ width: size, height: size }}
      />
    );
  }

  const hue = hueFor(user.email || user.name || "");
  return (
    <span
      aria-hidden="true"
      className={`${common} inline-flex items-center justify-center font-medium text-white`}
      style={{
        width: size,
        height: size,
        fontSize: Math.round(size * 0.42),
        // A fixed lightness keeps white text legible whatever hue comes out.
        backgroundColor: `hsl(${hue} 55% 45%)`,
      }}
    >
      {initial(user)}
    </span>
  );
}
