import { useEffect, useRef } from "react";
import type { CardView } from "../ws/protocol";

interface Props {
  timeline: CardView[];
  mode: "view" | "place" | "challenge";
  selectedSlot?: number | null;
  onSelectSlot?: (slot: number) => void;
  takenSlots?: number[];
  disabledSlot?: number | null;
  highlightSlot?: number | null;
  highlightCorrect?: boolean;
}

export default function Timeline({
  timeline,
  mode,
  selectedSlot = null,
  onSelectSlot,
  takenSlots = [],
  disabledSlot = null,
  highlightSlot = null,
  highlightCorrect,
}: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const selectedRef = useRef<HTMLButtonElement>(null);
  const compact = timeline.length >= 8;

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

  const renderSlot = (slot: number) => {
    const isSelected = selectedSlot === slot;
    const isTaken = takenSlots.includes(slot);
    const isDisabled = disabledSlot === slot;
    const clickable = slotClickable(slot);

    if (mode === "view") {
      return <div key={`slot-${slot}`} className="w-2 shrink-0" />;
    }

    return (
      <button
        key={`slot-${slot}`}
        ref={isSelected ? selectedRef : undefined}
        type="button"
        disabled={!clickable}
        onClick={() => clickable && onSelectSlot?.(slot)}
        aria-label={`slot-${slot}`}
        className={`relative shrink-0 w-8 h-20 sm:h-24 rounded-md transition-all duration-150 ease-game ${
          isSelected
            ? "bg-accent/30 border-2 border-accent scale-105"
            : isTaken
              ? "bg-white/5 border border-white/10 cursor-not-allowed"
              : isDisabled
                ? "bg-white/[0.02] border border-transparent cursor-not-allowed"
                : clickable
                  ? "bg-white/5 border border-dashed border-white/20 hover:border-accent/60 hover:bg-accent/10"
                  : "bg-transparent border-transparent"
        }`}
      >
        {mode === "challenge" && isTaken && <span className="absolute inset-0 flex items-center justify-center text-xs">🪙</span>}
      </button>
    );
  };

  return (
    <div
      ref={containerRef}
      className="w-full overflow-x-auto pb-2"
      style={{
        maskImage: "linear-gradient(to right, transparent, black 24px, black calc(100% - 24px), transparent)",
      }}
    >
      <div className="flex items-center gap-1 px-4 min-w-min mx-auto w-fit">
        {renderSlot(0)}
        {timeline.map((card, i) => (
          <div key={card.trackId + i} className="flex items-center gap-1">
            <div
              className={`shrink-0 card-surface flex flex-col items-center justify-center transition-all duration-200 ease-game ${
                compact ? "w-14 h-20" : "w-20 h-28"
              } ${highlightSlot === i && highlightCorrect === true ? "!border-success border-2" : ""} ${
                highlightSlot === i && highlightCorrect === false ? "!border-danger border-2" : ""
              }`}
            >
              <span className={`font-semibold tabular-nums ${compact ? "text-base" : "text-xl"}`}>{card.year}</span>
              <span className={`mt-1 px-1 text-center font-medium leading-tight line-clamp-2 ${compact ? "text-[8px]" : "text-[10px]"}`}>
                {card.title}
              </span>
              {!compact && <span className="text-[10px] text-white/40 mt-0.5 px-1 text-center line-clamp-1">{card.artist}</span>}
            </div>
            {renderSlot(i + 1)}
          </div>
        ))}
      </div>
    </div>
  );
}
