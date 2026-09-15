import { describe, it, expect } from "vitest";
import { computeDriftMs, computeExpectedSec, decideDriftAction, evaluateDrift } from "./drift";

describe("decideDriftAction", () => {
  it("does nothing for drift under 60ms when not correcting", () => {
    expect(decideDriftAction(0, false)).toEqual({ kind: "none" });
    expect(decideDriftAction(59, false)).toEqual({ kind: "none" });
    expect(decideDriftAction(-59, false)).toEqual({ kind: "none" });
  });

  it("starts slowing down when ahead by 60-350ms", () => {
    expect(decideDriftAction(100, false)).toEqual({ kind: "correct", rate: 0.985 });
  });

  it("starts speeding up when behind by 60-350ms", () => {
    expect(decideDriftAction(-100, false)).toEqual({ kind: "correct", rate: 1.015 });
  });

  it("hard seeks beyond 350ms in either direction", () => {
    expect(decideDriftAction(351, false)).toEqual({ kind: "hard_seek" });
    expect(decideDriftAction(-500, false)).toEqual({ kind: "hard_seek" });
  });

  it("keeps correcting between 25 and 60ms once already correcting", () => {
    expect(decideDriftAction(40, true)).toEqual({ kind: "correct", rate: 0.985 });
    expect(decideDriftAction(-40, true)).toEqual({ kind: "correct", rate: 1.015 });
  });

  it("resets to normal speed once drift falls under 25ms while correcting", () => {
    expect(decideDriftAction(10, true)).toEqual({ kind: "reset_rate" });
    expect(decideDriftAction(-24, true)).toEqual({ kind: "reset_rate" });
  });

  it("does not reset merely for being under 60ms if not yet correcting", () => {
    expect(decideDriftAction(30, false)).toEqual({ kind: "none" });
  });
});

describe("computeExpectedSec", () => {
  it("computes position within the loop", () => {
    expect(computeExpectedSec(1500, 1000, 10000)).toBeCloseTo(0.5);
  });

  it("wraps around the loop boundary", () => {
    // 12.5s elapsed in a 10s loop -> 2.5s into the second lap.
    expect(computeExpectedSec(13500, 1000, 10000)).toBeCloseTo(2.5);
  });
});

describe("computeDriftMs across a loop boundary", () => {
  it("treats a wrap as a small drift, not a nearly-full-duration one", () => {
    // 200s track; client is 20ms behind the wrap point.
    const durationMs = 200_000;
    const currentTimeSec = 199.99;
    const expectedSec = 0.01;
    const drift = computeDriftMs(currentTimeSec, expectedSec, durationMs);
    expect(drift).toBeCloseTo(-20, 0);
  });

  it("treats the symmetric case (client just ahead of the wrap) as small too", () => {
    const durationMs = 200_000;
    const currentTimeSec = 0.02;
    const expectedSec = 199.98;
    const drift = computeDriftMs(currentTimeSec, expectedSec, durationMs);
    expect(drift).toBeCloseTo(40, 0);
  });
});

describe("evaluateDrift", () => {
  it("combines expected position and drift into a decision", () => {
    const result = evaluateDrift(0.5, 1500, 1000, 10000, false);
    expect(result.expectedSec).toBeCloseTo(0.5);
    expect(result.driftMs).toBeCloseTo(0);
    expect(result.decision).toEqual({ kind: "none" });
  });
});
