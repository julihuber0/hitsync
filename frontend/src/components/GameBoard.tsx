import { useEffect, useRef, useState } from "react";
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
import { activeHighlight, Aurora, Avatar } from "./ui";

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

  // Bring the active player's timeline into view when a turn starts. Only on
  // wide screens, where just the other players' panel scrolls; on phones this
  // would move the whole page under the player's finger.
  const activeCardRef = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!window.matchMedia("(min-width: 1024px)").matches) return;
    // Browsers don't animate scrolling in background tabs; jump instead so the
    // card is in view when the player comes back.
    const behavior = document.visibilityState === "visible" ? "smooth" : "instant";
    activeCardRef.current?.scrollIntoView({ behavior, block: "nearest" });
  }, [state.activePlayerId, state.turnNumber]);

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

  const turnRunning = state.phase === "PREPARING" || inPlacing || inChallenging;
  const guessFields = state.settings.guessFields;
  const guessEnabled = guessFields.title || guessFields.artist || guessFields.album || guessFields.year;
  const preparing = (state.phase === "PREPARING" || audioState === "loading") && audioState !== "error";
  const showLiveGuess = !isActive && !!activePlayer && guessEnabled && (inPlacing || inChallenging);
  const showConfirm = inPlacing && isActive;
  const showGuessForm = isActive && guessEnabled && (inPlacing || inChallenging);

  // Density follows the number of other players, so a full table still fits
  // a 1080p screen: smaller cards from 3 opponents, a third column only from
  // 9 (narrower columns mean more scrolling within each timeline).
  const others = otherPlayers.length;
  const opponentCardSize = others >= 3 ? "small" : "default";
  const opponentColumns = others >= 9 ? "xl:grid-cols-2 2xl:grid-cols-3" : others >= 2 ? "xl:grid-cols-2" : "";

  const playerList = (
    <PlayerList
      players={state.players}
      youId={state.youId}
      hostId={state.hostId}
      variant="board"
      activePlayerId={state.activePlayerId}
      isHost={state.hostId === state.youId}
      onAdjustTokens={(playerId, delta) => socket?.adjustTokens(playerId, delta)}
    />
  );

  return (
    // Your own timeline must always be in view. On wide screens the board is
    // exactly one screen tall and only the other players' timelines scroll;
    // on phones your timeline sticks to the bottom while the page scrolls.
    <div className="relative flex min-h-dvh flex-col lg:h-dvh lg:flex-row lg:overflow-hidden">
      <Aurora />
      <div className="flex min-w-0 flex-1 flex-col gap-3 p-3 lg:min-h-0 lg:p-4">
        <TopBar />

        {(preparing || showLiveGuess || (audioState === "error" && trackPrepare)) && (
          <div className="flex flex-wrap items-center justify-center gap-3">
            {preparing && (
              <div role="status" className="flex items-center gap-2.5 rounded-full border border-white/[0.07] bg-white/4 px-4 py-2 text-sm text-fg/70" aria-live="polite">
                <span className="spinner" aria-hidden="true" />
                {t(audioState === "ready" ? "board.waitingForPlayers" : "board.loadingTrack")}
              </div>
            )}

            {showLiveGuess && <LiveGuess player={activePlayer} fields={guessFields} guess={liveGuess} />}

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
        )}

        {others > 0 ? (
          <section className="flex flex-col gap-0.5 lg:min-h-0 lg:flex-1">
            <h2 className="eyebrow px-1">{t("board.otherTimelines")}</h2>
            <div className={`grid grid-cols-[minmax(0,1fr)] content-start gap-x-2.5 gap-y-3.5 pt-2 lg:-mx-1 lg:min-h-0 lg:overflow-y-auto lg:px-1 lg:pb-3 lg:[mask-image:linear-gradient(to_bottom,black_calc(100%-14px),transparent)] ${opponentColumns}`}>
              {otherPlayers.map((player) => {
                const isStealTarget = selectingSteal && player.id === activePlayer?.id;
                const isActivePlayer = player.id === activePlayer?.id;
                return (
                  <article
                    key={player.id}
                    ref={isActivePlayer ? activeCardRef : undefined}
                    className={`surface relative flex min-w-0 scroll-my-2 flex-col rounded-2xl px-3 pt-2.5 transition-all duration-300 lg:flex-row lg:items-center lg:gap-2 lg:py-0.5 ${
                      isStealTarget ? "border-accent/60 shadow-glow" : ""
                    }`}
                    style={isActivePlayer && !isStealTarget ? activeHighlight(player.colour) : undefined}
                  >
                    {isActivePlayer && (
                      <span
                        className="absolute -top-2.5 right-3 z-10 flex max-w-[60%] items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-semibold leading-4 text-white shadow-card"
                        style={{ backgroundColor: `color-mix(in srgb, ${player.colour} 80%, #000)` }}
                      >
                        <span className="h-1.5 w-1.5 shrink-0 animate-glow-pulse rounded-full bg-white" />
                        <span className="truncate">{t("board.playerTurn", { name: player.name })}</span>
                      </span>
                    )}
                    <div className="flex min-w-0 items-center gap-2 lg:w-20 lg:shrink-0 lg:flex-col lg:items-start lg:gap-1">
                      <Avatar name={player.name} colour={player.colour} size={26} dimmed={!player.connected} />
                      <span className="flex min-w-0 max-w-full items-center gap-1.5 text-sm font-semibold">
                        <span className="truncate">{player.name}</span>
                      </span>
                    </div>
                    <div className="min-w-0 flex-1">
                      <Timeline
                        timeline={player.timeline}
                        size={opponentCardSize}
                        mode={isStealTarget ? "challenge" : "view"}
                        selectedSlot={isStealTarget ? challengeSlot : null}
                        onSelectSlot={isStealTarget ? selectChallengeSlot : undefined}
                        takenSlots={state.currentTurn?.challengeSlotsTaken ?? []}
                        disabledSlot={player.id === activePlayer?.id ? state.currentTurn?.activePlacementSlot : null}
                        placementSlot={player.id === activePlayer?.id ? state.currentTurn?.activePlacementSlot : null}
                        previewSlot={player.id === activePlayer?.id ? state.currentTurn?.activePlacementPreviewSlot : null}
                        previews={player.id === activePlayer?.id ? previews : []}
                      />
                    </div>
                  </article>
                );
              })}
            </div>
          </section>
        ) : (
          <div className="hidden lg:block lg:flex-1" />
        )}

        {/* Phones: the player list sits here so your timeline below can stay pinned. */}
        <section className="lg:hidden">
          <h2 className="eyebrow mb-2 px-1">{t("lobby.players")}</h2>
          {playerList}
        </section>

        <div className="z-20 flex shrink-0 flex-col gap-2 max-lg:sticky max-lg:bottom-0 max-lg:-mx-3 max-lg:-mb-3 max-lg:bg-bg/85 max-lg:px-3 max-lg:pb-3 max-lg:pt-2 max-lg:backdrop-blur-xl">
          {(canClaimSteal || selectingSteal || (pendingStealers.length > 0 && !stealWindowOpen)) && (
            <div className="flex justify-center">
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

          <section
            className="surface min-w-0 px-3 py-3 transition-all duration-300 lg:px-4"
            style={isActive && turnRunning && you ? activeHighlight(you.colour) : undefined}
          >
            <div className="flex items-center justify-between gap-3 px-1">
              <h2 className="text-sm font-semibold tracking-tight">{t("board.yourTimeline")}</h2>
              {isActive && turnRunning && you && (
                <span
                  className="flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-semibold text-white"
                  style={{ backgroundColor: `color-mix(in srgb, ${you.colour} 80%, #000)` }}
                >
                  <span className="h-1.5 w-1.5 animate-glow-pulse rounded-full bg-white" />
                  {t(inPlacing ? "board.placeCard" : "board.yourTurn")}
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

            {(showConfirm || showGuessForm) && (
              <div className="mt-1 flex animate-fade-in flex-col items-center gap-3 lg:flex-row lg:items-end lg:justify-center">
                {showConfirm && (
                  <button
                    onClick={confirmPlacement}
                    disabled={selectedSlot === null}
                    className="btn btn-primary btn-lg min-w-[220px] shrink-0 lg:order-last"
                  >
                    <Check size={18} />
                    {t("board.confirmPlacement")}
                  </button>
                )}

                {/* The guess is independent of placing and stealing and stays editable until the reveal. */}
                {showGuessForm && (
                  <div className="w-full min-w-0 lg:max-w-4xl lg:flex-1">
                    <SongGuessPanel
                      key={trackPrepare?.prepareId}
                      fields={guessFields}
                      initial={turn?.songGuess ?? null}
                      onChange={(guess) => socket?.songGuess(guess)}
                    />
                  </div>
                )}
              </div>
            )}
          </section>
        </div>

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

      <aside className="hidden shrink-0 lg:block lg:h-full lg:w-80 lg:overflow-y-auto lg:border-l lg:border-white/6 lg:bg-white/1.5 lg:p-4 lg:backdrop-blur-xl">
        <h2 className="eyebrow mb-3 px-1">{t("lobby.players")}</h2>
        {playerList}
      </aside>

      {state.phase === "REVEALING" && lastReveal && <RevealOverlay reveal={lastReveal} players={state.players} guessFields={guessFields} maxTokens={state.settings.maxTokens} />}
    </div>
  );
}
