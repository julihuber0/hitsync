# Hitsync

A self-hosted, real-time multiplayer music-timeline game (Hitster-style) that uses your private
Navidrome library as its music source. Players listen to a song streamed in sync, guess when it
came out, and place it on their personal timeline.

## Prerequisites

- Docker and Docker Compose v2
- An existing Navidrome server (v0.64.0+) reachable from the Docker host, with a user account
  Hitsync can use
- An existing Traefik v2/v3 instance with an HTTPS-capable entrypoint and a certificate resolver,
  running on a Docker network that this stack can join
- DNS records for two subdomains pointed at your Traefik host (see below)

Hitsync builds and runs four containers of its own — `frontend`, `backend`, `media`, `postgres` —
and expects Navidrome and Traefik to already exist.

**Further reading:**

- [`docs/deployment.md`](docs/deployment.md) — a full walkthrough of deploying this to a fresh VPS,
  including standing up Traefik if you don't already have it, DNS, backups, updates, and
  deployment-specific troubleshooting.
- [`docs/local-development.md`](docs/local-development.md) — running the backend, media service,
  and frontend natively with hot reload for fast iteration, instead of rebuilding Docker images on
  every change.

## Quickstart

```sh
git clone <this repo> hitsync
cd hitsync
cp .env.example .env
```

Edit `.env`:

1. Set `APP_DOMAIN` and `MEDIA_DOMAIN` to the two subdomains you're pointing at this host.
2. Set `APP_ACCESS_CODE`, `ADMIN_PASSWORD`, `JWT_SECRET`, `MEDIA_SHARED_SECRET`, and
   `POSTGRES_PASSWORD` to strong, unique values. `JWT_SECRET` and `MEDIA_SHARED_SECRET` must each be
   at least 32 characters — generate one with `openssl rand -base64 32`.
3. Point `NAVIDROME_URL` at your Navidrome server and fill in `NAVIDROME_USERNAME` /
   `NAVIDROME_PASSWORD` (see "Connecting to Navidrome" below).
4. Set `MUSICBRAINZ_CONTACT` to an email address or URL — MusicBrainz requires a contact string in
   the `User-Agent` of every request.

Then:

```sh
docker network create proxy   # only if it doesn't already exist
docker compose build
docker compose up -d
```

The backend runs its database migrations automatically on startup and begins syncing your
Navidrome library in the background. The first game is playable as soon as that sync finishes
(check `GET https://{APP_DOMAIN}/healthz` — `libraryTracks` and `eligibleTracks` go from `0` to your
library's size).

Run `make test` to run the backend (Go) and frontend (vitest) test suites locally without Docker.

## Environment variables

All variables are read from `.env` by `docker-compose.yml`; see `.env.example` for a filled-in
template with safe defaults. The backend fails fast on startup with a clear error if a required
variable is missing or looks invalid (e.g. a secret under the minimum length).

### General

| Variable | Required | Default | Notes |
|---|---|---|---|
| `APP_DOMAIN` | yes | — | e.g. `hitsync.example.com`. Used in Traefik labels, invite URLs, and CORS |
| `MEDIA_DOMAIN` | yes | — | e.g. `hitsync-media.example.com` |
| `TRAEFIK_NETWORK` | no | `proxy` | Name of the existing external Traefik docker network |
| `TRAEFIK_ENTRYPOINT` | no | `websecure` | Entrypoint name on the existing Traefik |
| `TRAEFIK_CERTRESOLVER` | no | `letsencrypt` | Cert resolver name on the existing Traefik |
| `LOG_LEVEL` | no | `info` | `debug` / `info` / `warn` / `error` |
| `TZ` | no | `Europe/Berlin` | |

### Secrets

| Variable | Required | Notes |
|---|---|---|
| `APP_ACCESS_CODE` | yes | The shared code users type to enter the app |
| `ADMIN_PASSWORD` | yes | Separate password for the exclusion/admin area |
| `JWT_SECRET` | yes | ≥32 chars. Signs session and player tokens |
| `MEDIA_SHARED_SECRET` | yes | ≥32 chars. HMAC key shared by backend and media service for stream tokens |

### Navidrome

| Variable | Required | Default | Notes |
|---|---|---|---|
| `NAVIDROME_URL` | yes | — | e.g. `http://navidrome:4533` — reachable from `backend` and `media` |
| `NAVIDROME_USERNAME` | yes | — | |
| `NAVIDROME_PASSWORD` | yes | — | Used with Subsonic salted-token auth; never sent in plaintext |
| `NAVIDROME_CLIENT_NAME` | no | `hitsync` | Subsonic `c` parameter |
| `NAVIDROME_TIMEOUT` | no | `30s` | |

### Audio

| Variable | Required | Default | Notes |
|---|---|---|---|
| `AUDIO_FORMAT` | no | `mp3` | Passed to Navidrome's stream endpoint |
| `AUDIO_BITRATE` | no | `192` | kbit/s |
| `MEDIA_CACHE_DIR` | no | `/cache` | Inside the media container |
| `MEDIA_CACHE_MAX_BYTES` | no | `2147483648` | 2 GiB, LRU eviction |
| `MEDIA_DIRECT_PORT` | no | *(empty)* | Escape hatch — see below |
| `MEDIA_TLS_CERT_FILE` / `MEDIA_TLS_KEY_FILE` | no | *(empty)* | For direct-port HTTPS |

### Library and year resolution

| Variable | Required | Default | Notes |
|---|---|---|---|
| `LIBRARY_SYNC_INTERVAL` | no | `6h` | Full re-index of the Navidrome library |
| `MUSICBRAINZ_ENABLED` | no | `true` | If false, Navidrome years are used exclusively |
| `MUSICBRAINZ_BASE_URL` | no | `https://musicbrainz.org/ws/2` | |
| `MUSICBRAINZ_CONTACT` | yes if enabled | — | Email or URL for the required User-Agent |
| `MUSICBRAINZ_RATE_PER_SEC` | no | `1` | Do not raise above 1 against the public instance |
| `MUSICBRAINZ_MIN_SCORE` | no | `90` | Minimum search score to consider a recording a match |
| `MUSICBRAINZ_CACHE_ENTRIES` | no | `2000` | In-memory LRU size. Nothing is persisted |
| `MUSICBRAINZ_CACHE_TTL` | no | `24h` | In-memory only; cleared on restart |
| `YEAR_LOOKAHEAD_DEPTH` | no | `2` | Candidate tracks resolved ahead of the current turn |
| `YEAR_LOOKUP_TIMEOUT` | no | `6s` | Max wait before falling back to the Navidrome year alone |
| `YEAR_MAX_BACKDATE` | no | `0` | `0` = off. See "The earliest-wins year rule" below |

### Game rules and limits

| Variable | Required | Default | Notes |
|---|---|---|---|
| `MAX_CONCURRENT_GAMES` | no | `10` | |
| `MIN_PLAYERS` | no | `2` | |
| `MAX_PLAYERS` | no | `12` | |
| `DEFAULT_TARGET_CARDS` | no | `10` | Cards needed to win |
| `DEFAULT_START_TOKENS` | no | `2` | |
| `MAX_TOKENS` | no | `5` | Cap on hoarding |
| `RULE_ENABLE_SONG_GUESS` | no | `true` | Optional title/artist bonus round |
| `TURN_PLACEMENT_TIMEOUT` | no | `90s` | |
| `TURN_CHALLENGE_WINDOW` | no | `20s` | |
| `REVEAL_DURATION` | no | `8s` | |
| `TRACK_MIN_DURATION` | no | `45s` | Skip very short tracks |
| `TRACK_MAX_DURATION` | no | `600s` | Skip 20-minute prog epics |
| `PLAYER_RECONNECT_GRACE` | no | `120s` | |
| `LOBBY_IDLE_TIMEOUT` | no | `30m` | Empty/idle games are reaped |

### Database

| Variable | Required | Default |
|---|---|---|
| `POSTGRES_USER` | no | `hitsync` |
| `POSTGRES_PASSWORD` | yes | — |
| `POSTGRES_DB` | no | `hitsync` |
| `DATABASE_URL` | no | derived from the three above + host `postgres` |

## Connecting to Navidrome

`backend` and `media` both need to reach `NAVIDROME_URL` directly (never through this app's own
reverse proxy). Two ways to set that up, depending on where Navidrome runs:

**Navidrome is its own Docker Compose stack on the same host.** Either:

- Put Navidrome on a network this stack can also join (add its container to `hitsync_internal`, or
  add a shared external network to both compose files), and set `NAVIDROME_URL` to
  `http://<navidrome-service-name>:4533`; or
- Point `NAVIDROME_URL` at whatever address/port Navidrome already publishes on the host, e.g.
  `http://host.docker.internal:4533` (Docker Desktop) or the host's LAN IP on Linux.

**Navidrome runs elsewhere** (bare metal, another host): set `NAVIDROME_URL` to its normal
address, e.g. `https://music.example.com`, as long as it's reachable from wherever this stack runs.

## DNS

Point both of the following at the host running Traefik:

- `APP_DOMAIN` (e.g. `hitsync.example.com`) — serves the web app, `/api`, and `/ws`
- `MEDIA_DOMAIN` (e.g. `hitsync-media.example.com`) — serves audio streams only

Traefik must already have an HTTPS-capable entrypoint (`websecure` by default) and certificate
resolver configured; this stack only adds routers/labels pointing at it. WebSocket upgrades pass
through Traefik v2/v3 automatically — no extra label is needed for `/ws`.

## Admin console

Reach it at `https://{APP_DOMAIN}/admin`. It has its own login (`ADMIN_PASSWORD`), completely
separate from the app access code — `/admin` is reachable without knowing `APP_ACCESS_CODE`, and
the access code grants nothing in `/admin`.

- **Overview** — library size, eligible pool size, active games, last sync time, MusicBrainz cache
  size, and a button to trigger an immediate resync.
- **Library** — search title/artist/album, see each track's Navidrome year and any manual override,
  exclude/include a track, and look up a track's MusicBrainz year on demand (bypasses the cache) to
  diagnose a suspicious card.
- **Exclusions** — the current exclusion list (track/album/artist), with removal.
- **Games** — active games with a force-end action.

## The earliest-wins year rule, and fixing a bad card

For each track, Hitsync asks both Navidrome's own year tag and MusicBrainz (searching for the
*original* release, not a compilation or remaster) and uses **whichever year is earlier**. This is
deliberate: mistagging in this domain is overwhelmingly in one direction — a remaster, reissue, or
"Greatest Hits" pressing pushes the year *later* than the song's real debut, essentially never
earlier. Taking the minimum of the two sources is a simple, effective correction for that.

The one failure mode this doesn't catch is MusicBrainz matching the wrong recording (e.g. a cover
version) onto a genuinely older release date. Hitsync guards against this by rejecting any
MusicBrainz match whose artist credit doesn't match the track's own artist — but if a bad card still
slips through:

1. Open `/admin` → **Library**, search for the track.
2. Click **"Look up year"** to see the Navidrome year and MusicBrainz year side by side, along with
   which release group MusicBrainz matched.
3. If MusicBrainz is simply wrong, set a manual **year override** — it takes precedence over both
   automatic sources for every future game.

If you're seeing a *pattern* of bad matches (not just one track), `YEAR_MAX_BACKDATE` lets you
reject any MusicBrainz year more than N years earlier than Navidrome's. Leave it at `0` (off)
unless you have specific evidence of a problem — a threshold tight enough to catch a mismatch would
also reject legitimate decades-old compilation corrections, which is the main thing this whole
mechanism exists to fix.

## Direct port (escape hatch)

By default, the media service is only reachable through Traefik, which terminates TLS for it like
everything else. If you need to bypass Traefik for media streaming — for example, to test the media
service standalone, or because Traefik can't be configured with buffering disabled in your setup —
set:

```
MEDIA_DIRECT_PORT=8443
MEDIA_TLS_CERT_FILE=/path/inside/container/to/fullchain.pem
MEDIA_TLS_KEY_FILE=/path/inside/container/to/privkey.pem
```

...then uncomment the `ports:` block under the `media` service in `docker-compose.yml`, and mount
your certificate files into the container (add a `volumes:` entry pointing at wherever
`MEDIA_TLS_CERT_FILE`/`MEDIA_TLS_KEY_FILE` point).

**Mixed-content caveat:** the web app is served over HTTPS, and browsers block an HTTPS page from
loading audio off a plain HTTP origin. If you publish the media service directly, it **must** serve
HTTPS itself (via the two TLS variables above) — a bare HTTP direct port will silently fail to play
any audio in the browser.

## Troubleshooting

**No audio plays.** Browsers require a user gesture before audio can play. Hitsync primes this on
the "Create game" / "Join" button press, but a browser can still reject the first `play()` call —
when that happens you'll see a full-screen "Tap to enable sound" overlay; tapping it retries
playback. If *no one* in a game ever hears audio, check that `MEDIA_DOMAIN` actually resolves and
serves HTTPS (open `https://{MEDIA_DOMAIN}/healthz` directly) and that your CSP/CORS isn't blocking
it — the browser console will show a blocked request if so.

**Audio drifts out of sync.** A few tens of milliseconds of drift is normal and self-corrects via
small playback-rate nudges every 5 seconds; you shouldn't be able to hear it. If drift is large and
persistent, it usually means clock sync itself is off — check that the backend container's clock is
reasonably accurate (NTP on the Docker host) and that nothing is unusually delaying WebSocket
messages between the client and `wss://{APP_DOMAIN}/ws`.

**The track pool seems empty / games end with no more songs.** Check `GET /healthz` — if
`eligibleTracks` is `0` or very low, the most common causes are: the library sync hasn't completed
yet (check `libraryTracks` too), `TRACK_MIN_DURATION`/`TRACK_MAX_DURATION` are set too narrowly for
your library, or you've excluded a very large chunk of the library in `/admin` → Exclusions.

## Repository layout

```
hitsync/
├── docker-compose.yml, .env.example, Makefile      — production stack
├── docker-compose.dev.yml, .env.dev.example        — local-dev support services only (docs/local-development.md)
├── docs/                                            — deployment and local-dev guides
├── backend/    — Go: REST API, WebSocket hub, game manager, rules engine, Navidrome/MusicBrainz clients
├── media/      — Go: dedicated audio-streaming service with its own MP3 cache
└── frontend/   — React + TypeScript + Vite + Tailwind SPA, served by nginx
```

## Testing

```sh
make test          # backend (go test) + frontend (vitest)
make vet           # go vet on backend and media
```

Testing is deliberately modest, per the project's own scope: no e2e tests, no browser automation,
no testcontainers. The heaviest coverage is on `backend/internal/game`, the pure turn-based rules
engine (placement correctness, challenge resolution, token accounting, win condition, turn
rotation, phase transitions), plus fixture-driven tests for MusicBrainz parsing/filtering, year
normalisation and combination, media token round-trips, and a handful of HTTP handler tests.
Frontend tests cover the drift-correction decision function, clock-sync min-RTT sample selection,
and English/German i18n key parity.

## Known limitations / not exercised in this environment

This was built without a live Navidrome or MusicBrainz-reachable sandbox, and without a real
Traefik instance to route through, so the following have been verified by code review and
component/unit testing but not by a full live end-to-end run:

- An actual Navidrome library sync against a real server (the Subsonic client, normalisation, and
  upsert logic are unit-tested independently; the sync loop was smoke-tested against an
  unreachable host to confirm it degrades gracefully rather than crashing).
- A real MusicBrainz resolution against the public API (the client is tested against recorded JSON
  fixtures covering the compilation-filtering and artist-verification logic).
- Full browser playback and drift correction against real synchronised audio across multiple
  clients (the drift-correction math and clock-sync sample selection are unit-tested; the
  `HTMLAudioElement` wiring itself needs a real browser to exercise).
- Traefik routing/priority behaviour, since no Traefik instance was available to route through.

The Go backend and media service, the full REST/WebSocket protocol, the rules engine, and the
frontend were built and verified locally: `go build`/`go vet`/`go test` pass for both Go modules,
`tsc`/`vite build`/`vitest` pass for the frontend, all three Docker images build successfully, and
the assembled stack (`postgres` + `backend` + `media`) was started with `docker compose up` and
confirmed to run migrations, serve `/healthz`, and set the app-access cookie correctly.
