import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "../api/client";

type Tab = "overview" | "games";

export default function AdminPage() {
  const { t } = useTranslation();
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("overview");

  useEffect(() => {
    api
      .adminStats()
      .then(() => setAuthenticated(true))
      .catch(() => setAuthenticated(false));
  }, []);

  const login = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await api.adminLogin(password);
      setAuthenticated(true);
    } catch (err) {
      setError(err instanceof ApiError ? t(`errors.${err.code}`, t("errors.generic")) : t("errors.generic"));
    }
  };

  if (authenticated === null) return null;

  if (!authenticated) {
    return (
      <div className="min-h-screen flex items-center justify-center px-4 animate-fade-in">
        <form onSubmit={login} className="card-surface w-full max-w-sm p-8 flex flex-col gap-4">
          <h1 className="neon-heading text-lg font-semibold">{t("admin.login.title")}</h1>
          <input
            type="password"
            autoFocus
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("admin.login.passwordPlaceholder")}
            className="neon-focus bg-black/30 border border-border rounded-lg py-2.5 px-3 outline-none focus:border-accent transition-colors"
          />
          {error && <p className="text-danger text-sm">{error}</p>}
          <button
            type="submit"
            className="bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-2.5 rounded-lg"
          >
            {t("admin.login.submit")}
          </button>
        </form>
      </div>
    );
  }

  return (
    <div className="min-h-screen p-6 text-sm animate-fade-in">
      <nav className="flex gap-2 mb-6">
        {(["overview", "games"] as Tab[]).map((tabName) => (
          <button
            key={tabName}
            onClick={() => setTab(tabName)}
            className={`px-4 py-2 rounded-md transition-all duration-150 ${
              tab === tabName ? "bg-accent text-white shadow-neon-sm" : "bg-white/5 hover:bg-white/10"
            }`}
          >
            {t(`admin.nav.${tabName}`)}
          </button>
        ))}
      </nav>

      {tab === "overview" && <OverviewTab />}
      {tab === "games" && <GamesTab />}
    </div>
  );
}

interface AdminStats {
  cardsFile: string;
  cards: number;
  playableCards: number;
  excludedCards: number;
  cardsMissingYear: number;
  lastScan: string | null;
  lastScanError: string | null;
  scanning: boolean;
  activeGames: number;
}

interface ScanSummary {
  added: number;
  updated: number;
  removed: number;
  total: number;
}

function OverviewTab() {
  const { t } = useTranslation();
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [scanning, setScanning] = useState(false);
  const [result, setResult] = useState<ScanSummary | null>(null);
  const [scanError, setScanError] = useState<string | null>(null);

  const load = () => void api.adminStats().then((s) => setStats(s as unknown as AdminStats));
  useEffect(load, []);

  const scan = async () => {
    setScanning(true);
    setResult(null);
    setScanError(null);
    try {
      setResult(await api.adminScanCards());
    } catch (err) {
      setScanError(err instanceof ApiError ? err.message : t("errors.generic"));
    } finally {
      setScanning(false);
      load();
    }
  };

  if (!stats) return null;

  const rows: [string, unknown][] = [
    [t("admin.overview.cardsFile"), stats.cardsFile],
    [t("admin.overview.cards"), stats.cards],
    [t("admin.overview.playableCards"), stats.playableCards],
    [t("admin.overview.excludedCards"), stats.excludedCards],
    [t("admin.overview.cardsMissingYear"), stats.cardsMissingYear],
    [t("admin.overview.lastScan"), stats.lastScan ?? "—"],
    [t("admin.overview.activeGames"), stats.activeGames],
  ];

  return (
    <div className="max-w-xl">
      <table className="w-full mb-4">
        <tbody>
          {rows.map(([label, value]) => (
            <tr key={label} className="border-b border-border">
              <td className="py-2 text-neon/60">{label}</td>
              <td className="py-2 text-right font-mono break-all">{String(value)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {stats.lastScanError && !scanError && (
        <p className="mb-4 text-danger">{t("admin.overview.lastScanFailed", { error: stats.lastScanError })}</p>
      )}
      <p className="mb-4 text-neon/50">{t("admin.overview.scanHint")}</p>
      <button
        onClick={() => void scan()}
        disabled={scanning || stats.scanning}
        className="bg-white/10 hover:bg-white/20 hover:shadow-neon-cyan transition-all duration-150 rounded-md px-4 py-2 disabled:opacity-40"
      >
        {scanning || stats.scanning ? t("admin.overview.scanning") : t("admin.overview.scan")}
      </button>
      {result && <p className="mt-3 text-success">{t("admin.overview.scanResult", { ...result })}</p>}
      {scanError && <p className="mt-3 text-danger">{scanError}</p>}
    </div>
  );
}

function GamesTab() {
  const { t } = useTranslation();
  const [games, setGames] = useState<Array<{ gameId: string; inviteCode: string; phase: string; playerCount: number; turnNumber: number }>>(
    [],
  );

  const load = () => void api.adminGames().then((res) => setGames((res.games as typeof games) ?? []));
  useEffect(() => {
    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <table className="w-full text-left max-w-3xl">
      <thead className="text-neon/50 text-xs uppercase">
        <tr>
          <th className="py-2 pr-3">{t("admin.games.inviteCode")}</th>
          <th className="py-2 pr-3">{t("admin.games.phase")}</th>
          <th className="py-2 pr-3">{t("admin.games.players")}</th>
          <th className="py-2 pr-3">{t("admin.games.turn")}</th>
          <th className="py-2" />
        </tr>
      </thead>
      <tbody>
        {games.map((g) => (
          <tr key={g.gameId} className="border-b border-border/50 hover:bg-white/[0.03] transition-colors">
            <td className="py-2 pr-3 font-mono">{g.inviteCode}</td>
            <td className="py-2 pr-3">{g.phase}</td>
            <td className="py-2 pr-3 tabular-nums">{g.playerCount}</td>
            <td className="py-2 pr-3 tabular-nums">{g.turnNumber}</td>
            <td className="py-2">
              <button
                onClick={() => void api.adminForceEndGame(g.gameId).then(load)}
                className="text-danger/80 hover:text-danger transition-colors"
              >
                {t("admin.games.forceEnd")}
              </button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
