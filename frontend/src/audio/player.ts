import { evaluateDrift } from "./drift";

const DRIFT_CHECK_INTERVAL_MS = 5000;
const FADE_STEP_MS = 30;

export class SyncedAudioPlayer {
  private audio: HTMLAudioElement | null = null;
  private driftTimer: ReturnType<typeof setInterval> | null = null;
  private startTimeout: ReturnType<typeof setTimeout> | null = null;
  private rafHandle: number | null = null;
  private isCorrecting = false;
  private durationMs = 0;
  private startAtServerMs = 0;
  private volume = SyncedAudioPlayer.loadVolume();
  private muted = SyncedAudioPlayer.loadMuted();
  onAutoplayBlocked: (() => void) | null = null;

  private static loadVolume(): number {
    const raw = localStorage.getItem("hs_volume");
    const v = raw ? Number(raw) : 0.8;
    return Number.isFinite(v) ? Math.min(1, Math.max(0, v)) : 0.8;
  }
  private static loadMuted(): boolean {
    return localStorage.getItem("hs_muted") === "true";
  }

  /** Call once, synchronously inside a user-gesture handler (join button). */
  primeAutoplay(): void {
    try {
      const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
      const ctx = new Ctx();
      const buffer = ctx.createBuffer(1, 1, 22050);
      const source = ctx.createBufferSource();
      source.buffer = buffer;
      source.connect(ctx.destination);
      source.start(0);
    } catch {
      // Best-effort; browsers that don't need priming will just no-op here.
    }
  }

  async prepare(streamUrl: string, durationMs: number): Promise<void> {
    this.teardown();
    this.durationMs = durationMs;

    const audio = new Audio();
    audio.preload = "auto";
    audio.loop = true;
    audio.crossOrigin = "anonymous";
    audio.volume = this.muted ? 0 : this.volume;
    audio.src = streamUrl;
    this.audio = audio;

    await new Promise<void>((resolve) => {
      let resolved = false;
      const done = () => {
        if (resolved) return;
        resolved = true;
        resolve();
      };
      audio.addEventListener("canplaythrough", done, { once: true });
      audio.addEventListener("canplay", () => setTimeout(done, 1500), { once: true });
      audio.addEventListener("error", done, { once: true });
      audio.load();
    });
  }

  start(startAtServerMs: number, durationMs: number, serverNow: () => number): void {
    if (!this.audio) return;
    this.startAtServerMs = startAtServerMs;
    this.durationMs = durationMs;

    const elapsed = serverNow() - startAtServerMs;
    if (elapsed < 0) {
      const waitMs = -elapsed;
      this.startTimeout = setTimeout(() => {
        this.spinToPlay(startAtServerMs, serverNow);
      }, Math.max(0, waitMs - 20));
    } else {
      this.audio.currentTime = ((elapsed % durationMs) + durationMs) % durationMs / 1000;
      void this.audio.play().catch(() => this.onAutoplayBlocked?.());
    }
    this.startDriftCorrection(serverNow);
  }

  private spinToPlay(startAtServerMs: number, serverNow: () => number): void {
    const spin = () => {
      if (serverNow() >= startAtServerMs) {
        if (this.audio) {
          this.audio.currentTime = 0;
          void this.audio.play().catch(() => this.onAutoplayBlocked?.());
        }
        return;
      }
      this.rafHandle = requestAnimationFrame(spin);
    };
    this.rafHandle = requestAnimationFrame(spin);
  }

  private startDriftCorrection(serverNow: () => number): void {
    this.stopDriftCorrection();
    this.driftTimer = setInterval(() => {
      if (!this.audio || this.audio.paused) return;
      const { decision } = evaluateDrift(this.audio.currentTime, serverNow(), this.startAtServerMs, this.durationMs, this.isCorrecting);
      switch (decision.kind) {
        case "none":
          break;
        case "correct":
          this.audio.playbackRate = decision.rate;
          this.isCorrecting = true;
          break;
        case "reset_rate":
          this.audio.playbackRate = 1.0;
          this.isCorrecting = false;
          break;
        case "hard_seek": {
          const expectedSec = evaluateDrift(this.audio.currentTime, serverNow(), this.startAtServerMs, this.durationMs, this.isCorrecting).expectedSec;
          this.audio.currentTime = expectedSec;
          this.audio.playbackRate = 1.0;
          this.isCorrecting = false;
          break;
        }
      }
    }, DRIFT_CHECK_INTERVAL_MS);
  }

  private stopDriftCorrection(): void {
    if (this.driftTimer) clearInterval(this.driftTimer);
    this.driftTimer = null;
    this.isCorrecting = false;
  }

  stop(fadeMs: number): void {
    const audio = this.audio;
    this.stopDriftCorrection();
    if (this.startTimeout) clearTimeout(this.startTimeout);
    if (this.rafHandle !== null) cancelAnimationFrame(this.rafHandle);
    if (!audio) return;

    const startVolume = audio.volume;
    const steps = Math.max(1, Math.floor(fadeMs / FADE_STEP_MS));
    let step = 0;
    const fade = setInterval(() => {
      step++;
      audio.volume = Math.max(0, startVolume * (1 - step / steps));
      if (step >= steps) {
        clearInterval(fade);
        audio.pause();
      }
    }, FADE_STEP_MS);
  }

  retryPlay(): void {
    void this.audio?.play().catch(() => this.onAutoplayBlocked?.());
  }

  setVolume(v: number): void {
    this.volume = Math.min(1, Math.max(0, v));
    localStorage.setItem("hs_volume", String(this.volume));
    if (this.audio && !this.muted) this.audio.volume = this.volume;
  }

  setMuted(muted: boolean): void {
    this.muted = muted;
    localStorage.setItem("hs_muted", String(muted));
    // Muting never touches `paused`, so a muted client stays perfectly in
    // sync and un-muting is instantaneous (§10.4).
    if (this.audio) this.audio.volume = muted ? 0 : this.volume;
  }

  getVolume(): number {
    return this.volume;
  }
  isMuted(): boolean {
    return this.muted;
  }

  teardown(): void {
    this.stopDriftCorrection();
    if (this.startTimeout) clearTimeout(this.startTimeout);
    if (this.rafHandle !== null) cancelAnimationFrame(this.rafHandle);
    if (this.audio) {
      this.audio.pause();
      this.audio.src = "";
    }
    this.audio = null;
  }
}
