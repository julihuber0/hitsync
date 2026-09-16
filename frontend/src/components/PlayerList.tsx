import { memo } from "react";
import { useTranslation } from "react-i18next";
import { Crown, Circle, Coins, Layers, Plus, Minus } from "lucide-react";
import type { PlayerView } from "../ws/protocol";
import { activeHighlight, Avatar } from "./ui";

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
    <ul className={`flex flex-col ${variant === "board" ? "gap-1.5" : "gap-2"}`}>
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
      className={`group relative flex items-center gap-2.5 rounded-xl border px-2.5 transition-all duration-200 ${variant === "board" ? "py-1.5" : "py-2.5"} ${
        isActive ? "" : "border-transparent bg-white/3 hover:bg-white/5"
      }`}
      style={isActive ? activeHighlight(player.colour) : undefined}
    >
      <span
        className={`relative rounded-full ${variant === "board" && isYou ? "ring-2 ring-white/70 ring-offset-2 ring-offset-bg" : ""}`}
        title={variant === "board" && isYou ? t("lobby.you") : undefined}
      >
        <Avatar name={player.name} colour={player.colour} size={variant === "board" ? 28 : 34} dimmed={!player.connected} />
        {!player.connected && (
          <span className="absolute -bottom-0.5 -right-0.5 flex h-3.5 w-3.5 items-center justify-center rounded-full bg-bg">
            <Circle size={8} className="fill-current text-danger" />
          </span>
        )}
      </span>

      <div className="min-w-0 flex-1">
        <div className={`flex items-center gap-1.5 text-sm font-medium ${player.connected ? "" : "text-fg/40"}`}>
          <span className="min-w-[3ch] truncate">{player.name}</span>
          {isHost && <Crown size={13} className="shrink-0 text-gold" />}
          {isYou && variant === "lobby" && <span className="shrink-0 text-xs font-normal text-fg/40">({t("lobby.you")})</span>}
        </div>
      </div>

      {variant === "board" && (
        <span className="flex shrink-0 items-center gap-1">
          <span className="chip" title={t("board.cardsCount", { count: player.timeline.length })}>
            <Layers size={11} className="text-accent" />
            {player.timeline.length}
          </span>
          <span className="chip" title={t("board.tokensCount", { count: player.tokens })}>
            <Coins size={11} className="text-gold" />
            {player.tokens}
          </span>
        </span>
      )}

      {onAdjustTokens && (
        <span className="flex shrink-0 items-center gap-0.5">
          <button
            onClick={() => onAdjustTokens(-1)}
            aria-label={t("board.removeToken")}
            className="flex h-6 w-6 items-center justify-center rounded-md border border-white/10 bg-white/4 text-fg/70 transition-all duration-150 hover:border-white/20 hover:bg-white/10 hover:text-fg active:scale-90 disabled:opacity-30"
            disabled={player.tokens <= 0}
          >
            <Minus size={13} />
          </button>
          <button
            onClick={() => onAdjustTokens(1)}
            aria-label={t("board.addToken")}
            className="flex h-6 w-6 items-center justify-center rounded-md border border-white/10 bg-white/4 text-fg/70 transition-all duration-150 hover:border-white/20 hover:bg-white/10 hover:text-fg active:scale-90"
          >
            <Plus size={13} />
          </button>
        </span>
      )}

      {onKick && (
        <button onClick={onKick} className="btn btn-danger btn-sm shrink-0">
          {t("lobby.kick")}
        </button>
      )}
    </li>
  );
});

export default memo(PlayerListImpl);
