import { SyncedAudioPlayer } from "./player";

// A single player per tab — there is only ever one active turn.
export const audioPlayer = new SyncedAudioPlayer();
