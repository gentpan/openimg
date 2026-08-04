/**
 * Rotating ring spinner — the default for "this is working" states.
 *
 * The source artwork hard-codes hsl(228,97%,42%); swapped to `fill-current` so
 * it takes the colour of whatever button or panel it sits in instead of
 * dropping a stray blue into a violet UI.
 */
export function RingSpinner({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={`fill-current ${className}`} aria-hidden="true">
      <path d="M10.72,19.9a8,8,0,0,1-6.5-9.79A7.77,7.77,0,0,1,10.4,4.16a8,8,0,0,1,9.49,6.52A1.54,1.54,0,0,0,21.38,12h.13a1.37,1.37,0,0,0,1.38-1.54,11,11,0,1,0-12.7,12.39A1.54,1.54,0,0,0,12,21.34h0A1.47,1.47,0,0,0,10.72,19.9Z">
        <animateTransform
          attributeName="transform"
          type="rotate"
          dur="0.75s"
          values="0 12 12;360 12 12"
          repeatCount="indefinite"
        />
      </path>
    </svg>
  );
}

/**
 * Three-dot pulse spinner. Inherits the current text colour via `fill-current`
 * rather than the hard-coded blue in the source artwork, so it matches whatever
 * button it sits inside.
 */
export default function Spinner({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={`fill-current ${className}`} aria-hidden="true">
      <circle cx="4" cy="12" r="3" opacity="1">
        <animate
          id="sp1"
          begin="0;sp3.end-0.25s"
          attributeName="opacity"
          dur="0.75s"
          values="1;.2"
          fill="freeze"
        />
      </circle>
      <circle cx="12" cy="12" r="3" opacity=".4">
        <animate begin="sp1.begin+0.15s" attributeName="opacity" dur="0.75s" values="1;.2" fill="freeze" />
      </circle>
      <circle cx="20" cy="12" r="3" opacity=".3">
        <animate
          id="sp3"
          begin="sp1.begin+0.3s"
          attributeName="opacity"
          dur="0.75s"
          values="1;.2"
          fill="freeze"
        />
      </circle>
    </svg>
  );
}
