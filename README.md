# Hitsync

Self-hosted multiplayer music-timeline game using a private Navidrome library.
Every player's browser downloads the current turn's song and plays it locally,
synchronised with the other players.

## Architecture

```text
Navidrome -> backend (FFmpeg -> MP3 128 kbit/s, LRU cache) -> /api/media -> browsers
```

The backend keeps game state, authorization, turn timing, and audio timing:

- **Preload.** While a turn is running, the backend tells clients which track
  the next turn uses. Clients download it in the background, so the next turn
  starts without a loading delay.
- **Prepare.** At the start of a turn (`PREPARING`) clients make sure the
  track is downloaded and report ready.
- **Start.** Once everyone is ready (or after 8 s at most), the backend fixes a
  start instant on its own clock. Each client, using the server clock measured
  over the game WebSocket, starts its copy at that instant (or, if it finishes
  downloading late or reconnects, at the position the song has reached by then)
  and then plays it on without further correction until the round ends.
- **Stop.** When the round ends, clients fade out and discard their copy. At
  most the current and the next track are held in browser memory.

Tracks are transcoded by FFmpeg inside the backend to MP3 at 128 kbit/s with
all tags stripped, so a downloaded file cannot reveal title, artist, or year.
Download URLs carry short-lived signed tokens that only the game hands out.

## Song cards

The game draws its cards from `./config/cards.json`, next to
`docker-compose.yml` (mounted into the backend at `/config`). The backend
writes this file itself, and you edit it by hand:

```json
[
  {
    "navidromeId": "4ccrvYWUdYra3PcIgm4cCH",
    "title": "Take On Me",
    "artist": "a-ha",
    "album": "80s Hits",
    "year": 2003,
    "durationSec": 225,
    "hitsyncyear": 1985,
    "excluded": false
  }
]
```

- `navidromeId`, `title`, `artist`, `album`, `year`, and `durationSec` come
  from Navidrome and are refreshed on every scan.
- `hitsyncyear` (initially `null`) overrides `year`, e.g. for a song that
  Navidrome dates by the compilation it appears on.
- `excluded` (initially `false`) keeps a song out of games when set to `true`.

**Scans.** On startup the backend reads the whole Navidrome library and
merges it into the file: new songs are added, songs no longer in Navidrome are
removed, and the Navidrome fields are updated. `hitsyncyear` and `excluded`
are never changed. The file is created on the first scan. Games use the result
once the scan finishes. If Navidrome can't be reached, the existing file is
used as it is.

**Editing.** Edits take effect on the next scan. After editing, trigger one
from the admin page (**Scan library now**) or with
`POST /api/admin/cards/scan` (admin session required). The file is read
strictly: a JSON syntax error, an unknown field, or a duplicate `navidromeId`
fails the scan and leaves the file untouched, and the admin page shows the
error.

A song is playable when it is not excluded, has a year (`hitsyncyear` or
`year`), and its duration is between `TRACK_MIN_DURATION` and
`TRACK_MAX_DURATION`. Other songs stay in the file.

## Prerequisites

- Docker Engine and Docker Compose v2
- Navidrome v0.64+ reachable from the Docker host
- Traefik v2/v3 on a shared Docker network, with TLS enabled
- A DNS name for the app pointing at Traefik

## Quick start

```sh
git clone <this repo> hitsync
cd hitsync
cp .env.example .env
```

Set the required values in `.env` (including `PUID`/`PGID`, your user's
`id -u`/`id -g`, so you own the generated `config/cards.json`), then:

```sh
mkdir -p config
docker network create proxy  # only when this Traefik network does not exist
docker compose build
docker compose up -d
```

Only Traefik is exposed; it routes `/api` and `/ws` to the backend and
everything else to the frontend.

## Configuration

| Variable | Required | Purpose |
|---|---:|---|
| `APP_DOMAIN` | yes | App, API, and game WebSocket hostname |
| `JWT_SECRET` | yes | ≥32 chars; signs sessions, player tokens, and media download URLs |
| `NAVIDROME_URL`, `NAVIDROME_USERNAME`, `NAVIDROME_PASSWORD` | yes | Private library source |
| `PUID`, `PGID` | no | User/group the backend runs as, and owner of `config/cards.json`; default `1000` |
| `CARDS_FILE` | no | Card collection path inside the container, default `/config/cards.json` |
| `AUDIO_BITRATE` | no | MP3 bitrate players download, default `128` kbit/s |
| `MEDIA_CACHE_DIR`, `MEDIA_CACHE_MAX_BYTES` | no | Server-side LRU cache of transcoded tracks, default `/cache` / 2 GiB |
| `FFMPEG_PATH` | no | FFmpeg binary, default `ffmpeg` (bundled in the backend image) |

All other game and database settings remain documented in
[`.env.example`](.env.example). Generate independent secrets with:

```sh
openssl rand -base64 32
```

Don't reuse `JWT_SECRET` anywhere else.

## Operational checks

```sh
curl -fsS https://APP_DOMAIN/healthz
docker compose logs -f backend
docker compose ps
```

If players get no audio, look for `failed to prepare media` or
`failed to pre-transcode track` in the backend log (FFmpeg or Navidrome
errors), and check the `/api/media/...` requests in the browser's network
panel.

The browser still needs one user gesture to enable audio output. Hitsync primes
audio on Create/Join and shows its existing “Tap to enable sound” fallback when
a browser blocks playback.

## Development and tests

[Local development](docs/local-development.md) explains the hot-reload setup.
[Deployment](docs/deployment.md) covers production DNS, firewall, backups, and
updates.

```sh
make test
make vet
docker compose config  # validates the rendered production stack
```

## Repository layout

```text
backend/   Go REST/WebSocket game server; transcodes and serves track downloads
frontend/  React client; downloads and plays tracks in sync
```
