import { SyncedAudioPlayer } from "./player";

// A single synchronised player per browser tab — there is only ever one
// active turn's audio at a time.
export const audioPlayer = new SyncedAudioPlayer();
