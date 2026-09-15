export interface ClockSample {
  rttMs: number;
  offsetMs: number;
}

/**
 * Picks the offset from the sample with the lowest RTT — min-RTT filtering
 * is far more robust than averaging (§10.3).
 */
export function pickBestSample(samples: ClockSample[]): ClockSample | null {
  if (samples.length === 0) return null;
  return samples.reduce((best, s) => (s.rttMs < best.rttMs ? s : best));
}

/**
 * Computes one clock-sync sample from a ping/pong round trip.
 * c0 = client send time, s = server time in the pong, c1 = client receive time.
 */
export function computeSample(c0: number, s: number, c1: number): ClockSample {
  const rttMs = c1 - c0;
  const offsetMs = s - (c0 + rttMs / 2);
  return { rttMs, offsetMs };
}

export class ClockSync {
  private offsetMs = 0;
  private lastUpdated = 0;

  offset(): number {
    return this.offsetMs;
  }

  serverNow(): number {
    return Date.now() + this.offsetMs;
  }

  /** Applies a new sample if it is better than the stored one (§10.3). */
  applySamples(samples: ClockSample[]): void {
    const best = pickBestSample(samples);
    if (!best) return;
    const now = Date.now();
    const stale = now - this.lastUpdated > 2 * 60 * 1000;
    if (this.lastUpdated === 0 || stale || samples.length > 1) {
      this.offsetMs = best.offsetMs;
      this.lastUpdated = now;
    }
  }

  applySampleIfBetter(sample: ClockSample, previousBestRtt: number): void {
    if (sample.rttMs < previousBestRtt || Date.now() - this.lastUpdated > 2 * 60 * 1000) {
      this.offsetMs = sample.offsetMs;
      this.lastUpdated = Date.now();
    }
  }
}
