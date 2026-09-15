export interface Envelope {
  type: string;
  payload: unknown;
}

export interface CardView {
  trackId: string;
  title: string;
  artist: string;
  year: number;
}

export interface PlayerView {
  id: string;
  name: string;
  colour: string;
  connected: boolean;
  isHost: boolean;
  tokens: number;
  timeline: CardView[];
  pendingChallengeSlot: number | null;
  pendingChallengePreviewSlot: number | null;
}

export interface CurrentTurnView {
  activePlacementSubmitted: boolean;
  activePlacementSlot: number | null;
  activePlacementPreviewSlot: number | null;
  challengeSlotsTaken: number[];
  hasPassed: string[];
}

export interface StatePayload {
  gameId: string;
  inviteCode: string;
  phase: "LOBBY" | "PREPARING" | "PLACING" | "CHALLENGING" | "REVEALING" | "GAME_OVER";
  settings: { targetCards: number; startTokens: number; enableSongGuess: boolean };
  hostId: string;
  youId: string;
  activePlayerId: string;
  turnNumber: number;
  phaseEndsAtServerMs: number | null;
  players: PlayerView[];
  currentTurn: CurrentTurnView | null;
  winnerId: string | null;
  tracksUsed: number;
}

export interface GuessOptions {
  titles: string[];
  artists: string[];
}

export interface TrackPreparePayload {
  prepareId: string;
  trackId: string;
  livekitUrl: string;
  livekitToken: string;
  roomName: string;
  durationMs: number;
  guessOptions?: GuessOptions;
}

export interface TrackStartPayload {
  prepareId: string;
}

export interface TrackStopPayload {
  fadeMs: number;
}

export interface RevealChallengeView {
  playerId: string;
  slot: number;
  correct: boolean;
}

export interface TokenChangeView {
  playerId: string;
  delta: number;
}

export interface RevealPayload {
  card: CardView;
  activePlayerId: string;
  activePlacement: number;
  activeCorrect: boolean;
  winnerPlayerId: string;
  challenges: RevealChallengeView[];
  outcome: "active_correct" | "challenger_correct" | "discarded";
  tokenChanges: TokenChangeView[];
  songGuessResult?: { correct: boolean; awarded: boolean };
  yearSource: string;
}

export interface ErrorPayload {
  code: string;
  message: string;
}
