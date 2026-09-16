export class ApiError extends Error {
  code: string;
  status: number;
  constructor(code: string, message: string, status: number) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  if (!res.ok) {
    let code = "generic";
    let message = res.statusText;
    try {
      const body = await res.json();
      code = body?.error?.code ?? code;
      message = body?.error?.message ?? message;
    } catch {
      // non-JSON error body; fall back to statusText
    }
    throw new ApiError(code, message, res.status);
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export interface AppConfig {
  minPlayers: number;
  maxPlayers: number;
  defaultTargetCards: number;
  defaultStartTokens: number;
  songGuessAvailable: boolean;
}

export interface PlayerIdentity {
  gameId: string;
  playerId: string;
  playerToken: string;
  inviteCode: string;
}

export interface GamePreview {
  exists: boolean;
  phase: string;
  playerCount: number;
  maxPlayers: number;
  hostName: string;
  joinable: boolean;
}

export const api = {
  accessStatus: () => request<{ authenticated: boolean }>("/api/auth/access"),
  access: (code: string) => request<{ ok: boolean }>("/api/auth/access", { method: "POST", body: JSON.stringify({ code }) }),
  logout: () => request<{ ok: boolean }>("/api/auth/logout", { method: "POST" }),
  config: () => request<AppConfig>("/api/config"),

  createGame: (displayName: string, settings?: { targetCards?: number; startTokens?: number; enableSongGuess?: boolean }) =>
    request<PlayerIdentity>("/api/games", { method: "POST", body: JSON.stringify({ displayName, settings }) }),
  joinGame: (inviteCode: string, displayName: string) =>
    request<PlayerIdentity>("/api/games/join", { method: "POST", body: JSON.stringify({ inviteCode, displayName }) }),
  preview: (inviteCode: string) => request<GamePreview>(`/api/games/${encodeURIComponent(inviteCode)}/preview`),

  adminLogin: (password: string) => request<{ ok: boolean }>("/api/admin/login", { method: "POST", body: JSON.stringify({ password }) }),
  adminLogout: () => request<{ ok: boolean }>("/api/admin/logout", { method: "POST" }),
  adminStats: () => request<Record<string, unknown>>("/api/admin/stats"),
  adminTracks: (params: { q?: string; excluded?: string; page?: number; pageSize?: number }) => {
    const qs = new URLSearchParams();
    if (params.q) qs.set("q", params.q);
    if (params.excluded) qs.set("excluded", params.excluded);
    if (params.page !== undefined) qs.set("page", String(params.page));
    if (params.pageSize !== undefined) qs.set("pageSize", String(params.pageSize));
    return request<Record<string, unknown>>(`/api/admin/tracks?${qs.toString()}`);
  },
  adminArtists: (q: string) => request<Record<string, unknown>>(`/api/admin/artists?q=${encodeURIComponent(q)}`),
  adminAlbums: (q: string) => request<Record<string, unknown>>(`/api/admin/albums?q=${encodeURIComponent(q)}`),
  adminCreateExclusion: (kind: string, refId: string, label: string, reason?: string) =>
    request<{ ok: boolean }>("/api/admin/exclusions", { method: "POST", body: JSON.stringify({ kind, refId, label, reason }) }),
  adminDeleteExclusion: (kind: string, refId: string) =>
    request<{ ok: boolean }>(`/api/admin/exclusions/${kind}/${encodeURIComponent(refId)}`, { method: "DELETE" }),
  adminListExclusions: (kind?: string) => request<Record<string, unknown>>(`/api/admin/exclusions${kind ? `?kind=${kind}` : ""}`),
  adminSetYearOverride: (trackId: string, year: number, note?: string) =>
    request<{ ok: boolean }>(`/api/admin/year-overrides/${encodeURIComponent(trackId)}`, { method: "PUT", body: JSON.stringify({ year, note }) }),
  adminDeleteYearOverride: (trackId: string) =>
    request<{ ok: boolean }>(`/api/admin/year-overrides/${encodeURIComponent(trackId)}`, { method: "DELETE" }),
  adminResyncLibrary: () => request<{ ok: boolean }>("/api/admin/resync-library", { method: "POST" }),
  adminResolveYear: (trackId: string) => request<Record<string, unknown>>(`/api/admin/resolve-year/${encodeURIComponent(trackId)}`, { method: "POST" }),
  adminGames: () => request<Record<string, unknown>>("/api/admin/games"),
  adminForceEndGame: (gameId: string) => request<{ ok: boolean }>(`/api/admin/games/${encodeURIComponent(gameId)}`, { method: "DELETE" }),
};
