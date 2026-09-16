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
    <div className="w-full rounded-2xl border border-white/[0.07] bg-black/20 p-3">
      <div className="mb-2 flex flex-wrap items-baseline gap-x-2.5 gap-y-0.5 px-0.5">
        <h3 className="flex items-center gap-1.5 text-sm font-semibold">
          <Coins size={14} className="shrink-0 self-center text-gold" />
          {t("board.guessTitle")}
        </h3>
        <p className="text-xs text-fg/45">{t("board.guessHint")}</p>
      </div>
      <div className={`grid grid-cols-2 gap-2 ${COLUMNS[inputs.length] ?? ""}`}>
        {inputs.map(({ key, label, year }) => (
          <label key={key} className="flex min-w-0 flex-col gap-1">
            <span className="px-0.5 text-[11px] font-medium text-fg/50">{label}</span>
            <input
              value={guess[key]}
              maxLength={year ? 4 : 200}
              inputMode={year ? "numeric" : undefined}
              autoComplete="off"
              onChange={(e) => update(key, year ? e.target.value.replace(/\D/g, "") : e.target.value)}
              onBlur={flush}
              className={`input h-10 px-3 text-sm ${year ? "tabular-nums tracking-wider" : ""}`}
            />
          </label>
        ))}
      </div>
    </div>
  );
}

// Static class names so Tailwind generates them.
const COLUMNS: Record<number, string> = {
  1: "lg:grid-cols-1",
  2: "lg:grid-cols-2",
  3: "lg:grid-cols-3",
  4: "lg:grid-cols-4",
};
