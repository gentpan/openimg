interface LogoProps {
  size?: number;
  className?: string;
  /**
   * Whether the mark is its own home link. Off when the caller already wraps
   * it in one — an <a> inside an <a> is invalid HTML, and React 19 says so
   * loudly in the console.
   */
  asLink?: boolean;
}

/**
 * Brand mark. Swaps to a house on hover because it doubles as the home link —
 * a logo that navigates without saying so is a small trap, and the swap says
 * it without spending a label.
 */
export default function Logo({ size = 28, className = "", asLink = true }: LogoProps) {
  const mark = (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 32 32"
      width={size}
      height={size}
      className={className}
      aria-label="Openimg"
    >
      <path fill={ACCENT} d={MARK_PATH} />
    </svg>
  );

  const house = (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 32 32"
      width={size}
      height={size}
      aria-label="返回首页"
    >
      <g transform="translate(4 4) scale(2)" fill={ACCENT}>
        <path d="M5.735 1.92a.375.375 0 0 1 .53 0l4.345 4.345a.375.375 0 1 0 .53-.53L6.795 1.39a1.125 1.125 0 0 0-1.59 0L.86 5.735a.375.375 0 1 0 .53.53l4.345-4.345Z" />
        <path d="M6 2.715 1.92 6.795c-.015.015-.03.03-.045.045v3.099c0 .518.42.937.938.937h1.687a.375.375 0 0 0 .375-.375V8.25a.375.375 0 0 1 .375-.375h1.5a.375.375 0 0 1 .375.375v2.25c0 .207.168.375.375.375h1.687a.937.937 0 0 0 .938-.937V6.84a1.3 1.3 0 0 1-.045-.045L6 2.715Z" />
      </g>
    </svg>
  );

  // The hover swap needs a `group` ancestor; when we aren't the link, a plain
  // span provides it so the effect survives either way.
  const inner = (
    <>
      <span className="absolute inset-0 transition-opacity duration-200 group-hover:opacity-0">
        {mark}
      </span>
      <span className="absolute inset-0 opacity-0 transition-opacity duration-200 group-hover:opacity-100">
        {house}
      </span>
      <span className="invisible block">{mark}</span>
    </>
  );

  if (!asLink) {
    return (
      <span
        className="group relative inline-block transition-transform duration-200 hover:scale-110"
        style={{ width: size, height: size, lineHeight: 0 }}
      >
        {inner}
      </span>
    );
  }

  // A plain <a>, not react-router's <Link>: clicking the mark should tear the
  // app down and build it again, which a client-side route change explicitly
  // does not do. Navigating to the URL you are already on still reloads, so
  // this works as a reset button from the home page too.
  return (
    <a
      href="/"
      title="返回首页"
      className="group relative inline-block transition-transform duration-200 hover:scale-110"
      style={{ width: size, height: size, lineHeight: 0 }}
    >
      {inner}
    </a>
  );
}


const ACCENT = "#8E47FF";

// Kept in sync with public/favicon.svg by hand — that file is the source of
// the generated PNG/ICO set, this constant is what React renders.
const MARK_PATH =
  "M 16.03125 3.9628906 C 12.681977 3.9628906 9.3335639 4.2968064 5.9882812 4.9648438 C 4.4895682 5.2648566 3.3346311 6.4177966 2.9414062 7.8574219 A 1.0001 1.0001 0 0 0 2.9414062 7.859375 C 1.4620652 13.29634 1.4620652 18.768113 2.9414062 24.205078 C 3.3336046 25.64578 4.4884663 26.800728 5.9882812 27.099609 C 12.678846 28.435684 19.385607 28.435684 26.076172 27.099609 C 27.57631 26.800933 28.731081 25.646245 29.123047 24.205078 C 30.602388 18.768113 30.602388 13.29634 29.123047 7.859375 A 1.0001 1.0001 0 0 0 29.123047 7.8574219 C 28.730848 6.41672 27.575987 5.2637248 26.076172 4.9648438 C 22.730889 4.2968064 19.380523 3.9628906 16.03125 3.9628906 z M 16.03125 5.9628906 C 19.247477 5.9628906 22.463876 6.2828186 25.683594 6.9257812 C 26.401779 7.0689003 26.991558 7.6415143 27.193359 8.3828125 C 27.984136 11.289123 28.309721 14.183874 28.201172 17.080078 C 26.494775 17.077209 24.743174 17.360826 23.148438 18.132812 C 21.366297 18.995973 19.990317 19.807101 18.402344 19.417969 C 16.797152 19.024531 15.99883 18.420332 14.71875 17.060547 C 13.062344 15.301587 11.610388 14.26942 9.7207031 14.074219 A 1.0001 1.0001 0 0 0 9.71875 14.074219 C 8.1503866 13.913301 6.54228 14.279865 5.2226562 15.193359 C 4.722014 15.539704 4.2676276 15.937341 3.8398438 16.355469 C 3.8091355 13.701272 4.1465228 11.048401 4.8710938 8.3847656 L 4.8710938 8.3828125 C 5.0741837 7.6418544 5.6609541 7.0702275 6.3789062 6.9257812 C 9.5986237 6.2828186 12.815023 5.9628906 16.03125 5.9628906 z M 20 9 C 18.35503 9 17 10.35503 17 12 C 17 13.64497 18.35503 15 20 15 C 21.64497 15 23 13.64497 23 12 C 23 10.35503 21.64497 9 20 9 z M 20 11 C 20.56503 11 21 11.43497 21 12 C 21 12.56503 20.56503 13 20 13 C 19.43497 13 19 12.56503 19 12 C 19 11.43497 19.43497 11 20 11 z M 8.6738281 16.048828 C 8.9519186 16.030137 9.2334226 16.035811 9.5136719 16.064453 C 10.837987 16.201252 11.722078 16.794601 13.263672 18.431641 C 14.649592 19.903856 15.956973 20.878766 17.925781 21.361328 C 20.377808 21.962196 22.353672 20.738481 24.019531 19.931641 C 25.233862 19.343802 26.64118 19.093745 28.056641 19.078125 C 27.889935 20.610025 27.611419 22.143207 27.193359 23.679688 C 26.991325 24.422521 26.405409 24.993395 25.685547 25.136719 A 1.0001 1.0001 0 0 0 25.683594 25.136719 C 19.244159 26.422644 12.818341 26.422644 6.3789062 25.136719 C 5.6607212 24.9936 5.0728954 24.420986 4.8710938 23.679688 C 4.4613347 22.173716 4.1877559 20.671447 4.0195312 19.169922 C 4.7111657 18.271246 5.4960844 17.436466 6.3613281 16.837891 C 7.0332171 16.372783 7.8395566 16.104901 8.6738281 16.048828 z";
