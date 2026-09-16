import { useTranslation } from "react-i18next";
import { Coins } from "lucide-react";
import type { GuessFields, PlayerView, SongGuess } from "../ws/protocol";
import { Avatar } from "./ui";

interface Props {
  player: PlayerView;
  fields: GuessFields;
  guess: SongGuess | null;
}

/** Read-only, live view of the active player's song guess for everyone else. */
export default function LiveGuess({ player, fields, guess }: Props) {
  const { t } = useTranslation();
  const rows: Array<{ key: keyof SongGuess; label: string }> = [
    { key: "title" as const, label: t("board.guessTitleLabel") },
    { key: "artist" as const, label: t("board.guessArtistLabel") },
    { key: "album" as const, label: t("board.guessAlbumLabel") },
    { key: "year" as const, label: t("board.guessYearLabel") },
  ].filter((row) => fields[row.key]);
  const empty = !guess || rows.every((row) => !guess[row.key].trim());

  return (
    <div className="surface flex max-w-full flex-wrap items-center gap-x-4 gap-y-1.5 rounded-2xl px-3.5 py-2">
      <span className="flex min-w-0 items-center gap-2">
        <Coins size={14} className="shrink-0 text-gold" />
        <Avatar name={player.name} colour={player.colour} size={20} />
        <span className="truncate text-sm font-semibold">{t("board.liveGuessBy", { name: player.name })}</span>
      </span>
      {empty ? (
        <span className="flex items-center gap-2 text-sm text-fg/40">
          <TypingDots />
          {t("board.liveGuessEmpty")}
        </span>
      ) : (
        <dl className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1">
          {rows.map(({ key, label }) => (
            <div key={key} className="flex min-w-0 items-baseline gap-1.5">
              <dt className="text-[11px] font-medium text-fg/45">{label}</dt>
              <dd className={`max-w-[16rem] truncate text-sm font-medium ${guess![key] ? "text-fg" : "text-fg/25"}`}>{guess![key] || "—"}</dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  );
}

function TypingDots() {
  return (
    <span className="flex gap-1" aria-hidden>
      {[0, 1, 2].map((i) => (
        <span key={i} className="h-1.5 w-1.5 animate-glow-pulse rounded-full bg-fg/40" style={{ animationDelay: `${i * 0.2}s` }} />
      ))}
    </span>
  );
}
