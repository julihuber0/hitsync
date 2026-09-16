import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "react-i18next";
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

          {reveal.songGuessResult?.awarded && <div className="text-xs text-accent2">{t("reveal.songGuessCorrect")}</div>}

          <div className="text-xs text-neon/30 mt-2">{t("reveal.continuing")}</div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
