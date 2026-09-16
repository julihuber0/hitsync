import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Coins, Copy, Check, Play, Users } from "lucide-react";
import type { GuessFields } from "../ws/protocol";
import { useGameStore } from "../store/gameStore";
import { useAppStore } from "../store/appStore";
import PlayerList from "./PlayerList";
import QrCode from "./QrCode";
import LanguageToggle from "./LanguageToggle";
import { Aurora, Logo, Slider } from "./ui";

function formatInviteCode(code: string): string {
  return code.length === 6 ? `${code.slice(0, 3)}-${code.slice(3)}` : code;
}

export default function Lobby() {
  const { t } = useTranslation();
  const state = useGameStore((s) => s.state)!;
  const socket = useGameStore((s) => s.socket);
  const minPlayers = useAppStore((s) => s.config?.minPlayers ?? 1);
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
    <div className="relative min-h-screen px-4 pb-12 pt-5 sm:px-6">
      <Aurora />
      <header className="mx-auto flex max-w-5xl items-center justify-between">
        <div className="flex items-center gap-3">
          <Logo />
          <span className="h-5 w-px bg-white/10" />
          <h1 className="text-sm font-medium text-fg/60">{t("lobby.title")}</h1>
        </div>
        <LanguageToggle />
      </header>

      <div className="mx-auto mt-8 grid max-w-5xl animate-fade-in gap-5 lg:grid-cols-[1fr_1.15fr]">
        <section className="surface self-start p-6">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="eyebrow">{t("lobby.players")}</h2>
            <span className="chip">
              <Users size={12} />
              {state.players.length}
            </span>
          </div>
          <PlayerList
            players={state.players}
            youId={state.youId}
            hostId={state.hostId}
            variant="lobby"
            isHost={isHost}
            onKick={(playerId) => socket?.kickPlayer(playerId)}
          />
        </section>

        <div className="flex flex-col gap-5">
          <section className="surface p-6">
            <h2 className="eyebrow mb-5">{t("lobby.settings")}</h2>
            <SettingsPanel isHost={isHost} />
          </section>

          <section className="surface flex flex-col items-center gap-5 p-6 sm:flex-row sm:items-center">
            <QrCode value={joinUrl} size={132} />
            <div className="flex w-full flex-1 flex-col items-center gap-3 sm:items-start">
              <h2 className="eyebrow">{t("lobby.invite")}</h2>
              <div className="brand-text font-mono text-4xl font-bold tracking-[0.18em]">{formatInviteCode(state.inviteCode)}</div>
              <button onClick={() => void copyLink()} className="btn btn-secondary btn-md">
                {copied ? <Check size={16} className="text-success" /> : <Copy size={16} />}
                {copied ? t("common.copied") : t("lobby.copyLink")}
              </button>
            </div>
          </section>

          {isHost && (
            <div className="group relative">
              <button onClick={() => socket?.startGame()} disabled={!canStart} className="btn btn-primary btn-lg h-14 w-full text-base">
                <Play size={18} className="fill-current" />
                {t("lobby.start")}
              </button>
              {!canStart && (
                <div className="absolute -top-10 left-1/2 -translate-x-1/2 whitespace-nowrap rounded-lg border border-white/10 bg-elevated px-3 py-1.5 text-xs opacity-0 shadow-card transition-opacity group-hover:opacity-100">
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
  const maxTokensLimit = useAppStore((s) => s.config?.maxTokensLimit ?? 10);
  const { targetCards, startTokens, maxTokens, guessFields } = state.settings;
  const guessOptions: Array<{ key: keyof GuessFields; label: string }> = [
    { key: "title", label: t("lobby.guessTitle") },
    { key: "artist", label: t("lobby.guessArtist") },
    { key: "album", label: t("lobby.guessAlbum") },
    { key: "year", label: t("lobby.guessYear") },
  ];

  return (
    <div className="flex flex-col gap-5">
      <SettingRow label={t("lobby.targetCards", { count: targetCards })} value={targetCards}>
        <Slider
          min={5}
          max={20}
          value={targetCards}
          disabled={!isHost}
          onChange={(e) => socket?.updateSettings({ targetCards: Number(e.target.value) })}
        />
      </SettingRow>
      <SettingRow label={t("lobby.maxTokens")} value={maxTokens}>
        <Slider
          min={1}
          max={maxTokensLimit}
          value={maxTokens}
          disabled={!isHost}
          onChange={(e) => socket?.updateSettings({ maxTokens: Number(e.target.value) })}
        />
      </SettingRow>
      <SettingRow label={t("lobby.startTokens")} value={startTokens}>
        <Slider
          min={0}
          max={maxTokens}
          value={startTokens}
          disabled={!isHost}
          onChange={(e) => socket?.updateSettings({ startTokens: Number(e.target.value) })}
        />
      </SettingRow>
      <div className="flex flex-col gap-3 border-t border-white/6 pt-4">
        <div>
          <div className="flex items-center gap-2 text-sm font-medium text-fg/80">
            <Coins size={15} className="text-gold" />
            {t("lobby.guessFields")}
          </div>
          <p className="mt-1 text-xs leading-relaxed text-fg/45">{t("lobby.guessFieldsHint")}</p>
        </div>
        <div className="grid grid-cols-2 gap-2">
          {guessOptions.map(({ key, label }) => (
            <label
              key={key}
              className={`flex items-center justify-between gap-3 rounded-xl border px-3 py-2.5 text-sm transition-colors ${
                guessFields[key] ? "border-accent/35 bg-accent/[0.07] text-fg" : "border-white/[0.07] bg-white/2 text-fg/60"
              } ${isHost ? "cursor-pointer hover:border-white/15" : ""}`}
            >
              {label}
              <input
                type="checkbox"
                checked={guessFields[key]}
                disabled={!isHost}
                onChange={(e) => socket?.updateSettings({ guessFields: { ...guessFields, [key]: e.target.checked } })}
                className="switch"
              />
            </label>
          ))}
        </div>
      </div>
    </div>
  );
}

function SettingRow({ label, value, children }: { label: string; value: number; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex items-center justify-between text-sm font-medium text-fg/80">
        <span>{label}</span>
        <span className="min-w-8 rounded-md bg-white/6 px-2 py-0.5 text-center text-xs font-semibold tabular-nums text-fg">{value}</span>
      </div>
      {children}
    </div>
  );
}
