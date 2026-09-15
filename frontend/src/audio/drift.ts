// Pure drift-correction math for the synchronised audio player (§10.4).
// Kept free of DOM/timer side effects so it can be unit tested directly.

const SLOW_DOWN_RATE = 0.985;
const SPEED_UP_RATE = 1.015;
const START_CORRECTING_MS = 60;
const STOP_CORRECTING_MS = 25;
const HARD_SEEK_MS = 350;

/** Where in the loop the client should be right now, in seconds. */
export function computeExpectedSec(serverNowMs: number, startAtServerMs: number, durationMs: number): number {
  if (durationMs <= 0) return 0;
  const elapsed = (((serverNowMs - startAtServerMs) % durationMs) + durationMs) % durationMs;
  return elapsed / 1000;
}

/**
 * Signed drift in ms, normalised to the shortest distance around the loop
 * boundary (e.g. currentTime just before the end and expected just after 0
 * are actually only a few ms apart, not nearly a full duration apart).
 */
export function computeDriftMs(currentTimeSec: number, expectedSec: number, durationMs: number): number {
  if (durationMs <= 0) return 0;
  const halfDurationMs = durationMs / 2;
  const rawMs = (currentTimeSec - expectedSec) * 1000;
  return (((rawMs + halfDurationMs) % durationMs) + durationMs) % durationMs - halfDurationMs;
}

export type DriftDecision =
  | { kind: "none" }
  | { kind: "correct"; rate: number }
  | { kind: "reset_rate" }
  | { kind: "hard_seek" };

/**
 * Decides the correction action for a given drift, with hysteresis: once
 * correcting, keep correcting until drift falls under the tighter
 * STOP_CORRECTING_MS threshold rather than the wider START_CORRECTING_MS one
 * (§10.4).
 */
export function decideDriftAction(driftMs: number, isCurrentlyCorrecting: boolean): DriftDecision {
  const abs = Math.abs(driftMs);
  if (abs > HARD_SEEK_MS) return { kind: "hard_seek" };
  if (isCurrentlyCorrecting) {
    if (abs < STOP_CORRECTING_MS) return { kind: "reset_rate" };
    return { kind: "correct", rate: driftMs > 0 ? SLOW_DOWN_RATE : SPEED_UP_RATE };
  }
  if (abs >= START_CORRECTING_MS) return { kind: "correct", rate: driftMs > 0 ? SLOW_DOWN_RATE : SPEED_UP_RATE };
  return { kind: "none" };
}

export function evaluateDrift(
  currentTimeSec: number,
  serverNowMs: number,
  startAtServerMs: number,
  durationMs: number,
  isCurrentlyCorrecting: boolean,
): { decision: DriftDecision; expectedSec: number; driftMs: number } {
  const expectedSec = computeExpectedSec(serverNowMs, startAtServerMs, durationMs);
  const driftMs = computeDriftMs(currentTimeSec, expectedSec, durationMs);
  return { decision: decideDriftAction(driftMs, isCurrentlyCorrecting), expectedSec, driftMs };
}
