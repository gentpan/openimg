import { useEffect, useRef, useState, type ReactNode } from "react";

type Direction = "up" | "down" | "left" | "right" | "scale" | "fade";

const HIDDEN: Record<Direction, string> = {
  up: "opacity-0 translate-y-6",
  down: "opacity-0 -translate-y-6",
  left: "opacity-0 translate-x-6",
  right: "opacity-0 -translate-x-6",
  scale: "opacity-0 scale-95",
  fade: "opacity-0",
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
export default function Reveal({
  children,
  direction = "up",
  delay = 0,
  duration = 600,
  threshold = 0.15,
  once = true,
  className = "",
}: {
  children: ReactNode;
  direction?: Direction;
  /** Stagger, in ms. */
  delay?: number;
  duration?: number;
  threshold?: number;
  once?: boolean;
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
          if (once) io.disconnect();
        } else if (!once) {
          setVisible(false);
        }
      },
      { threshold, rootMargin: "0px 0px -40px 0px" },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [threshold, once]);

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
  maxDelay = 400,
  direction = "up",
  className = "",
}: {
  children: ReactNode[];
  step?: number;
  maxDelay?: number;
  direction?: Direction;
  className?: string;
}) {
  return (
    <>
      {children.map((child, i) => (
        <Reveal key={i} direction={direction} delay={Math.min(i * step, maxDelay)} className={className}>
          {child}
        </Reveal>
      ))}
    </>
  );
}
