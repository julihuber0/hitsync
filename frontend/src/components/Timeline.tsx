import { useEffect, useRef } from "react";
import { Coins, Plus } from "lucide-react";
import type { CardView } from "../ws/protocol";

interface Props {
  timeline: CardView[];
  mode: "view" | "place" | "challenge";
  size?: "default" | "large";
  selectedSlot?: number | null;
  onSelectSlot?: (slot: number) => void;
  takenSlots?: number[];
  disabledSlot?: number | null;
  placementSlot?: number | null;
  previewSlot?: number | null;
  highlightSlot?: number | null;
  highlightCorrect?: boolean;
  previews?: Array<{ playerId: string; name: string; colour: string; slot: number }>;
}

export default function Timeline({
  timeline,
  mode,
  size = "default",
  selectedSlot = null,
  onSelectSlot,
  takenSlots = [],
  disabledSlot = null,
  placementSlot = null,
  previewSlot = null,
  highlightSlot = null,
  highlightCorrect,
  previews = [],
}: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const selectedRef = useRef<HTMLButtonElement>(null);
  const compact = size === "default" && timeline.length >= 8;

  useEffect(() => {
    selectedRef.current?.scrollIntoView({ behavior: "smooth", inline: "center", block: "nearest" });
  }, [selectedSlot]);

  useEffect(() => {
    if (mode !== "place" || !onSelectSlot) return;
    const handler = (e: KeyboardEvent) => {
      const n = timeline.length;
      const current = selectedSlot ?? 0;
      if (e.key === "ArrowLeft") onSelectSlot(Math.max(0, current - 1));
      else if (e.key === "ArrowRight") onSelectSlot(Math.min(n, current + 1));
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [mode, onSelectSlot, selectedSlot, timeline.length]);

  const slotClickable = (slot: number): boolean => {
    if (mode === "view") return false;
    if (disabledSlot === slot) return false;
    if (mode === "challenge" && takenSlots.includes(slot)) return false;
    return true;
  };

  const previewDots = (slotPreviews: Props["previews"] & object) =>
    slotPreviews.length > 0 && (
      <span className="absolute -top-2 left-1/2 z-10 flex -translate-x-1/2 -space-x-1" aria-label={slotPreviews.map((p) => p.name).join(", ")}>
        {slotPreviews.map((preview) => (
          <span
            key={preview.playerId}
            className="h-3 w-3 rounded-full ring-2 ring-bg"
            style={{ backgroundColor: preview.colour, boxShadow: `0 0 8px ${preview.colour}` }}
            title={preview.name}
          />
        ))}
      </span>
    );

  const markers = (isPlacement: boolean, isPreview: boolean) => (
    <>
      {isPlacement && (
        <span
          className="absolute inset-y-2 left-1/2 w-1 -translate-x-1/2 rounded-full bg-gradient-to-b from-accent to-accent-end shadow-glow-sm"
          aria-label="submitted placement"
        />
      )}
      {isPreview && (
        <span
          className="absolute inset-y-2 left-1/2 w-1 -translate-x-1/2 animate-glow-pulse rounded-full bg-accent/70"
          aria-label="selecting placement"
        />
      )}
    </>
  );

  const cardHeight = compact ? "h-[88px]" : size === "large" ? "h-36 sm:h-40" : "h-28";

  const renderSlot = (slot: number) => {
    const isSelected = selectedSlot === slot;
    const isTaken = takenSlots.includes(slot);
    const isDisabled = disabledSlot === slot;
    const isPlacement = placementSlot === slot;
    const isPreview = previewSlot === slot && !isPlacement;
    const clickable = slotClickable(slot);
    const slotPreviews = previews.filter((preview) => preview.slot === slot);

    if (mode === "view") {
      return (
        <div key={`slot-${slot}`} className={`relative w-3 shrink-0 ${cardHeight}`}>
          {previewDots(slotPreviews)}
          {markers(isPlacement, isPreview)}
        </div>
      );
    }

    return (
      <button
        key={`slot-${slot}`}
        ref={isSelected ? selectedRef : undefined}
        type="button"
        disabled={!clickable}
        onClick={() => clickable && onSelectSlot?.(slot)}
        aria-label={`slot-${slot}`}
        className={`group/slot relative flex w-9 shrink-0 items-center justify-center rounded-xl transition-all duration-200 ease-game ${cardHeight} ${
          isSelected
            ? "w-12 bg-gradient-to-b from-accent/40 to-accent-end/30 shadow-glow ring-2 ring-accent"
            : isTaken
              ? "cursor-not-allowed border border-white/10 bg-white/[0.04]"
              : isDisabled
                ? "cursor-not-allowed border border-transparent bg-white/[0.015]"
                : clickable
                  ? "border border-dashed border-white/15 bg-white/[0.02] hover:w-11 hover:border-accent/60 hover:bg-accent/10"
                  : "border-transparent bg-transparent"
        }`}
      >
        {clickable && !isSelected && !isTaken && (
          <Plus size={14} className="text-fg/25 transition-colors group-hover/slot:text-accent" aria-hidden />
        )}
        {isSelected && <Plus size={16} className="text-white" aria-hidden />}
        {mode === "challenge" && isTaken && <Coins size={14} className="text-gold" aria-hidden />}
        {previewDots(slotPreviews)}
        {markers(isPlacement, isPreview)}
      </button>
    );
  };

  return (
    <div ref={containerRef} className="fade-edges-x w-full overflow-x-auto pb-3 pt-3">
      <div className="mx-auto flex w-fit min-w-min items-center gap-1.5 px-5">
        {renderSlot(0)}
        {timeline.map((card, i) => (
          <div key={card.trackId + i} className="flex items-center gap-1.5">
            <div
              className={`relative flex shrink-0 flex-col items-center justify-center overflow-hidden rounded-2xl border bg-gradient-to-b from-elevated to-surface px-2 text-center shadow-card transition-all duration-200 ease-game ${
                compact ? "w-[68px]" : size === "large" ? "w-28 sm:w-32" : "w-24"
              } ${cardHeight} ${
                highlightSlot === i && highlightCorrect === true
                  ? "border-success/70 shadow-[0_0_24px_-4px_rgba(52,211,153,0.5)]"
                  : highlightSlot === i && highlightCorrect === false
                    ? "border-danger/70 shadow-[0_0_24px_-4px_rgba(251,113,133,0.5)]"
                    : "border-white/[0.08]"
              }`}
            >
              <span className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/25 to-transparent" />
              <span
                className={`brand-text font-bold tabular-nums tracking-tight ${
                  compact ? "text-lg" : size === "large" ? "text-3xl" : "text-2xl"
                }`}
              >
                {card.year}
              </span>
              <span
                className={`mt-1.5 line-clamp-2 flex min-h-[2.5em] items-center font-medium leading-tight text-fg/90 ${
                  compact ? "text-[9px]" : size === "large" ? "text-xs" : "text-[11px]"
                }`}
              >
                {card.title}
              </span>
              {!compact && (
                <span className={`mt-0.5 line-clamp-1 text-fg/45 ${size === "large" ? "text-[11px]" : "text-[10px]"}`}>{card.artist}</span>
              )}
            </div>
            {renderSlot(i + 1)}
          </div>
        ))}
      </div>
    </div>
  );
}
