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
  // Outside a challenge, show the player's own established timeline. During
  // a challenge the target timeline remains visible so its slots can be
  // claimed. This prevents a player's starting card from appearing to change
  // whenever the active player rotates.
  const displayedTimeline = (inChallenging ? activePlayer : you)?.timeline ?? [];

  const youChallenged = you?.pendingChallengeSlot != null;
  const youPassed = state.currentTurn?.hasPassed?.includes(state.youId) ?? false;
  const alreadyActed = youChallenged || youPassed;
  const canChallenge = inChallenging && !isActive && (you?.tokens ?? 0) > 0 && !alreadyActed;

  const confirmPlacement = () => {
    if (selectedSlot === null) return;
    socket?.placeCard(selectedSlot, titleGuess ?? undefined, artistGuess ?? undefined);
  };

  const confirmChallenge = () => {
    if (challengeSlot === null) return;
    socket?.challenge(challengeSlot);
  };

  return (
    <div className="min-h-screen bg-bg flex flex-col lg:flex-row">
      <div className="flex-1 flex flex-col gap-6 p-4">
        <TopBar />

        <div className="flex-1 flex flex-col items-center justify-center gap-6">
          <div className="relative w-32 h-32 sm:w-40 sm:h-40 rounded-2xl card-surface flex items-center justify-center overflow-hidden">
            <div className="absolute inset-0 flex items-center justify-center gap-1">
              {[...Array(5)].map((_, i) => (
                <span
                  key={i}
                  className="w-1.5 bg-accent/60 rounded-full animate-pulse"
                  style={{
                    height: `${20 + (i % 3) * 12}px`,
                    animationDelay: `${i * 120}ms`,
                    animationDuration: "1.2s",
                  }}
                />
              ))}
            </div>
          </div>

          {(state.phase === "PREPARING" || audioState === "connecting" || audioState === "subscribed") && (
            <div role="status" className="flex items-center gap-2 text-sm text-white/65" aria-live="polite">
              <span className="w-4 h-4 rounded-full border-2 border-accent/30 border-t-accent animate-spin" aria-hidden="true" />
              {state.phase === "PREPARING"
                ? t(audioState === "connecting" ? "board.connectingBroadcast" : "board.waitingForReady")
                : t(audioState === "subscribed" ? "board.broadcastReady" : "board.connectingBroadcast")}
            </div>
          )}

          {audioState === "error" && !autoplayBlocked && trackPrepare && (
            <div role="alert" className="flex items-center gap-3 text-sm text-danger">
              <span>{t("board.broadcastUnavailable")}</span>
              <button
                type="button"
                onClick={() => {
                  readySentFor.current = null;
                  void audioPlayer.prepare(trackPrepare.prepareId, trackPrepare.livekitUrl, trackPrepare.livekitToken)
                    .then(() => {
                      if (readySentFor.current !== trackPrepare.prepareId) {
                        readySentFor.current = trackPrepare.prepareId;
                        socket?.ready(trackPrepare.prepareId);
                      }
                    })
                    .catch(() => setAudioState("error"));
                }}
                className="rounded-md bg-white/10 px-3 py-1.5 font-medium text-white hover:bg-white/20"
              >
                {t("board.retryAudio")}
              </button>
            </div>
          )}

          {autoplayBlocked && (
            <button
              onClick={() => {
                audioPlayer.retryPlay();
                setAutoplayBlocked(false);
              }}
              className="fixed inset-0 z-50 bg-black/80 flex items-center justify-center text-xl font-semibold"
            >
              {t("board.tapToEnableSound")}
            </button>
          )}

          <Timeline
            timeline={displayedTimeline}
            mode={inPlacing && isActive ? "place" : inChallenging && canChallenge ? "challenge" : "view"}
            selectedSlot={inPlacing ? selectedSlot : challengeSlot}
            onSelectSlot={inPlacing ? setSelectedSlot : setChallengeSlot}
            takenSlots={state.currentTurn?.challengeSlotsTaken ?? []}
          />

          {inPlacing && isActive && (
            <div className="flex flex-col items-center gap-3 w-full max-w-md">
              {trackPrepare?.guessOptions && (
                <SongGuessPanel
                  options={trackPrepare.guessOptions}
                  titleGuess={titleGuess}
                  artistGuess={artistGuess}
                  onSelectTitle={setTitleGuess}
                  onSelectArtist={setArtistGuess}
                />
              )}
              <button
                onClick={confirmPlacement}
                disabled={selectedSlot === null}
                className="bg-accent hover:brightness-110 transition-[filter] text-white font-semibold py-2.5 px-8 rounded-lg disabled:opacity-40"
              >
                {t("board.confirmPlacement")}
              </button>
            </div>
          )}

          {inChallenging && (
            <div className="flex items-center gap-3">
              {canChallenge && (
                <button
                  onClick={confirmChallenge}
                  disabled={challengeSlot === null}
                  className="bg-accent hover:brightness-110 transition-[filter] text-white font-semibold py-2 px-6 rounded-lg disabled:opacity-40 text-sm"
                >
                  {t("board.challenge")}
                </button>
              )}
              {!isActive && (
                <button
                  onClick={() => socket?.passChallenge()}
                  disabled={youChallenged || alreadyActed}
                  className="bg-white/10 hover:bg-white/20 transition-colors py-2 px-6 rounded-lg text-sm disabled:opacity-40"
                >
                  {t("board.pass")}
                </button>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="w-full lg:w-72 shrink-0 p-4 card-surface lg:rounded-none">
        <PlayerList players={state.players} youId={state.youId} hostId={state.hostId} variant="board" activePlayerId={state.activePlayerId} />
      </div>

      {state.phase === "REVEALING" && lastReveal && <RevealOverlay reveal={lastReveal} players={state.players} />}
    </div>
  );
}
