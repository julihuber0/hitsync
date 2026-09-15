import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { api, ApiError } from "../api/client";
import { saveIdentity, loadRecentGames, loadPlayerToken } from "../api/identity";
import { useAppStore } from "../store/appStore";
import LanguageToggle from "../components/LanguageToggle";
import { audioPlayer } from "../audio/instance";

export default function HomePage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const logout = useAppStore((s) => s.logout);

  const [hostName, setHostName] = useState("");
  const [joinName, setJoinName] = useState("");
  const [joinCode, setJoinCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const recentGames = loadRecentGames().filter((g) => loadPlayerToken(g.gameId));

  const handleError = (e: unknown) => {
    if (e instanceof ApiError) {
      setError(t(`errors.${e.code}`, t("errors.generic")));
    } else {
      setError(t("errors.generic"));
    }
  };

  const createGame = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!hostName.trim() || busy) return;
    audioPlayer.primeAutoplay();
    setBusy(true);
    setError(null);
    try {
      const identity = await api.createGame(hostName.trim());
      saveIdentity(identity);
      navigate(`/game/${identity.gameId}`);
    } catch (err) {
      handleError(err);
    } finally {
      setBusy(false);
    }
  };

  const joinGame = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!joinName.trim() || !joinCode.trim() || busy) return;
    audioPlayer.primeAutoplay();
    setBusy(true);
    setError(null);
    try {
      const identity = await api.joinGame(joinCode.trim(), joinName.trim());
      saveIdentity(identity);
      navigate(`/game/${identity.gameId}`);
    } catch (err) {
      handleError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="min-h-screen px-4 py-10 animate-fade-in">
      <div className="max-w-4xl mx-auto flex items-center justify-between mb-10">
        <h1 className="neon-heading text-2xl font-semibold text-accent">{t("home.title")}</h1>
        <div className="flex items-center gap-3">
          <LanguageToggle />
          <button onClick={() => void logout()} className="text-sm text-neon/50 hover:text-neon transition-colors">
            {t("home.logout")}
          </button>
        </div>
      </div>

      {error && <div className="max-w-4xl mx-auto mb-6 text-danger text-sm text-center">{error}</div>}

      <div className="max-w-4xl mx-auto grid md:grid-cols-2 gap-6">
        <form onSubmit={createGame} className="card-surface p-6 flex flex-col gap-4">
          <h2 className="text-lg font-semibold">{t("home.hostCard.title")}</h2>
          <input
            value={hostName}
            onChange={(e) => setHostName(e.target.value)}
            placeholder={t("home.hostCard.namePlaceholder")}
            className="neon-focus bg-black/30 border border-border rounded-lg py-2.5 px-3 outline-none focus:border-accent transition-colors"
            maxLength={20}
          />
          <button
            type="submit"
            disabled={busy}
            className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2.5 rounded-lg disabled:opacity-50 disabled:hover:shadow-none"
          >
            {t("home.hostCard.submit")}
          </button>
        </form>

        <form onSubmit={joinGame} className="card-surface p-6 flex flex-col gap-4">
          <h2 className="text-lg font-semibold">{t("home.joinCard.title")}</h2>
          <input
            value={joinName}
            onChange={(e) => setJoinName(e.target.value)}
            placeholder={t("home.joinCard.namePlaceholder")}
            className="neon-focus bg-black/30 border border-border rounded-lg py-2.5 px-3 outline-none focus:border-accent transition-colors"
            maxLength={20}
          />
          <input
            value={joinCode}
            onChange={(e) => setJoinCode(e.target.value.toUpperCase())}
            placeholder={t("home.joinCard.codePlaceholder")}
            className="neon-focus bg-black/30 border border-border rounded-lg py-2.5 px-3 outline-none focus:border-accent font-mono tracking-widest transition-colors"
            maxLength={7}
          />
          <button
            type="submit"
            disabled={busy}
            className="bg-white/10 hover:bg-white/20 hover:shadow-neon-cyan active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2.5 rounded-lg disabled:opacity-50 disabled:hover:shadow-none"
          >
            {t("home.joinCard.submit")}
          </button>
        </form>
      </div>

      {recentGames.length > 0 && (
        <div className="max-w-4xl mx-auto mt-8 card-surface p-6 animate-fade-in">
          <h3 className="text-sm font-semibold text-neon/70 mb-3">{t("home.recentGames.title")}</h3>
          <ul className="flex flex-col gap-2">
            {recentGames.map((g) => (
              <li key={g.gameId} className="flex items-center justify-between text-sm">
                <span className="font-mono tracking-widest text-neon/60">{g.inviteCode}</span>
                <button onClick={() => navigate(`/game/${g.gameId}`)} className="text-accent hover:underline hover:drop-shadow-neon transition-all">
                  {t("home.recentGames.rejoin")}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
