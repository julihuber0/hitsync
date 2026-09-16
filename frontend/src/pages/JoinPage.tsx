import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams, Link } from "react-router-dom";
import { api, ApiError, type GamePreview } from "../api/client";
import { saveIdentity } from "../api/identity";
import { audioPlayer } from "../audio/instance";
import { ArrowRight, Users } from "lucide-react";
import { Aurora, Logo } from "../components/ui";

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
    <div className="relative flex min-h-screen items-center justify-center px-4">
      <Aurora />
      <form onSubmit={submit} className="surface flex w-full max-w-sm animate-fade-in flex-col items-center gap-6 p-8 sm:p-10">
        <Logo size="lg" />
        <div className="flex flex-col items-center gap-2 text-center">
          <h1 className="text-xl font-semibold tracking-tight">{t("join.hostedBy", { name: preview.hostName })}</h1>
          <span className="chip px-2.5 py-1 text-xs">
            <Users size={13} />
            {t("join.playerCount", { count: preview.playerCount, max: preview.maxPlayers })}
          </span>
        </div>
        <div className="flex w-full flex-col gap-2">
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("join.namePlaceholder")}
            className="input"
            maxLength={20}
          />
          {error && <p className="text-center text-sm text-danger">{error}</p>}
        </div>
        <button type="submit" disabled={busy} className="btn btn-primary btn-lg w-full">
          {t("join.submit")}
          <ArrowRight size={17} />
        </button>
      </form>
    </div>
  );
}

function ProblemScreen({ message }: { message: string }) {
  const { t } = useTranslation();
  return (
    <div className="relative flex min-h-screen items-center justify-center px-4">
      <Aurora />
      <div className="surface flex w-full max-w-sm animate-fade-in flex-col items-center gap-6 p-8 text-center sm:p-10">
        <Logo size="lg" />
        <p className="text-fg/80">{message}</p>
        <Link to="/" className="btn btn-secondary btn-md">
          {t("join.goHome")}
        </Link>
      </div>
    </div>
  );
}
