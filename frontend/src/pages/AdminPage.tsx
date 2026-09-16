import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Library, RefreshCw, ShieldCheck } from "lucide-react";
import { api, ApiError } from "../api/client";
import { Aurora, Logo } from "../components/ui";

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
      <div className="relative flex min-h-screen items-center justify-center px-4">
        <Aurora />
        <form onSubmit={login} className="surface flex w-full max-w-sm animate-fade-in flex-col gap-6 p-8 sm:p-10">
          <div className="flex flex-col items-center gap-4">
            <Logo size="lg" />
            <h1 className="flex items-center gap-2 text-lg font-semibold tracking-tight">
              <ShieldCheck size={18} className="text-accent" />
              {t("admin.login.title")}
            </h1>
          </div>
          <div className="flex flex-col gap-2">
            <input
              type="password"
              autoFocus
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={t("admin.login.passwordPlaceholder")}
              className="input"
            />
            {error && <p className="text-sm text-danger">{error}</p>}
          </div>
          <button type="submit" className="btn btn-primary btn-lg w-full">
            {t("admin.login.submit")}
          </button>
        </form>
      </div>
    );
  }

  return (
    <div className="relative min-h-screen px-4 pb-12 pt-5 text-sm sm:px-6">
      <Aurora />
      <header className="mx-auto flex max-w-4xl flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Logo />
          <span className="h-5 w-px bg-white/10" />
          <span className="flex items-center gap-1.5 text-sm font-medium text-fg/60">
            <ShieldCheck size={15} />
            Admin
          </span>
        </div>
        <nav className="flex gap-1 rounded-xl border border-white/[0.07] bg-white/3 p-1">
          {(["overview", "games"] as Tab[]).map((tabName) => (
            <button
              key={tabName}
              onClick={() => setTab(tabName)}
              className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-all duration-150 ${
                tab === tabName ? "bg-white/10 text-fg shadow-card" : "text-fg/55 hover:text-fg"
              }`}
            >
              {t(`admin.nav.${tabName}`)}
            </button>
          ))}
        </nav>
      </header>

      <main className="mx-auto mt-8 max-w-4xl animate-fade-in">
        {tab === "overview" && <OverviewTab />}
        {tab === "games" && <GamesTab />}
      </main>
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

  const [fileRow, ...statRows] = rows;

  return (
    <div className="flex flex-col gap-5">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {statRows.map(([label, value]) => (
          <div key={label} className="surface rounded-2xl p-4">
            <div className="text-xs text-fg/50">{label}</div>
            <div
              className={`mt-1.5 wrap-break-word font-semibold tabular-nums tracking-tight ${typeof value === "number" ? "text-2xl" : "text-sm leading-6"}`}
            >
              {String(value)}
            </div>
          </div>
        ))}
      </div>

      <section className="surface flex flex-col gap-4 p-6">
        <div className="flex items-start gap-3">
          <span className="brand-mark h-10 w-10 shrink-0">
            <Library size={19} />
          </span>
          <div className="min-w-0">
            <div className="text-xs text-fg/50">{fileRow[0]}</div>
            <div className="mt-0.5 break-all font-mono text-sm">{String(fileRow[1])}</div>
          </div>
        </div>
        {stats.lastScanError && !scanError && (
          <p className="rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-danger">
            {t("admin.overview.lastScanFailed", { error: stats.lastScanError })}
          </p>
        )}
        <p className="leading-relaxed text-fg/55">{t("admin.overview.scanHint")}</p>
        <div>
          <button onClick={() => void scan()} disabled={scanning || stats.scanning} className="btn btn-primary btn-md">
            {scanning || stats.scanning ? <span className="spinner border-white/30 border-t-white" /> : <RefreshCw size={15} />}
            {scanning || stats.scanning ? t("admin.overview.scanning") : t("admin.overview.scan")}
          </button>
        </div>
        {result && (
          <p className="rounded-xl border border-success/30 bg-success/10 px-4 py-3 text-success">
            {t("admin.overview.scanResult", { ...result })}
          </p>
        )}
        {scanError && <p className="rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-danger">{scanError}</p>}
      </section>
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
    <div className="surface overflow-hidden p-0">
      <table className="w-full text-left">
        <thead className="border-b border-white/6 text-[11px] uppercase tracking-[0.12em] text-fg/45">
          <tr>
            <th className="px-5 py-3 font-semibold">{t("admin.games.inviteCode")}</th>
            <th className="px-5 py-3 font-semibold">{t("admin.games.phase")}</th>
            <th className="px-5 py-3 font-semibold">{t("admin.games.players")}</th>
            <th className="px-5 py-3 font-semibold">{t("admin.games.turn")}</th>
            <th className="px-5 py-3" />
          </tr>
        </thead>
        <tbody className="divide-y divide-white/5">
          {games.map((g) => (
            <tr key={g.gameId} className="transition-colors hover:bg-white/3">
              <td className="px-5 py-3 font-mono tracking-[0.15em]">{g.inviteCode}</td>
              <td className="px-5 py-3">
                <span className="chip">{g.phase}</span>
              </td>
              <td className="px-5 py-3 tabular-nums">{g.playerCount}</td>
              <td className="px-5 py-3 tabular-nums">{g.turnNumber}</td>
              <td className="px-5 py-3 text-right">
                <button onClick={() => void api.adminForceEndGame(g.gameId).then(load)} className="btn btn-danger btn-sm">
                  {t("admin.games.forceEnd")}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
