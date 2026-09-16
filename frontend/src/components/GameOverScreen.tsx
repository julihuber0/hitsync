import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useGameStore } from "../store/gameStore";
import Confetti from "./Confetti";
import LanguageToggle from "./LanguageToggle";
import { Home, Layers, RotateCcw, Trophy } from "lucide-react";
import { Aurora, Avatar, Logo } from "./ui";

export default function GameOverScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const state = useGameStore((s) => s.state)!;
  const socket = useGameStore((s) => s.socket);

  const winner = state.players.find((p) => p.id === state.winnerId);
  const isHost = state.hostId === state.youId;

  return (
    <div className="relative min-h-screen px-4 pb-14 pt-5 sm:px-6">
      <Aurora />
      {winner && <Confetti />}

      <header className="mx-auto flex max-w-4xl items-center justify-between">
        <div className="flex items-center gap-3">
          <Logo />
          <span className="h-5 w-px bg-white/10" />
          <h1 className="text-sm font-medium text-fg/60">{t("gameOver.title")}</h1>
        </div>
        <LanguageToggle />
      </header>

      <div className="mx-auto mt-12 flex max-w-4xl animate-fade-in flex-col items-center text-center">
        <div className="relative mb-5">
          <div className="absolute inset-0 rounded-full bg-gradient-to-br from-gold/40 to-accent/30 blur-2xl" />
          {winner ? (
            <div className="relative">
              <Avatar name={winner.name} colour={winner.colour} size={88} />
              <span className="absolute -bottom-1 -right-1 flex h-9 w-9 items-center justify-center rounded-full border-4 border-bg bg-gold text-bg">
                <Trophy size={16} strokeWidth={2.5} />
              </span>
            </div>
          ) : (
            <span className="surface relative flex h-[88px] w-[88px] items-center justify-center rounded-full text-fg/50">
              <Trophy size={34} />
            </span>
          )}
        </div>
        <p className={`text-4xl font-bold tracking-tight sm:text-5xl ${winner ? "brand-text" : "text-fg/80"}`}>
          {winner ? t("gameOver.winner", { name: winner.name }) : t("gameOver.noWinner")}
        </p>
      </div>

      <div className="mx-auto mt-10 grid max-w-4xl gap-4 sm:grid-cols-2">
        {state.players.map((p, i) => {
          const isWinner = p.id === winner?.id;
          return (
            <div
              key={p.id}
              className={`surface animate-fade-in p-5 ${isWinner ? "border-gold/40 shadow-[0_20px_50px_-20px_rgba(251,191,36,0.35)]" : ""}`}
              style={{ animationDelay: `${i * 60}ms` }}
            >
              <div className="mb-4 flex items-center gap-3">
                <Avatar name={p.name} colour={p.colour} size={36} />
                <span className="truncate font-semibold">{p.name}</span>
                {isWinner && <Trophy size={15} className="shrink-0 text-gold" />}
                <span className="chip ml-auto shrink-0">
                  <Layers size={11} className="text-accent" />
                  {t("board.cardsCount", { count: p.timeline.length })}
                </span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {p.timeline
                  .slice()
                  .sort((a, b) => a.year - b.year)
                  .map((c, i) => (
                    <span
                      key={c.trackId + i}
                      title={`${c.title} — ${c.artist}`}
                      className="rounded-lg border border-white/[0.07] bg-white/[0.04] px-2.5 py-1 text-xs font-semibold tabular-nums text-fg/80"
                    >
                      {c.year}
                    </span>
                  ))}
              </div>
            </div>
          );
        })}
      </div>

      <div className="mx-auto mt-10 flex max-w-4xl flex-wrap justify-center gap-3">
        {isHost && (
          <button onClick={() => socket?.playAgain()} className="btn btn-primary btn-lg min-w-[180px]">
            <RotateCcw size={17} />
            {t("gameOver.playAgain")}
          </button>
        )}
        <button onClick={() => navigate("/")} className="btn btn-secondary btn-lg min-w-[180px]">
          <Home size={17} />
          {t("gameOver.backHome")}
        </button>
      </div>
    </div>
  );
}
