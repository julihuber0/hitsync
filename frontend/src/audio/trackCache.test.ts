import { describe, it, expect, vi, beforeEach } from "vitest";
import { TrackCache } from "./trackCache";

function okResponse(): Response {
  // A string body: jsdom's Blob can't be streamed by Node's Response.
  return new Response("mp3", { status: 200, headers: { "Content-Type": "audio/mpeg" } });
}

describe("TrackCache", () => {
  beforeEach(() => {
    let n = 0;
    URL.createObjectURL = vi.fn(() => `blob:track-${++n}`);
    URL.revokeObjectURL = vi.fn();
  });

  it("downloads a track once for concurrent loads", async () => {
    const fetchImpl = vi.fn(async () => okResponse());
    const cache = new TrackCache(fetchImpl);
    const [a, b] = await Promise.all([cache.load("t1", "/api/media/x"), cache.load("t1", "/api/media/y")]);
    expect(a).toBe(b);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it("frees the object URL on release", async () => {
    const cache = new TrackCache(async () => okResponse());
    const url = await cache.load("t1", "/api/media/x");
    cache.release("t1");
    expect(URL.revokeObjectURL).toHaveBeenCalledWith(url);
    expect(cache.has("t1")).toBe(false);
  });

  it("aborts a download released before it finishes", async () => {
    let signal: AbortSignal | undefined;
    const cache = new TrackCache((_input, init) => {
      signal = init?.signal ?? undefined;
      return new Promise<Response>(() => {});
    });
    void cache.load("t1", "/api/media/x").catch(() => {});
    cache.release("t1");
    expect(signal?.aborted).toBe(true);
  });

  it("retries after a failed download", async () => {
    const fetchImpl = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 502 }))
      .mockResolvedValueOnce(okResponse());
    const cache = new TrackCache(fetchImpl);
    await expect(cache.load("t1", "/api/media/x")).rejects.toThrow();
    await expect(cache.load("t1", "/api/media/x")).resolves.toMatch(/^blob:/);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });
});
