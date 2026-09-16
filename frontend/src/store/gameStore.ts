import { create } from "zustand";
import { GameSocket } from "../ws/socket";
import { audioPlayer } from "../audio/instance";
import type { AudioState } from "../audio/player";
import type { ErrorPayload, RevealPayload, SongGuess, StatePayload, TrackPreparePayload, TrackStartPayload } from "../ws/protocol";

interface GameStore {
  socket: GameSocket | null;
  connected: boolean;
  state: StatePayload | null;
  trackPrepare: TrackPreparePayload | null;
  trackStart: TrackStartPayload | null;
  /** The active player's guess as they type it, for everyone else to watch. */
  liveGuess: SongGuess | null;
  audioState: AudioState;
  autoplayBlocked: boolean;
  lastReveal: RevealPayload | null;
  lastError: ErrorPayload | null;
  kickedReason: string | null;

  connect: (playerToken: string) => void;
  disconnect: () => void;
  clearError: () => void;
  /** Re-downloads the current track after a failed download. */
  retryAudio: () => void;
  /** Resumes playback from a user gesture after the browser blocked autoplay. */
  resumeAudio: () => void;
}

function wsBaseUrl(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/ws`;
}

export const useGameStore = create<GameStore>((set, get) => {
  audioPlayer.onStateChange = (audioState) => set({ audioState });
  audioPlayer.onAutoplayBlocked = () => set({ autoplayBlocked: true });

  const prepareTrack = (socket: GameSocket, p: TrackPreparePayload) => {
    audioPlayer.prepare(p.prepareId, p.trackId, p.mediaUrl, p.durationMs)
      .then(() => socket.ready(p.prepareId))
      .catch(() => {}); // surfaced as audioState "error"
  };
  const startTrack = (socket: GameSocket, p: TrackStartPayload) => {
    audioPlayer.start(p.prepareId, p.startAtServerMs, () => socket.clock.serverNow());
  };

  return {
    socket: null,
    connected: false,
    state: null,
    trackPrepare: null,
    trackStart: null,
    liveGuess: null,
    audioState: "idle",
    autoplayBlocked: false,
    lastReveal: null,
    lastError: null,
    kickedReason: null,

    connect: (playerToken: string) => {
      get().socket?.close();

      const socket: GameSocket = new GameSocket(wsBaseUrl(), playerToken, {
        onConnectionChange: (connected) => set({ connected }),
        onState: (state) => {
          // Outside a running game nothing may still hold a downloaded track.
          if (state.phase === "LOBBY" || state.phase === "GAME_OVER") audioPlayer.teardown();
          // Snapshots and live updates both carry the server's current guess,
          // in order, so the latest one received is always right.
          set({ state, liveGuess: state.currentTurn?.songGuess ?? null });
        },
        onSongGuessUpdate: (guess) => set({ liveGuess: guess }),
        onTrackPreload: (p) => audioPlayer.preload(p.trackId, p.mediaUrl),
        onTrackPrepare: (trackPrepare) => {
          set({ trackPrepare, trackStart: null });
          prepareTrack(socket, trackPrepare);
        },
        onTrackStart: (trackStart) => {
          set({ trackStart });
          startTrack(socket, trackStart);
        },
        onTrackStop: (p) => {
          audioPlayer.stop(p.prepareId, p.fadeMs);
          set({ trackStart: null, autoplayBlocked: false });
        },
        onReveal: (lastReveal) => set({ lastReveal }),
        onError: (lastError) => set({ lastError }),
        onKicked: (kickedReason) => set({ kickedReason }),
      });
      socket.connect();
      set({ socket });
    },

    disconnect: () => {
      get().socket?.close();
      audioPlayer.teardown();
      set({ socket: null, connected: false, state: null, trackPrepare: null, trackStart: null, liveGuess: null, autoplayBlocked: false, lastReveal: null });
    },

    clearError: () => set({ lastError: null }),

    retryAudio: () => {
      const { socket, trackPrepare, trackStart } = get();
      if (!socket || !trackPrepare) return;
      prepareTrack(socket, trackPrepare);
      if (trackStart?.prepareId === trackPrepare.prepareId) startTrack(socket, trackStart);
    },

    resumeAudio: () => {
      audioPlayer.retryPlay();
      set({ autoplayBlocked: false });
    },
  };
});
