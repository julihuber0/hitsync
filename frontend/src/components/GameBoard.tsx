import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useGameStore } from "../store/gameStore";
import TopBar from "./TopBar";
import Timeline from "./Timeline";
import PlayerList from "./PlayerList";
import RevealOverlay from "./RevealOverlay";
import SongGuessPanel from "./SongGuessPanel";
import CountdownRing from "./CountdownRing";

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

  return (
    <div className="min-h-screen flex flex-col lg:flex-row">
      <div className="flex-1 flex flex-col gap-5 p-4 min-w-0">
        <TopBar />

        <div className="flex flex-col items-center gap-3">
          <div className="relative w-20 h-20 rounded-2xl card-surface flex items-center justify-center overflow-hidden">
            <div className="absolute inset-0 flex items-center justify-center gap-1">
              {[...Array(5)].map((_, i) => (
                <span
                  key={i}
                  className="w-1.5 bg-accent rounded-full animate-pulse shadow-neon-sm"
                  style={{ height: `${20 + (i % 3) * 12}px`, animationDelay: `${i * 120}ms`, animationDuration: "1.2s" }}
                />
              ))}
            </div>
          </div>

          {(state.phase === "PREPARING" || audioState === "loading") && audioState !== "error" && (
            <div role="status" className="flex items-center gap-2 text-sm text-neon/65" aria-live="polite">
              <span className="w-4 h-4 rounded-full border-2 border-accent/30 border-t-accent animate-spin" aria-hidden="true" />
              {t(audioState === "ready" ? "board.waitingForPlayers" : "board.loadingTrack")}
            </div>
          )}

          {audioState === "error" && trackPrepare && (
            <div role="alert" className="flex items-center gap-3 text-sm text-danger">
              <span>{t("board.trackUnavailable")}</span>
              <button type="button" onClick={retryAudio} className="rounded-md bg-white/10 px-3 py-1.5 font-medium text-neon hover:bg-white/20 hover:shadow-neon-sm transition-all duration-150">
                {t("board.retryAudio")}
              </button>
            </div>
          )}
        </div>

        {otherPlayers.length > 0 && (
          <section className="space-y-3">
            <h2 className="text-sm font-semibold text-neon/60">{t("board.otherTimelines")}</h2>
            <div className="grid gap-3 xl:grid-cols-2">
              {otherPlayers.map((player) => {
                const isStealTarget = selectingSteal && player.id === activePlayer?.id;
                return (
                  <article
                    key={player.id}
                    className={`rounded-xl border p-3 transition-all duration-200 ${
                      player.id === activePlayer?.id ? "border-accent/40 bg-accent/5 shadow-neon-sm" : "border-white/10 bg-white/[0.02]"
                    }`}
                  >
                    <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                      <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: player.colour }} />
                      {player.name}
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

        <section className="mt-auto rounded-2xl border border-white/10 bg-white/[0.03] p-4 space-y-3">
          <h2 className="text-base font-semibold">{t("board.yourTimeline")}</h2>
          <Timeline
            timeline={you?.timeline ?? []}
            size="large"
            mode={inPlacing && isActive ? "place" : "view"}
            selectedSlot={inPlacing && isActive ? selectedSlot : null}
            onSelectSlot={inPlacing && isActive ? selectPlacementSlot : undefined}
            placementSlot={isActive ? state.currentTurn?.activePlacementSlot : null}
          />

          {inPlacing && isActive && (
            <div className="flex flex-col items-center gap-3 animate-fade-in">
              <button
                onClick={confirmPlacement}
                disabled={selectedSlot === null}
                className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2.5 px-8 rounded-lg disabled:opacity-40 disabled:hover:shadow-none"
              >
                {t("board.confirmPlacement")}
              </button>
            </div>
          )}

          {/* The guess is independent of placing and stealing and stays editable until the reveal. */}
          {isActive && state.settings.enableSongGuess && (inPlacing || inChallenging) && (
            <div className="flex justify-center animate-fade-in">
              <SongGuessPanel
                key={trackPrepare?.prepareId}
                initial={turn?.songGuess ?? null}
                onChange={(title, artist) => socket?.songGuess(title, artist)}
              />
            </div>
          )}
        </section>

        {canClaimSteal && (
          <div className="flex flex-wrap items-center justify-center gap-3 animate-fade-in">
            <CountdownRing
              deadlineMs={state.phaseEndsAtServerMs}
              totalMs={CHALLENGE_WINDOW_MS}
              serverNow={() => socket?.clock.serverNow() ?? Date.now()}
              size={32}
            />
            <button
              onClick={() => socket?.claimSteal()}
              className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2 px-6 rounded-lg text-sm"
            >
              {t("board.steal")}
            </button>
            <button
              onClick={() => socket?.passChallenge()}
              className="bg-white/10 hover:bg-white/20 hover:shadow-neon-cyan active:scale-[0.97] transition-all duration-200 py-2 px-6 rounded-lg text-sm"
            >
              {t("board.pass")}
            </button>
          </div>
        )}

        {selectingSteal && (
          <div className="flex flex-wrap items-center justify-center gap-3 animate-fade-in">
            <p className="w-full text-center text-sm text-neon/65">{t("board.stealing", { name: activePlayer?.name ?? "" })}</p>
            <button
              onClick={confirmChallenge}
              disabled={challengeSlot === null}
              className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2 px-6 rounded-lg disabled:opacity-40 disabled:hover:shadow-none text-sm"
            >
              {t("board.submitSteal")}
            </button>
          </div>
        )}

        {pendingStealers.length > 0 && !stealWindowOpen && (
          <p role="status" className="text-center text-sm text-neon/65 animate-fade-in">
            {t("board.waitingForStealers", { names: pendingStealers.map((p) => p.name).join(", ") })}
          </p>
        )}

        {autoplayBlocked && (
          <button
            onClick={resumeAudio}
            className="neon-heading fixed inset-0 z-50 bg-black/80 flex items-center justify-center text-xl font-semibold text-accent animate-glow-pulse"
          >
            {t("board.tapToEnableSound")}
          </button>
        )}
      </div>

      <div className="w-full lg:w-72 shrink-0 p-4 card-surface lg:rounded-none">
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

      {state.phase === "REVEALING" && lastReveal && <RevealOverlay reveal={lastReveal} players={state.players} />}
    </div>
  );
}
