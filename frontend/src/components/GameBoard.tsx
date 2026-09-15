import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useGameStore } from "../store/gameStore";
import { audioPlayer } from "../audio/instance";
import type { AudioConnectionState } from "../audio/player";
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
  const trackStart = useGameStore((s) => s.trackStart);
  const trackStopSignal = useGameStore((s) => s.trackStopSignal);
  const lastReveal = useGameStore((s) => s.lastReveal);

  const [selectedSlot, setSelectedSlot] = useState<number | null>(null);
  const [challengeSlot, setChallengeSlot] = useState<number | null>(null);
  const [stealMode, setStealMode] = useState(false);
  const [titleGuess, setTitleGuess] = useState<string | null>(null);
  const [artistGuess, setArtistGuess] = useState<string | null>(null);
  const [autoplayBlocked, setAutoplayBlocked] = useState(false);
  const [audioState, setAudioState] = useState<AudioConnectionState>("idle");
  const readySentFor = useRef<string | null>(null);

  const isActive = state.activePlayerId === state.youId;
  const activePlayer = state.players.find((p) => p.id === state.activePlayerId);
  const you = state.players.find((p) => p.id === state.youId);

  // Reset per-turn local UI state whenever a new track_prepare arrives.
  useEffect(() => {
    setSelectedSlot(null);
    setChallengeSlot(null);
    setStealMode(false);
    setTitleGuess(null);
    setArtistGuess(null);
  }, [trackPrepare?.prepareId]);

  // Subscribe to the server's LiveKit broadcast during PREPARING.
  useEffect(() => {
    if (!trackPrepare) return;
    audioPlayer.onAutoplayBlocked = () => setAutoplayBlocked(true);
    audioPlayer.onConnectionStateChange = setAudioState;
    void audioPlayer.prepare(trackPrepare.prepareId, trackPrepare.livekitUrl, trackPrepare.livekitToken)
      .then(() => {
        // Multiple effects may await the same idempotent prepare promise.
        // Send one ready acknowledgement for this game turn.
        if (readySentFor.current !== trackPrepare.prepareId) {
          readySentFor.current = trackPrepare.prepareId;
          socket?.ready(trackPrepare.prepareId);
        }
      })
      .catch(() => setAudioState("error"));
  }, [trackPrepare?.prepareId, socket]);

  useEffect(() => {
    if (!trackStart) return;
    audioPlayer.start();
    setAutoplayBlocked(false);
  }, [trackStart]);

  useEffect(() => {
    if (trackStopSignal === 0) return;
    audioPlayer.stop(400);
  }, [trackStopSignal]);

  const inPlacing = state.phase === "PLACING";
  const inChallenging = state.phase === "CHALLENGING";
  const youChallenged = you?.pendingChallengeSlot != null;
  const youPassed = state.currentTurn?.hasPassed?.includes(state.youId) ?? false;
  const alreadyActed = youChallenged || youPassed;
  const canSteal = inChallenging && !isActive && (you?.tokens ?? 0) > 0 && !alreadyActed;
  const selectingSteal = canSteal && stealMode;
  const otherPlayers = state.players.filter((player) => player.id !== state.youId);
  const previews = state.players.flatMap((player) => player.pendingChallengePreviewSlot == null
    ? []
    : [{ playerId: player.id, name: player.name, colour: player.colour, slot: player.pendingChallengePreviewSlot }]);

  const confirmPlacement = () => {
    if (selectedSlot === null) return;
    socket?.placeCard(selectedSlot, titleGuess ?? undefined, artistGuess ?? undefined);
  };

  const confirmChallenge = () => {
    if (challengeSlot === null) return;
    socket?.challenge(challengeSlot);
    setStealMode(false);
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

          {(state.phase === "PREPARING" || audioState === "connecting" || audioState === "subscribed") && (
            <div role="status" className="flex items-center gap-2 text-sm text-neon/65" aria-live="polite">
              <span className="w-4 h-4 rounded-full border-2 border-accent/30 border-t-accent animate-spin" aria-hidden="true" />
              {state.phase === "PREPARING"
                ? t(audioState === "connecting" ? "board.connectingBroadcast" : "board.waitingForReady")
                : t(audioState === "subscribed" ? "board.broadcastReady" : "board.connectingBroadcast")}
            </div>
          )}

          {audioState === "error" && !autoplayBlocked && trackPrepare && (
            <div role="alert" className="flex items-center gap-3 text-sm text-danger">
              <span>{t("board.broadcastUnavailable")}</span>
              <button type="button" onClick={() => {
                readySentFor.current = null;
                void audioPlayer.prepare(trackPrepare.prepareId, trackPrepare.livekitUrl, trackPrepare.livekitToken)
                  .then(() => {
                    if (readySentFor.current !== trackPrepare.prepareId) {
                      readySentFor.current = trackPrepare.prepareId;
                      socket?.ready(trackPrepare.prepareId);
                    }
                  })
                  .catch(() => setAudioState("error"));
              }} className="rounded-md bg-white/10 px-3 py-1.5 font-medium text-neon hover:bg-white/20 hover:shadow-neon-sm transition-all duration-150">
                {t("board.retryAudio")}
              </button>
            </div>
          )}
        </div>

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
              {trackPrepare?.guessOptions && <SongGuessPanel options={trackPrepare.guessOptions} titleGuess={titleGuess} artistGuess={artistGuess} onSelectTitle={setTitleGuess} onSelectArtist={setArtistGuess} />}
              <button
                onClick={confirmPlacement}
                disabled={selectedSlot === null}
                className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2.5 px-8 rounded-lg disabled:opacity-40 disabled:hover:shadow-none"
              >
                {t("board.confirmPlacement")}
              </button>
            </div>
          )}
        </section>

        {inChallenging && !isActive && (
          <div className="flex flex-wrap items-center justify-center gap-3 animate-fade-in">
            {!alreadyActed && (
              <CountdownRing
                deadlineMs={state.phaseEndsAtServerMs}
                totalMs={CHALLENGE_WINDOW_MS}
                serverNow={() => socket?.clock.serverNow() ?? Date.now()}
                size={32}
              />
            )}
            {canSteal && !stealMode && (
              <button
                onClick={() => setStealMode(true)}
                className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2 px-6 rounded-lg text-sm"
              >
                {t("board.steal")}
              </button>
            )}
            {selectingSteal && (
              <>
                <p className="w-full text-center text-sm text-neon/65">{t("board.stealing", { name: activePlayer?.name ?? "" })}</p>
                <button
                  onClick={confirmChallenge}
                  disabled={challengeSlot === null}
                  className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2 px-6 rounded-lg disabled:opacity-40 disabled:hover:shadow-none text-sm"
                >
                  {t("board.submitSteal")}
                </button>
              </>
            )}
            <button
              onClick={() => { setStealMode(false); socket?.passChallenge(); }}
              disabled={alreadyActed}
              className="bg-white/10 hover:bg-white/20 hover:shadow-neon-cyan active:scale-[0.97] transition-all duration-200 py-2 px-6 rounded-lg text-sm disabled:opacity-40 disabled:hover:shadow-none"
            >
              {t("board.pass")}
            </button>
          </div>
        )}

        {autoplayBlocked && (
          <button
            onClick={() => { audioPlayer.retryPlay(); setAutoplayBlocked(false); }}
            className="neon-heading fixed inset-0 z-50 bg-black/80 flex items-center justify-center text-xl font-semibold text-accent animate-glow-pulse"
          >
            {t("board.tapToEnableSound")}
          </button>
        )}
      </div>

      <div className="w-full lg:w-72 shrink-0 p-4 card-surface lg:rounded-none">
        <PlayerList players={state.players} youId={state.youId} hostId={state.hostId} variant="board" activePlayerId={state.activePlayerId} />
      </div>

      {state.phase === "REVEALING" && lastReveal && <RevealOverlay reveal={lastReveal} players={state.players} />}
    </div>
  );
}
