# Deploying to a VPS

This walks through taking Hitsync from zero to running on a fresh VPS, including standing up
Traefik if you don't already have it. If you already run Traefik (and Navidrome) for other
services, skip to [§4](#4-clone-and-configure-hitsync).

For everyday environment variable reference, the earliest-wins year rule, the admin console, and
the direct-port escape hatch, see the main [`README.md`](../README.md) — this document is about the
surrounding infrastructure and operational workflow.

## Contents

1. [Choosing and sizing a VPS](#1-choosing-and-sizing-a-vps)
2. [Base server setup](#2-base-server-setup)
3. [Traefik, if you don't already have it](#3-traefik-if-you-dont-already-have-it)
4. [Clone and configure Hitsync](#4-clone-and-configure-hitsync)
5. [DNS](#5-dns)
6. [First deploy](#6-first-deploy)
7. [Verifying the deployment](#7-verifying-the-deployment)
8. [Firewall](#8-firewall)
9. [Backups](#9-backups)
10. [Updating / redeploying](#10-updating--redeploying)
11. [Logs and monitoring](#11-logs-and-monitoring)
12. [Rollback](#12-rollback)
13. [Deployment troubleshooting](#13-deployment-troubleshooting)

## 1. Choosing and sizing a VPS

Per the spec this was built against, target hardware is 16 GB RAM / 10 vCPU, but the stack idles
at roughly 250 MB RAM and near-0% CPU between turns (§15.3 of the spec), and stays well under 1
vCPU / 600 MB even under a worst-case 10 games × 12 players. In practice:

- **Minimum realistic**: 1 vCPU / 2 GB RAM — fine for a handful of concurrent games, tight if you
  also run Navidrome and Traefik on the same box.
- **Comfortable**: 2 vCPU / 4 GB RAM — comfortable headroom for Navidrome's own transcoding load
  plus Hitsync, on one box.
- Disk: a few GB for the OS and images, plus whatever your Navidrome media library needs (Hitsync
  itself only caches transcoded audio transiently — `MEDIA_CACHE_MAX_BYTES`, 2 GiB by default — and
  stores no music of its own).

Any mainstream Ubuntu/Debian VPS (Hetzner, DigitalOcean, OVH, etc.) works. These instructions
assume **Ubuntu 22.04 or 24.04**; adjust package manager commands for other distros.

## 2. Base server setup

SSH in as a non-root user with `sudo`, then:

```sh
sudo apt update && sudo apt upgrade -y

# Docker Engine + Compose plugin (official convenience script)
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
newgrp docker   # or log out and back in

docker compose version   # sanity check — should print a v2.x version
```

If you'll run Traefik on this box too (§3), also open the firewall now (§8 has the full picture);
at minimum you'll need 80/tcp and 443/tcp reachable from the internet for Let's Encrypt's HTTP-01
challenge and normal HTTPS traffic.

## 3. Traefik, if you don't already have it

Hitsync's `docker-compose.yml` deliberately does **not** include Traefik — it's meant to sit behind
one that may also front other services on the same host. If this VPS is dedicated to Hitsync (plus
optionally Navidrome), here's a minimal Traefik v3 setup that satisfies what Hitsync's labels
expect: an entrypoint named `websecure` and a certificate resolver named `letsencrypt`, both on an
external network named `proxy`.

```sh
mkdir -p ~/traefik/letsencrypt
cd ~/traefik
touch letsencrypt/acme.json
chmod 600 letsencrypt/acme.json   # Traefik refuses to start if this is too permissive

docker network create proxy
```

`~/traefik/docker-compose.yml`:

```yaml
name: traefik

networks:
  proxy:
    external: true

services:
  traefik:
    image: traefik:v3.1
    restart: unless-stopped
    command:
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --providers.docker.network=proxy
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      # Redirect all plain HTTP to HTTPS
      - --entrypoints.web.http.redirections.entrypoint.to=websecure
      - --entrypoints.web.http.redirections.entrypoint.scheme=https
      - --certificatesresolvers.letsencrypt.acme.tlschallenge=true
      - --certificatesresolvers.letsencrypt.acme.email=you@example.com
      - --certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json
      # Uncomment while testing to avoid Let's Encrypt's real-cert rate limits:
      # - --certificatesresolvers.letsencrypt.acme.caserver=https://acme-staging-v02.api.letsencrypt.org/directory
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./letsencrypt:/letsencrypt
    networks: [proxy]
```

```sh
docker compose up -d
docker compose logs -f   # watch for errors, Ctrl+C once it's quiet
```

This matches Hitsync's `.env` defaults (`TRAEFIK_NETWORK=proxy`, `TRAEFIK_ENTRYPOINT=websecure`,
`TRAEFIK_CERTRESOLVER=letsencrypt`) exactly, so you won't need to change those. If you already run a
Traefik with different names, set the three `TRAEFIK_*` variables in Hitsync's `.env` to match
instead of changing Traefik.

**Testing certificate issuance first is worth it.** Let Encrypt's production rate limits are easy
to hit while iterating on a broken config. Uncomment the `caserver` staging line above, confirm
everything issues a (browser-untrusted) staging cert successfully end to end, then comment it back
out and restart Traefik for real certificates.

### Navidrome, if you don't already have it

Not part of this deliverable, but if you're setting up from scratch, Navidrome is a single
container with your music mounted read-only:

```yaml
services:
  navidrome:
    image: deluan/navidrome:latest
    restart: unless-stopped
    volumes:
      - ./navidrome-data:/data
      - /path/to/your/music:/music:ro
    networks: [proxy, internal]   # internal here is Navidrome's own, if it has one
```

It doesn't strictly need a Traefik router of its own for Hitsync to work — `backend` and `media`
just need to reach it directly (see the README's "Connecting to Navidrome" section for the two
ways to wire that up).

## 4. Clone and configure Hitsync

```sh
git clone <this-repo-url> hitsync
cd hitsync
cp .env.example .env
```

Generate strong secrets rather than typing something memorable:

```sh
openssl rand -base64 32   # run twice — once for JWT_SECRET, once for MEDIA_SHARED_SECRET
openssl rand -base64 24   # good enough for APP_ACCESS_CODE, ADMIN_PASSWORD, POSTGRES_PASSWORD
```

Edit `.env` and fill in at least:

| Variable | What to put |
|---|---|
| `APP_DOMAIN` | The subdomain you'll point at this box for the app, e.g. `hitster.example.com` |
| `MEDIA_DOMAIN` | A second subdomain for audio, e.g. `hitster-media.example.com` |
| `APP_ACCESS_CODE` | A code you'll share with players |
| `ADMIN_PASSWORD` | A separate password only you know |
| `JWT_SECRET` | The first `openssl rand -base64 32` output |
| `MEDIA_SHARED_SECRET` | The second one — must differ from `JWT_SECRET` |
| `POSTGRES_PASSWORD` | Any generated secret |
| `NAVIDROME_URL` / `NAVIDROME_USERNAME` / `NAVIDROME_PASSWORD` | Wherever Navidrome ends up (§3) |
| `MUSICBRAINZ_CONTACT` | Your email — required by MusicBrainz's usage policy |

Everything else has a sensible default; see the README's environment variable tables if you want to
tune game rules (target cards, timeouts, etc.) before first launch — they can also be changed later
and take effect on the next `docker compose up -d` (a restart), no migration needed.

**Do not commit `.env`.** It's already covered by `.gitignore`.

## 5. DNS

Point both `APP_DOMAIN` and `MEDIA_DOMAIN` at this VPS's public IP (A records, or AAAA for IPv6).
Propagation is usually fast, but if Traefik logs certificate errors immediately after DNS changes,
give it a few minutes and retry — Let's Encrypt's validator needs the records to actually resolve.

## 6. First deploy

```sh
docker compose build
docker compose up -d
docker compose logs -f backend   # watch startup; Ctrl+C once it settles
```

On first startup the backend runs its Postgres migrations automatically and kicks off a background
Navidrome library sync. This can take anywhere from seconds to a couple of minutes depending on
library size (§9.2 of the spec: roughly one paginated request per 500 tracks). The stack is usable
immediately — the lobby, admin console, etc. all work — but games won't have tracks to draw from
until that first sync finishes.

## 7. Verifying the deployment

```sh
curl -s https://YOUR_APP_DOMAIN/healthz | jq
```

Expect `"status": "ok"` and `"db": "ok"` immediately. `"navidrome"` should flip to `"ok"` once
Navidrome is reachable, and `"libraryTracks"` / `"eligibleTracks"` should climb from `0` to your
library's real size once the first sync completes.

Then, in a browser:

1. Visit `https://YOUR_APP_DOMAIN` — you should see the access gate, not a Traefik or nginx default
   page. If you see a certificate warning, DNS/cert issuance hasn't finished yet (§13 below).
2. Enter `APP_ACCESS_CODE`, create a game, and confirm the invite code and QR code render.
3. Visit `https://YOUR_APP_DOMAIN/admin` and log in with `ADMIN_PASSWORD` — check the Overview tab
   shows a nonzero library size once sync finishes.
4. Open the invite link on a second device (or another browser) and confirm both players see the
   lobby update in real time — this exercises the WebSocket path specifically, which is the part
   most likely to be misconfigured if Traefik or DNS is subtly wrong.

## 8. Firewall

If you're using `ufw` (Ubuntu's default):

```sh
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

Nothing else needs to be open to the internet. Postgres is never published to the host
(`docker-compose.yml` intentionally omits a `ports:` mapping for it — §11.4 of the spec) and the
backend/media containers are only reachable through Traefik unless you've deliberately enabled
`MEDIA_DIRECT_PORT` (see the README's "Direct port" section) — if you have, open that port too and
make sure it's actually serving HTTPS, not plain HTTP (mixed content will otherwise silently break
audio for every player).

## 9. Backups

The only durable state is Postgres (library index, exclusions, year overrides, game history) — the
media cache is disposable by design (it repopulates from Navidrome on demand).

**Ad hoc dump:**

```sh
docker compose exec postgres pg_dump -U hitsync hitsync > hitsync-backup-$(date +%F).sql
```

**Restore:**

```sh
docker compose exec -T postgres psql -U hitsync hitsync < hitsync-backup-2026-01-01.sql
```

**Scheduled backups** — a simple cron entry works well for a single-VPS deployment:

```
0 3 * * * cd /home/you/hitsync && docker compose exec -T postgres pg_dump -U hitsync hitsync | gzip > /home/you/backups/hitsync-$(date +\%F).sql.gz
```

Prune old backups on whatever schedule fits, and copy them off-box periodically (VPS snapshots
alone aren't a backup strategy if the whole VPS is what fails).

## 10. Updating / redeploying

```sh
cd hitsync
git pull
docker compose build
docker compose up -d
```

Migrations run automatically on backend startup and are additive/idempotent (`golang-migrate`
tracks what's already applied), so this is safe to run repeatedly. Compose only recreates
containers whose image or config actually changed, so an update where only, say, the frontend
changed leaves `postgres` untouched and running.

If you changed `.env` (new variables, changed values), `docker compose up -d` picks those up on the
next recreation of the affected containers — no rebuild needed for env-only changes, just
`docker compose up -d` again.

**In-flight games during a backend restart:** the backend broadcasts a `server_restarting` message
and flushes crash-recovery snapshots on `SIGTERM` (§17 of the spec), and reloads any snapshot
updated in the last 30 minutes on the next startup, restoring players via their existing tokens. A
short restart (seconds) during a quiet moment is low-risk; expect players mid-turn to see a brief
disconnect/reconnect.

## 11. Logs and monitoring

```sh
docker compose logs -f              # everything
docker compose logs -f backend      # just the backend (structured JSON via slog)
docker compose logs -f media
```

Backend logs are JSON (`LOG_LEVEL` controls verbosity); worth grepping for:

- `"msg":"library sync complete"` — added/updated/removed counts per sync, and whether it failed
- Any line where the two year sources disagreed by more than 5 years — logged deliberately per the
  spec as the cheapest way to spot systematic MusicBrainz matching problems
- `"msg":"server error"` at `ERROR` level — anything here means the process is about to exit

There's no bundled metrics/alerting stack (out of scope for this deliverable) — if you want one,
`/healthz` is a natural target for an external uptime check (it returns 200 as long as the database
is reachable, by design, so it won't false-positive on a transient Navidrome outage), and Docker's
own `--restart unless-stopped` (already set on every service) handles simple crash recovery.

## 12. Rollback

```sh
cd hitsync
git log --oneline -5          # find the commit you want to roll back to
git checkout <previous-commit>
docker compose build
docker compose up -d
```

Since migrations are additive, rolling the code back while a newer migration has already run is the
one case worth checking — look at `backend/internal/store/migrations/` on the target commit versus
what's already applied (`docker compose exec postgres psql -U hitsync hitsync -c "select * from schema_migrations"`)
before rolling back across a schema change. For this project as shipped there's only the one
migration (`0001_init`), so this doesn't currently apply, but it will as the schema grows.

## 13. Deployment troubleshooting

**Traefik shows a default/404 page instead of Hitsync.** Check `docker compose logs traefik` for
router errors, and confirm the `proxy` network name matches `TRAEFIK_NETWORK` in Hitsync's `.env`
(default `proxy` on both sides). Confirm with `docker network ls` that only one `proxy` network
exists — a typo that creates a second one is a common cause of "it's running but Traefik can't see
it."

**Certificate never issues / browser shows "not secure."** Almost always DNS not yet resolving to
this VPS, or port 80/443 not reachable from the internet (check §8's firewall rules, and any
cloud-provider-level security group in addition to `ufw`). Check `docker compose logs traefik` for
the specific ACME error — it's usually explicit about which challenge failed and why.

**`/api` requests return the frontend's HTML instead of JSON.** This means Traefik is routing
`APP_DOMAIN` requests to the `frontend` router instead of the `backend` router. Check the router
priorities in `docker-compose.yml` — `hitsync-api` must be priority `20` and `hitsync-web` priority
`10` so the more specific path-matching router wins; this ordering is already correct as shipped,
so this only comes up if it's been edited.

**WebSocket connects then immediately closes.** Check the backend logs at `debug` level
(`LOG_LEVEL=debug`) for the close reason. The most common cause post-deployment is `APP_DOMAIN`
not matching what's actually in the browser's address bar (e.g. a `www.` prefix mismatch) — the
origin check is exact.

**Everything above the app is fine, but no track ever plays.** See the README's "No audio plays"
troubleshooting entry — check `https://YOUR_MEDIA_DOMAIN/healthz` directly first to isolate whether
the media service itself is reachable through Traefik before looking at the browser/audio layer.
