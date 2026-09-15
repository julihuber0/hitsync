import type { PlayerIdentity } from "./client";

const RECENT_GAMES_KEY = "hs_recent_games";

export function playerTokenKey(gameId: string): string {
  return `hs_player_${gameId}`;
}

export function saveIdentity(identity: PlayerIdentity): void {
  try {
    localStorage.setItem(playerTokenKey(identity.gameId), identity.playerToken);
    const recent = loadRecentGames().filter((g) => g.gameId !== identity.gameId);
    recent.unshift({ gameId: identity.gameId, inviteCode: identity.inviteCode, savedAt: Date.now() });
    localStorage.setItem(RECENT_GAMES_KEY, JSON.stringify(recent.slice(0, 5)));
  } catch {
    // localStorage unavailable (private mode, etc.) — non-fatal.
  }
}

export function loadPlayerToken(gameId: string): string | null {
  try {
    return localStorage.getItem(playerTokenKey(gameId));
  } catch {
    return null;
  }
}

export interface RecentGame {
  gameId: string;
  inviteCode: string;
  savedAt: number;
}

export function loadRecentGames(): RecentGame[] {
  try {
    const raw = localStorage.getItem(RECENT_GAMES_KEY);
    return raw ? (JSON.parse(raw) as RecentGame[]) : [];
  } catch {
    return [];
  }
}
