import { describe, it, expect } from "vitest";
import { expectedPosition } from "./sync";

describe("expectedPosition", () => {
  it("is negative before the start instant", () => {
    expect(expectedPosition(1000, 1500, 180)).toBe(-0.5);
  });

  it("counts seconds since the start", () => {
    expect(expectedPosition(13_500, 1500, 180)).toBe(12);
  });

  it("wraps around for a looping track", () => {
    expect(expectedPosition(1500 + 185_000, 1500, 180)).toBeCloseTo(5);
  });

  it("does not wrap when the duration is unknown", () => {
    expect(expectedPosition(1500 + 185_000, 1500, NaN)).toBe(185);
  });
});
