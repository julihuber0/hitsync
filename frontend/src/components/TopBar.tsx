import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Flag, SkipForward, Volume2, VolumeX, Wifi, WifiOff } from "lucide-react";
import { useGameStore } from "../store/gameStore";
import { audioPlayer } from "../audio/instance";
import CountdownRing from "./CountdownRing";
import LanguageToggle from "./LanguageToggle";
import { Slider } from "./ui";

// PLACING has no entry: it has no deadline (the track loops until the
// active player submits or the host skips), so CountdownRing renders idle.
const PHASE_DURATIONS: Record<string, number> = {
  PREPARING: 8000,
  CHALLENGING: 5000,
  REVEALING: 8000,
};

export default function TopBar() {
  const { t } = useTranslation();
  const state = useGameStore((s) => s.state)!;
  const socket = useGameStore((s) => s.socket);
  const connected = useGameStore((s) => s.connected);
  const playing = useGameStore((s) => s.audioState === "playing");
  const [volume, setVolume] = useState(audioPlayer.getVolume());
  const [muted, setMuted] = useState(audioPlayer.isMuted());

  const activePlayer = state.players.find((p) => p.id === state.activePlayerId);
  const isYourTurn = state.activePlayerId === state.youId;

  const serverNow = () => useGameStore.getState().socket?.clock.serverNow() ?? Date.now();

  return (
    <header className="surface flex items-center gap-3 rounded-2xl px-3 py-2.5 sm:gap-4 sm:px-4">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <NowPlaying playing={playing} />
        <div className="min-w-0">
          <div className="eyebrow tabular-nums">{t("board.turn", { number: state.turnNumber })}</div>
          <div className="flex items-center gap-2 truncate text-sm font-semibold">
            {activePlayer && <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: activePlayer.colour }} />}
            <span className={`truncate ${isYourTurn ? "brand-text" : ""}`}>
              {isYourTurn ? t("board.yourTurn") : t("board.playerTurn", { name: activePlayer?.name ?? "" })}
            </span>
          </div>
        </div>
      </div>

      {/* An idle ring only takes space on narrow screens. */}
      <span className={state.phaseEndsAtServerMs === null ? "hidden sm:inline-flex" : "inline-flex"}>
        <CountdownRing deadlineMs={state.phaseEndsAtServerMs} totalMs={PHASE_DURATIONS[state.phase] ?? 0} serverNow={serverNow} size={42} />
      </span>

      <div className="flex flex-1 items-center justify-end gap-1.5 sm:gap-2">
        <div className="hidden items-center gap-1 rounded-lg border border-white/[0.07] bg-white/3 pl-1 pr-3 md:flex">
          <button
            onClick={() => {
              const next = !muted;
              setMuted(next);
              audioPlayer.setMuted(next);
            }}
            aria-label="mute"
            className="icon-btn h-8 w-8"
          >
            {muted ? <VolumeX size={16} /> : <Volume2 size={16} />}
          </button>
          <Slider
            min={0}
            max={1}
            step={0.05}
            value={volume}
            onChange={(e) => {
              const v = Number(e.target.value);
              setVolume(v);
              audioPlayer.setVolume(v);
            }}
            className="w-20"
          />
        </div>
        <button
          onClick={() => {
            const next = !muted;
            setMuted(next);
            audioPlayer.setMuted(next);
          }}
          aria-label="mute"
          className="icon-btn md:hidden"
        >
          {muted ? <VolumeX size={17} /> : <Volume2 size={17} />}
        </button>

        <span
          className={`flex h-9 w-9 items-center justify-center rounded-lg ${connected ? "text-success" : "text-danger"}`}
          title={connected ? "connected" : "disconnected"}
        >
          {connected ? <Wifi size={16} /> : <WifiOff size={16} />}
        </span>
        <LanguageToggle className="hidden sm:inline-flex" />

        {state.hostId === state.youId && (
          <div className="flex items-center gap-1 border-l border-white/8 pl-1.5 sm:pl-2">
            <button onClick={() => socket?.skipTrack()} className="btn btn-secondary btn-sm h-9" title={t("board.skip")}>
              <SkipForward size={14} />
              <span className="hidden lg:inline">{t("board.skip")}</span>
            </button>
            <button onClick={() => socket?.endGame()} className="btn btn-danger btn-sm h-9" title={t("board.endGame")}>
              <Flag size={14} />
              <span className="hidden lg:inline">{t("board.endGame")}</span>
            </button>
          </div>
        )}
      </div>
    </header>
  );
}

/** The brand mark as a small equalizer that moves while the song plays. */
function NowPlaying({ playing }: { playing: boolean }) {
  return (
    <span
      className={`brand-mark hidden h-9 w-9 shrink-0 items-end gap-[3px] pb-2.5 sm:flex ${playing ? "" : "opacity-80"}`}
      aria-hidden
    >
      {[0.45, 0.85, 1, 0.65, 0.4].map((height, i) => (
        <span
          key={i}
          className={`w-[3px] origin-bottom rounded-full bg-white ${playing ? "animate-equalizer" : ""}`}
          style={{
            height: `${height * 16}px`,
            animationDelay: `${i * -0.18}s`,
            animationDuration: `${0.9 + (i % 3) * 0.2}s`,
            transform: playing ? undefined : "scaleY(0.4)",
            transition: "transform 0.4s ease",
          }}
        />
      ))}
    </span>
  );
}
