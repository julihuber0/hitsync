# Hitsync implementation specification

## Scope

Hitsync is a self-hosted multiplayer timeline game backed by a private
Navidrome library. The Go backend is authoritative for players, game rules,
turns, scoring, and state snapshots. The React frontend renders state and
submits actions over the game WebSocket.

## Audio transport

Audio uses a LiveKit SFU broadcast, not client-side synchronized media
elements.

```text
Navidrome stream -> media worker cache -> FFmpeg -> LiveKit SFU -> browsers
```

1. The backend selects a track and asks the private media worker to prepare a
   broadcast for `hitsync-{gameId}`.
2. The media worker fetches/transcodes the source from Navidrome once, stores
   it in its local LRU cache, joins the room as `broadcast`, and publishes an
   initially silent Opus audio track.
3. The backend sends each connected game player a short-lived, room-scoped
   LiveKit JWT with `canSubscribe=true` and `canPublish=false`.
4. During `PREPARING`, browsers connect to LiveKit and acknowledge after the
   remote audio track is subscribed. They do not receive a source URL.
5. At `PLACING`, the backend asks the media worker to run FFmpeg in real time
   and feed Ogg/Opus into the existing publication. LiveKit distributes the
   one live track to all subscribers.
6. At reveal, skip, or game end, the backend stops the media worker
   publication. Browser volume fade is cosmetic; it does not control the
   server stream.

There is no per-client MP3 download, `HTMLAudioElement.currentTime` seeking,
playback-rate drift correction, clock-synchronised start timestamp, CORS audio
origin, or public media-service port.

## Components

| Component | Responsibility |
|---|---|
| `frontend` | App UI, game WebSocket, LiveKit subscriber and local volume/mute UI |
| `backend` | REST API, game WebSocket, rules, LiveKit subscriber-token minting, private media control |
| `media` | Navidrome fetch/cache, FFmpeg, LiveKit server-side publisher |
| `livekit` | Single-node SFU; signalling on 7880 and direct WebRTC media |
| `redis` | LiveKit coordination/state backend |
| `postgres` | Game/library persistence |

## Network model

Traefik terminates HTTPS for `APP_DOMAIN` and `LIVEKIT_DOMAIN`. LiveKit
signalling is proxied to port 7880. WebRTC media is direct to the SFU on TCP
7881 and UDP 50000-50100. The media worker is only on the internal Compose
network and accepts authenticated control calls from the backend.

`livekit.yaml` sets `rtc.use_external_ip: true` for a normal public VPS.
Production deployments should add TURN when their topology requires it.

## Secrets and authorization

- `JWT_SECRET` signs Hitsync app/player sessions.
- `LIVEKIT_API_SECRET` signs LiveKit room tokens and is supplied to the SFU
  through an internal generated key file.
- `MEDIA_SHARED_SECRET` HMAC-signs backend-to-media prepare/start/stop calls.

Every LiveKit player token is room-bound, expires after two hours, and cannot
publish media/data. Media-source credentials and Navidrome URLs never leave
the server network.

## Game phases

`LOBBY -> PREPARING -> PLACING -> CHALLENGING -> REVEALING -> ...`

The original eight-second preparing cap remains. It now gates LiveKit
subscription readiness rather than browser buffering. Placement and challenge
timeouts, scoring, reconnect grace, snapshotting, and all game rules are
unchanged.

## Operational constraints

- Source cache size is controlled with `MEDIA_CACHE_MAX_BYTES`.
- Each simultaneous game has one media publisher and one audio SFU track.
- FFmpeg must be available in the media image/native development environment.
- A broadcast preparation/start failure ends the current game rather than
  silently falling back to unsynchronised client playback.
