import { ClockSync, computeSample, type ClockSample } from "./clock";
import type {
  Envelope,
  ErrorPayload,
  RevealPayload,
  StatePayload,
  TrackPreloadPayload,
  TrackPreparePayload,
  TrackStartPayload,
  TrackStopPayload,
} from "./protocol";

export interface SocketHandlers {
  onState?: (s: StatePayload) => void;
  onTrackPreload?: (p: TrackPreloadPayload) => void;
  onTrackPrepare?: (p: TrackPreparePayload) => void;
  onTrackStart?: (p: TrackStartPayload) => void;
  onTrackStop?: (p: TrackStopPayload) => void;
  onReveal?: (p: RevealPayload) => void;
  onError?: (p: ErrorPayload) => void;
  onKicked?: (reason: string) => void;
  onConnectionChange?: (connected: boolean) => void;
}

const RECONNECT_DELAYS = [500, 1000, 2000, 4000, 10000];

export class GameSocket {
  private ws: WebSocket | null = null;
  private reconnectAttempt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private resyncTimer: ReturnType<typeof setInterval> | null = null;
  private closedByUser = false;
  readonly clock = new ClockSync();

  constructor(
    private readonly url: string,
    private readonly playerToken: string,
    private readonly handlers: SocketHandlers,
  ) {}

  connect(): void {
    this.closedByUser = false;
    this.open();
  }

  private open(): void {
    const ws = new WebSocket(this.url);
    this.ws = ws;

    ws.onopen = () => {
      this.reconnectAttempt = 0;
      this.send("hello", { playerToken: this.playerToken });
      this.startClockSync();
      this.handlers.onConnectionChange?.(true);
    };

    ws.onmessage = (event) => {
      let env: Envelope;
      try {
        env = JSON.parse(event.data as string);
      } catch {
        return;
      }
      this.dispatch(env);
    };

    ws.onclose = () => {
      this.stopClockSync();
      this.handlers.onConnectionChange?.(false);
      if (!this.closedByUser) this.scheduleReconnect();
    };

    ws.onerror = () => {
      ws.close();
    };
  }

  private scheduleReconnect(): void {
    const base = RECONNECT_DELAYS[Math.min(this.reconnectAttempt, RECONNECT_DELAYS.length - 1)];
    const jitter = base * 0.2 * (Math.random() * 2 - 1);
    const delay = Math.max(0, base + jitter);
    this.reconnectAttempt++;
    this.reconnectTimer = setTimeout(() => this.open(), delay);
  }

  close(): void {
    this.closedByUser = true;
    this.stopClockSync();
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.ws?.close();
  }

  send(type: string, payload: unknown): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, payload }));
    }
  }

  private dispatch(env: Envelope): void {
    switch (env.type) {
      case "pong": {
        const p = env.payload as { c0: number; s: number };
        this.clock.addSample(computeSample(p.c0, p.s, Date.now()));
        break;
      }
      case "state":
        this.handlers.onState?.(env.payload as StatePayload);
        break;
      case "track_preload":
        this.handlers.onTrackPreload?.(env.payload as TrackPreloadPayload);
        break;
      case "track_prepare":
        this.handlers.onTrackPrepare?.(env.payload as TrackPreparePayload);
        break;
      case "track_start":
        this.handlers.onTrackStart?.(env.payload as TrackStartPayload);
        break;
      case "track_stop":
        this.handlers.onTrackStop?.(env.payload as TrackStopPayload);
        break;
      case "reveal":
        this.handlers.onReveal?.(env.payload as RevealPayload);
        break;
      case "error":
        this.handlers.onError?.(env.payload as ErrorPayload);
        break;
      case "kicked":
        this.handlers.onKicked?.((env.payload as { reason: string }).reason);
        break;
    }
  }

  private ping(): void {
    const c0 = Date.now();
    this.send("ping", { c0 });
  }

  private startClockSync(): void {
    this.stopClockSync();
    // 7 samples 120ms apart on connect (§10.3).
    for (let i = 0; i < 7; i++) {
      setTimeout(() => this.ping(), i * 120);
    }
    this.resyncTimer = setInterval(() => {
      for (let i = 0; i < 3; i++) {
        setTimeout(() => this.ping(), i * 120);
      }
    }, 30_000);
  }

  private stopClockSync(): void {
    if (this.resyncTimer) clearInterval(this.resyncTimer);
    this.resyncTimer = null;
  }

  ready(prepareId: string): void {
    this.send("ready", { prepareId });
  }
  updateSettings(settings: { targetCards?: number; startTokens?: number; enableSongGuess?: boolean }): void {
    this.send("update_settings", settings);
  }
  startGame(): void {
    this.send("start_game", {});
  }
  placeCard(slotIndex: number, titleGuess?: string, artistGuess?: string): void {
    this.send("place_card", { slotIndex, titleGuess, artistGuess });
  }
  previewPlacement(slotIndex: number): void {
    this.send("place_preview", { slotIndex });
  }
  challenge(slotIndex: number): void {
    this.send("challenge", { slotIndex });
  }
  previewChallenge(slotIndex: number): void {
    this.send("challenge_preview", { slotIndex });
  }
  passChallenge(): void {
    this.send("pass_challenge", {});
  }
  skipTrack(): void {
    this.send("skip_track", {});
  }
  kickPlayer(playerId: string): void {
    this.send("kick_player", { playerId });
  }
  adjustTokens(playerId: string, delta: number): void {
    this.send("adjust_tokens", { playerId, delta });
  }
  endGame(): void {
    this.send("end_game", {});
  }
  playAgain(): void {
    this.send("play_again", {});
  }
  leave(): void {
    this.send("leave", {});
  }
}

export type { ClockSample };
