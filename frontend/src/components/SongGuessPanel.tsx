import { useTranslation } from "react-i18next";
import type { GuessOptions } from "../ws/protocol";

interface Props {
  options: GuessOptions;
  titleGuess: string | null;
  artistGuess: string | null;
  onSelectTitle: (title: string) => void;
  onSelectArtist: (artist: string) => void;
}

export default function SongGuessPanel({ options, titleGuess, artistGuess, onSelectTitle, onSelectArtist }: Props) {
  const { t } = useTranslation();
  return (
    <div className="card-surface p-4 flex flex-col gap-3">
      <h3 className="text-sm font-semibold text-neon/70">{t("board.guessTitle")}</h3>
      <GuessGroup label={t("board.guessTitleLabel")} choices={options.titles} selected={titleGuess} onSelect={onSelectTitle} />
      <GuessGroup label={t("board.guessArtistLabel")} choices={options.artists} selected={artistGuess} onSelect={onSelectArtist} />
    </div>
  );
}

function GuessGroup({
  label,
  choices,
  selected,
  onSelect,
}: {
  label: string;
  choices: string[];
  selected: string | null;
  onSelect: (v: string) => void;
}) {
  return (
    <div>
      <div className="text-xs text-neon/50 mb-1.5">{label}</div>
      <div className="grid grid-cols-2 gap-1.5">
        {choices.map((choice) => (
          <button
            key={choice}
            type="button"
            onClick={() => onSelect(choice)}
            className={`text-xs px-2 py-1.5 rounded-md border transition-all duration-150 truncate ${
              selected === choice
                ? "bg-accent/30 border-accent shadow-neon-sm"
                : "bg-white/5 border-white/10 hover:border-accent/40"
            }`}
            title={choice}
          >
            {choice}
          </button>
        ))}
      </div>
    </div>
  );
}
