// Where a looping track started at a shared server-clock instant should be.
// Clients use this once, when playback starts (or a late client joins); after
// that each plays on uncorrected until the server stops the track.

/**
 * The position, in seconds, a looping track started at startAtServerMs should
 * be at. Negative before the start instant.
 */
export function expectedPosition(serverNowMs: number, startAtServerMs: number, durationSec: number): number {
  const elapsed = (serverNowMs - startAtServerMs) / 1000;
  if (elapsed < 0 || !(durationSec > 0)) return elapsed;
  return elapsed % durationSec;
}
