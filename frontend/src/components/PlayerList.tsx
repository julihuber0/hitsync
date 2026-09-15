import { memo } from "react";
import { useTranslation } from "react-i18next";
import { Crown, Circle, Plus, Minus } from "lucide-react";
import type { PlayerView } from "../ws/protocol";

interface Props {
  players: PlayerView[];
  youId: string;
  hostId: string;
  variant: "lobby" | "board";
  activePlayerId?: string;
  onKick?: (playerId: string) => void;
  onAdjustTokens?: (playerId: string, delta: number) => void;
  isHost?: boolean;
}

function PlayerListImpl({ players, youId, hostId, variant, activePlayerId, onKick, onAdjustTokens, isHost }: Props) {
  const { t } = useTranslation();
  return (
    <ul className="flex flex-col gap-2">
      {players.map((p) => (
        <PlayerRow
          key={p.id}
          player={p}
          isYou={p.id === youId}
          isHost={p.id === hostId}
          isActive={p.id === activePlayerId}
          variant={variant}
          onKick={variant === "lobby" && isHost && p.id !== youId ? () => onKick?.(p.id) : undefined}
          onAdjustTokens={variant === "board" && isHost ? (delta) => onAdjustTokens?.(p.id, delta) : undefined}
          kickLabel={t("lobby.kick")}
          youLabel={t("common.yes")}
        />
      ))}
    </ul>
  );
}

interface RowProps {
  player: PlayerView;
  isYou: boolean;
  isHost: boolean;
  isActive: boolean;
  variant: "lobby" | "board";
  onKick?: () => void;
  onAdjustTokens?: (delta: number) => void;
  kickLabel: string;
  youLabel: string;
}

const PlayerRow = memo(function PlayerRow({ player, isYou, isHost, isActive, variant, onKick, onAdjustTokens }: RowProps) {
  const { t } = useTranslation();
  return (
    <li
      className={`flex items-center gap-3 rounded-lg px-3 py-2 transition-all duration-200 ${
        isActive ? "bg-white/10 shadow-neon-sm ring-1 ring-accent/30" : "bg-white/[0.03]"
      }`}
    >
      <span
        className="w-3 h-3 rounded-full shrink-0"
        style={{
          backgroundColor: player.colour,
          opacity: player.connected ? 1 : 0.35,
          boxShadow: player.connected ? `0 0 6px ${player.colour}` : "none",
        }}
      />
      <span className={`flex-1 text-sm truncate ${player.connected ? "" : "text-neon/40"}`}>
        {player.name}
        {isYou && <span className="text-neon/40"> ({t("lobby.you")})</span>}
      </span>
      {isHost && <Crown size={14} className="text-accent drop-shadow-neon shrink-0" />}
      {!player.connected && <Circle size={8} className="text-danger fill-current shrink-0" />}

      {variant === "board" && (
        <span className="text-xs text-neon/50 tabular-nums shrink-0">
          {t("board.cardsCount", { count: player.timeline.length })} · {t("board.tokensCount", { count: player.tokens })}
        </span>
      )}

      {onAdjustTokens && (
        <span className="flex items-center gap-1 shrink-0">
          <button
            onClick={() => onAdjustTokens(-1)}
            aria-label={t("board.removeToken")}
            className="w-5 h-5 flex items-center justify-center rounded bg-white/10 hover:bg-white/20 hover:shadow-neon-cyan active:scale-90 transition-all duration-150 disabled:opacity-30"
            disabled={player.tokens <= 0}
          >
            <Minus size={12} />
          </button>
          <button
            onClick={() => onAdjustTokens(1)}
            aria-label={t("board.addToken")}
            className="w-5 h-5 flex items-center justify-center rounded bg-white/10 hover:bg-white/20 hover:shadow-neon-cyan active:scale-90 transition-all duration-150"
          >
            <Plus size={12} />
          </button>
        </span>
      )}

      {onKick && (
        <button onClick={onKick} className="text-xs text-danger/70 hover:text-danger shrink-0">
          {t("lobby.kick")}
        </button>
      )}
    </li>
  );
});

export default memo(PlayerListImpl);
