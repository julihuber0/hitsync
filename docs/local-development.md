# Local development

This is the fast-iteration workflow: Postgres runs in Docker, but the backend, media service, and
frontend all run natively on your machine with hot reload. No Traefik, no built images, no
`docker-compose.yml` (the production one) involved at all.

If you just want to run the whole stack as it will run in production, see
[`deployment.md`](./deployment.md) instead — you can also point that stack's `.env` at
`localhost`-style values and run it locally with `docker compose up`, but you'll rebuild images on
every change, which is slow for iteration. This document covers the faster path.

## Prerequisites

- Go 1.23+
- Node.js 22+ and npm
- Docker (only for Postgres, and optionally a throwaway Navidrome)
- A terminal multiplexer or multiple terminal tabs — you'll run four things at once: Postgres,
  the backend, the media service, and the Vite dev server

## 1. Start Postgres

A separate compose file, `docker-compose.dev.yml`, exists purely for local dev support services —
it is never used in production and is independent of the real `docker-compose.yml`.

```sh
make dev-up
# or directly: docker compose -f docker-compose.dev.yml up -d postgres
```

This starts Postgres on `localhost:5432` with user/password/db `hitsync`/`devpassword`/`hitsync`,
persisted in its own `hitsync-dev_pgdata-dev` volume (separate from anything the production compose
file creates). `make dev-down` stops it.

## 2. Point at a Navidrome

**The normal case: use your real server, local or remote, exactly as in production.** `NAVIDROME_URL`
is just a URL the backend process calls over the network — there's nothing local-dev-specific about
it, and no special networking is needed for a remote one (unlike Docker-to-Docker setups, a native
process on your machine reaching out to `https://music.example.com` is no different from any other
outbound HTTPS call). In `.env.dev` (created in step 3), set:

```
NAVIDROME_URL=https://music.example.com
NAVIDROME_USERNAME=hitsync
NAVIDROME_PASSWORD=<your real password>
```

using whatever credentials you'd use in a real deployment. That's the whole setup — the backend
syncs your actual library on startup, same as it would in production.

Two fallbacks if you don't already have a Navidrome to test against:

<details>
<summary><strong>No Navidrome yet? Spin up a throwaway one with a few test files.</strong></summary>

Drop a handful of MP3s you have the rights to use into `./dev-music/` (git-ignored), then:

```sh
docker compose -f docker-compose.dev.yml --profile navidrome up -d navidrome
```

This runs Navidrome on `localhost:4533`, indexing `./dev-music`. Open `http://localhost:4533` once
to create its own admin account (this is Navidrome's own login, unrelated to Hitsync's), then
create a second user for Hitsync to authenticate as (Settings → Users), or just reuse the admin
account for local testing. Point `.env.dev`'s `NAVIDROME_URL` at `http://localhost:4533`.

</details>

<details>
<summary><strong>Just testing the UI? Skip Navidrome entirely.</strong></summary>

The backend and frontend both start up fine with an unreachable `NAVIDROME_URL` — the library sync
just logs an error every `LIBRARY_SYNC_INTERVAL` and keeps retrying (this is by design: a
temporarily-down Navidrome must never crash the backend). You can exercise the access gate, lobby,
settings, and admin console this way, but no game will have any tracks to draw from, since the
library never populates.

</details>

## 3. Configure `.env.dev`

```sh
cp .env.dev.example .env.dev
```

The defaults are pre-filled dev secrets and match the Postgres container from step 1. The one thing
you'll actually want to change is `NAVIDROME_URL`/`NAVIDROME_USERNAME`/`NAVIDROME_PASSWORD` to
point at your real server from step 2 (or the throwaway one, if you went that route).

Two things in this file are important to understand, because they exist specifically to make local
dev work over plain HTTP without Traefik or a certificate:

- **`APP_DOMAIN=localhost:5173`** must match the host **and port** the frontend dev server actually
  runs on (§4 below uses the default Vite port, 5173). The backend uses this for its CORS check, its
  WebSocket origin check, and to decide whether to mark its cookies `Secure` — a real browser
  refuses to store a `Secure` cookie set over plain HTTP at all, so the access gate would silently
  never let you in if this didn't detect "localhost" and relax that flag. If you run the frontend on
  a different port, update this to match.
- **`HTTP_ADDR` / `MEDIA_HTTP_ADDR`** let the backend and media binaries listen on a free port if
  `8080`/`8090` are already taken by something else on your machine (these are not Docker port
  mappings — these processes run directly on the host). If you change `HTTP_ADDR`, also set
  `VITE_BACKEND_URL` in step 4 to match.

## 4. Run the backend

```sh
make dev-backend
# or directly: cd backend && set -a && . ../.env.dev && set +a && go run ./cmd/server
```

On startup it runs Postgres migrations automatically and begins a background library sync. Watch
the log line for `"listening"` to confirm the port. Leave this running; `go run` doesn't hot-reload,
so restart it (`Ctrl+C`, rerun) after backend code changes. If you want actual hot reload, install
[`air`](https://github.com/air-verse/air) or similar and point it at `./cmd/server` — it isn't
wired up by default to keep the toolchain dependency-free.

## 5. Run the media service

```sh
make dev-media
# or directly: cd media && set -a && . ../.env.dev && set +a && go run ./cmd/media
```

`.env.dev.example` sets `MEDIA_CACHE_DIR=.cache`, so it caches downloaded tracks under
`media/.cache` (created automatically) rather than the production default of `/cache`, which is a
container path and not writable as a relative path on a normal machine. Delete `media/.cache` any
time to clear it.

## 6. Run the frontend

```sh
make dev-frontend
# or directly: cd frontend && npm install && npm run dev
```

This starts Vite's dev server on `http://localhost:5173` with hot module reload. `vite.config.ts`
proxies `/api`, `/healthz`, and `/ws` straight to the backend (`http://localhost:8080` by default —
set the `VITE_BACKEND_URL` environment variable before running if you changed `HTTP_ADDR` in step
3). This mirrors what Traefik does in production (routing those paths to the backend by prefix)
without needing Traefik locally.

Open `http://localhost:5173`. You should see the access gate; the code is whatever you set
`APP_ACCESS_CODE` to in `.env.dev` (`devcode` by default). The admin console is at
`http://localhost:5173/admin` (`ADMIN_PASSWORD`, default `devadminpass`).

## Running two players locally

Since there are no real accounts, testing multiplayer locally just means opening a second browser
tab (or a private/incognito window, so `localStorage` — where the player token lives — doesn't
collide) at the same invite link. Both tabs talk to the same backend process and will see each
other's moves over WebSocket in real time.

## Running the test suites

```sh
make test            # backend (go test) + frontend (vitest)
make test-backend     # just the backend
make test-frontend    # just the frontend
make test-media       # media service: go build + go vet (it has no unit tests of its own)
make vet              # go vet on both Go modules
make fmt              # gofmt -l on both Go modules (lists unformatted files; run `gofmt -w .` to fix)
```

None of these need Postgres, Navidrome, or any running services — they're pure unit/component
tests (see the README's "Testing" section for what's covered and why the suite is intentionally
modest).

## Building the frontend for production locally

```sh
cd frontend
npm run build      # tsc -b && vite build, output in frontend/dist
npm run preview    # serve that build locally to sanity-check it
```

## Testing the actual Docker images without a VPS

If you want to confirm the containerized build works (not just the native dev workflow), you can
build and run the real stack locally — see [`deployment.md`](./deployment.md)'s steps, but set
`APP_DOMAIN=localhost` / `MEDIA_DOMAIN=media.localhost` in `.env` and either run a local Traefik (a
minimal example is in that doc) or, for a quick check, `docker compose up -d postgres backend
media` and reach the backend directly with `docker compose exec` or a temporary port mapping — the
production compose file doesn't publish the backend/media ports to the host by default, since
Traefik is expected to front them.

## Troubleshooting

**"Address already in use"** on the backend or media service — something else on your machine is
using `8080`/`8090`. Set `HTTP_ADDR`/`MEDIA_HTTP_ADDR` in `.env.dev` to a free port (e.g. `:18080`)
and update `VITE_BACKEND_URL` (and `mediaBaseUrl` will follow `MEDIA_DOMAIN`/`MEDIA_HTTP_ADDR`
automatically) to match.

**Access code / admin login "succeeds" but you're immediately logged out again** — almost always an
`APP_DOMAIN` mismatch: it must equal the host and port shown in your browser's address bar exactly
(`localhost:5173`, not `localhost` or `127.0.0.1:5173`). The cookie's implicit domain, the CORS
check, and the WebSocket origin check all key off this one value.

**WebSocket won't connect (game loads but shows "Connecting…" forever)** — check the backend log
for the actual close reason, and confirm `APP_DOMAIN` matches as above; a mismatched origin closes
the socket immediately after `hello`.

**No tracks, `eligibleTracks: 0` in `/healthz`** — expected if you skipped Navidrome entirely (§2
above). Otherwise, check the backend log for `"library sync: page fetch failed"` and verify
`NAVIDROME_URL`/credentials are actually correct for your server.

**Config errors on startup** ("invalid configuration: ... must be at least N characters") — the
same fail-fast validation that runs in production runs locally too; the provided
`.env.dev.example` values already satisfy it, so this usually means a copy-paste truncated a secret.
