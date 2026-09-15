# Local development

Run Postgres and a development LiveKit SFU in Docker; run the backend, media
worker, and Vite client natively for quick rebuilds.

```sh
cp .env.dev.example .env.dev
make dev-up
```

The development compose stack exposes LiveKit at `ws://localhost:7880`, TCP
7881, and UDP `50000` for WebRTC media. It advertises `127.0.0.1` deliberately:
the browser and SFU run on the same development machine, while the SFU itself
is inside Docker. Its `--dev` mode uses the credentials in
`.env.dev.example` (`devkey` / `secret`); keep those values aligned if you
change the LiveKit development server settings.

In separate terminals:

```sh
make dev-backend
make dev-media
make dev-frontend
```

Open `http://localhost:5173`. The browser connects to LiveKit directly at
`ws://localhost:7880`; the native media process connects to the same SFU,
downloads source files from Navidrome, and uses local FFmpeg to publish them.

At the beginning of every turn the game shows **Buffering audio** followed by
**Connecting to the live broadcast**. The first occurrence of a song can take
longer because the media worker must cache it from Navidrome. If the UI shows
an audio error, use **Retry audio**; if it persists, inspect the `make
dev-media` terminal first for Navidrome or FFmpeg errors.

## Dependencies

- Go 1.23+
- Node 22+
- FFmpeg on your PATH (or set `FFMPEG_PATH` in `.env.dev`)
- Docker Compose v2
- Navidrome reachable through the `NAVIDROME_*` values in `.env.dev`

`MEDIA_CACHE_DIR=.cache` keeps server-only cached files under `media/.cache`.
Remove that directory whenever you want to force a fresh Navidrome fetch.

For a disposable Navidrome instance loaded from `./dev-music`, use:

```sh
docker compose -f docker-compose.dev.yml --profile navidrome up -d
```

Set `NAVIDROME_URL=http://localhost:4533` in `.env.dev` for that option.

## Checks

```sh
make test
make test-media
make vet
```

If a local player joins but gets no audio, check that `docker compose -f
docker-compose.dev.yml logs livekit` is healthy, then inspect the native media
process for FFmpeg or Navidrome errors. Confirm that both `make dev-backend`
and `make dev-media` are running; `make dev-media` is required to publish the
LiveKit track. Unlike the old implementation, there is no media-domain CORS
setup or per-client MP3 buffer to debug.

Stop support services with `make dev-down`.
