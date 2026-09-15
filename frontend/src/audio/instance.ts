import { LiveAudioPlayer } from "./player";

// A single LiveKit subscription per tab — there is only ever one active turn.
export const audioPlayer = new LiveAudioPlayer();
