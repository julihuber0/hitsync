import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "../api/client";

type Tab = "overview" | "library" | "exclusions" | "games";

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
        {(["overview", "library", "exclusions", "games"] as Tab[]).map((tabName) => (
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
      {tab === "library" && <LibraryTab />}
      {tab === "exclusions" && <ExclusionsTab />}
      {tab === "games" && <GamesTab />}
    </div>
  );
}

function OverviewTab() {
  const { t } = useTranslation();
  const [stats, setStats] = useState<Record<string, unknown> | null>(null);

  const load = () => void api.adminStats().then(setStats);
  useEffect(load, []);

  if (!stats) return null;

  const rows: [string, unknown][] = [
    [t("admin.overview.libraryTracks"), stats.libraryTracks],
    [t("admin.overview.eligibleTracks"), stats.eligibleTracks],
    [t("admin.overview.activeGames"), stats.activeGames],
    [t("admin.overview.lastSync"), stats.lastLibrarySync ?? "—"],
    [t("admin.overview.cacheSize"), stats.discogsCacheLen],
  ];

  return (
    <div className="max-w-xl">
      <table className="w-full mb-4">
        <tbody>
          {rows.map(([label, value]) => (
            <tr key={label} className="border-b border-border">
              <td className="py-2 text-neon/60">{label}</td>
              <td className="py-2 text-right font-mono">{String(value)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <button
        onClick={() => void api.adminResyncLibrary().then(load)}
        className="bg-white/10 hover:bg-white/20 hover:shadow-neon-cyan transition-all duration-150 rounded-md px-4 py-2"
      >
        {t("admin.overview.resync")}
      </button>
    </div>
  );
}

interface AdminTrack {
  id: string;
  title: string;
  artist: string;
  album: string;
  navidromeYear: number | null;
  overrideYear: number | null;
  excludedKind: string | null;
}

function LibraryTab() {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [excluded, setExcluded] = useState<string>("");
  const [tracks, setTracks] = useState<AdminTrack[]>([]);
  const [lookups, setLookups] = useState<Record<string, unknown>>({});

  const search = () => {
    void api
      .adminTracks({ q, excluded: excluded || undefined, page: 0, pageSize: 50 })
      .then((res) => setTracks((res.tracks as AdminTrack[]) ?? []));
  };
  useEffect(search, [q, excluded]);

  const toggleExclude = (track: AdminTrack) => {
    const action = track.excludedKind
      ? api.adminDeleteExclusion("track", track.id)
      : api.adminCreateExclusion("track", track.id, `${track.title} — ${track.artist}`);
    void action.then(search);
  };

  const lookupYear = (id: string) => {
    void api.adminResolveYear(id).then((res) => setLookups((prev) => ({ ...prev, [id]: res })));
  };

  const setOverride = (id: string) => {
    const year = window.prompt(t("admin.library.overrideYearPlaceholder"));
    if (!year) return;
    void api.adminSetYearOverride(id, Number(year)).then(search);
  };

  return (
    <div>
      <div className="flex gap-2 mb-4">
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={t("admin.library.search")}
          className="neon-focus bg-black/30 border border-border rounded-lg py-2 px-3 outline-none focus:border-accent flex-1 max-w-sm transition-colors"
        />
        <select
          value={excluded}
          onChange={(e) => setExcluded(e.target.value)}
          className="neon-focus bg-black/30 border border-border rounded-lg py-2 px-3 transition-colors"
        >
          <option value="">{t("admin.library.filterAll")}</option>
          <option value="true">{t("admin.library.filterExcluded")}</option>
          <option value="false">{t("admin.library.filterIncluded")}</option>
        </select>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left">
          <thead className="text-neon/50 text-xs uppercase">
            <tr>
              <th className="py-2 pr-3">{t("admin.library.columnTitle")}</th>
              <th className="py-2 pr-3">{t("admin.library.columnArtist")}</th>
              <th className="py-2 pr-3">{t("admin.library.columnAlbum")}</th>
              <th className="py-2 pr-3">{t("admin.library.columnYear")}</th>
              <th className="py-2 pr-3">{t("admin.library.columnOverride")}</th>
              <th className="py-2 pr-3">{t("admin.library.columnExcluded")}</th>
              <th className="py-2" />
            </tr>
          </thead>
          <tbody>
            {tracks.map((track) => (
              <tr key={track.id} className="border-b border-border/50 hover:bg-white/[0.03] transition-colors">
                <td className="py-2 pr-3">{track.title}</td>
                <td className="py-2 pr-3">{track.artist}</td>
                <td className="py-2 pr-3 text-neon/50">{track.album}</td>
                <td className="py-2 pr-3 tabular-nums">{track.navidromeYear ?? "—"}</td>
                <td className="py-2 pr-3 tabular-nums">{track.overrideYear ?? "—"}</td>
                <td className="py-2 pr-3">{track.excludedKind ?? "—"}</td>
                <td className="py-2 flex gap-2 whitespace-nowrap">
                  <button onClick={() => toggleExclude(track)} className="text-accent hover:underline hover:drop-shadow-neon transition-all">
                    {track.excludedKind ? t("admin.library.include") : t("admin.library.exclude")}
                  </button>
                  <button onClick={() => lookupYear(track.id)} className="text-neon/60 hover:underline">
                    {t("admin.library.lookupYear")}
                  </button>
                  <button onClick={() => setOverride(track.id)} className="text-neon/60 hover:underline">
                    {t("admin.library.setOverride")}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {Object.entries(lookups).map(([id, result]) => (
        <pre key={id} className="mt-3 bg-black/30 rounded-lg p-3 text-xs overflow-x-auto">
          {JSON.stringify(result, null, 2)}
        </pre>
      ))}
    </div>
  );
}

function ExclusionsTab() {
  const { t } = useTranslation();
  const [exclusions, setExclusions] = useState<Array<{ kind: string; refId: string; label: string; reason?: string }>>([]);

  const load = () => void api.adminListExclusions().then((res) => setExclusions((res.exclusions as typeof exclusions) ?? []));
  useEffect(load, []);

  if (exclusions.length === 0) return <p className="text-neon/50">{t("admin.exclusions.empty")}</p>;

  return (
    <ul className="flex flex-col gap-2 max-w-2xl">
      {exclusions.map((e) => (
        <li key={`${e.kind}-${e.refId}`} className="card-surface p-3 flex items-center justify-between">
          <div>
            <span className="text-xs uppercase text-neon/40 mr-2">{e.kind}</span>
            {e.label}
          </div>
          <button
            onClick={() => void api.adminDeleteExclusion(e.kind, e.refId).then(load)}
            className="text-danger/80 hover:text-danger transition-colors text-xs"
          >
            {t("common.remove")}
          </button>
        </li>
      ))}
    </ul>
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
