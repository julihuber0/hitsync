import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "react-i18next";
import { Check, Coins, X, Zap } from "lucide-react";
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
        className="fixed inset-0 z-40 flex items-center justify-center bg-bg/85 p-4 backdrop-blur-lg"
      >
        <motion.div
          initial={{ rotateY: 90, opacity: 0, scale: 0.96 }}
          animate={{ rotateY: 0, opacity: 1, scale: 1 }}
          transition={{ duration: 0.35, ease: [0.2, 0.8, 0.2, 1] }}
          style={{ transformStyle: "preserve-3d" }}
          className="relative w-full max-w-sm overflow-hidden rounded-[28px] border border-white/10 bg-gradient-to-b from-elevated to-surface text-center shadow-[0_40px_80px_-20px_rgba(0,0,0,0.8)]"
        >
          <div aria-hidden className="pointer-events-none absolute -top-24 left-1/2 h-48 w-72 -translate-x-1/2 rounded-full bg-gradient-to-r from-accent/40 to-accent-end/40 blur-3xl" />
          <div className="relative flex flex-col items-center gap-2 px-8 pb-7 pt-9">
            <span className="chip mb-2 uppercase tracking-[0.12em]">{sourceLabel(t, reveal.yearSource)}</span>
            <div className="brand-text text-7xl font-bold tabular-nums tracking-tighter">{reveal.card.year}</div>
            <div className="mt-2 text-xl font-semibold leading-snug tracking-tight">{reveal.card.title}</div>
            <div className="text-sm text-fg/55">{reveal.card.artist}</div>

            <div className="mt-5">
              {reveal.outcome === "active_correct" && (
                <span className="inline-flex items-center gap-1.5 rounded-full border border-success/30 bg-success/10 px-3.5 py-1.5 text-sm font-semibold text-success">
                  <Check size={15} />
                  {t("reveal.correct")}
                </span>
              )}
              {reveal.outcome === "challenger_correct" && winner && (
                <span className="inline-flex items-center gap-1.5 rounded-full border border-accent2/30 bg-accent2/10 px-3.5 py-1.5 text-sm font-semibold text-accent2">
                  <Zap size={15} className="fill-current" />
                  {t("reveal.stolen", { name: winner.name })}
                </span>
              )}
              {reveal.outcome === "discarded" && (
                <span className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.05] px-3.5 py-1.5 text-sm font-medium text-fg/60">
                  <X size={15} />
                  {t("reveal.discarded")}
                </span>
              )}
            </div>
          </div>

          {reveal.songGuessResult && (
            <SongGuessResult
              result={reveal.songGuessResult}
              playerName={players.find((p) => p.id === reveal.activePlayerId)?.name ?? ""}
            />
          )}

          <div className="flex items-center justify-center gap-2 border-t border-white/[0.06] py-3.5 text-xs text-fg/40">
            <span className="spinner h-3 w-3 border" />
            {t("reveal.continuing")}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}

function SongGuessResult({ result, playerName }: { result: NonNullable<RevealPayload["songGuessResult"]>; playerName: string }) {
  const { t } = useTranslation();
  const outcome = result.awarded
    ? { text: t("reveal.songGuessCorrect"), className: "text-gold" }
    : result.correct
      ? { text: t("reveal.songGuessAtMax"), className: "text-fg/60" }
      : { text: t("reveal.songGuessWrong"), className: "text-fg/45" };

  return (
    <div className="mx-5 mb-5 rounded-2xl border border-white/[0.07] bg-black/25 p-4 text-left">
      <div className="eyebrow mb-2.5 flex items-center gap-1.5">
        <Coins size={12} className="text-gold" />
        {t("reveal.songGuessBy", { name: playerName })}
      </div>
      <div className="flex flex-col gap-1.5">
        <GuessPart label={t("reveal.title")} value={result.title} correct={result.titleCorrect} />
        <GuessPart label={t("reveal.artist")} value={result.artist} correct={result.artistCorrect} />
      </div>
      <div className={`mt-3 text-xs font-medium ${outcome.className}`}>{outcome.text}</div>
    </div>
  );
}

function GuessPart({ label, value, correct }: { label: string; value: string; correct: boolean }) {
  return (
    <div className="flex items-center gap-2 text-sm">
      <span
        className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full ${
          correct ? "bg-success/15 text-success" : "bg-danger/15 text-danger"
        }`}
      >
        {correct ? <Check size={12} strokeWidth={3} /> : <X size={12} strokeWidth={3} />}
      </span>
      <span className="w-12 shrink-0 text-xs text-fg/40">{label}</span>
      <span className="truncate font-medium">{value || "—"}</span>
    </div>
  );
}
