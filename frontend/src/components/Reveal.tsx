import { useEffect, useRef, useState, type ReactNode } from "react";

// Only the two that are used. The other four shipped as options nothing
// ever passed; HIDDEN is a Record<Direction, string> so the union and the
// map have to be trimmed in the same edit or tsc breaks.
type Direction = "up" | "scale";

const HIDDEN: Record<Direction, string> = {
  up: "opacity-0 translate-y-6",
  scale: "opacity-0 scale-95",
};

const SHOWN = "opacity-100 translate-y-0 translate-x-0 scale-100";

/**
 * Reveals its children when they scroll into view.
 *
 * Two things this deliberately gets right:
 *
 *   - `prefers-reduced-motion` skips the animation entirely and renders the
 *     content visible from the start. Motion-triggered nausea is a real
 *     accessibility problem, and a decorative fade is never worth it.
 *   - Without IntersectionObserver the content shows immediately rather than
 *     staying invisible. A failed animation must never eat the page.
 *
 * `once` defaults to true: re-animating on every scroll-by is distracting on
 * a page people scroll up and down.
 */
/// How much of the element has to be on screen before it fades in, and the
/// stagger ceiling for a group.
///
/// Both were props with defaults, and in ten call sites nobody ever set either
/// one. `once` was a prop too, always true, which left `else if (!once)` as a
/// branch that could not run. Constants say the same thing without offering a
/// knob that has never been turned — git still has the parameterised version if
/// one is ever needed.
const THRESHOLD = 0.15;
const MAX_STAGGER = 400;

export default function Reveal({
  children,
  direction = "up",
  delay = 0,
  duration = 600,
  className = "",
}: {
  children: ReactNode;
  direction?: Direction;
  /** Stagger, in ms. */
  delay?: number;
  duration?: number;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  const [instant, setInstant] = useState(false);

  useEffect(() => {
    const reduced = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    if (reduced || typeof IntersectionObserver === "undefined") {
      setInstant(true);
      setVisible(true);
      return;
    }

    const el = ref.current;
    if (!el) return;

    const io = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setVisible(true);
          io.disconnect();
        }
      },
      { threshold: THRESHOLD, rootMargin: "0px 0px -40px 0px" },
    );
    io.observe(el);

    // Failsafe. The observer normally reports on observe() for anything
    // already on screen, but "normally" is doing real work there: a tab opened
    // in the background, or a window that has not painted, can leave the
    // callback pending indefinitely — and the content stays at opacity 0 until
    // the visitor scrolls, on a page whose first screen is the entire pitch.
    //
    // A decorative fade is never worth a blank page, so if nothing has fired
    // by the time the animation would have finished anyway, just show it.
    const failsafe = window.setTimeout(() => {
      setVisible(true);
      io.disconnect();
    }, 800);

    return () => {
      clearTimeout(failsafe);
      io.disconnect();
    };
  }, []);

  return (
    <div
      ref={ref}
      className={`${instant ? "" : "transition-all ease-out will-change-transform"} ${
        visible ? SHOWN : HIDDEN[direction]
      } ${className}`}
      style={
        instant
          ? undefined
          : { transitionDuration: `${duration}ms`, transitionDelay: visible ? `${delay}ms` : "0ms" }
      }
    >
      {children}
    </div>
  );
}

/**
 * Convenience wrapper for a list that should cascade in. Each child gets an
 * incremental delay, capped so a long list doesn't leave the last item
 * hanging for two seconds.
 */
export function RevealGroup({
  children,
  step = 80,
  direction = "up",
  className = "",
}: {
  children: ReactNode[];
  step?: number;
  direction?: Direction;
  className?: string;
}) {
  return (
    <>
      {children.map((child, i) => (
        <Reveal key={i} direction={direction} delay={Math.min(i * step, MAX_STAGGER)} className={className}>
          {child}
        </Reveal>
      ))}
    </>
  );
}
