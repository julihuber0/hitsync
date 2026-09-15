import { Room, RoomEvent, Track, type RemoteTrack } from "livekit-client";

const FADE_STEP_MS = 30;

export type AudioConnectionState = "idle" | "connecting" | "subscribed" | "playing" | "error";

// The SFU owns the audio clock. Browsers subscribe to a remote track; they
// never download, seek, or rate-correct an MP3.
export class LiveAudioPlayer {
  private room: Room | null = null;
  private element: HTMLAudioElement | null = null;
  private prepareID: string | null = null;
  private preparePromise: Promise<void> | null = null;
  private generation = 0;
  private fadeTimer: ReturnType<typeof setInterval> | null = null;
  private volume = LiveAudioPlayer.loadVolume();
  private muted = LiveAudioPlayer.loadMuted();
  onAutoplayBlocked: (() => void) | null = null;
  onConnectionStateChange: ((state: AudioConnectionState) => void) | null = null;

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

  // A prepare ID belongs to one game turn. React's development Strict Mode
  // deliberately runs effects twice; returning the same promise here keeps
  // that second effect from disconnecting the room created by the first.
  prepare(prepareID: string, url: string, token: string): Promise<void> {
    if (this.prepareID === prepareID && this.preparePromise) return this.preparePromise;

    this.teardown();
    const generation = this.generation;
    const room = new Room();
    this.room = room;
    this.prepareID = prepareID;
    this.setConnectionState("connecting");

    const promise = new Promise<void>((resolve, reject) => {
      let settled = false;
      const done = (error?: Error) => {
        if (settled) return;
        settled = true;
        error ? reject(error) : resolve();
      };

      room.on(RoomEvent.TrackSubscribed, (track: RemoteTrack) => {
        if (generation !== this.generation || track.kind !== Track.Kind.Audio || this.element) return;
        const element = track.attach() as HTMLAudioElement;
        element.autoplay = true;
        element.volume = this.muted ? 0 : this.volume;
        element.style.display = "none";
        document.body.appendChild(element);
        this.element = element;
        this.setConnectionState("subscribed");
        void element.play()
          .then(() => {
            if (generation === this.generation && this.element === element) this.setConnectionState("playing");
          })
          .catch(() => this.onAutoplayBlocked?.());
        done();
      });

      room.on(RoomEvent.TrackUnsubscribed, (track: RemoteTrack) => {
        track.detach().forEach((element) => element.remove());
        if (generation === this.generation && this.element) {
          this.element = null;
          this.setConnectionState("connecting");
        }
      });

      room.on(RoomEvent.Disconnected, () => {
        if (generation !== this.generation) return;
        const error = new Error("LiveKit disconnected before the audio track arrived");
        if (!settled) done(error);
        this.removeElement();
        this.room = null;
        this.prepareID = null;
        this.preparePromise = null;
        this.setConnectionState("error");
      });
      void room.connect(url, token, { autoSubscribe: true })
        .catch((error: unknown) => {
          if (generation !== this.generation) return;
          done(error instanceof Error ? error : new Error("LiveKit connection failed"));
          this.removeElement();
          this.room = null;
          this.prepareID = null;
          this.preparePromise = null;
          this.setConnectionState("error");
        });
    });
    this.preparePromise = promise;
    return promise;
  }

  start(): void { this.retryPlay(); }

  stop(fadeMs: number): void {
    const element = this.element;
    if (!element) {
      this.teardown();
      return;
    }
    if (this.fadeTimer) clearInterval(this.fadeTimer);
    const startVolume = element.volume;
    const steps = Math.max(1, Math.floor(fadeMs / FADE_STEP_MS));
    let step = 0;
    this.fadeTimer = setInterval(() => {
      step++;
      element.volume = Math.max(0, startVolume * (1 - step / steps));
      if (step >= steps) {
        if (this.fadeTimer) clearInterval(this.fadeTimer);
        this.fadeTimer = null;
        // A new turn may have prepared a replacement while the previous
        // track was fading. Never tear down that replacement.
        if (this.element === element) this.teardown();
      }
    }, FADE_STEP_MS);
  }

  retryPlay(): void {
    const element = this.element;
    void element?.play()
      .then(() => {
        if (element === this.element) this.setConnectionState("playing");
      })
      .catch(() => this.onAutoplayBlocked?.());
  }
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
    this.generation++;
    if (this.fadeTimer) clearInterval(this.fadeTimer);
    this.fadeTimer = null;
    this.removeElement();
    this.room?.disconnect();
    this.room = null;
    this.prepareID = null;
    this.preparePromise = null;
    this.setConnectionState("idle");
  }

  private removeElement(): void {
    this.element?.pause();
    this.element?.remove();
    this.element = null;
  }

  private setConnectionState(state: AudioConnectionState): void {
    this.onConnectionStateChange?.(state);
  }
}
