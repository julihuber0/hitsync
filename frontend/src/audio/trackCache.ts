interface Entry {
  controller: AbortController;
  promise: Promise<string>;
  objectUrl: string | null;
}

/**
 * Downloaded tracks held in memory as object URLs, keyed by track id. Entries
 * live only as long as the player needs them; release() frees the memory.
 */
export class TrackCache {
  private entries = new Map<string, Entry>();

  constructor(private readonly fetchImpl: typeof fetch = (input, init) => fetch(input, init)) {}

  /** Starts, or joins, the download of trackId and resolves to an object URL. */
  load(trackId: string, url: string): Promise<string> {
    const existing = this.entries.get(trackId);
    if (existing) return existing.promise;

    const controller = new AbortController();
    const entry: Entry = { controller, objectUrl: null, promise: Promise.resolve("") };
    entry.promise = this.fetchImpl(url, { credentials: "same-origin", signal: controller.signal })
      .then((res) => {
        if (!res.ok) throw new Error(`media download failed with status ${res.status}`);
        return res.blob();
      })
      .then((blob) => {
        if (this.entries.get(trackId) !== entry) throw new DOMException("track released", "AbortError");
        entry.objectUrl = URL.createObjectURL(blob);
        return entry.objectUrl;
      });
    // Forget a failed download so the next load() retries it.
    entry.promise.catch(() => {
      if (this.entries.get(trackId) === entry) this.entries.delete(trackId);
    });
    this.entries.set(trackId, entry);
    return entry.promise;
  }

  has(trackId: string): boolean {
    return this.entries.has(trackId);
  }

  /** Cancels a pending download or frees a downloaded track. */
  release(trackId: string): void {
    const entry = this.entries.get(trackId);
    if (!entry) return;
    this.entries.delete(trackId);
    entry.controller.abort();
    if (entry.objectUrl) URL.revokeObjectURL(entry.objectUrl);
  }

  clear(): void {
    for (const trackId of [...this.entries.keys()]) this.release(trackId);
  }
}
