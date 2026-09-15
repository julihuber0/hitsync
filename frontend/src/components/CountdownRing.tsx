import { useEffect, useRef } from "react";

interface Props {
  /** Absolute deadline in server time, ms. Null renders an idle ring. */
  deadlineMs: number | null;
  totalMs: number;
  serverNow: () => number;
  size?: number;
}

/**
 * A countdown ring driven entirely by requestAnimationFrame writing to a CSS
 * custom property — never React state — so a 60Hz tick never triggers a
 * re-render (§14.4).
 */
export default function CountdownRing({ deadlineMs, totalMs, serverNow, size = 56 }: Props) {
  const ref = useRef<SVGCircleElement>(null);
  const labelRef = useRef<HTMLSpanElement>(null);
  const radius = (size - 8) / 2;
  const circumference = 2 * Math.PI * radius;

  useEffect(() => {
    if (deadlineMs === null) {
      ref.current?.style.setProperty("--progress", "1");
      return;
    }
    let raf = 0;
    const tick = () => {
      const remainingMs = Math.max(0, deadlineMs - serverNow());
      const fraction = totalMs > 0 ? Math.min(1, remainingMs / totalMs) : 0;
      if (ref.current) {
        ref.current.style.strokeDashoffset = String(circumference * (1 - fraction));
      }
      if (labelRef.current) {
        labelRef.current.textContent = String(Math.ceil(remainingMs / 1000));
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [deadlineMs, totalMs, serverNow, circumference]);

  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={size / 2} cy={size / 2} r={radius} stroke="rgba(255,255,255,0.1)" strokeWidth={4} fill="none" />
        <circle
          ref={ref}
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke="#e8734a"
          strokeWidth={4}
          fill="none"
          strokeDasharray={circumference}
          strokeDashoffset={0}
          strokeLinecap="round"
          style={{ transition: deadlineMs === null ? "none" : "stroke-dashoffset 0.1s linear" }}
        />
      </svg>
      <span ref={labelRef} className="absolute text-xs font-semibold tabular-nums" />
    </div>
  );
}
