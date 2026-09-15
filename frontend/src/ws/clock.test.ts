import { describe, it, expect } from "vitest";
import { pickBestSample, computeSample } from "./clock";

describe("pickBestSample", () => {
  it("picks the sample with the lowest RTT", () => {
    const samples = [
      { rttMs: 80, offsetMs: 12 },
      { rttMs: 20, offsetMs: 5 },
      { rttMs: 45, offsetMs: 8 },
    ];
    expect(pickBestSample(samples)).toEqual({ rttMs: 20, offsetMs: 5 });
  });

  it("returns null for an empty sample set", () => {
    expect(pickBestSample([])).toBeNull();
  });

  it("handles a single sample", () => {
    const samples = [{ rttMs: 30, offsetMs: -4 }];
    expect(pickBestSample(samples)).toEqual(samples[0]);
  });
});

describe("computeSample", () => {
  it("computes RTT and offset from a round trip", () => {
    // Server clock is 100ms ahead; 40ms RTT split evenly.
    const c0 = 1000;
    const s = 1120; // server time when pong sent, server is +100ms and 20ms elapsed
    const c1 = 1040;
    const sample = computeSample(c0, s, c1);
    expect(sample.rttMs).toBe(40);
    expect(sample.offsetMs).toBe(100);
  });
});
