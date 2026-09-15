# Hitsync — Technical Specification

A self-hosted, real-time multiplayer music-timeline game (Hitster-style) that uses a private
Navidrome server as its music source.

This document is the complete implementation brief. It is written to be handed to a code-generation
agent (Codex) as the single source of truth. Where a decision could reasonably go several ways, the
decision has already been made here — implement what is written rather than substituting an
alternative.

---

## 1. Product summary

Players take turns listening to a randomly chosen song from the host's private music library and
placing it into their personal chronological timeline of previously won songs. Correct placement
wins the card; incorrect placement gives other players a chance to steal it by spending a token.
First player to a target number of cards wins.

Key differences from the physical Hitster board game:

- No QR codes and no physical cards. Songs are picked server-side from the Navidrome library.
- Audio is streamed to every connected client and plays in near-perfect synchrony.
- Everything is remote-friendly: players can be in different cities.

### 1.1 Naming

- Project / repository name: `hitsync`
- Docker image prefix: `hitsync-`
- Go module path: `github.com/OWNER/hitsync` (make `OWNER` easy to change; it appears in one place)

---

## 2. Hard requirements checklist

These come directly from the product owner. Every one of them must hold in the finished system.

| # | Requirement |
|---|---|
| R1 | Web app, real-time multiplayer, host creates a round and shares an invite code/URL |
| R2 | No accounts. A player types a display name each time they join |
| R3 | Access to the app itself is gated by a single shared secret code. Anyone past the gate may host |
| R4 | 2–12 players per game; up to 10 concurrent games |
| R5 | Music comes from Navidrome (v0.64.0, ~5000 tracks), transcoded to MP3 @ 192 kbit/s |
| R6 | The **full** song plays, looping endlessly from the start until the turn is resolved |
| R7 | Playback is synchronised across all clients in the game |
| R8 | Each client controls its own volume / mute; muting does **not** pause or desync playback |
| R9 | Title and artist come from Navidrome. The release year is fetched on the fly per turn from a free public music database, resolved by **title + artist only**, using the **original** release, never a compilation reissue. The MusicBrainz and Navidrome years are compared and the **earlier** one wins; either alone is used if the other is missing |
| R10 | Individual tracks, albums and artists can be excluded from the pool |
| R11 | Classic Hitster turn structure: turn-based with steal tokens |
| R12 | Exclusion management is behind a **separate admin password** |
| R13 | Go backend; classic frontend / backend / database split with one container per layer |
| R14 | Runs as a Docker stack behind an **existing** external Traefik instance (Traefik itself is not part of this deliverable) |
| R15 | Audio bytes are served by a dedicated media service, not by the application backend |
| R16 | All configuration (Traefik hostnames, Navidrome address and credentials, secrets, tuning) via environment variables |
| R17 | UI in English and German, modern and sleek, desktop-optimised, usable on mobile |
| R18 | Smooth gameplay, no stutter or frame drops |
| R19 | Resource-efficient. Target host: 16 GB RAM / 10 vCPU, but the stack should idle in the low hundreds of MB |
| R20 | Reasonable baseline security, not enterprise-grade. Simple tests, no e2e tests |

---

## 3. Architecture

### 3.1 Containers

```
                    ┌──────────────────────────────────────┐
  Internet ────────▶│  Traefik (already exists, external)   │
                    └───┬───────────────┬──────────────┬────┘
                        │               │              │
              app.DOMAIN│    app.DOMAIN │     media.DOMAIN
                   /    │   /api  /ws   │      /stream
                        ▼               ▼              ▼
                 ┌────────────┐  ┌────────────┐  ┌────────────┐
                 │  frontend  │  │  backend   │  │   media    │
                 │  nginx +   │  │  Go        │  │  Go        │
                 │  static SPA│  │  API + WS  │  │  MP3 cache │
                 └────────────┘  └─────┬──────┘  └─────┬──────┘
                                       │               │
                                       │        ┌──────▼──────┐
                                       │        │  Navidrome  │
                                       │        │  (external) │
                                       │        └─────────────┘
                                 ┌─────▼──────┐
                                 │ postgres   │
                                 └────────────┘
```

Four containers are built and run by this stack: `frontend`, `backend`, `media`, `postgres`.
Navidrome and Traefik are pre-existing and external.

### 3.2 Why the media service is separate — and a note on the "open port" idea

The original intent was a LiveKit-style service on a directly published port, bypassing the reverse
proxy. Two things shape the actual design:

1. **Client-side synchronised playback was chosen over server-side broadcast.** Each client
   downloads the MP3 once and plays it locally, starting at an agreed wall-clock timestamp. This
   gives sub-100 ms alignment, seamless looping, instant per-client volume control, and near-zero
   server CPU — far better than an SFU, which would also force a re-encode away from MP3.
2. **Browsers block mixed content.** An HTTPS page cannot load audio from a plain-HTTP origin. A
   directly published port would therefore need its own TLS certificate and renewal, duplicating
   what Traefik already does well.

So the media service stays a **separate container with its own hostname and its own routing
lifecycle** — it is fully decoupled from the backend and can be scaled or replaced independently —
but by default Traefik terminates TLS for it. Traefik must be configured for it with buffering
disabled so bytes stream through rather than being accumulated.

An escape hatch is provided: setting `MEDIA_DIRECT_PORT` publishes the media container on a host
port directly, and `MEDIA_TLS_CERT_FILE` / `MEDIA_TLS_KEY_FILE` let it serve HTTPS itself. This is
documented in the README but is not the default.

### 3.3 Traffic volume sanity check

Worst case: 10 games × 12 players = 120 clients. At 192 kbit/s, a 3.5-minute track is ≈ 5 MB. Each
client downloads a given track **once** — the endless loop replays from the browser's buffer, it
does not re-request. With a new track roughly every 60–120 s of turn time, sustained egress is
around 5–10 Mbit/s, with brief bursts at track changes. This is comfortable for both Traefik and
a typical VPS uplink.

Upstream load on Navidrome is one transcode per distinct track per cache lifetime, because the
media service caches the transcoded file and fans it out.

### 3.4 State management

Authoritative game state lives **in memory in the backend process**, one goroutine-owned struct per
game, mutated only through a serialised command channel. This is what makes it fast and
stutter-free: no database round-trip is on the gameplay path.

After every state transition the game's state is written asynchronously to Postgres as a JSONB
snapshot. This is purely for crash recovery — on startup the backend rehydrates games whose
`updated_at` is younger than 30 minutes. Snapshot writes never block gameplay; a failed write is
logged and dropped.

Postgres is the durable store for: the library index, exclusions, manual year overrides, and game
snapshots/results. Release years fetched from MusicBrainz are deliberately **not** persisted (§9.5).

No Redis. A single backend replica is assumed and sufficient.

---

## 4. Technology choices

### Backend (`backend`)
- Go 1.23+
- HTTP: `net/http` with `github.com/go-chi/chi/v5` for routing
- WebSocket: `github.com/coder/websocket`
- Postgres: `github.com/jackc/pgx/v5` (pool), `github.com/golang-migrate/migrate/v4` with embedded
  SQL migrations
- JWT: `github.com/golang-jwt/jwt/v5` (HS256)
- Config: `github.com/caarlos0/env/v11` over plain environment variables
- Logging: `log/slog`, JSON handler, level via `LOG_LEVEL`
- Validation: hand-written; no heavyweight framework

### Media service (`media`)
- Go 1.23+, `net/http` only. No database access. No external dependencies beyond the standard
  library plus `github.com/caarlos0/env/v11`.

### Frontend (`frontend`)
- React 18 + TypeScript + Vite
- Tailwind CSS v3 with a custom theme
- `zustand` for client state
- `react-i18next` + `i18next-browser-languagedetector`
- `framer-motion` for the reveal and card animations (used sparingly; transform/opacity only)
- `lucide-react` for icons
- Served in production by `nginx:alpine` as static files

### Database
- PostgreSQL 16 (`postgres:16-alpine`)

---

## 5. Repository layout

```
hitsync/
├── docker-compose.yml
├── .env.example
├── README.md
├── Makefile
├── backend/
│   ├── Dockerfile
│   ├── go.mod
│   ├── cmd/server/main.go
│   └── internal/
│       ├── config/          # env parsing
│       ├── httpapi/         # chi router, REST handlers, middleware
│       ├── ws/              # websocket hub, envelope codec, clock sync
│       ├── game/            # rules engine (pure, no I/O)
│       ├── gamesvc/         # game manager, goroutines, lifecycle, snapshots
│       ├── library/         # navidrome sync + track pool selection
│       ├── navidrome/       # subsonic API client
│       ├── musicbrainz/     # year resolution client + rate limiter
│       ├── years/           # normalisation, resolution orchestration, cache
│       ├── exclusions/      # exclusion queries
│       ├── admin/           # admin endpoints
│       ├── store/           # pgx queries + migrations/*.sql
│       └── tokens/          # JWT issue/verify, media token HMAC
├── media/
│   ├── Dockerfile
│   ├── go.mod
│   └── cmd/media/main.go
│   └── internal/{cache,upstream,tokens,config}/
└── frontend/
    ├── Dockerfile
    ├── nginx.conf
    ├── package.json
    ├── vite.config.ts
    ├── tailwind.config.ts
    └── src/
        ├── main.tsx, App.tsx
        ├── api/            # REST client
        ├── ws/             # socket client, clock sync, reconnect
        ├── audio/          # synchronised player
        ├── store/          # zustand slices
        ├── i18n/           # en.json, de.json
        ├── components/
        ├── pages/
        └── styles/
```

---

## 6. Configuration (environment variables)

All variables live in a single `.env` consumed by `docker-compose.yml`. Ship a complete
`.env.example` with comments and safe defaults. The backend must **fail fast on startup** with a
clear message if any variable marked *required* is missing or obviously invalid (e.g. a secret
shorter than 16 characters).

### 6.1 General

| Variable | Required | Default | Notes |
|---|---|---|---|
| `APP_DOMAIN` | yes | — | e.g. `hitsync.example.com`. Used in Traefik labels and invite URLs |
| `MEDIA_DOMAIN` | yes | — | e.g. `hitsync-media.example.com` |
| `TRAEFIK_NETWORK` | no | `proxy` | Name of the existing external Traefik docker network |
| `TRAEFIK_ENTRYPOINT` | no | `websecure` | Entrypoint name on the existing Traefik |
| `TRAEFIK_CERTRESOLVER` | no | `letsencrypt` | Cert resolver name on the existing Traefik |
| `LOG_LEVEL` | no | `info` | `debug` / `info` / `warn` / `error` |
| `TZ` | no | `Europe/Berlin` | |

### 6.2 Secrets

| Variable | Required | Default | Notes |
|---|---|---|---|
| `APP_ACCESS_CODE` | yes | — | The shared code users type to enter the app |
| `ADMIN_PASSWORD` | yes | — | Separate password for the exclusion/admin area |
| `JWT_SECRET` | yes | — | ≥32 chars. Signs session and player tokens |
| `MEDIA_SHARED_SECRET` | yes | — | ≥32 chars. HMAC key shared by backend and media service for stream tokens |

### 6.3 Navidrome

| Variable | Required | Default | Notes |
|---|---|---|---|
| `NAVIDROME_URL` | yes | — | e.g. `http://navidrome:4533` — reachable from backend and media containers |
| `NAVIDROME_USERNAME` | yes | — | |
| `NAVIDROME_PASSWORD` | yes | — | Used with Subsonic salted-token auth; never sent in plaintext |
| `NAVIDROME_CLIENT_NAME` | no | `hitsync` | Subsonic `c` parameter |
| `NAVIDROME_TIMEOUT` | no | `30s` | |

### 6.4 Audio

| Variable | Required | Default | Notes |
|---|---|---|---|
| `AUDIO_FORMAT` | no | `mp3` | Passed to Navidrome's stream endpoint |
| `AUDIO_BITRATE` | no | `192` | kbit/s |
| `MEDIA_CACHE_DIR` | no | `/cache` | Inside the media container |
| `MEDIA_CACHE_MAX_BYTES` | no | `2147483648` | 2 GiB, LRU eviction |
| `MEDIA_DIRECT_PORT` | no | *(empty)* | If set, also publish this host port directly |
| `MEDIA_TLS_CERT_FILE` / `MEDIA_TLS_KEY_FILE` | no | *(empty)* | For direct-port HTTPS |

### 6.5 Library and year resolution

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
| `YEAR_MAX_BACKDATE` | no | `0` | `0` = off. If set, reject a MusicBrainz year more than N years earlier than Navidrome's |

### 6.6 Game rules and limits

| Variable | Required | Default | Notes |
|---|---|---|---|
| `MAX_CONCURRENT_GAMES` | no | `10` | |
| `MIN_PLAYERS` | no | `2` | |
| `MAX_PLAYERS` | no | `12` | |
| `DEFAULT_TARGET_CARDS` | no | `10` | Cards needed to win |
| `DEFAULT_START_TOKENS` | no | `2` | |
| `MAX_TOKENS` | no | `5` | Cap on hoarding |
| `RULE_ENABLE_SONG_GUESS` | no | `true` | Optional title/artist bonus round (§8.6) |
| `TURN_PLACEMENT_TIMEOUT` | no | `90s` | |
| `TURN_CHALLENGE_WINDOW` | no | `20s` | |
| `REVEAL_DURATION` | no | `8s` | |
| `TRACK_MIN_DURATION` | no | `45s` | Skip very short tracks |
| `TRACK_MAX_DURATION` | no | `600s` | Skip 20-minute prog epics |
| `PLAYER_RECONNECT_GRACE` | no | `120s` | |
| `LOBBY_IDLE_TIMEOUT` | no | `30m` | Empty/idle games are reaped |

### 6.7 Database

| Variable | Required | Default |
|---|---|---|
| `POSTGRES_USER` | no | `hitsync` |
| `POSTGRES_PASSWORD` | yes | — |
| `POSTGRES_DB` | no | `hitsync` |
| `DATABASE_URL` | no | derived from the three above + host `postgres` |

---

## 7. Data model

### 7.1 Postgres schema

Migrations live in `backend/internal/store/migrations/` as numbered `*.up.sql` / `*.down.sql`
pairs, embedded with `//go:embed` and applied automatically at backend startup.

```sql
-- 0001_init.up.sql

CREATE TABLE tracks (
    id              TEXT PRIMARY KEY,           -- Navidrome/Subsonic song id
    title           TEXT NOT NULL,
    artist          TEXT NOT NULL,
    artist_id       TEXT,
    album           TEXT,
    album_id        TEXT,
    navidrome_year  INT,
    duration_sec    INT NOT NULL,
    norm_title      TEXT NOT NULL,              -- see §9.2 normalisation
    norm_artist     TEXT NOT NULL,
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX tracks_norm_idx    ON tracks (norm_title, norm_artist);
CREATE INDEX tracks_artist_idx  ON tracks (artist_id);
CREATE INDEX tracks_album_idx   ON tracks (album_id);

-- MusicBrainz results are NOT stored. Years are resolved per turn and cached
-- only in the backend's memory (§9.5).

-- Manual corrections set by the admin. Highest precedence.
CREATE TABLE year_overrides (
    track_id    TEXT PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
    year        INT NOT NULL,
    note        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE exclusion_kind AS ENUM ('track', 'album', 'artist');

CREATE TABLE exclusions (
    kind        exclusion_kind NOT NULL,
    ref_id      TEXT NOT NULL,                  -- track id / album id / artist id
    label       TEXT NOT NULL,                  -- human-readable, for the admin list
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, ref_id)
);

-- Crash-recovery snapshots. Not on the gameplay hot path.
CREATE TABLE game_snapshots (
    game_id     TEXT PRIMARY KEY,
    invite_code TEXT NOT NULL,
    state       JSONB NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX game_snapshots_updated_idx ON game_snapshots (updated_at);

-- Finished games, for the admin stats page.
CREATE TABLE game_results (
    game_id     TEXT PRIMARY KEY,
    started_at  TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ NOT NULL,
    player_count INT NOT NULL,
    winner_name TEXT,
    turns_played INT NOT NULL
);
```

### 7.2 Eligible track pool

A track is eligible for selection when **all** of the following hold:

1. It exists in `tracks` with `last_seen_at` from the most recent successful library sync.
2. `duration_sec` is between `TRACK_MIN_DURATION` and `TRACK_MAX_DURATION`.
3. There is no `exclusions` row for the track id, its `album_id`, or its `artist_id`.
4. It has not already been used in the current game.

Note what is **not** in this list: eligibility says nothing about the year. Since years are resolved
per turn rather than pre-imported (§9.5), a track's usable year is unknown until it has been drawn.
A drawn candidate that resolves to no year from either source is discarded at that point and
another is drawn in its place. Tracks with no year tag in Navidrome therefore stay in the pool and
become playable whenever MusicBrainz can identify them, which is the opposite of the previous
design and slightly enlarges the usable library.

Implement this as a single SQL view `eligible_tracks` plus a parameterised query that takes the
array of already-used track ids and returns `N` random rows (`ORDER BY random() LIMIT n`). With
~5000 rows this is sub-millisecond and does not need optimising.

The backend keeps a small in-memory shuffled buffer of ~30 candidate track ids per game so drawing
never waits on the database mid-turn. Drawing from this buffer is separate from, and feeds, the
year-resolution pipeline in §9.5.

---

## 8. Game rules

### 8.1 Entities

- **Game** — one round/match. Identified by an opaque `gameId` (UUIDv4) and a human-facing
  `inviteCode`.
- **Player** — a display name plus a `playerId` (UUIDv4) bound to a player token stored in the
  browser. Players hold a **timeline** (ordered list of won cards) and a **token count**.
- **Card** — a track that has been revealed: `{ trackId, title, artist, year }`.

### 8.2 Invite codes

6 characters from Crockford base32 minus ambiguous glyphs: alphabet `23456789ABCDEFGHJKLMNPQRSTUVWXYZ`.
Uppercase, displayed grouped as `ABC-123`. Input is case-insensitive and strips hyphens/spaces.
Codes are unique among *active* games and are recycled once a game ends. Join URL format:
`https://{APP_DOMAIN}/j/{CODE}`.

### 8.3 Lobby

- The creating player is the **host**. Host powers: change settings, start the game, kick a player,
  skip the current track, end the game.
- If the host leaves, host status transfers to the longest-connected remaining player automatically.
- Lobby-configurable settings (defaults from env): `targetCards` (5–20), `startTokens` (0–5),
  `enableSongGuess` (bool).
- Display names: 2–20 characters after trimming, must be unique within the game
  case-insensitively, rejected if they contain control characters. Server assigns each player a
  colour from a fixed 12-colour palette, unique within the game.
- Seat order is the order of joining and is fixed once the game starts.

### 8.4 Game start

For each player in seat order, the server draws a track, resolves it, and places it into that
player's timeline as their **starting card**, revealed immediately and without audio. Starting
cards count toward the target. (So with `targetCards = 10`, a player needs 9 more.)

### 8.5 Turn structure

A turn moves through these phases. The server is the only authority on phase transitions and
broadcasts a full state snapshot on every one.

```
PREPARING ──▶ PLACING ──▶ CHALLENGING ──▶ REVEALING ──▶ (next turn | GAME_OVER)
```

**PREPARING** (typically <1 s, hard cap 8 s)
- Server picks the next eligible track and resolves its year.
- Server issues a media token and sends `track_prepare` to every client with the stream URL and
  duration.
- Clients buffer the beginning of the file and reply `ready`.
- When all connected clients are ready — or the 8 s cap elapses — the server computes
  `startAt = serverNowMs + 400` and broadcasts `track_start`.

**PLACING** (up to `TURN_PLACEMENT_TIMEOUT`)
- Audio plays, looping, on all clients.
- Only the **active player** may act. Their UI shows their own timeline with insertion slots.
- Other players see the active player's timeline, greyed, with a countdown.
- The active player submits `place_card { slotIndex }`. If `enableSongGuess` is on, the submission
  may also carry `titleGuess` and `artistGuess` (see §8.6).
- On timeout, the placement is recorded as `slotIndex = -1` (auto-fail).

**CHALLENGING** (`TURN_CHALLENGE_WINDOW`; skipped entirely if no other player has ≥1 token)
- Audio keeps looping.
- Every player other than the active player may spend **one token** to claim a *different* slot in
  the active player's timeline via `challenge { slotIndex }`.
- A given slot may be claimed by at most one challenger — first submission wins the slot; a second
  attempt on the same slot is rejected with an error and the token is not spent.
- The slot chosen by the active player may not be challenged.
- Players may explicitly `pass_challenge` to end their participation early. When every eligible
  player has challenged or passed, the phase ends immediately.

**REVEALING** (`REVEAL_DURATION`)
- Server broadcasts `track_stop`; clients fade audio out over 400 ms and stop.
- Server broadcasts the reveal: title, artist, year, the active player's slot, every challenge, and
  the resolved outcome.
- Cards animate into the winner's timeline.

### 8.6 Optional song guess

When `enableSongGuess` is on, the active player's placement UI also offers a "Name that tune" panel
containing two multiple-choice groups of four options each: one for the title, one for the artist.
The correct value is in each group; the three decoys are drawn at random from other tracks in the
library (decoy titles from other tracks, decoy artists from other artists, never equal to the
correct value).

Selecting is optional and costs nothing. On reveal:

- Both correct → +1 token (capped at `MAX_TOKENS`).
- Otherwise → nothing. There is no penalty.

Decoys are generated server-side during PREPARING and sent only to the active player.

### 8.7 Placement correctness

A timeline is an array of cards sorted ascending by year. For a timeline of length `n`, valid slot
indices are `0..n`, where slot `i` means "insert before the card currently at index `i`" (so slot
`n` means "after everything").

A placement of a card with year `y` at slot `i` is **correct** when:

```
(i == 0 || y >= timeline[i-1].year) && (i == n || y <= timeline[i].year)
```

Equal years are always acceptable. Where duplicate years create several equally-correct slots, any
of them is accepted — the check above handles this naturally.

### 8.8 Resolution

1. If the active player's slot is correct, they receive the card. Challenges are discarded (tokens
   are still spent).
2. If the active player's slot is wrong, challengers are evaluated in **seat order starting from
   the seat after the active player**, wrapping around. The first challenger whose slot is correct
   receives the card into their own timeline, and gets their spent token back.
3. If no one is correct, the card is discarded.
4. Tokens spent on unsuccessful challenges are gone.
5. The track is marked used for this game regardless of outcome.

### 8.9 Win condition

Immediately after resolution, if any player's timeline length ≥ `targetCards`, the game moves to
`GAME_OVER` and that player wins. If, through a challenge, two players would reach the target on
the same turn — impossible under these rules, since only one card is awarded per turn — the
question does not arise.

The final screen shows every player's timeline, card count and tokens. The host may press "Play
again", which returns everyone to the lobby with the same players and settings and a fresh
used-track set.

### 8.10 Disconnects and timeouts

- A disconnected player's slot on the board is kept; their name is shown dimmed with a "reconnecting"
  indicator.
- If the **active** player is disconnected when their turn begins, the PLACING phase runs with a
  shortened timeout of 20 s and then auto-fails.
- A player who does not reconnect within `PLAYER_RECONNECT_GRACE` is removed from the game. Their
  timeline is discarded.
- If removal would drop the player count below `MIN_PLAYERS`, the game ends with no winner and a
  "not enough players" notice.
- A player reconnecting with a valid player token is restored to their seat, timeline and tokens.

### 8.11 Host actions during play

- **Skip track** — abandons the current turn without scoring, marks the track used, and starts a
  new PREPARING phase for the same active player. Rate-limited to once per 10 s.
- **Kick player** — removes a player immediately.
- **End game** — jumps to `GAME_OVER` with no winner.

---

## 9. Music data

### 9.1 Navidrome (Subsonic API) client

Base: `${NAVIDROME_URL}/rest/`. Every request carries:

```
u = NAVIDROME_USERNAME
t = md5(NAVIDROME_PASSWORD + salt)
s = salt            (fresh 16-char random hex per request)
v = 1.16.1
c = NAVIDROME_CLIENT_NAME
f = json
```

The plaintext password is never placed in a query string.

Endpoints used:

| Purpose | Endpoint |
|---|---|
| Health check | `ping.view` |
| Library paging | `search3.view?query=""&songCount=500&songOffset=N&artistCount=0&albumCount=0` |
| Stream (media service only) | `stream.view?id=ID&format=mp3&maxBitRate=192` |

`search3` with an empty query returns the whole library in pages; ~10 requests for 5000 songs.

### 9.2 Library sync

Runs at startup and every `LIBRARY_SYNC_INTERVAL`, in a background goroutine with a jittered timer.

1. Page through the library, collecting `id, title, artist, artistId, album, albumId, year,
   duration`.
2. Upsert each row into `tracks`, computing `norm_title` and `norm_artist`, and setting
   `last_seen_at = <sync start timestamp>`.
3. After a **fully successful** pass, delete rows whose `last_seen_at` is older than the sync start
   (they disappeared from the library). On a partial/failed pass, delete nothing.
4. Log a summary: added / updated / removed counts and duration.

**Normalisation** (used for both `norm_*` columns and MusicBrainz lookup keys), applied in this
order:

1. Unicode NFKD, strip combining marks (`é` → `e`).
2. Lowercase.
3. Remove bracketed suffixes: `(...)`, `[...]`, `{...}` — but only when the bracket content matches
   a noise pattern: `remaster`, `remastered`, `live`, `mono`, `stereo`, `radio edit`, `single
   version`, `album version`, `bonus`, `deluxe`, `explicit`, `clean`, a bare 4-digit year, or
   anything starting with `feat`/`ft`/`featuring`/`with`.
4. Remove trailing ` - <noise>` segments matching the same patterns.
5. Strip a leading `the ` from artists.
6. Replace `&` with `and`.
7. Remove all characters outside `[a-z0-9 ]`.
8. Collapse whitespace runs to a single space and trim.

For artists, additionally split on `;`, `/`, `,`, ` feat `, ` ft ` and keep only the first segment —
the primary artist. This markedly improves MusicBrainz hit rates.

### 9.3 MusicBrainz year resolution

MusicBrainz is chosen because it is free, requires no API key, has excellent coverage, and — most
importantly — models *release groups* with a `first-release-date`, which is exactly "when this
recording first came out" rather than "when this compilation was pressed".

Years are resolved **on the fly, per turn**. There is no bulk import and no persistent year table:
nothing MusicBrainz returns is written to Postgres. See §9.5 for how the latency is hidden.

**Rate limiting is mandatory.** A single global token-bucket limiter at `MUSICBRAINZ_RATE_PER_SEC`
(default 1/s) guards every outbound call, shared across all games and all lookups.
`User-Agent` must be exactly:

```
Hitsync/1.0 ( ${MUSICBRAINZ_CONTACT} )
```

Retry on HTTP 503 with exponential backoff (1 s, 2 s, 4 s, then give up). Never retry 404.

**Resolution algorithm** for a normalised `(title, artist)` pair:

1. **Search** —
   `GET /recording?query=recording:"{title}" AND artist:"{artist}"&fmt=json&limit=8`
   Quote and escape Lucene special characters in the interpolated values.
2. Discard recordings with `score < MUSICBRAINZ_MIN_SCORE` (default 90).
3. **Verify the artist.** Discard any recording whose `artist-credit` entries, normalised by the
   §9.2 rules, do not contain the track's normalised artist. This guard matters more than it used
   to: under the earliest-wins rule of §9.4 a wrong match can only ever drag the year *earlier*, so
   a cover version or a same-titled unrelated song would silently produce an unwinnable card.
   Rejecting on artist mismatch is the cheapest defence against that.
4. Take up to the top 3 remaining recordings. For each, in order:
   1. `GET /recording/{mbid}?inc=releases+release-groups&fmt=json`
   2. Collect every distinct release group across the returned releases.
   3. **Filter out** any release group whose `secondary-types` array intersects
      `{Compilation, Live, Remix, DJ-mix, Mixtape/Street, Interview, Soundtrack, Demo}`.
      This is the step that prevents "Greatest Hits 2004" from poisoning a 1971 song.
   4. From the surviving release groups, take the minimum `first-release-date`.
   5. If **no** release group survives the filter, fall back to the minimum `first-release-date`
      across *all* release groups for that recording, but mark the result
      `source = 'musicbrainz_loose'` so it can be flagged in logs and in the admin lookup tool.
5. Take the earliest year found across the candidate recordings.
6. Sanity check: reject years < 1860 or > current year + 1.
7. Return the year, or "not found". Nothing is written to the database.

### 9.4 Combining the two sources

For a given track the gameplay year is determined as follows:

1. If a `year_overrides` row exists for the track id, use it and stop. Manual admin corrections beat
   everything.
2. Otherwise let `nd` = `tracks.navidrome_year` (may be absent) and `mb` = the MusicBrainz result
   (may be absent).
   - Both present → **use `min(nd, mb)`**.
   - Only one present → use it.
   - Neither present → the track cannot be used; discard it and draw another (§9.5).

Taking the earlier of the two is the right default because year errors in this domain are
overwhelmingly *too late*, never too early: a remaster, a reissue, a "Greatest Hits" pressing and a
mistagged file all push the year forward. The one case it gets wrong is a MusicBrainz mismatch onto
an older recording of the same title — which is exactly what the artist verification in §9.3 step 3
is there to prevent.

An optional guard is available for the remaining tail. `YEAR_MAX_BACKDATE` (default `0` = disabled)
rejects a MusicBrainz year that is more than N years earlier than Navidrome's. Leave it off unless
you see specific bad cards; a setting tight enough to catch a cover-version mismatch would also
reject legitimate 30-plus-year compilation corrections, which are the main thing this whole
mechanism exists to fix.

The card shown at reveal displays the year plus a small source indicator (`MusicBrainz` /
`Library` / `Both` / `Manual`), where `Both` means the two sources agreed. When they disagreed, the
tooltip shows both values. This makes bad data obvious during real play, which is when you actually
notice it.

### 9.5 On-demand resolution and lookahead

Resolution happens per turn, but it must never be something players wait on. The global 1 req/s
limit means a naive "resolve when the turn starts" design would stall the PREPARING phase for
2–4 seconds in the best case, and far longer when several games change turns at once — ten
concurrent games needing two requests each is twenty seconds of queue.

**Lookahead solves this.** Each game keeps a small pipeline of resolved candidate tracks:

1. When a game starts, the manager draws `YEAR_LOOKAHEAD_DEPTH` (default 2) candidate tracks and
   resolves them in the background.
2. The moment a turn's track is consumed, the manager draws and resolves a replacement, so
   resolution for turn *n+1* runs during the whole of turn *n* — typically 60–120 seconds of cover
   for work that takes 2–4 seconds.
3. PREPARING takes the head of the pipeline, which is already resolved, and proceeds immediately.
4. If the pipeline is empty (start of game, or an unusually fast turn), PREPARING waits on the
   in-flight resolution up to `YEAR_LOOKUP_TIMEOUT` (default `6s`) and then falls back to the
   Navidrome year alone. The turn is never blocked beyond that.
5. If a candidate resolves to no year from either source, it is discarded and another is drawn, up
   to 5 attempts per slot.

**In-memory cache only.** A process-local LRU keyed on normalised `(title, artist)` holds
`MUSICBRAINZ_CACHE_ENTRIES` (default 2000) results for `MUSICBRAINZ_CACHE_TTL` (default `24h`),
including negative results. It exists purely so that repeated draws within a session — and the same
popular track coming up in two concurrent games — do not re-query MusicBrainz. It holds a few
hundred kilobytes, is never written to disk, and is empty again after a restart.

Startup does no MusicBrainz work at all. The first game is playable as soon as the library sync
finishes.

---

## 10. Audio: synchronised playback

### 10.1 Media service

A small, stateless-apart-from-its-cache Go HTTP service.

**Endpoint:** `GET /stream/{trackId}?token={token}`

1. Verify the token (§10.2). Reject with 401 on failure or expiry.
2. Look for `{MEDIA_CACHE_DIR}/{trackId}.mp3`.
   - **Hit:** serve it with `http.ServeContent` — this gives Range support, `ETag`, and
     `Content-Length` for free. Touch the LRU entry.
   - **Miss:** request
     `{NAVIDROME_URL}/rest/stream.view?id={trackId}&format=mp3&maxBitRate=192&...auth...`,
     stream the response body simultaneously to a temp file and to the client, then atomically
     rename the temp file into the cache on successful completion. A failed or truncated fetch
     deletes the temp file and never populates the cache.
   - **Concurrent miss:** a per-trackId singleflight ensures only one upstream fetch happens.
     Additional requesters for an in-flight track wait for completion and are then served from the
     finished cache file. Since the backend sends `track_prepare` to all clients at the same
     instant, this is the common case and must be handled correctly.
3. Response headers: `Content-Type: audio/mpeg`, `Accept-Ranges: bytes`,
   `Cache-Control: private, max-age=3600`, `Access-Control-Allow-Origin: https://{APP_DOMAIN}`,
   `Cross-Origin-Resource-Policy: cross-origin`.

**Cache eviction:** on write, if total cache size exceeds `MEDIA_CACHE_MAX_BYTES`, delete
least-recently-accessed files until under 90% of the cap. Access times tracked in an in-memory map,
rebuilt from file mtimes on startup.

**Also exposes:** `GET /healthz` (200 if the cache dir is writable) and `GET /time` returning
`{"serverTimeMs": <unix millis>}` — unused by default but handy for debugging.

### 10.2 Stream tokens

The backend mints a token per (game, track); the media service verifies it. Neither service needs
to talk to the other.

```
payload = base64url(JSON{ "t": trackId, "g": gameId, "exp": unixSeconds })
sig     = base64url(HMAC_SHA256(MEDIA_SHARED_SECRET, payload))
token   = payload + "." + sig
```

Expiry: 30 minutes. The media service checks the signature, the expiry, and that `t` matches the
path parameter. Constant-time comparison for the signature.

### 10.3 Clock synchronisation

The client estimates its offset from the backend's clock over the existing WebSocket, NTP-style.

- Client sends `ping { c0: clientNowMs }`.
- Server replies `pong { c0, s: serverNowMs }` immediately, handled before any other queued work.
- On receipt at `c1`, the client computes `rtt = c1 - c0` and `offset = s - (c0 + rtt/2)`.
- Take 7 samples 120 ms apart on connect; keep the offset from the sample with the **lowest RTT**
  (min-RTT filtering is far more robust than averaging).
- Re-sample 3 times every 30 s thereafter and update the offset if the new min-RTT sample is better
  than the stored one or the stored one is older than 2 minutes.
- `serverNow() = Date.now() + offset`.

### 10.4 Playback protocol

```
server → all:  track_prepare { trackId, streamUrl, durationMs, prepareId }
client → server: ready { prepareId }
server → all:  track_start { prepareId, startAtServerMs, durationMs }
server → all:  track_stop   { fadeMs: 400 }
```

**Client behaviour on `track_prepare`:**
1. Create a fresh `HTMLAudioElement` with `preload="auto"`, `loop=true`, `crossOrigin="anonymous"`,
   `src = streamUrl`.
2. Wait for `canplaythrough`, or `canplay` plus 1.5 s, whichever comes first.
3. Send `ready`.

**Client behaviour on `track_start`:**
1. Compute `elapsed = serverNow() - startAtServerMs`.
2. If `elapsed < 0`, wait `elapsed` ms (via a `setTimeout` that ends with a short
   `requestAnimationFrame` spin for the last 20 ms, for millisecond-accurate firing), then
   `audio.currentTime = 0; audio.play()`.
3. If `elapsed >= 0` (late joiner, or a client that reconnected mid-turn), set
   `audio.currentTime = (elapsed % durationMs) / 1000` and play immediately.

**Drift correction**, on a 5 s interval while playing:
```
expectedSec = ((serverNow() - startAtServerMs) % durationMs) / 1000
driftMs     = (audio.currentTime - expectedSec) * 1000

|drift| < 60ms    → do nothing
60–350 ms         → audio.playbackRate = drift > 0 ? 0.985 : 1.015, until |drift| < 25 ms,
                    then reset to 1.0   (inaudible pitch shift, smooth correction)
> 350 ms          → hard seek to expectedSec
```

**Looping.** `loop=true` on the media element gives a seamless restart from the browser's buffer
with no network request. The modulo arithmetic above means a client that corrects across a loop
boundary lands in the right place automatically. Note that MP3 encoder padding adds a few tens of
milliseconds of silence at the loop point; this is acceptable and must not be worked around with a
Web Audio double-buffer scheme, which would cost far more memory and complexity than it is worth
here.

**Volume.** Purely local: `audio.volume` (0–1, stored in `localStorage`) and `audio.muted`. Muting
sets `muted = true` and never touches `paused`, so a muted client stays perfectly in sync and
un-muting is instantaneous.

**Autoplay policy.** Browsers block audio before a user gesture. The join flow includes an explicit
"Join game" button press, which satisfies the gesture requirement; the client primes an
`AudioContext` and plays a muted 50 ms silent buffer at that moment. If `play()` is nevertheless
rejected, show a persistent, unmissable "Tap to enable sound" overlay and retry on the next click.
The game continues for everyone else regardless.

### 10.5 Explicitly not doing

No server-side mixing, no Icecast, no WebRTC/SFU, no Web Audio scheduling graph, no HLS/LL-HLS.
Each adds latency, CPU, or complexity without improving the experience for remote players who
cannot hear each other's speakers.

---

## 11. Authentication and authorisation

Three separate, unrelated credentials. None of them is a user account.

### 11.1 App access gate

`POST /api/auth/access { "code": "..." }`

- Compares against `APP_ACCESS_CODE` in constant time.
- On success, sets an httpOnly, `Secure`, `SameSite=Lax` cookie `hs_access` containing a JWT with
  `{ scope: "app", exp: +12h }`.
- Rate limited: 10 attempts per IP per minute, then 429 with a `Retry-After`. A simple in-memory
  sliding-window limiter is sufficient.
- Every `/api/*` route except `/api/auth/*` and `/healthz` requires this cookie.

### 11.2 Player identity

Returned when creating or joining a game:

```json
{ "gameId": "...", "playerId": "...", "playerToken": "<JWT>", "inviteCode": "ABC123" }
```

The player token is a JWT `{ gameId, playerId, exp: +6h }`, stored in `localStorage` under
`hs_player_{gameId}`. It is sent in the WebSocket `hello` message and is what makes reconnection
work. Possession of the token *is* the identity; there is no password.

### 11.3 Admin

`POST /api/admin/login { "password": "..." }` → httpOnly cookie `hs_admin`, JWT
`{ scope: "admin", exp: +4h }`. Constant-time comparison, same rate limiting as the access gate.
All `/api/admin/*` routes require it. The admin cookie does **not** grant the app scope and vice
versa — `/admin` is reachable without knowing `APP_ACCESS_CODE`, and knowing the access code grants
nothing in `/admin`.

### 11.4 Baseline hardening

- All inputs length-checked and type-checked before use; JSON bodies capped at 32 KB.
- Every string rendered in the UI goes through React's default escaping. No `dangerouslySetInnerHTML`.
- WebSocket messages capped at 8 KB; a client exceeding 30 messages/second is disconnected.
- Origin check on the WebSocket upgrade against `APP_DOMAIN`.
- CORS on the API restricted to `https://{APP_DOMAIN}`.
- Security headers from nginx: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: same-origin`, and a CSP allowing `self`, the media domain in `media-src`, and
  `connect-src` for the API/WS origin.
- Containers run as a non-root user; Go images are `FROM scratch` or distroless with a static
  binary.
- Postgres is **not** published to the host; it is reachable only on the internal compose network.
- No secret is ever logged. The config loader redacts secret-typed fields in its startup dump.

---

## 12. HTTP API

All responses are JSON. Errors use:

```json
{ "error": { "code": "invalid_invite_code", "message": "No game found with that code." } }
```

`code` is a stable machine-readable string; the frontend maps it to a translated message and only
falls back to `message` for unknown codes.

### 12.1 Public / access

| Method | Path | Body | Response |
|---|---|---|---|
| `POST` | `/api/auth/access` | `{ code }` | `{ ok: true }` + cookie |
| `POST` | `/api/auth/logout` | — | `{ ok: true }` |
| `GET` | `/api/config` | — | `{ minPlayers, maxPlayers, defaultTargetCards, defaultStartTokens, songGuessAvailable, mediaBaseUrl }` |
| `GET` | `/healthz` | — | `{ status, db, navidrome, libraryTracks, eligibleTracks }` |

### 12.2 Games (require app scope)

| Method | Path | Body | Response |
|---|---|---|---|
| `POST` | `/api/games` | `{ displayName, settings? }` | player identity payload |
| `POST` | `/api/games/join` | `{ inviteCode, displayName }` | player identity payload |
| `GET` | `/api/games/{inviteCode}/preview` | — | `{ exists, phase, playerCount, maxPlayers, hostName, joinable }` |

`preview` is what the `/j/{CODE}` landing page calls to show "Anna's game — 4/12 players" before the
name is entered. It leaks nothing beyond that.

### 12.3 Admin (require admin scope)

| Method | Path | Notes |
|---|---|---|
| `POST` | `/api/admin/login` | |
| `POST` | `/api/admin/logout` | |
| `GET` | `/api/admin/stats` | library size, eligible pool size, active games, last library sync, MusicBrainz cache hit rate and in-flight queue depth |
| `GET` | `/api/admin/tracks?q=&excluded=&page=&pageSize=` | Searches title/artist/album. Returns the Navidrome year, any manual override, and whether the track is excluded and by which rule (track/album/artist). It does **not** trigger MusicBrainz lookups — listing a page must never fire 50 rate-limited requests |
| `GET` | `/api/admin/artists?q=` / `/api/admin/albums?q=` | For excluding at those levels |
| `POST` | `/api/admin/exclusions` | `{ kind, refId, label, reason? }` |
| `DELETE` | `/api/admin/exclusions/{kind}/{refId}` | |
| `GET` | `/api/admin/exclusions?kind=&page=` | |
| `PUT` | `/api/admin/year-overrides/{trackId}` | `{ year, note? }` |
| `DELETE` | `/api/admin/year-overrides/{trackId}` | |
| `POST` | `/api/admin/resync-library` | Triggers an immediate library sync |
| `POST` | `/api/admin/resolve-year/{trackId}` | On-demand lookup for one track, bypassing the in-memory cache. Returns the Navidrome year, the MusicBrainz year, the combined result and the matched release group, so a suspicious card can be diagnosed and then corrected with an override |
| `GET` | `/api/admin/games` | Active games: invite code, phase, players, turn count. No player names shown beyond what is needed |
| `DELETE` | `/api/admin/games/{gameId}` | Force-end a game |

---

## 13. WebSocket protocol

**Endpoint:** `wss://{APP_DOMAIN}/ws`

Envelope in both directions:

```json
{ "type": "string", "payload": { } }
```

### 13.1 Client → server

| Type | Payload | Notes |
|---|---|---|
| `hello` | `{ playerToken }` | Must be the first message, within 5 s of connect, or the socket is closed |
| `ping` | `{ c0 }` | Clock sync; answered on a fast path |
| `ready` | `{ prepareId }` | |
| `update_settings` | `{ targetCards?, startTokens?, enableSongGuess? }` | Host only, lobby only |
| `start_game` | `{}` | Host only |
| `place_card` | `{ slotIndex, titleGuess?, artistGuess? }` | Active player only, PLACING only |
| `challenge` | `{ slotIndex }` | Non-active players, CHALLENGING only |
| `pass_challenge` | `{}` | |
| `skip_track` | `{}` | Host only |
| `kick_player` | `{ playerId }` | Host only |
| `end_game` | `{}` | Host only |
| `play_again` | `{}` | Host only, GAME_OVER only |
| `leave` | `{}` | Graceful departure |

### 13.2 Server → client

| Type | Payload |
|---|---|
| `pong` | `{ c0, s }` |
| `state` | Full game state snapshot (§13.3) |
| `track_prepare` | `{ prepareId, trackId, streamUrl, durationMs, guessOptions? }` |
| `track_start` | `{ prepareId, startAtServerMs, durationMs }` |
| `track_stop` | `{ fadeMs }` |
| `reveal` | `{ card, activePlacement, challenges[], outcome, tokenChanges[], songGuessResult? }` |
| `error` | `{ code, message }` |
| `kicked` | `{ reason }` |

`guessOptions` is sent **only to the active player's socket** and contains
`{ titles: [4 strings], artists: [4 strings] }` in randomised order.

The `state` message is a complete snapshot rather than a patch. Game state is small (a dozen
players with a dozen cards each is under 8 KB of JSON), snapshots make reconnection trivial, and
they eliminate an entire category of desync bugs. Broadcast on every transition, never on a timer.

### 13.3 State snapshot shape

```jsonc
{
  "gameId": "…",
  "inviteCode": "ABC123",
  "phase": "LOBBY|PREPARING|PLACING|CHALLENGING|REVEALING|GAME_OVER",
  "settings": { "targetCards": 10, "startTokens": 2, "enableSongGuess": true },
  "hostId": "…",
  "youId": "…",
  "activePlayerId": "…",
  "turnNumber": 7,
  "phaseEndsAtServerMs": 1737000000000,   // null when the phase has no deadline
  "players": [
    {
      "id": "…", "name": "Anna", "colour": "#e8734a",
      "connected": true, "tokens": 2,
      "timeline": [ { "trackId": "…", "title": "…", "artist": "…", "year": 1979 } ],
      "pendingChallengeSlot": 3          // null unless visible in this phase
    }
  ],
  "currentTurn": {
    "activePlacementSubmitted": true,
    "activePlacementSlot": null,          // hidden from others until REVEALING
    "challengeSlotsTaken": [1, 4],
    "hasPassed": ["playerId", "…"]
  },
  "winnerId": null,
  "tracksUsed": 23
}
```

**Information hiding is a server responsibility.** The snapshot is serialised per-recipient: the
active player's chosen slot is stripped from everyone else's copy until REVEALING, and challenge
slot owners are not revealed until REVEALING (only which slots are taken). Never rely on the client
to hide anything.

### 13.4 Reconnection

The client reconnects with exponential backoff (500 ms, 1 s, 2 s, 4 s, capped at 10 s, with ±20%
jitter), replays `hello` with the stored player token, redoes clock sync, and applies the fresh
`state`. If the current phase involves audio, the server includes the active `track_prepare` /
`track_start` data in the post-`hello` burst so the reconnecting client rejoins the loop at the
correct offset.

---

## 14. Frontend

### 14.1 Routes

| Route | Screen |
|---|---|
| `/` | Access gate if no session, otherwise Home |
| `/j/:code` | Join landing — shows the game preview, asks for a display name |
| `/game/:gameId` | Lobby / game board / game over (driven by `phase`) |
| `/admin` | Admin login, then the admin console |

### 14.2 Screens

**Access gate.** Full-bleed dark background with a subtle animated gradient. Centred card: logo,
one large code input (auto-uppercase, monospace, generous letter-spacing), submit button, language
toggle in the corner. Shake animation plus a translated error on rejection.

**Home.** Two large side-by-side cards: *Host a game* (name input → Create) and *Join a game* (name
+ invite code → Join). Below, a small "Recent games" list from `localStorage` offering one-click
rejoin for still-valid player tokens.

**Join landing (`/j/:code`).** Shows who is hosting and how full the game is, then a single name
field. If the game is full, already in progress, or gone, show a clear translated explanation and a
link Home.

**Lobby.** Left: player list with colour chips, host crown, connection dots. Right: settings panel
(editable for the host, read-only otherwise) and the invite block — the code in very large
characters, a copy-link button, and a QR code (generated client-side with `qrcode`) so phone
players can join by camera. Start button is disabled with a reason tooltip until
`MIN_PLAYERS` is reached.

**Game board.** This is the screen that must feel good.

- **Top bar:** turn number, whose turn it is, a circular countdown ring for the current phase,
  volume slider and mute button, and a small connection/sync indicator.
- **Centre:** the card being played, face down, pulsing gently in time with a lightweight
  waveform-ish animation (pure CSS, no audio analysis — see §14.4). Underneath, the active
  player's timeline rendered as a horizontal row of year cards with **drop slots between them**.
- **Placement interaction:** click or tap a slot to select it (primary), with drag-and-drop of the
  face-down card as an equivalent alternative. Selecting highlights the slot; a confirm button
  commits. Keyboard: arrow keys move the selection, Enter commits.
- **Timeline overflow:** with 10+ cards the row scrolls horizontally with edge fades; the selected
  slot is always scrolled into view. Cards shrink to a compact variant beyond 8.
- **Challenge phase:** non-active players get the same timeline with claimable slots, each labelled
  with its cost (one token icon). Taken slots show the claiming state without revealing who. A
  prominent "Pass" button.
- **Right rail:** all players as compact rows — name, colour, card count, token icons, mini
  timeline preview on hover.
- **Reveal:** the centre card flips (3D transform) to show album-less but bold typography — title,
  artist, and the year at large size — then the card animates into the winner's timeline while the
  correct slot is highlighted in green and wrong ones in red.

**Game over.** Winner announcement with a restrained confetti burst (canvas, 2 s, respects
`prefers-reduced-motion`), then every player's full timeline laid out for review, and "Play again"
for the host.

**Admin console.** Tabbed: *Overview* (stats, library sync status, MusicBrainz cache and queue
stats, resync button), *Library* (searchable, paginated table showing the Navidrome year and any
override, with an exclude toggle per row, an inline year-override editor, and a per-row "look up
year" button that calls `/api/admin/resolve-year` and shows both sources side by side),
*Exclusions* (the current list grouped by kind, with removal), *Games* (active
games with a force-end action). Functional and dense rather than styled like the game.

### 14.3 Visual design

- **Dark by default.** Background `#0b0d12`, raised surfaces `#151922`, borders
  `rgba(255,255,255,0.08)`.
- **Accent** `#e8734a` (warm amber-orange), success `#3fb984`, danger `#e05263`.
- **Type:** Inter (variable, self-hosted via `@fontsource-variable/inter`; no external font CDN,
  which keeps the CSP tight). Years and counters use `font-variant-numeric: tabular-nums`. Big
  display numbers at 600 weight with tightened tracking.
- **Cards** use 12 px radius, a 1 px light border and a soft inner highlight on top
  (`inset 0 1px 0 rgba(255,255,255,0.06)`) rather than heavy drop shadows.
- **Motion:** 150–250 ms, `cubic-bezier(0.2, 0.8, 0.2, 1)`. All animation on `transform` and
  `opacity` only. Respect `prefers-reduced-motion: reduce` by cutting durations to 0 and disabling
  confetti and the pulse.
- **Responsive:** desktop-first at `≥1024px` with the three-region layout. Below that, the right
  rail collapses into a bottom sheet and the timeline becomes a vertical scroll list. Touch targets
  minimum 44 px.

### 14.4 Performance rules

These are requirements, not suggestions, because R18 asks for no stutter:

- Countdown timers render from a single `requestAnimationFrame` loop that writes to a CSS custom
  property, not from React state. A ticking `setState` at 60 Hz is forbidden.
- The pulsing/waveform animation is a pure CSS keyframe animation. **Do not** use
  `AnalyserNode`/`getByteFrequencyData` — real spectrum analysis would force the audio through a
  Web Audio graph for a purely decorative effect.
- `state` snapshots are shallow-compared into zustand slices so unaffected components do not
  re-render. Player rows are memoised on their own player object.
- The drift-correction interval is 5 s and does nothing when the drift is under 60 ms.
- Production bundle target: under 250 KB gzipped for the main chunk. The admin console and the
  confetti library are lazy-loaded route chunks.
- Images: none required. Icons are inline SVG.

### 14.5 Internationalisation

- `react-i18next`, two resource files: `src/i18n/en.json` and `src/i18n/de.json`.
- Detection order: `localStorage` → `navigator.language` → fallback `en`.
- Toggle (EN/DE) present on the gate, home, lobby and in the game's top bar; persisted.
- Every user-visible string goes through `t()`. No literal English in JSX.
- Use i18next pluralisation for counts ("1 Karte" / "5 Karten"), and interpolation for names.
- Both files must contain identical key sets. Add a unit test that asserts this (§16).
- Dates/years are plain integers; no locale date formatting is needed.
- German copy should use informal "du" throughout — it is a party game.

---

## 15. Docker and deployment

### 15.1 Dockerfiles

**`backend/Dockerfile`** — multi-stage:
```
FROM golang:1.23-alpine AS build   → CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath
FROM gcr.io/distroless/static-debian12:nonroot
```
Exposes 8080. Includes `ca-certificates` from the builder (needed for HTTPS to MusicBrainz).

**`media/Dockerfile`** — identical pattern, exposes 8090, declares `VOLUME /cache`.

**`frontend/Dockerfile`**:
```
FROM node:22-alpine AS build  → npm ci && npm run build
FROM nginx:1.27-alpine        → copy dist + nginx.conf
```
`nginx.conf`: SPA fallback (`try_files $uri /index.html`), gzip and brotli-ready static serving,
long cache headers on hashed assets, `no-store` on `index.html`, and the security headers from
§11.4.

Build-time variables `VITE_API_BASE` and `VITE_MEDIA_BASE` are injected as Docker build args.
Alternatively — and preferably — the frontend reads them at runtime from `/api/config`, so the
image is environment-independent. Implement the runtime approach; `mediaBaseUrl` is already in the
config endpoint for this reason.

### 15.2 `docker-compose.yml`

```yaml
name: hitsync

networks:
  proxy:
    external: true
    name: ${TRAEFIK_NETWORK:-proxy}
  internal:
    driver: bridge

volumes:
  pgdata:
  mediacache:

services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-hitsync}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?required}
      POSTGRES_DB: ${POSTGRES_DB:-hitsync}
    volumes: [pgdata:/var/lib/postgresql/data]
    networks: [internal]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-hitsync}"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits: { memory: 512M }

  backend:
    build: ./backend
    restart: unless-stopped
    env_file: [.env]
    depends_on:
      postgres: { condition: service_healthy }
    networks: [internal, proxy]
    healthcheck:
      test: ["CMD", "/app/server", "-healthcheck"]
      interval: 30s
      timeout: 5s
      retries: 3
    deploy:
      resources:
        limits: { memory: 512M, cpus: "2" }
    labels:
      - traefik.enable=true
      - traefik.docker.network=${TRAEFIK_NETWORK:-proxy}
      - traefik.http.routers.hitsync-api.rule=Host(`${APP_DOMAIN}`) && (PathPrefix(`/api`) || PathPrefix(`/ws`))
      - traefik.http.routers.hitsync-api.entrypoints=${TRAEFIK_ENTRYPOINT:-websecure}
      - traefik.http.routers.hitsync-api.tls.certresolver=${TRAEFIK_CERTRESOLVER:-letsencrypt}
      - traefik.http.routers.hitsync-api.priority=20
      - traefik.http.services.hitsync-api.loadbalancer.server.port=8080

  media:
    build: ./media
    restart: unless-stopped
    env_file: [.env]
    volumes: [mediacache:/cache]
    networks: [internal, proxy]
    deploy:
      resources:
        limits: { memory: 256M, cpus: "1" }
    labels:
      - traefik.enable=true
      - traefik.docker.network=${TRAEFIK_NETWORK:-proxy}
      - traefik.http.routers.hitsync-media.rule=Host(`${MEDIA_DOMAIN}`)
      - traefik.http.routers.hitsync-media.entrypoints=${TRAEFIK_ENTRYPOINT:-websecure}
      - traefik.http.routers.hitsync-media.tls.certresolver=${TRAEFIK_CERTRESOLVER:-letsencrypt}
      - traefik.http.services.hitsync-media.loadbalancer.server.port=8090
      # stream bytes through instead of buffering them
      - traefik.http.middlewares.hitsync-nobuffer.buffering.maxResponseBodyBytes=0
      - traefik.http.routers.hitsync-media.middlewares=hitsync-nobuffer

  frontend:
    build: ./frontend
    restart: unless-stopped
    networks: [proxy]
    deploy:
      resources:
        limits: { memory: 128M }
    labels:
      - traefik.enable=true
      - traefik.docker.network=${TRAEFIK_NETWORK:-proxy}
      - traefik.http.routers.hitsync-web.rule=Host(`${APP_DOMAIN}`)
      - traefik.http.routers.hitsync-web.entrypoints=${TRAEFIK_ENTRYPOINT:-websecure}
      - traefik.http.routers.hitsync-web.tls.certresolver=${TRAEFIK_CERTRESOLVER:-letsencrypt}
      - traefik.http.routers.hitsync-web.priority=10
      - traefik.http.services.hitsync-web.loadbalancer.server.port=80
```

Notes for the implementer:

- Router **priority** matters: the API router (20) must outrank the catch-all frontend router (10)
  on the same host, or `/api` requests land on nginx.
- Traefik must already have a WebSocket-capable entrypoint. No special label is needed for
  WebSockets in Traefik v2/v3; upgrades pass through automatically.
- `backend` and `media` need to reach `NAVIDROME_URL`. If Navidrome runs in another compose stack
  on the same host, either put it on a shared network or point `NAVIDROME_URL` at its published
  address. Document both options in the README.
- If `MEDIA_DIRECT_PORT` is set, add `ports: ["${MEDIA_DIRECT_PORT}:8090"]` to the media service.
  Document the mixed-content caveat prominently.

### 15.3 Expected footprint at idle

Roughly 60 MB (backend) + 25 MB (media) + 40 MB (nginx) + 120 MB (Postgres) ≈ 250 MB RAM, and
effectively 0% CPU between turns. Under full load (10 games, 120 clients) expect well under 1 vCPU
and under 600 MB.

---

## 16. Testing

Deliberately modest, per R20. No e2e tests, no browser automation, no testcontainers.

**Go — `internal/game` (the pure rules engine) is where the real coverage goes:**
- `TestPlacementCorrectness` — table-driven: empty timeline, single card, boundaries, exact-year
  matches, duplicate years, invalid slot indices.
- `TestChallengeResolution` — active correct (challenges ignored), active wrong with one correct
  challenger, multiple correct challengers (seat order wins), no correct challenger, wrap-around
  seat ordering.
- `TestTokenAccounting` — spend on challenge, refund on success, `MAX_TOKENS` cap, cannot challenge
  with zero tokens, cannot claim a taken slot or the active player's slot.
- `TestWinCondition` and `TestTurnRotation` including rotation past removed players.
- `TestPhaseTransitions` — timeouts, skipping the challenge phase when nobody has tokens.

**Go — other packages:**
- `internal/years`: normalisation table tests (the §9.2 rules, including the `feat.` and
  `(Remastered 2011)` cases), and the combiner from §9.4 — both sources present and MusicBrainz
  earlier, both present and Navidrome earlier, equal, each one missing, both missing, an override
  present, and `YEAR_MAX_BACKDATE` both off and triggering.
- `internal/musicbrainz`: parsing against 4–5 checked-in JSON fixtures, including one where the
  only releases are compilations, one where a compilation must be filtered out in favour of an
  earlier original, and one where a same-titled recording by a different artist must be rejected by
  the artist verification in §9.3 step 3. Also test that the rate limiter serialises concurrent
  callers and that the in-memory cache returns a second lookup without a second HTTP call. Uses
  `httptest.Server`; never hits the network.
- `internal/tokens`: media token round-trip, tampering rejection, expiry.
- `internal/httpapi`: a handful of handler tests with `httptest` and a fake store.

**Frontend — `vitest`, a small set:**
- `i18n`: `en.json` and `de.json` have identical key sets, and no value is an empty string.
- `audio/drift`: the correction decision function returns the right action for a range of drift
  values, including across a loop boundary.
- `ws/clock`: min-RTT offset selection picks the lowest-RTT sample.

Wire `make test` to run both suites. CI is not required.

---

## 17. Operational concerns

- **Structured logging** with `slog`, JSON output. Include `game_id` and `player_id` as attributes
  on gameplay logs. Log at `info`: game created/started/ended, library sync summaries, and any turn
  where the two year sources disagreed by more than 5 years (with both values and the chosen one) —
  this is the cheapest way to spot systematic matching problems. At `debug`: every phase transition
  and every MusicBrainz request with its latency.
- **Health** — `/healthz` on the backend reports database reachability, Navidrome reachability,
  library size, and eligible-pool size. It returns 200 as long as the database is reachable, so a
  temporarily unreachable Navidrome does not cause a container restart loop.
- **Graceful shutdown** — on `SIGTERM`, stop accepting new games, broadcast a `error` message with
  code `server_restarting` to all sockets, flush all snapshots, close the pool, exit within 10 s.
- **Crash recovery** — on startup, load `game_snapshots` updated within the last 30 minutes,
  rehydrate them, and delete older rows. Players reconnect with their stored tokens. Any game whose
  phase involved audio resumes in PLACING with a fresh timer.
- **Reaping** — a janitor every 60 s ends games idle beyond `LOBBY_IDLE_TIMEOUT` and games with
  zero connected players for longer than `PLAYER_RECONNECT_GRACE`.

---

## 18. README requirements

The generated `README.md` must include: prerequisites; a copy-`.env.example`-and-fill quickstart;
the full environment variable table; how to point the stack at an existing Navidrome (both
shared-network and published-address variants); the DNS records needed for `APP_DOMAIN` and
`MEDIA_DOMAIN`; an explanation of the earliest-wins year rule and how to correct a bad card with an
admin year override; how to reach and use `/admin`; the `MEDIA_DIRECT_PORT` escape
hatch with its mixed-content caveat; and a short troubleshooting section covering no-audio
(autoplay), audio drift, and empty-track-pool symptoms.

---

## 19. Build order

A suggested sequence that keeps something runnable at every step:

1. Repo skeleton, compose file, Dockerfiles, config loading, `/healthz`, Postgres + migrations.
2. Navidrome client and library sync. Verify ~5000 rows land in `tracks`.
3. MusicBrainz client, normalisation, the in-memory cache, the rate limiter, and the earliest-wins
   combiner. Exercise it with a CLI subcommand that resolves a given title/artist and prints both
   sources — useful long after the build, too.
4. Admin API and the eligible-track query, including exclusions and year overrides.
5. Media service: token verification, upstream fetch, singleflight, cache, Range serving.
6. Access gate, game creation/join REST endpoints, WebSocket hub, `hello`, clock sync, `state`.
7. `internal/game` rules engine with its unit tests, driven headlessly before any UI exists.
8. Game manager: phases, timers, track selection, snapshots, reconnection.
9. Frontend shell: gate, home, join, lobby, i18n scaffolding, WebSocket client.
10. Synchronised audio player with drift correction.
11. Game board, placement, challenge, reveal, game over.
12. Admin console.
13. Polish pass against §14.3 and §14.4, then the remaining tests and the README.
