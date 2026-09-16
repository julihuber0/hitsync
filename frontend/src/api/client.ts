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
  adminScanCards: () =>
    request<{ added: number; updated: number; removed: number; total: number }>("/api/admin/cards/scan", { method: "POST" }),
  adminGames: () => request<Record<string, unknown>>("/api/admin/games"),
  adminForceEndGame: (gameId: string) => request<{ ok: boolean }>(`/api/admin/games/${encodeURIComponent(gameId)}`, { method: "DELETE" }),
};
