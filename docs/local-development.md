# Local development

Run Postgres in Docker; run the backend and the Vite client natively for quick
rebuilds.

```sh
cp .env.dev.example .env.dev
make dev-up
```

In separate terminals:

```sh
make dev-backend
make dev-frontend
```

Open `http://localhost:5173`. Vite proxies `/api` and `/ws` to the backend,
which transcodes each track with your local FFmpeg and serves it to the
browser as `/api/media/...`.

At the beginning of every turn the game shows **Loading song** until the track
is downloaded, then **Waiting for the other players** until everyone has it.
The very first turn of a game takes longest, because nothing has been
transcoded or preloaded yet; later turns use the copy clients preloaded during
the previous turn. If the UI shows a loading error, use **Retry**; if it
persists, inspect the `make dev-backend` terminal for Navidrome or FFmpeg
errors.

To try synchronisation, open the game in two browser windows (one private, so
each gets its own player) and listen for echo.

## Dependencies

- Go 1.23+
- Node 22+
- FFmpeg on your PATH (or set `FFMPEG_PATH` in `.env.dev`)
- Docker Compose v2
- Navidrome reachable through the `NAVIDROME_*` values in `.env.dev`

`MEDIA_CACHE_DIR=.cache` keeps transcoded files under `backend/.cache`.
Remove that directory whenever you want to force a fresh transcode.

For a disposable Navidrome instance loaded from `./dev-music`, use:

```sh
docker compose -f docker-compose.dev.yml --profile navidrome up -d
```

Set `NAVIDROME_URL=http://localhost:4533` in `.env.dev` for that option.

## Checks

```sh
make test
make vet
```

The backend's transcoding tests run FFmpeg and are skipped when it is not
installed.

Stop support services with `make dev-down`.
