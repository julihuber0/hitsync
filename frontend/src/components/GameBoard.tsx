import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useGameStore } from "../store/gameStore";
import TopBar from "./TopBar";
import Timeline from "./Timeline";
import PlayerList from "./PlayerList";
import RevealOverlay from "./RevealOverlay";
import SongGuessPanel from "./SongGuessPanel";
import LiveGuess from "./LiveGuess";
import CountdownRing from "./CountdownRing";
import { Check, RefreshCw, Volume2, Zap } from "lucide-react";
import { Aurora, Avatar } from "./ui";

const CHALLENGE_WINDOW_MS = 5000;

export default function GameBoard() {
  const { t } = useTranslation();
  const state = useGameStore((s) => s.state)!;
  const socket = useGameStore((s) => s.socket);
  const trackPrepare = useGameStore((s) => s.trackPrepare);
  const audioState = useGameStore((s) => s.audioState);
  const autoplayBlocked = useGameStore((s) => s.autoplayBlocked);
  const retryAudio = useGameStore((s) => s.retryAudio);
  const resumeAudio = useGameStore((s) => s.resumeAudio);
  const lastReveal = useGameStore((s) => s.lastReveal);
  const liveGuess = useGameStore((s) => s.liveGuess);

  const [selectedSlot, setSelectedSlot] = useState<number | null>(null);
  const [challengeSlot, setChallengeSlot] = useState<number | null>(null);

  const isActive = state.activePlayerId === state.youId;
  const activePlayer = state.players.find((p) => p.id === state.activePlayerId);
  const you = state.players.find((p) => p.id === state.youId);

  // Reset per-turn local UI state whenever a new track_prepare arrives.
  useEffect(() => {
    setSelectedSlot(null);
    setChallengeSlot(null);
  }, [trackPrepare?.prepareId]);

  const inPlacing = state.phase === "PLACING";
  const inChallenging = state.phase === "CHALLENGING";
  const turn = state.currentTurn;
  const stealWindowOpen = inChallenging && (turn?.stealWindowOpen ?? false);
  const youClaimed = turn?.stealClaims.includes(state.youId) ?? false;
  const youPassed = turn?.hasPassed.includes(state.youId) ?? false;
  // Pressing Steal is only possible during the window; placing the claimed
  // steal afterwards has no time limit.
  const canClaimSteal = stealWindowOpen && !isActive && (you?.tokens ?? 0) > 0 && !youClaimed && !youPassed;
  const selectingSteal = inChallenging && youClaimed && !(turn?.stealsPlaced.includes(state.youId) ?? false);
  const pendingStealers = inChallenging && turn
    ? state.players.filter((p) => p.id !== state.youId && turn.stealClaims.includes(p.id) && !turn.stealsPlaced.includes(p.id))
    : [];
  const otherPlayers = state.players.filter((player) => player.id !== state.youId);
  const previews = state.players.flatMap((player) => player.pendingChallengePreviewSlot == null
    ? []
    : [{ playerId: player.id, name: player.name, colour: player.colour, slot: player.pendingChallengePreviewSlot }]);

  const confirmPlacement = () => {
    if (selectedSlot === null) return;
    socket?.placeCard(selectedSlot);
  };

  const confirmChallenge = () => {
    if (challengeSlot === null) return;
    socket?.challenge(challengeSlot);
  };

  const selectChallengeSlot = (slot: number) => {
    setChallengeSlot(slot);
    socket?.previewChallenge(slot);
  };

  const selectPlacementSlot = (slot: number) => {
    setSelectedSlot(slot);
    socket?.previewPlacement(slot);
  };

  const playing = audioState === "playing";
  const guessFields = state.settings.guessFields;
  const guessEnabled = guessFields.title || guessFields.artist || guessFields.album || guessFields.year;
  const preparing = (state.phase === "PREPARING" || audioState === "loading") && audioState !== "error";

  return (
    <div className="relative flex min-h-screen flex-col lg:flex-row">
      <Aurora />
      <div className="flex min-w-0 flex-1 flex-col gap-5 p-3 sm:p-5">
        <TopBar />

        {/* Now playing */}
        <div className="flex flex-col items-center gap-3 pt-2">
          <div className="relative flex h-24 w-24 items-center justify-center">
            <div className={`absolute inset-0 rounded-full bg-linear-to-br from-accent/30 to-accent-end/30 blur-2xl transition-opacity duration-700 ${playing ? "opacity-100" : "opacity-40"}`} />
            <div className="surface relative flex h-24 w-24 items-center justify-center gap-[5px] rounded-[28px]">
              {[0.55, 0.9, 0.7, 1, 0.6].map((height, i) => (
                <span
                  key={i}
                  className={`w-[5px] origin-bottom rounded-full bg-linear-to-t from-accent to-accent-end ${playing ? "animate-equalizer" : ""}`}
                  style={{
                    height: `${height * 40}px`,
                    animationDelay: `${i * -0.18}s`,
                    animationDuration: `${0.9 + (i % 3) * 0.2}s`,
                    transform: playing ? undefined : "scaleY(0.35)",
                    transition: "transform 0.4s ease",
                  }}
                />
              ))}
            </div>
          </div>

          {preparing && (
            <div role="status" className="flex items-center gap-2.5 rounded-full border border-white/[0.07] bg-white/4 px-4 py-2 text-sm text-fg/70" aria-live="polite">
              <span className="spinner" aria-hidden="true" />
              {t(audioState === "ready" ? "board.waitingForPlayers" : "board.loadingTrack")}
            </div>
          )}

          {!isActive && activePlayer && guessEnabled && (inPlacing || inChallenging) && (
            <div className="flex w-full justify-center animate-fade-in">
              <LiveGuess player={activePlayer} fields={guessFields} guess={liveGuess} />
            </div>
          )}

          {audioState === "error" && trackPrepare && (
            <div role="alert" className="flex items-center gap-3 rounded-full border border-danger/30 bg-danger/10 py-1.5 pl-4 pr-1.5 text-sm text-danger">
              <span>{t("board.trackUnavailable")}</span>
              <button type="button" onClick={retryAudio} className="btn btn-secondary btn-sm rounded-full">
                <RefreshCw size={13} />
                {t("board.retryAudio")}
              </button>
            </div>
          )}
        </div>

        {otherPlayers.length > 0 && (
          <section className="space-y-3">
            <h2 className="eyebrow px-1">{t("board.otherTimelines")}</h2>
            <div className="grid grid-cols-[minmax(0,1fr)] gap-3 xl:grid-cols-2">
              {otherPlayers.map((player) => {
                const isStealTarget = selectingSteal && player.id === activePlayer?.id;
                const isActivePlayer = player.id === activePlayer?.id;
                return (
                  <article
                    key={player.id}
                    className={`surface min-w-0 rounded-2xl p-4 transition-all duration-300 ${
                      isStealTarget ? "border-accent/60 shadow-glow" : isActivePlayer ? "border-accent/30 bg-accent/5" : ""
                    }`}
                  >
                    <div className="flex items-center gap-2.5">
                      <Avatar name={player.name} colour={player.colour} size={26} dimmed={!player.connected} />
                      <span className="truncate text-sm font-semibold">{player.name}</span>
                      {isActivePlayer && (
                        <span className="ml-auto flex items-center gap-1.5 text-[11px] font-medium text-accent">
                          <span className="h-1.5 w-1.5 animate-glow-pulse rounded-full bg-accent" />
                          {t("board.playerTurn", { name: player.name })}
                        </span>
                      )}
                    </div>
                    <Timeline
                      timeline={player.timeline}
                      mode={isStealTarget ? "challenge" : "view"}
                      selectedSlot={isStealTarget ? challengeSlot : null}
                      onSelectSlot={isStealTarget ? selectChallengeSlot : undefined}
                      takenSlots={state.currentTurn?.challengeSlotsTaken ?? []}
                      disabledSlot={player.id === activePlayer?.id ? state.currentTurn?.activePlacementSlot : null}
                      placementSlot={player.id === activePlayer?.id ? state.currentTurn?.activePlacementSlot : null}
                      previewSlot={player.id === activePlayer?.id ? state.currentTurn?.activePlacementPreviewSlot : null}
                      previews={player.id === activePlayer?.id ? previews : []}
                    />
                  </article>
                );
              })}
            </div>
          </section>
        )}

        <section className={`surface mt-auto min-w-0 p-4 sm:p-5 ${inPlacing && isActive ? "border-accent/40 shadow-glow" : ""}`}>
          <div className="flex items-center justify-between gap-3 px-1">
            <h2 className="text-base font-semibold tracking-tight">{t("board.yourTimeline")}</h2>
            {inPlacing && isActive && (
              <span className="flex items-center gap-1.5 text-xs font-medium text-accent">
                <span className="h-1.5 w-1.5 animate-glow-pulse rounded-full bg-accent" />
                {t("board.placeCard")}
              </span>
            )}
          </div>
          <Timeline
            timeline={you?.timeline ?? []}
            size="large"
            mode={inPlacing && isActive ? "place" : "view"}
            selectedSlot={inPlacing && isActive ? selectedSlot : null}
            onSelectSlot={inPlacing && isActive ? selectPlacementSlot : undefined}
            placementSlot={isActive ? state.currentTurn?.activePlacementSlot : null}
          />

          {inPlacing && isActive && (
            <div className="mt-2 flex flex-col items-center gap-3 animate-fade-in">
              <button onClick={confirmPlacement} disabled={selectedSlot === null} className="btn btn-primary btn-lg min-w-[220px]">
                <Check size={18} />
                {t("board.confirmPlacement")}
              </button>
            </div>
          )}

          {/* The guess is independent of placing and stealing and stays editable until the reveal. */}
          {isActive && guessEnabled && (inPlacing || inChallenging) && (
            <div className="mt-5 flex justify-center animate-fade-in">
              <SongGuessPanel
                key={trackPrepare?.prepareId}
                fields={state.settings.guessFields}
                initial={turn?.songGuess ?? null}
                onChange={(guess) => socket?.songGuess(guess)}
              />
            </div>
          )}
        </section>

        {(canClaimSteal || selectingSteal || (pendingStealers.length > 0 && !stealWindowOpen)) && (
          <div className="sticky bottom-3 z-30 flex justify-center">
            <div className="surface flex max-w-full animate-fade-in flex-wrap items-center justify-center gap-3 rounded-2xl border-white/10 bg-elevated/80 px-4 py-3">
              {canClaimSteal && (
                <div className="flex items-center gap-2 sm:gap-3">
                  <CountdownRing
                    deadlineMs={state.phaseEndsAtServerMs}
                    totalMs={CHALLENGE_WINDOW_MS}
                    serverNow={() => socket?.clock.serverNow() ?? Date.now()}
                    size={38}
                  />
                  <button onClick={() => socket?.claimSteal()} className="btn btn-primary btn-md">
                    <Zap size={16} className="fill-current" />
                    {t("board.steal")}
                  </button>
                  <button onClick={() => socket?.passChallenge()} className="btn btn-secondary btn-md">
                    {t("board.pass")}
                  </button>
                </div>
              )}

              {selectingSteal && (
                <>
                  <p className="px-1 text-sm text-fg/75">{t("board.stealing", { name: activePlayer?.name ?? "" })}</p>
                  <button onClick={confirmChallenge} disabled={challengeSlot === null} className="btn btn-primary btn-md">
                    <Check size={16} />
                    {t("board.submitSteal")}
                  </button>
                </>
              )}

              {pendingStealers.length > 0 && !stealWindowOpen && (
                <p role="status" className={`flex items-center gap-2.5 px-1 text-sm text-fg/70 ${selectingSteal ? "sm:border-l sm:border-white/10 sm:pl-4" : ""}`}>
                  <span className="spinner" aria-hidden="true" />
                  {t("board.waitingForStealers", { names: pendingStealers.map((p) => p.name).join(", ") })}
                </p>
              )}
            </div>
          </div>
        )}

        {autoplayBlocked && (
          <button
            onClick={resumeAudio}
            className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-5 bg-bg/80 backdrop-blur-md"
          >
            <span className="brand-mark h-20 w-20 animate-glow-pulse rounded-3xl">
              <Volume2 size={36} />
            </span>
            <span className="text-xl font-semibold tracking-tight">{t("board.tapToEnableSound")}</span>
          </button>
        )}
      </div>

      <aside className="w-full shrink-0 p-3 pt-0 sm:p-5 sm:pt-0 lg:w-80 lg:border-l lg:border-white/6 lg:bg-white/1.5 lg:p-5 lg:backdrop-blur-xl">
        <div className="lg:sticky lg:top-5">
          <h2 className="eyebrow mb-3 px-1">{t("lobby.players")}</h2>
          <PlayerList
            players={state.players}
            youId={state.youId}
            hostId={state.hostId}
            variant="board"
            activePlayerId={state.activePlayerId}
            isHost={state.hostId === state.youId}
            onAdjustTokens={(playerId, delta) => socket?.adjustTokens(playerId, delta)}
          />
        </div>
      </aside>

      {state.phase === "REVEALING" && lastReveal && <RevealOverlay reveal={lastReveal} players={state.players} guessFields={guessFields} maxTokens={state.settings.maxTokens} />}
    </div>
  );
}
