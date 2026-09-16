import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { api, ApiError } from "../api/client";
import { saveIdentity, loadRecentGames, loadPlayerToken } from "../api/identity";
import { useAppStore } from "../store/appStore";
import { ArrowRight, LogOut, Plus, Users } from "lucide-react";
import LanguageToggle from "../components/LanguageToggle";
import { Aurora, Logo } from "../components/ui";
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
    <div className="relative min-h-screen px-4 pb-16 pt-5 sm:px-6">
      <Aurora />
      <header className="mx-auto flex max-w-5xl items-center justify-between">
        <Logo />
        <div className="flex items-center gap-2">
          <LanguageToggle />
          <button onClick={() => void logout()} className="btn btn-ghost btn-md">
            <LogOut size={15} />
            <span className="hidden sm:inline">{t("home.logout")}</span>
          </button>
        </div>
      </header>

      <main className="mx-auto mt-14 max-w-5xl animate-fade-in sm:mt-20">
        <div className="mb-10 text-center sm:mb-14">
          <h1 className="text-5xl font-bold leading-[1.2] tracking-tight sm:text-6xl sm:leading-[1.2]">
            <span className="brand-text pb-[0.12em]">{t("home.title")}</span>
          </h1>
        </div>

        {error && (
          <div role="alert" className="mx-auto mb-6 max-w-3xl rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-center text-sm text-danger">
            {error}
          </div>
        )}

        <div className="mx-auto grid max-w-3xl gap-5 md:grid-cols-2">
          <form onSubmit={createGame} className="surface flex flex-col gap-5 p-6 sm:p-7">
            <div className="flex items-center gap-3">
              <span className="brand-mark h-10 w-10">
                <Plus size={20} />
              </span>
              <h2 className="text-lg font-semibold tracking-tight">{t("home.hostCard.title")}</h2>
            </div>
            <input
              value={hostName}
              onChange={(e) => setHostName(e.target.value)}
              placeholder={t("home.hostCard.namePlaceholder")}
              className="input"
              maxLength={20}
            />
            <button type="submit" disabled={busy} className="btn btn-primary btn-lg mt-auto w-full">
              {t("home.hostCard.submit")}
              <ArrowRight size={17} />
            </button>
          </form>

          <form onSubmit={joinGame} className="surface flex flex-col gap-5 p-6 sm:p-7">
            <div className="flex items-center gap-3">
              <span className="flex h-10 w-10 items-center justify-center rounded-xl border border-white/10 bg-white/6 text-accent2">
                <Users size={19} />
              </span>
              <h2 className="text-lg font-semibold tracking-tight">{t("home.joinCard.title")}</h2>
            </div>
            <input
              value={joinName}
              onChange={(e) => setJoinName(e.target.value)}
              placeholder={t("home.joinCard.namePlaceholder")}
              className="input"
              maxLength={20}
            />
            <input
              value={joinCode}
              onChange={(e) => setJoinCode(e.target.value.toUpperCase())}
              placeholder={t("home.joinCard.codePlaceholder")}
              className="input font-mono tracking-[0.25em] placeholder:font-sans placeholder:tracking-normal"
              maxLength={7}
            />
            <button type="submit" disabled={busy} className="btn btn-secondary btn-lg w-full">
              {t("home.joinCard.submit")}
              <ArrowRight size={17} />
            </button>
          </form>
        </div>

        {recentGames.length > 0 && (
          <section className="surface mx-auto mt-5 max-w-3xl animate-fade-in p-6 sm:p-7">
            <h3 className="eyebrow mb-3">{t("home.recentGames.title")}</h3>
            <ul className="flex flex-col divide-y divide-white/6">
              {recentGames.map((g) => (
                <li key={g.gameId} className="flex items-center justify-between py-2.5 first:pt-0 last:pb-0">
                  <span className="font-mono text-sm tracking-[0.2em] text-fg/70">{g.inviteCode}</span>
                  <button onClick={() => navigate(`/game/${g.gameId}`)} className="btn btn-secondary btn-sm">
                    {t("home.recentGames.rejoin")}
                    <ArrowRight size={14} />
                  </button>
                </li>
              ))}
            </ul>
          </section>
        )}
      </main>
    </div>
  );
}
