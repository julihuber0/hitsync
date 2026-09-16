import type { CSSProperties, InputHTMLAttributes } from "react";
import { AudioLines } from "lucide-react";
import { useTranslation } from "react-i18next";

/** App logo: gradient mark plus wordmark. */
export function Logo({ size = "md" }: { size?: "md" | "lg" }) {
  const { t } = useTranslation();
  const large = size === "lg";
  return (
    <div className="flex items-center gap-2.5">
      <span className={`brand-mark ${large ? "h-11 w-11 rounded-2xl" : "h-8 w-8"}`}>
        <AudioLines size={large ? 24 : 18} strokeWidth={2.25} />
      </span>
      <span className={`font-semibold tracking-tight ${large ? "text-2xl" : "text-lg"}`}>{t("app.name")}</span>
    </div>
  );
}

/** Soft, slowly drifting colour fields behind landing-style screens. */
export function Aurora() {
  return (
    <div aria-hidden className="pointer-events-none fixed inset-0 -z-10 overflow-hidden opacity-50 sm:opacity-100">
      <div className="absolute -left-40 -top-40 h-[520px] w-[520px] animate-aurora rounded-full bg-accent/20 blur-[120px]" />
      <div
        className="absolute -right-32 top-1/4 h-[460px] w-[460px] animate-aurora rounded-full bg-accent-end/20 blur-[120px]"
        style={{ animationDelay: "-6s" }}
      />
      <div
        className="absolute -bottom-48 left-1/3 h-[420px] w-[420px] animate-aurora rounded-full bg-accent2/10 blur-[120px]"
        style={{ animationDelay: "-12s" }}
      />
    </div>
  );
}

type SliderProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & { min: number; max: number; value: number };

/** A range input whose track fills with the brand gradient up to the thumb. */
export function Slider({ className = "", style, ...props }: SliderProps) {
  const span = props.max - props.min;
  const fill = span > 0 ? ((props.value - props.min) / span) * 100 : 0;
  return (
    <input
      type="range"
      {...props}
      className={`slider ${className}`}
      style={{ ...style, "--fill": `${fill}%` } as CSSProperties}
    />
  );
}

/** Player colour avatar with the name's initial. */
export function Avatar({ name, colour, size = 32, dimmed = false }: { name: string; colour: string; size?: number; dimmed?: boolean }) {
  return (
    <span
      aria-hidden
      className="flex shrink-0 items-center justify-center rounded-full font-semibold text-white/95 transition-opacity"
      style={{
        width: size,
        height: size,
        fontSize: size * 0.42,
        background: `linear-gradient(135deg, ${colour}, color-mix(in srgb, ${colour} 55%, #000))`,
        boxShadow: `inset 0 1px 0 rgba(255,255,255,0.25), 0 4px 14px -4px ${colour}`,
        opacity: dimmed ? 0.4 : 1,
      }}
    >
      {name.trim().charAt(0).toUpperCase()}
    </span>
  );
}
