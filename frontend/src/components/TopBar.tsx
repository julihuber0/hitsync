import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Volume2, VolumeX, Wifi, WifiOff } from "lucide-react";
import { useGameStore } from "../store/gameStore";
import { audioPlayer } from "../audio/instance";
import CountdownRing from "./CountdownRing";
import LanguageToggle from "./LanguageToggle";

const PHASE_DURATIONS: Record<string, number> = {
  PREPARING: 8000,
  PLACING: 90000,
  CHALLENGING: 20000,
  REVEALING: 8000,
};

export default function TopBar() {
  const { t } = useTranslation();
  const state = useGameStore((s) => s.state)!;
  const socket = useGameStore((s) => s.socket);
  const connected = useGameStore((s) => s.connected);
  const [volume, setVolume] = useState(audioPlayer.getVolume());
  const [muted, setMuted] = useState(audioPlayer.isMuted());

  const activePlayer = state.players.find((p) => p.id === state.activePlayerId);
  const isYourTurn = state.activePlayerId === state.youId;

  const serverNow = () => useGameStore.getState().socket?.clock.serverNow() ?? Date.now();

  return (
    <div className="flex items-center justify-between gap-4 px-4 py-3 card-surface">
      <div className="flex items-center gap-3 min-w-0">
        <span className="text-sm font-semibold tabular-nums shrink-0">{t("board.turn", { number: state.turnNumber })}</span>
        <span className="text-sm text-white/60 truncate">
          {isYourTurn ? t("board.yourTurn") : t("board.playerTurn", { name: activePlayer?.name ?? "" })}
        </span>
      </div>

      <CountdownRing deadlineMs={state.phaseEndsAtServerMs} totalMs={PHASE_DURATIONS[state.phase] ?? 0} serverNow={serverNow} size={44} />

      <div className="flex items-center gap-4 shrink-0">
        <div className="flex items-center gap-2">
          <button
            onClick={() => {
              const next = !muted;
              setMuted(next);
              audioPlayer.setMuted(next);
            }}
            aria-label="mute"
          >
            {muted ? <VolumeX size={18} /> : <Volume2 size={18} />}
          </button>
          <input
            type="range"
            min={0}
            max={1}
            step={0.05}
            value={volume}
            onChange={(e) => {
              const v = Number(e.target.value);
              setVolume(v);
              audioPlayer.setVolume(v);
            }}
            className="w-20 accent-accent"
          />
        </div>

        {connected ? <Wifi size={16} className="text-success" /> : <WifiOff size={16} className="text-danger" />}
        <LanguageToggle />

        {state.hostId === state.youId && (
          <div className="flex items-center gap-2">
            <button onClick={() => socket?.skipTrack()} className="text-xs bg-white/10 hover:bg-white/20 rounded-md px-2.5 py-1.5">
              {t("board.skip")}
            </button>
            <button onClick={() => socket?.endGame()} className="text-xs text-danger/80 hover:text-danger px-2">
              {t("board.endGame")}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
