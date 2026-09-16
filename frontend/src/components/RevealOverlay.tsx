import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "react-i18next";
import { Check, X } from "lucide-react";
import type { RevealPayload, PlayerView } from "../ws/protocol";

function sourceLabel(t: (k: string) => string, source: string): string {
  switch (source) {
    case "Navidrome":
      return t("reveal.sourceNavidrome");
    case "hitsyncyear":
      return t("reveal.sourceHitsyncYear");
    default:
      return source;
  }
}

export default function RevealOverlay({ reveal, players }: { reveal: RevealPayload; players: PlayerView[] }) {
  const { t } = useTranslation();
  const winner = players.find((p) => p.id === reveal.winnerPlayerId);

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.2, ease: [0.2, 0.8, 0.2, 1] }}
        className="fixed inset-0 z-40 bg-black/70 flex items-center justify-center p-4"
      >
        <motion.div
          initial={{ rotateY: 90, opacity: 0 }}
          animate={{ rotateY: 0, opacity: 1 }}
          transition={{ duration: 0.25, ease: [0.2, 0.8, 0.2, 1] }}
          style={{ transformStyle: "preserve-3d" }}
          className="card-surface shadow-neon w-full max-w-xs p-8 flex flex-col items-center gap-3 text-center"
        >
          <div className="neon-heading text-5xl font-semibold tabular-nums tracking-tight text-accent">{reveal.card.year}</div>
          <div className="text-lg font-medium">{reveal.card.title}</div>
          <div className="text-sm text-neon/60">{reveal.card.artist}</div>
          <div className="text-[10px] uppercase tracking-wide text-neon/30">{sourceLabel(t, reveal.yearSource)}</div>

          <div className="mt-4 text-sm">
            {reveal.outcome === "active_correct" && <span className="text-success font-semibold">{t("reveal.correct")}</span>}
            {reveal.outcome === "challenger_correct" && winner && (
              <span className="text-success font-semibold">{t("reveal.stolen", { name: winner.name })}</span>
            )}
            {reveal.outcome === "discarded" && <span className="text-neon/50">{t("reveal.discarded")}</span>}
          </div>

          {reveal.songGuessResult && (
            <SongGuessResult
              result={reveal.songGuessResult}
              playerName={players.find((p) => p.id === reveal.activePlayerId)?.name ?? ""}
            />
          )}

          <div className="text-xs text-neon/30 mt-2">{t("reveal.continuing")}</div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}

function SongGuessResult({ result, playerName }: { result: NonNullable<RevealPayload["songGuessResult"]>; playerName: string }) {
  const { t } = useTranslation();
  const outcome = result.awarded
    ? { text: t("reveal.songGuessCorrect"), className: "text-accent2" }
    : result.correct
      ? { text: t("reveal.songGuessAtMax"), className: "text-neon/60" }
      : { text: t("reveal.songGuessWrong"), className: "text-neon/50" };

  return (
    <div className="mt-2 w-full border-t border-white/10 pt-3 flex flex-col gap-1 text-xs">
      <div className="text-neon/50">{t("reveal.songGuessBy", { name: playerName })}</div>
      <GuessPart label={t("reveal.title")} value={result.title} correct={result.titleCorrect} />
      <GuessPart label={t("reveal.artist")} value={result.artist} correct={result.artistCorrect} />
      <div className={`mt-1 ${outcome.className}`}>{outcome.text}</div>
    </div>
  );
}

function GuessPart({ label, value, correct }: { label: string; value: string; correct: boolean }) {
  return (
    <div className="flex items-center justify-center gap-1.5">
      <span className="text-neon/40">{label}:</span>
      <span className="truncate max-w-[12rem]">{value || "—"}</span>
      {correct ? <Check size={14} className="text-success shrink-0" /> : <X size={14} className="text-danger shrink-0" />}
    </div>
  );
}
