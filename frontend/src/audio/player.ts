import { Room, RoomEvent, Track, type RemoteTrack } from "livekit-client";

const FADE_STEP_MS = 30;

// The SFU owns the audio clock. Browsers subscribe to a remote track; they
// never download, seek, or rate-correct an MP3.
export class LiveAudioPlayer {
  private room: Room | null = null;
  private element: HTMLAudioElement | null = null;
  private volume = LiveAudioPlayer.loadVolume();
  private muted = LiveAudioPlayer.loadMuted();
  onAutoplayBlocked: (() => void) | null = null;

  private static loadVolume(): number {
    const raw = localStorage.getItem("hs_volume");
    const v = raw ? Number(raw) : 0.8;
    return Number.isFinite(v) ? Math.min(1, Math.max(0, v)) : 0.8;
  }
  private static loadMuted(): boolean { return localStorage.getItem("hs_muted") === "true"; }

  // Called inside the create/join gesture so remote WebRTC output is allowed
  // without a second interaction in browsers that gate audio playback.
  primeAutoplay(): void {
    try {
      const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
      const ctx = new Ctx();
      const source = ctx.createBufferSource();
      source.buffer = ctx.createBuffer(1, 1, 22050);
      source.connect(ctx.destination);
      source.start();
      void ctx.resume();
    } catch {
      // Best effort; browsers that do not need priming ignore it.
    }
  }

  async prepare(url: string, token: string): Promise<void> {
    this.teardown();
    const room = new Room();
    this.room = room;
    await new Promise<void>((resolve, reject) => {
      let settled = false;
      const done = (error?: Error) => {
        if (settled) return;
        settled = true;
        error ? reject(error) : resolve();
      };
      room.on(RoomEvent.TrackSubscribed, (track: RemoteTrack) => {
        if (track.kind !== Track.Kind.Audio) return;
        const element = track.attach() as HTMLAudioElement;
        element.autoplay = true;
        element.volume = this.muted ? 0 : this.volume;
        element.style.display = "none";
        document.body.appendChild(element);
        this.element = element;
        void element.play().catch(() => this.onAutoplayBlocked?.());
        done();
      });
      room.on(RoomEvent.Disconnected, () => {
        if (!settled) done(new Error("LiveKit disconnected before the audio track arrived"));
      });
      void room.connect(url, token, { autoSubscribe: true })
        .catch((error: unknown) => done(error instanceof Error ? error : new Error("LiveKit connection failed")));
    });
  }

  start(): void { this.retryPlay(); }

  stop(fadeMs: number): void {
    const element = this.element;
    if (!element) {
      this.teardown();
      return;
    }
    const startVolume = element.volume;
    const steps = Math.max(1, Math.floor(fadeMs / FADE_STEP_MS));
    let step = 0;
    const fade = setInterval(() => {
      step++;
      element.volume = Math.max(0, startVolume * (1 - step / steps));
      if (step >= steps) {
        clearInterval(fade);
        this.teardown();
      }
    }, FADE_STEP_MS);
  }

  retryPlay(): void { void this.element?.play().catch(() => this.onAutoplayBlocked?.()); }
  setVolume(v: number): void {
    this.volume = Math.min(1, Math.max(0, v));
    localStorage.setItem("hs_volume", String(this.volume));
    if (this.element && !this.muted) this.element.volume = this.volume;
  }
  setMuted(muted: boolean): void {
    this.muted = muted;
    localStorage.setItem("hs_muted", String(muted));
    if (this.element) this.element.volume = muted ? 0 : this.volume;
  }
  getVolume(): number { return this.volume; }
  isMuted(): boolean { return this.muted; }

  teardown(): void {
    if (this.element) {
      this.element.pause();
      this.element.remove();
    }
    this.element = null;
    this.room?.disconnect();
    this.room = null;
  }
}
