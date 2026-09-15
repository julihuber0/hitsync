import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Copy, Check } from "lucide-react";
import { useGameStore } from "../store/gameStore";
import { useAppStore } from "../store/appStore";
import PlayerList from "./PlayerList";
import QrCode from "./QrCode";
import LanguageToggle from "./LanguageToggle";

function formatInviteCode(code: string): string {
  return code.length === 6 ? `${code.slice(0, 3)}-${code.slice(3)}` : code;
}

export default function Lobby() {
  const { t } = useTranslation();
  const state = useGameStore((s) => s.state)!;
  const socket = useGameStore((s) => s.socket);
  const minPlayers = useAppStore((s) => s.config?.minPlayers ?? 2);
  const [copied, setCopied] = useState(false);

  const isHost = state.hostId === state.youId;
  const joinUrl = `${window.location.origin}/j/${state.inviteCode}`;

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(joinUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard API unavailable; nothing more we can do.
    }
  };

  const canStart = state.players.length >= minPlayers;

  return (
    <div className="min-h-screen bg-bg px-4 py-8">
      <div className="max-w-5xl mx-auto flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold">{t("lobby.title")}</h1>
        <LanguageToggle />
      </div>

      <div className="max-w-5xl mx-auto grid lg:grid-cols-[1fr_1.2fr] gap-6">
        <div className="card-surface p-6">
          <h2 className="text-sm font-semibold text-white/70 mb-3">
            {t("lobby.players")} ({state.players.length})
          </h2>
          <PlayerList
            players={state.players}
            youId={state.youId}
            hostId={state.hostId}
            variant="lobby"
            isHost={isHost}
            onKick={(playerId) => socket?.kickPlayer(playerId)}
          />
        </div>

        <div className="flex flex-col gap-6">
          <div className="card-surface p-6">
            <h2 className="text-sm font-semibold text-white/70 mb-4">{t("lobby.settings")}</h2>
            <SettingsPanel isHost={isHost} />
          </div>

          <div className="card-surface p-6 flex flex-col items-center gap-4">
            <h2 className="text-sm font-semibold text-white/70 self-start">{t("lobby.invite")}</h2>
            <div className="text-4xl font-mono font-semibold tracking-[0.2em]">{formatInviteCode(state.inviteCode)}</div>
            <QrCode value={joinUrl} />
            <button
              onClick={() => void copyLink()}
              className="flex items-center gap-2 text-sm bg-white/10 hover:bg-white/20 transition-colors rounded-lg px-4 py-2"
            >
              {copied ? <Check size={16} className="text-success" /> : <Copy size={16} />}
              {copied ? t("common.copied") : t("lobby.copyLink")}
            </button>
          </div>

          {isHost && (
            <div className="relative group">
              <button
                onClick={() => socket?.startGame()}
                disabled={!canStart}
                className="w-full bg-accent hover:brightness-110 transition-[filter] text-white font-semibold py-3 rounded-lg disabled:opacity-40 disabled:cursor-not-allowed"
              >
                {t("lobby.start")}
              </button>
              {!canStart && (
                <div className="absolute -top-9 left-1/2 -translate-x-1/2 text-xs bg-black/80 px-3 py-1.5 rounded-md whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity">
                  {t("lobby.startDisabled", { count: minPlayers })}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function SettingsPanel({ isHost }: { isHost: boolean }) {
  const { t } = useTranslation();
  const state = useGameStore((s) => s.state)!;
  const socket = useGameStore((s) => s.socket);
  const { targetCards, startTokens, enableSongGuess } = state.settings;

  return (
    <div className="flex flex-col gap-4">
      <SettingRow label={t("lobby.targetCards", { count: targetCards })}>
        <input
          type="range"
          min={5}
          max={20}
          value={targetCards}
          disabled={!isHost}
          onChange={(e) => socket?.updateSettings({ targetCards: Number(e.target.value) })}
          className="w-full accent-accent"
        />
      </SettingRow>
      <SettingRow label={t("lobby.startTokens")}>
        <input
          type="range"
          min={0}
          max={5}
          value={startTokens}
          disabled={!isHost}
          onChange={(e) => socket?.updateSettings({ startTokens: Number(e.target.value) })}
          className="w-full accent-accent"
        />
        <span className="text-sm tabular-nums w-6 text-right">{startTokens}</span>
      </SettingRow>
      <label className="flex items-center justify-between text-sm">
        {t("lobby.songGuess")}
        <input
          type="checkbox"
          checked={enableSongGuess}
          disabled={!isHost}
          onChange={(e) => socket?.updateSettings({ enableSongGuess: e.target.checked })}
          className="accent-accent w-4 h-4"
        />
      </label>
    </div>
  );
}

function SettingRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between text-sm text-white/70">
        <span>{label}</span>
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  );
}
