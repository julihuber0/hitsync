export interface Envelope {
  type: string;
  payload: unknown;
}

export interface CardView {
  trackId: string;
  title: string;
  artist: string;
  album: string;
  year: number;
}

/** What the active player must name for the song guess bonus token. */
export interface GuessFields {
  title: boolean;
  artist: boolean;
  album: boolean;
  year: boolean;
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
  stealWindowOpen: boolean;
  stealClaims: string[];
  stealsPlaced: string[];
  /** The active player's current song guess, visible to everyone. */
  songGuess: SongGuess | null;
}

export interface StatePayload {
  gameId: string;
  inviteCode: string;
  phase: "LOBBY" | "PREPARING" | "PLACING" | "CHALLENGING" | "REVEALING" | "GAME_OVER";
  settings: { targetCards: number; startTokens: number; maxTokens: number; guessFields: GuessFields };
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

/** A song guess as typed; fields the game doesn't ask for are empty. */
export interface SongGuess {
  title: string;
  artist: string;
  album: string;
  year: string;
}

export interface TrackPreloadPayload {
  trackId: string;
  mediaUrl: string;
}

export interface TrackPreparePayload {
  prepareId: string;
  trackId: string;
  mediaUrl: string;
  durationMs: number;
}

export interface TrackStartPayload {
  prepareId: string;
  startAtServerMs: number;
}

export interface TrackStopPayload {
  prepareId: string;
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
  songGuessResult?: SongGuess & {
    titleCorrect: boolean;
    artistCorrect: boolean;
    albumCorrect: boolean;
    yearCorrect: boolean;
    correct: boolean;
    awarded: boolean;
  };
  yearSource: string;
}

export interface ErrorPayload {
  code: string;
  message: string;
}
