import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useGameStore } from "../store/gameStore";
import Confetti from "./Confetti";
import LanguageToggle from "./LanguageToggle";

export default function GameOverScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const state = useGameStore((s) => s.state)!;
  const socket = useGameStore((s) => s.socket);

  const winner = state.players.find((p) => p.id === state.winnerId);
  const isHost = state.hostId === state.youId;

  return (
    <div className="min-h-screen px-4 py-10 animate-fade-in">
      {winner && <Confetti />}

      <div className="max-w-3xl mx-auto flex items-center justify-between mb-8">
        <h1 className="neon-heading text-2xl font-semibold">{t("gameOver.title")}</h1>
        <LanguageToggle />
      </div>

      <div className="max-w-3xl mx-auto text-center mb-10">
        <p className="neon-heading text-3xl font-semibold text-accent">
          {winner ? t("gameOver.winner", { name: winner.name }) : t("gameOver.noWinner")}
        </p>
      </div>

      <div className="max-w-3xl mx-auto grid sm:grid-cols-2 gap-4 mb-10">
        {state.players.map((p, i) => (
          <div
            key={p.id}
            className="card-surface p-4 animate-fade-in"
            style={{ animationDelay: `${i * 60}ms` }}
          >
            <div className="flex items-center gap-2 mb-2">
              <span className="w-3 h-3 rounded-full" style={{ backgroundColor: p.colour, boxShadow: `0 0 8px ${p.colour}` }} />
              <span className="font-medium">{p.name}</span>
              <span className="text-xs text-neon/40 ml-auto tabular-nums">
                {t("board.cardsCount", { count: p.timeline.length })}
              </span>
            </div>
            <div className="flex flex-wrap gap-1">
              {p.timeline
                .slice()
                .sort((a, b) => a.year - b.year)
                .map((c, i) => (
                  <span key={c.trackId + i} className="text-xs bg-white/5 border border-white/5 rounded px-2 py-1 tabular-nums">
                    {c.year}
                  </span>
                ))}
            </div>
          </div>
        ))}
      </div>

      <div className="max-w-3xl mx-auto flex justify-center gap-4">
        {isHost && (
          <button
            onClick={() => socket?.playAgain()}
            className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2.5 px-8 rounded-lg"
          >
            {t("gameOver.playAgain")}
          </button>
        )}
        <button
          onClick={() => navigate("/")}
          className="bg-white/10 hover:bg-white/20 hover:shadow-neon-cyan active:scale-[0.97] transition-all duration-200 py-2.5 px-8 rounded-lg"
        >
          {t("gameOver.backHome")}
        </button>
      </div>
    </div>
  );
}
