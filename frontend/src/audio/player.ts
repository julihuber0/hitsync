import { TrackCache } from "./trackCache";
import { expectedPosition } from "./sync";

const FADE_STEP_MS = 30;
// iOS Safari may never report metadata for an element it hasn't been allowed
// to play yet; the downloaded file is playable regardless.
const METADATA_TIMEOUT_MS = 2000;

export type AudioState = "idle" | "loading" | "ready" | "playing" | "error";

interface CurrentTrack {
  prepareId: string;
  trackId: string;
  fallbackDurationSec: number;
  promise: Promise<void>;
  loaded: boolean;
  failed: boolean;
  startAtServerMs: number | null;
}

/**
 * Plays the turn's track from a local, fully downloaded copy. Only the start
 * is synchronised: playback begins at the position implied by a shared start
 * instant on the server clock and then runs untouched until stopped. Holds
 * at most the current track and the next turn's preloaded track in memory.
 */
export class SyncedAudioPlayer {
  // One element for the whole session: once the user has allowed playback on
  // it (iOS), later turns can play without another gesture.
  private readonly element: HTMLAudioElement;
  private readonly cache = new TrackCache();
  private current: CurrentTrack | null = null;
  private preloadTrackId: string | null = null;
  private serverNow: () => number = Date.now;
  private state: AudioState = "idle";
  private startTimer: ReturnType<typeof setTimeout> | null = null;
  private fadeTimer: ReturnType<typeof setInterval> | null = null;
  private volume = SyncedAudioPlayer.loadVolume();
  private muted = SyncedAudioPlayer.loadMuted();
  onAutoplayBlocked: (() => void) | null = null;
  onStateChange: ((state: AudioState) => void) | null = null;

  constructor() {
    this.element = new Audio();
    this.element.loop = true;
    this.element.preload = "auto";
    this.element.volume = this.effectiveVolume();
  }

  private static loadVolume(): number {
    const raw = localStorage.getItem("hs_volume");
    const v = raw ? Number(raw) : 0.8;
    return Number.isFinite(v) ? Math.min(1, Math.max(0, v)) : 0.8;
  }
  private static loadMuted(): boolean { return localStorage.getItem("hs_muted") === "true"; }

  // Called inside the create/join gesture so later playback is allowed
  // without a second interaction in browsers that gate audio.
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

  /** Downloads the next turn's track in the background. */
  preload(trackId: string, url: string): void {
    const previous = this.preloadTrackId;
    if (previous && previous !== trackId && previous !== this.current?.trackId) this.cache.release(previous);
    this.preloadTrackId = trackId;
    // A failure is retried when the turn's track_prepare arrives.
    this.cache.load(trackId, url).catch(() => {});
  }

  /**
   * Makes the turn's track playable, reusing a preloaded copy. Resolves once
   * playback can start. Idempotent per prepareId unless the last attempt failed.
   */
  prepare(prepareId: string, trackId: string, url: string, durationMs: number): Promise<void> {
    if (this.current?.prepareId === prepareId && !this.current.failed) return this.current.promise;

    this.unloadCurrent(trackId);
    if (this.preloadTrackId === trackId) this.preloadTrackId = null;
    const current: CurrentTrack = {
      prepareId, trackId, fallbackDurationSec: durationMs / 1000,
      promise: Promise.resolve(), loaded: false, failed: false, startAtServerMs: null,
    };
    this.current = current;
    this.setState("loading");

    current.promise = this.cache.load(trackId, url)
      .then((objectUrl) => {
        if (this.current !== current) throw new DOMException("superseded", "AbortError");
        return this.loadElement(objectUrl);
      })
      .then(() => {
        if (this.current !== current) throw new DOMException("superseded", "AbortError");
        current.loaded = true;
        if (current.startAtServerMs === null) this.setState("ready");
        else this.beginPlayback();
      })
      .catch((error: unknown) => {
        if (this.current === current) {
          current.failed = true;
          this.setState("error");
        }
        throw error;
      });
    return current.promise;
  }

  /** Starts playback at the position implied by the shared start instant. */
  start(prepareId: string, startAtServerMs: number, serverNow: () => number): void {
    const current = this.current;
    if (!current || current.prepareId !== prepareId) return;
    // A reconnect replays track_start; don't interrupt playback that is
    // already following the same timeline.
    if (current.startAtServerMs === startAtServerMs && this.state === "playing") {
      this.serverNow = serverNow;
      return;
    }
    current.startAtServerMs = startAtServerMs;
    this.serverNow = serverNow;
    // Otherwise prepare() begins playback once the track is loaded.
    if (current.loaded) this.beginPlayback();
  }

  /** Fades out, then frees the track. */
  stop(prepareId: string, fadeMs: number): void {
    const current = this.current;
    if (!current || current.prepareId !== prepareId) return;
    this.clearPlaybackTimers();
    if (this.element.paused) {
      this.unloadCurrent();
      return;
    }
    const startVolume = this.element.volume;
    const steps = Math.max(1, Math.floor(fadeMs / FADE_STEP_MS));
    let step = 0;
    this.fadeTimer = setInterval(() => {
      step++;
      this.element.volume = Math.max(0, startVolume * (1 - step / steps));
      if (step >= steps && this.current === current) this.unloadCurrent();
    }, FADE_STEP_MS);
  }

  /** Retries playback from a user gesture after the browser blocked autoplay. */
  retryPlay(): void {
    if (this.current?.loaded && this.current.startAtServerMs !== null) this.beginPlayback();
  }

  setVolume(v: number): void {
    this.volume = Math.min(1, Math.max(0, v));
    localStorage.setItem("hs_volume", String(this.volume));
    if (!this.fadeTimer) this.element.volume = this.effectiveVolume();
  }
  setMuted(muted: boolean): void {
    this.muted = muted;
    localStorage.setItem("hs_muted", String(muted));
    // Muting never pauses, so a muted client keeps its place in the song.
    if (!this.fadeTimer) this.element.volume = this.effectiveVolume();
  }
  getVolume(): number { return this.volume; }
  isMuted(): boolean { return this.muted; }

  /** Stops playback and frees every downloaded track. */
  teardown(): void {
    this.preloadTrackId = null;
    this.unloadCurrent();
    this.cache.clear();
  }

  private loadElement(src: string): Promise<void> {
    const element = this.element;
    return new Promise((resolve, reject) => {
      const done = (error?: Error) => {
        clearTimeout(timeout);
        element.removeEventListener("loadedmetadata", onLoaded);
        element.removeEventListener("error", onError);
        if (error) reject(error);
        else resolve();
      };
      const onLoaded = () => done();
      const onError = () => done(new Error("the downloaded track could not be decoded"));
      const timeout = setTimeout(() => done(), METADATA_TIMEOUT_MS);
      element.addEventListener("loadedmetadata", onLoaded);
      element.addEventListener("error", onError);
      element.src = src;
      element.load();
    });
  }

  private beginPlayback(): void {
    const current = this.current;
    if (!current || current.startAtServerMs === null) return;
    this.clearPlaybackTimers();
    this.element.volume = this.effectiveVolume();

    const position = this.expectedPosition(current.startAtServerMs);
    if (position < 0) {
      this.element.currentTime = 0;
      this.startTimer = setTimeout(() => this.beginPlayback(), -position * 1000);
      return;
    }
    this.element.currentTime = position;
    this.element.play()
      .then(() => {
        if (this.current === current) this.setState("playing");
      })
      .catch((error: unknown) => {
        if (this.current === current && error instanceof DOMException && error.name === "NotAllowedError") {
          this.onAutoplayBlocked?.();
        }
      });
  }

  private expectedPosition(startAtServerMs: number): number {
    return expectedPosition(this.serverNow(), startAtServerMs, this.durationSec());
  }

  // Prefer the decoded file's own length: every client has the same file, so
  // they all loop at exactly the same point.
  private durationSec(): number {
    const d = this.element.duration;
    return Number.isFinite(d) && d > 0 ? d : this.current?.fallbackDurationSec ?? NaN;
  }

  private effectiveVolume(): number {
    return this.muted ? 0 : this.volume;
  }

  private clearPlaybackTimers(): void {
    if (this.startTimer) clearTimeout(this.startTimer);
    if (this.fadeTimer) clearInterval(this.fadeTimer);
    this.startTimer = null;
    this.fadeTimer = null;
  }

  /** Stops the current track and frees it, unless keepTrackId still needs it. */
  private unloadCurrent(keepTrackId?: string): void {
    this.clearPlaybackTimers();
    const current = this.current;
    this.current = null;
    if (current) {
      this.element.pause();
      this.element.removeAttribute("src");
      this.element.load();
      if (current.trackId !== keepTrackId && current.trackId !== this.preloadTrackId) this.cache.release(current.trackId);
    }
    this.element.volume = this.effectiveVolume();
    this.setState("idle");
  }

  private setState(state: AudioState): void {
    if (this.state === state) return;
    this.state = state;
    this.onStateChange?.(state);
  }
}
