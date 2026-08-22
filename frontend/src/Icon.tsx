import { ICONS } from "./icons";

// Inline SVG replacement for the Font Awesome webfont. Renders like the old
// <i className="fa-solid fa-..."> did — inline-block, sized by the
// surrounding font-size, colored by currentColor — so the Tailwind size and
// color utilities around former <i> tags keep working unchanged.
//
// Unknown names render nothing rather than a broken glyph: a missing icon is
// visible in the UI the moment someone looks at the page, while a console
// warning makes it findable.
export default function Icon({ name, className }: { name: string; className?: string }) {
  const ic = ICONS[name];
  if (import.meta.env.DEV && !ic) {
    console.warn(`Icon: unknown name "${name}"`);
  }
  if (!ic) return null;
  return (
    <svg
      viewBox={`0 0 ${ic.vb}`}
      width="1em"
      height="1em"
      fill="currentColor"
      aria-hidden="true"
      className={`inline-block align-[-0.125em]${className ? " " + className : ""}`}
    >
      <path d={ic.d} />
    </svg>
  );
}
