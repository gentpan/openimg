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

  if (user.avatar_url) {
    return (
      <img
        src={user.avatar_url}
        alt=""
        width={size}
        height={size}
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
