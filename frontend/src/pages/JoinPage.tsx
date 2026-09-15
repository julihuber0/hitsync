import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams, Link } from "react-router-dom";
import { api, ApiError, type GamePreview } from "../api/client";
import { saveIdentity } from "../api/identity";
import { audioPlayer } from "../audio/instance";

export default function JoinPage() {
  const { t } = useTranslation();
  const { code = "" } = useParams();
  const navigate = useNavigate();

  const [preview, setPreview] = useState<GamePreview | null>(null);
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void api
      .preview(code)
      .then(setPreview)
      .catch(() => setPreview({ exists: false, phase: "", playerCount: 0, maxPlayers: 0, hostName: "", joinable: false }));
  }, [code]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || busy) return;
    audioPlayer.primeAutoplay();
    setBusy(true);
    setError(null);
    try {
      const identity = await api.joinGame(code, name.trim());
      saveIdentity(identity);
      navigate(`/game/${identity.gameId}`);
    } catch (err) {
      setError(err instanceof ApiError ? t(`errors.${err.code}`, t("errors.generic")) : t("errors.generic"));
    } finally {
      setBusy(false);
    }
  };

  if (!preview) return null;

  if (!preview.exists) {
    return <ProblemScreen message={t("join.notFound")} />;
  }
  if (!preview.joinable && preview.phase !== "LOBBY") {
    return <ProblemScreen message={t("join.inProgress")} />;
  }
  if (!preview.joinable) {
    return <ProblemScreen message={t("join.full")} />;
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-4 animate-fade-in">
      <form onSubmit={submit} className="card-surface shadow-neon w-full max-w-sm p-8 flex flex-col gap-4 items-center">
        <h1 className="text-lg font-semibold">{t("join.hostedBy", { name: preview.hostName })}</h1>
        <p className="text-sm text-neon/60">{t("join.playerCount", { count: preview.playerCount, max: preview.maxPlayers })}</p>
        <input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("join.namePlaceholder")}
          className="neon-focus w-full bg-black/30 border border-border rounded-lg py-2.5 px-3 outline-none focus:border-accent transition-colors"
          maxLength={20}
        />
        {error && <p className="text-danger text-sm">{error}</p>}
        <button
          type="submit"
          disabled={busy}
          className="w-full bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2.5 rounded-lg disabled:opacity-50 disabled:hover:shadow-none"
        >
          {t("join.submit")}
        </button>
      </form>
    </div>
  );
}

function ProblemScreen({ message }: { message: string }) {
  const { t } = useTranslation();
  return (
    <div className="min-h-screen flex items-center justify-center px-4 animate-fade-in">
      <div className="card-surface w-full max-w-sm p-8 flex flex-col gap-4 items-center text-center">
        <p>{message}</p>
        <Link to="/" className="text-accent hover:underline hover:drop-shadow-neon transition-all">
          {t("join.goHome")}
        </Link>
      </div>
    </div>
  );
}
