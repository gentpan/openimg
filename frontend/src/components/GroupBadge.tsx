import Icon from "../Icon";
import { useLang } from "../LangContext";

/**
 * The tier badge: one glance says which group an account is in.
 *
 * Three fixed groups today (admin / trusted / free), each with its own icon
 * and tint so they read apart before the label is even parsed. Anything else —
 * the admin console can mint new groups — falls back to a neutral badge with
 * the raw name, so an unknown tier degrades to "still legible" rather than to
 * a crash or a blank.
 *
 * Colors follow the roles already established in the app: amber is the admin
 * accent (the console's own header), teal marks positive status (已验证,
 * 正常), neutral is the unmarked case.
 */
const STYLES: Record<string, { icon: string; cls: string }> = {
  admin: {
    icon: "shield-halved",
    cls: "border-amber-500/30 bg-amber-500/10 text-amber-300",
  },
  trusted: {
    icon: "medal",
    cls: "border-teal-500/30 bg-teal-500/10 text-teal-300",
  },
  free: {
    icon: "user",
    cls: "border-neutral-700 bg-neutral-800/60 text-neutral-400",
  },
};

const FALLBACK = { icon: "layer-group", cls: STYLES.free.cls };

export default function GroupBadge({ name }: { name: string }) {
  const { t } = useLang();
  const s = STYLES[name] ?? FALLBACK;
  const labels: Record<string, string> = {
    admin: t.groupBadge.admin,
    trusted: t.groupBadge.trusted,
    free: t.groupBadge.free,
  };
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium ${s.cls}`}
    >
      <Icon name={s.icon} className={`text-[9px]`}  />
      {labels[name] ?? name}
    </span>
  );
}
