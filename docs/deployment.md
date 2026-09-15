# Production deployment

Hitsync’s production Compose stack runs Postgres, Redis, LiveKit, a private
media broadcaster, the backend, and the frontend. Traefik is expected to
already run on the external `proxy` Docker network.

## DNS and firewall

Create A/AAAA records for:

- `APP_DOMAIN`, such as `hitsync.example.com`
- `LIVEKIT_DOMAIN`, such as `livekit.example.com`

Both point at Traefik. Traefik terminates TLS and proxies `/api`/`/ws` for the
app and LiveKit signalling on port 7880. Do not create a public media-worker
hostname.

Open these ports directly to the LiveKit container on the host:

| Protocol | Port(s) | Reason |
|---|---|---|
| TCP | 80, 443 | Traefik / HTTPS / WebSocket signalling |
| TCP | 7883 | WebRTC ICE/TCP fallback |
| UDP | 51000-51100 | WebRTC media |

For example, with UFW:

```sh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 7883/tcp
sudo ufw allow 51000:51100/udp
```

These ports are intentionally off LiveKit's defaults (7881/50000-50100) so
that a second, independently-managed LiveKit instance on the same host (e.g.
from another project's Compose stack) doesn't collide with this one on the
host's port bindings — Traefik routes both by hostname on 7880 internally, but
the raw WebRTC TCP/UDP ports are published straight to the host and must be
unique per LiveKit instance. If you deploy a third instance, give it its own
non-overlapping TCP port and UDP range too, changed consistently in both
`docker-compose.yml`'s `ports:` list and `rtc.tcp_port`/`rtc.port_range_*` in
[`livekit.yaml`](../livekit.yaml) — the host and container ports must match
exactly, since LiveKit advertises the container-side port in the ICE
candidates it hands to browsers.

If the server has a public IP that is not visible inside Docker, leave
`rtc.use_external_ip: true` in [`livekit.yaml`](../livekit.yaml). For a more
complex NAT or restrictive-client environment, add LiveKit TURN configuration
and expose its port too.

## Configure and launch

```sh
git clone <this repo> hitsync
cd hitsync
cp .env.example .env
```

Set unique `JWT_SECRET`, `MEDIA_SHARED_SECRET`, and `LIVEKIT_API_SECRET`.
Set `LIVEKIT_URL` to the public secure URL (`wss://LIVEKIT_DOMAIN`) while
keeping `LIVEKIT_INTERNAL_URL=ws://livekit:7880` for the media worker.

```sh
docker network create proxy  # only if absent
docker compose build
docker compose up -d
docker compose ps
```

`livekit-config` writes the `key_file` used by the SFU. It must complete
successfully before the LiveKit service starts. The key is never stored in the
repository or exposed by Traefik.

## Verify

```sh
curl -fsS https://YOUR_APP_DOMAIN/healthz
docker compose logs --tail=100 livekit media backend
```

Start a two-player game after the Navidrome sync completes. In browser DevTools
you should see a WebSocket to `LIVEKIT_DOMAIN`; each player should receive one
remote audio track. The worker log should show its FFmpeg process only when a
turn begins.

## Updating and backups

```sh
git pull
docker compose build
docker compose up -d
```

Back up Postgres (`pgdata`) regularly. The `mediacache` and `livekitconfig`
volumes are disposable: cached source tracks can be re-fetched and the LiveKit
key file is rebuilt from `.env` on the next deployment. Keep `.env` in secure
secret storage.

## Troubleshooting

- **Signalling connects but no audio track arrives:** inspect `media` logs;
  confirm `LIVEKIT_INTERNAL_URL`, API key, and API secret match the SFU.
- **Track arrives but silence/no playback:** inspect FFmpeg errors in `media`
  logs and ensure the Navidrome track can be fetched by the worker.
- **Works on LAN but not remotely:** check the UDP range, TCP 7883, DNS,
  external-IP discovery, and firewall/NAT forwarding.
- **Browser rejects audio:** use the in-app “Tap to enable sound” prompt; this
  is browser autoplay policy, not an MP3 preload failure.
