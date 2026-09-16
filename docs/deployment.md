# Production deployment

Hitsync’s production Compose stack runs Postgres, the backend (which also
transcodes the tracks players download), and the frontend. Traefik is
expected to already run on the external `proxy` Docker network.

## DNS and firewall

Create an A/AAAA record for `APP_DOMAIN`, such as `hitsync.example.com`,
pointing at Traefik. Traefik terminates TLS, routes `/api` and `/ws` to the
backend, and everything else to the frontend. Only TCP 80 and 443 need to be
open:

```sh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

## Configure and launch

```sh
git clone <this repo> hitsync
cd hitsync
cp .env.example .env
```

Set a unique `JWT_SECRET` and the other required values. Set `PUID`/`PGID`
to the ids of the user who will edit `config/cards.json` (`id -u`, `id -g`).

```sh
mkdir -p config
docker network create proxy  # only if absent
docker compose build
docker compose up -d
docker compose ps
```

## Verify

```sh
curl -fsS https://YOUR_APP_DOMAIN/healthz
docker compose logs --tail=100 backend
```

Wait for `card scan complete` in the backend log; `config/cards.json` now
exists. Then start a two-player game. In browser DevTools
each player should download one `/api/media/...` file (`audio/mpeg`) per turn,
plus the next turn's file while a turn is playing.

## Updating and backups

```sh
git pull
docker compose build
docker compose up -d
```

Back up `config/cards.json`: it holds your `hitsyncyear` and `excluded`
edits. Postgres (`pgdata`) holds only game snapshots and results. The
`mediacache` volume is disposable:
transcoded tracks are recreated from Navidrome on demand. Keep `.env` in secure
secret storage.

### Upgrading from the LiveKit broadcast version

The `livekit`, `redis`, `livekit-config`, and `media` services no longer exist.
Remove their containers and the unused key volume, and drop the old
`LIVEKIT_*`, `MEDIA_SHARED_SECRET`, `MEDIA_INTERNAL_URL`, and `AUDIO_FORMAT`
values from `.env`:

```sh
docker compose up -d --remove-orphans
docker volume rm hitsync_livekitconfig
```

The backend stores transcoded tracks in a bitrate-specific subdirectory
(`mp3-128k/`) of the `mediacache` volume, so the old worker's files are never
served. They are not counted against the cache budget either; to reclaim the
space, recreate the volume while the stack is down:

```sh
docker compose down
docker volume rm hitsync_mediacache
docker compose up -d
```

LiveKit's TCP 7883 and UDP 51000-51100 firewall rules and the LiveKit DNS
record can be removed.

### Upgrading to the cards file

MusicBrainz lookups, the `HITSYNCYEAR`/`HITSYNCEXCLUDE` file tags, and the
admin console's exclusions and year overrides were replaced by
`config/cards.json`. A database migration drops their tables, so write down
any exclusions or overrides you want to keep before upgrading, and re-apply
them in the file after the first scan. Remove `LIBRARY_SYNC_INTERVAL`,
`MUSICBRAINZ_*`, and `YEAR_*` from `.env`.

## Troubleshooting

- **No cards / scan failed:** the admin page and the backend log
  (`card scan failed`) show the reason. A scan fails without touching the file
  when Navidrome is unreachable or returns no songs, or when `cards.json` has
  a syntax error, an unknown field, or a duplicate `navidromeId`.
- **`permission denied` writing `/config/cards.json`:** set `PUID`/`PGID` to
  the owner of the `config` directory.

- **A song never finishes loading:** look for `failed to prepare media` in the
  backend log. FFmpeg errors there usually mean Navidrome could not deliver the
  original file; check the Navidrome credentials and that the track still
  exists.
- **Downloads fail with 403:** the media link expired (30 minutes) or
  `JWT_SECRET` changed since it was issued; reconnecting fetches fresh links.
- **Players start out of step:** each client aligns its start to the server
  clock measured over the game WebSocket. Large, unstable latency makes that
  measurement less accurate; Bluetooth speakers add their own delay, which no
  client can see. After the start, playback is deliberately never corrected.
- **Browser rejects audio:** use the in-app “Tap to enable sound” prompt; this
  is browser autoplay policy, not a download failure.
