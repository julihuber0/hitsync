import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { SongGuess } from "../ws/protocol";

// Typing sends the guess after a short pause; leaving a field sends it at once.
const SEND_DELAY_MS = 400;

interface Props {
  /** The guess the server already holds, e.g. after a reconnect. */
  initial: SongGuess | null;
  onChange: (title: string, artist: string) => void;
}

/**
 * Free-text "Name that tune" guess for the active player. It can be edited
 * until the reveal, where the server checks it: title and artist both right
 * earns a token.
 */
export default function SongGuessPanel({ initial, onChange }: Props) {
  const { t } = useTranslation();
  const [title, setTitle] = useState(initial?.title ?? "");
  const [artist, setArtist] = useState(initial?.artist ?? "");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const latest = useRef({ title, artist });

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const send = () => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = null;
    onChange(latest.current.title, latest.current.artist);
  };
  const update = (next: { title: string; artist: string }) => {
    latest.current = next;
    setTitle(next.title);
    setArtist(next.artist);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(send, SEND_DELAY_MS);
  };
  const flush = () => {
    if (timer.current) send();
  };

  const inputClass =
    "neon-focus w-full bg-black/30 border border-border rounded-lg py-2 px-3 text-sm outline-none focus:border-accent transition-colors";

  return (
    <div className="card-surface w-full max-w-md p-4 flex flex-col gap-3">
      <div>
        <h3 className="text-sm font-semibold text-neon/70">{t("board.guessTitle")}</h3>
        <p className="text-xs text-neon/45">{t("board.guessHint")}</p>
      </div>
      <label className="flex flex-col gap-1 text-xs text-neon/50">
        {t("board.guessTitleLabel")}
        <input
          value={title}
          maxLength={200}
          autoComplete="off"
          onChange={(e) => update({ title: e.target.value, artist })}
          onBlur={flush}
          className={inputClass}
        />
      </label>
      <label className="flex flex-col gap-1 text-xs text-neon/50">
        {t("board.guessArtistLabel")}
        <input
          value={artist}
          maxLength={200}
          autoComplete="off"
          onChange={(e) => update({ title, artist: e.target.value })}
          onBlur={flush}
          className={inputClass}
        />
      </label>
    </div>
  );
}
