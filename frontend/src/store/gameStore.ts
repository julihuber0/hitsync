import { create } from "zustand";
import { GameSocket } from "../ws/socket";
import type { ErrorPayload, RevealPayload, StatePayload, TrackPreparePayload, TrackStartPayload } from "../ws/protocol";

interface GameStore {
  socket: GameSocket | null;
  connected: boolean;
  state: StatePayload | null;
  trackPrepare: TrackPreparePayload | null;
  trackStart: TrackStartPayload | null;
  trackStopSignal: number;
  lastReveal: RevealPayload | null;
  lastError: ErrorPayload | null;
  kickedReason: string | null;

  connect: (playerToken: string) => void;
  disconnect: () => void;
  clearError: () => void;
}

function wsBaseUrl(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/ws`;
}

export const useGameStore = create<GameStore>((set, get) => ({
  socket: null,
  connected: false,
  state: null,
  trackPrepare: null,
  trackStart: null,
  trackStopSignal: 0,
  lastReveal: null,
  lastError: null,
  kickedReason: null,

  connect: (playerToken: string) => {
    get().socket?.close();

    const socket = new GameSocket(wsBaseUrl(), playerToken, {
      onConnectionChange: (connected) => set({ connected }),
      onState: (state) => set({ state }),
      // A stop signal belongs only to the preceding publication. Reset it as
      // soon as a new turn's broadcast is announced so a GameBoard remount
      // cannot tear down the fresh LiveKit room with an old stop event.
      onTrackPrepare: (trackPrepare) => set({ trackPrepare, trackStart: null, trackStopSignal: 0 }),
      onTrackStart: (trackStart) => set({ trackStart }),
      onTrackStop: () => set((s) => ({ trackStopSignal: s.trackStopSignal + 1 })),
      onReveal: (lastReveal) => set({ lastReveal }),
      onError: (lastError) => set({ lastError }),
      onKicked: (kickedReason) => set({ kickedReason }),
    });
    socket.connect();
    set({ socket, trackStopSignal: 0 });
  },

  disconnect: () => {
    get().socket?.close();
    set({ socket: null, connected: false, state: null, trackPrepare: null, trackStart: null, trackStopSignal: 0, lastReveal: null });
  },

  clearError: () => set({ lastError: null }),
}));
