import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Coins } from "lucide-react";
import type { GuessFields, SongGuess } from "../ws/protocol";

// Short enough that other players see the guess as it is typed; leaving a
// field sends it at once.
const SEND_DELAY_MS = 150;

const EMPTY_GUESS: SongGuess = { title: "", artist: "", album: "", year: "" };

interface Props {
  fields: GuessFields;
  /** The guess the server already holds, e.g. after a reconnect. */
  initial: SongGuess | null;
  onChange: (guess: SongGuess) => void;
}

/**
 * The active player's guess for the bonus token, with one input per field
 * the host selected. It can be edited until the reveal, where the server
 * checks it.
 */
export default function SongGuessPanel({ fields, initial, onChange }: Props) {
  const { t } = useTranslation();
  const [guess, setGuess] = useState<SongGuess>({ ...EMPTY_GUESS, ...initial });
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const latest = useRef(guess);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const send = () => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = null;
    onChange(latest.current);
  };
  const update = (key: keyof SongGuess, value: string) => {
    const next = { ...latest.current, [key]: value };
    latest.current = next;
    setGuess(next);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(send, SEND_DELAY_MS);
  };
  const flush = () => {
    if (timer.current) send();
  };

  const inputs: Array<{ key: keyof SongGuess; label: string; year?: boolean }> = [
    { key: "title" as const, label: t("board.guessTitleLabel") },
    { key: "artist" as const, label: t("board.guessArtistLabel") },
    { key: "album" as const, label: t("board.guessAlbumLabel") },
    { key: "year" as const, label: t("board.guessYearLabel"), year: true },
  ].filter((input) => fields[input.key]);

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
        {inputs.map(({ key, label, year }) => (
          <label key={key} className="flex flex-col gap-1.5">
            <span className="text-xs font-medium text-fg/55">{label}</span>
            <input
              value={guess[key]}
              maxLength={year ? 4 : 200}
              inputMode={year ? "numeric" : undefined}
              autoComplete="off"
              onChange={(e) => update(key, year ? e.target.value.replace(/\D/g, "") : e.target.value)}
              onBlur={flush}
              className={`input h-11 text-sm ${year ? "tabular-nums tracking-wider" : ""}`}
            />
          </label>
        ))}
      </div>
    </div>
  );
}
