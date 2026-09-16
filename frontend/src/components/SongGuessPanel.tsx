import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Coins } from "lucide-react";
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

  return (
    <div className="w-full max-w-xl rounded-2xl border border-white/[0.07] bg-black/20 p-4 sm:p-5">
      <div className="mb-4 flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-gold/25 bg-gold/10 text-gold">
          <Coins size={17} />
        </span>
        <div>
          <h3 className="text-sm font-semibold">{t("board.guessTitle")}</h3>
          <p className="mt-0.5 text-xs leading-relaxed text-fg/50">{t("board.guessHint")}</p>
        </div>
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-fg/55">{t("board.guessTitleLabel")}</span>
          <input
            value={title}
            maxLength={200}
            autoComplete="off"
            onChange={(e) => update({ title: e.target.value, artist })}
            onBlur={flush}
            className="input h-11 text-sm"
          />
        </label>
        <label className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-fg/55">{t("board.guessArtistLabel")}</span>
          <input
            value={artist}
            maxLength={200}
            autoComplete="off"
            onChange={(e) => update({ title, artist: e.target.value })}
            onBlur={flush}
            className="input h-11 text-sm"
          />
        </label>
      </div>
    </div>
  );
}
