# Hitsync

Self-hosted multiplayer music-timeline game using a private Navidrome library.
Audio is a **server-side LiveKit broadcast**: the media worker downloads and
transcodes each selected track, publishes one Opus/WebRTC audio track to a
per-game LiveKit room, and every browser subscribes to that SFU track. No
browser receives an MP3 URL, downloads an MP3, seeks it, or performs
clock/drift correction.

## Architecture

```text
Navidrome -> media worker (cache + FFmpeg) -> LiveKit SFU -> player browsers
                     ^                         ^
                     |                         |
               backend control API       short-lived subscribe-only JWTs
```

The backend keeps game state, authorization, and turn timing. During
`PREPARING`, the media worker publishes a silent LiveKit track and clients
connect/subscribe. Once they are ready (or the readiness cap expires), the
backend starts FFmpeg; the same SFU publication is then heard by all players.

## Prerequisites

- Docker Engine and Docker Compose v2
- Navidrome v0.64+ reachable from the Docker host
- Traefik v2/v3 on a shared Docker network, with TLS enabled
- Three DNS names pointing at Traefik: the app, the LiveKit signalling host,
  and (optionally) the Navidrome host you already use
- Firewall access to UDP `51000-51100` and TCP `7883` for LiveKit media

## Quick start

```sh
git clone <this repo> hitsync
cd hitsync
cp .env.example .env
```

Set the required values in `.env`, then:

```sh
docker network create proxy  # only when this Traefik network does not exist
docker compose build
docker compose up -d
```

The `livekit-config` init container writes the SFU key file from `.env` into
an internal volume. It is not publicly exposed. The media worker is also
internal-only; only Traefik exposes the frontend, backend, and LiveKit
signalling endpoint.

## Configuration

| Variable | Required | Purpose |
|---|---:|---|
| `APP_DOMAIN` | yes | App, API, and game WebSocket hostname |
| `LIVEKIT_DOMAIN` | yes | Public LiveKit signalling hostname, e.g. `livekit.example.com` |
| `LIVEKIT_URL` | yes | Browser-facing URL, normally `wss://livekit.example.com` |
| `LIVEKIT_INTERNAL_URL` | yes | Media worker -> SFU URL, normally `ws://livekit:7880` |
| `MEDIA_INTERNAL_URL` | no | Backend -> media worker URL, default `http://media:8090` |
| `LIVEKIT_API_KEY` / `LIVEKIT_API_SECRET` | yes | SFU API credentials; secret must be at least 16 characters |
| `MEDIA_SHARED_SECRET` | yes | ≥32 chars; authenticates backend control calls to the media worker |
| `NAVIDROME_URL`, `NAVIDROME_USERNAME`, `NAVIDROME_PASSWORD` | yes | Private library source |
| `AUDIO_FORMAT`, `AUDIO_BITRATE` | no | Navidrome’s server-side transcode request, defaults `mp3` / `192` |
| `MEDIA_CACHE_DIR`, `MEDIA_CACHE_MAX_BYTES` | no | Worker-only LRU cache, default `/cache` / 2 GiB |

All other game, database, and MusicBrainz settings remain documented in
[`.env.example`](.env.example). Generate independent secrets with:

```sh
openssl rand -base64 32
```

Never reuse `LIVEKIT_API_SECRET`, `MEDIA_SHARED_SECRET`, or `JWT_SECRET`.

## Networking and TLS

Traefik proxies HTTPS/WebSocket signalling from `LIVEKIT_DOMAIN` to port 7880.
WebRTC media does **not** traverse Traefik: expose TCP 7883 and the UDP range
51000-51100 directly to the Internet and open those ports in the VPS firewall.
`livekit.yaml` enables external-IP discovery for this single-node deployment.
These ports are deliberately off LiveKit's defaults (7881/50000-50100) so
another LiveKit instance on the same host doesn't collide with this one; if
you run more than two, give each its own non-overlapping TCP port and UDP
range in both `livekit.yaml` and `docker-compose.yml`.

If clients are behind restrictive networks, configure TURN for your domain in
[`livekit.yaml`](livekit.yaml) and publish its port as described in the
[LiveKit self-hosting documentation](https://docs.livekit.io/transport/self-hosting/deployment/).

## Operational checks

```sh
curl -fsS https://APP_DOMAIN/healthz
docker compose logs -f livekit media backend
docker compose ps
```

For a game that cannot hear audio, first verify that the browser can open
`wss://LIVEKIT_DOMAIN`, then confirm UDP `51000-51100` and TCP `7883` are
reachable. A LiveKit connection failure is visible in the browser console;
media-worker/FFmpeg failures are in `docker compose logs media`.

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
backend/   Go REST/WebSocket game server; issues LiveKit subscriber tokens
media/     Go private worker; server-side cache, FFmpeg, and LiveKit publisher
frontend/  React client; subscribes to the SFU audio track
livekit.yaml  Single-node SFU configuration
```
