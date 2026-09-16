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

/** How many recent samples the offset is chosen from. */
const SAMPLE_WINDOW = 10;

/** Estimates the server clock from a sliding window of ping/pong samples. */
export class ClockSync {
  private samples: ClockSample[] = [];
  private offsetMs = 0;

  offset(): number {
    return this.offsetMs;
  }

  serverNow(): number {
    return Date.now() + this.offsetMs;
  }

  /**
   * Adds a sample and re-derives the offset from the lowest-RTT sample in
   * the window. The window slides, so a stale best sample eventually gives
   * way to fresh ones if the clocks drift apart.
   */
  addSample(sample: ClockSample): void {
    this.samples.push(sample);
    if (this.samples.length > SAMPLE_WINDOW) this.samples.shift();
    this.offsetMs = pickBestSample(this.samples)!.offsetMs;
  }
}
