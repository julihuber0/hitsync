import { memo } from "react";
import { useTranslation } from "react-i18next";
import { Crown, Circle } from "lucide-react";
import type { PlayerView } from "../ws/protocol";

interface Props {
  players: PlayerView[];
  youId: string;
  hostId: string;
  variant: "lobby" | "board";
  activePlayerId?: string;
  onKick?: (playerId: string) => void;
  isHost?: boolean;
}

function PlayerListImpl({ players, youId, hostId, variant, activePlayerId, onKick, isHost }: Props) {
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
  kickLabel: string;
  youLabel: string;
}

const PlayerRow = memo(function PlayerRow({ player, isYou, isHost, isActive, variant, onKick }: RowProps) {
  const { t } = useTranslation();
  return (
    <li
      className={`flex items-center gap-3 rounded-lg px-3 py-2 transition-colors ${
        isActive ? "bg-white/10" : "bg-white/[0.03]"
      }`}
    >
      <span
        className="w-3 h-3 rounded-full shrink-0"
        style={{ backgroundColor: player.colour, opacity: player.connected ? 1 : 0.35 }}
      />
      <span className={`flex-1 text-sm truncate ${player.connected ? "" : "text-white/40"}`}>
        {player.name}
        {isYou && <span className="text-white/40"> ({t("lobby.you")})</span>}
      </span>
      {isHost && <Crown size={14} className="text-accent shrink-0" />}
      {!player.connected && <Circle size={8} className="text-danger fill-current shrink-0" />}

      {variant === "board" && (
        <span className="text-xs text-white/50 tabular-nums shrink-0">
          {t("board.cardsCount", { count: player.timeline.length })} · {t("board.tokensCount", { count: player.tokens })}
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
