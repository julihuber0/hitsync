# Hitsync implementation specification

## Scope

Hitsync is a self-hosted multiplayer timeline game backed by a private
Navidrome library. The Go backend is authoritative for players, game rules,
turns, scoring, and state snapshots. The React frontend renders state and
submits actions over the game WebSocket.

## Audio transport

Every client downloads the turn's track and plays it locally. The backend
decides which track plays and when; clients align to a shared start instant on
the server clock.

```text
Navidrome original file -> backend FFmpeg (MP3, 128 kbit/s, no tags) -> LRU cache -> GET /api/media/{token} -> browsers
```

1. When a candidate track for an upcoming turn is resolved, the backend starts
   transcoding it into its cache. While a turn is running, clients receive
   `track_preload` for the next turn's track and download it in the
   background.
2. At `PREPARING`, `track_prepare` names the turn's track. Clients reuse the
   preloaded copy or download it, then send `ready`.
3. When all connected clients are ready, or after the eight-second preparing
   cap, the game enters `PLACING` and sends `track_start` with
   `startAtServerMs` (now + 400 ms). Using the WebSocket ping/pong clock
   offset (lowest RTT of the last ten samples), each client starts the looping
   track at that instant, or, if it only becomes ready later, at
   `(serverNow - startAtServerMs) mod duration`. Only the start is
   synchronised: once playing, clients never seek or change the playback rate
   until `track_stop`.
4. At reveal, skip, or game end, `track_stop` makes clients fade out and
   release their copy. Clients hold at most the current and the next track;
   they release everything in `LOBBY` and `GAME_OVER` or when leaving.
5. A reconnecting client receives `track_prepare`, `track_start` (if playing),
   and `track_preload` again and joins at the correct position.

Transcoded files carry no ID3 tags, so a download cannot reveal title, artist,
or year. The FFmpeg output keeps its Xing/LAME header so browsers know the
exact duration.

## Components

| Component | Responsibility |
|---|---|
| `frontend` | App UI, game WebSocket, track download/playback sync, local volume/mute UI |
| `backend` | REST API, game WebSocket, rules, track transcoding and download endpoint |
| `postgres` | Game/library persistence |

## Network model

Traefik terminates HTTPS for `APP_DOMAIN` and routes `/api` and `/ws` to the
backend. Media downloads are same-origin requests; no additional hostname,
port, or CORS configuration exists.

## Secrets and authorization

- `JWT_SECRET` signs Hitsync app/player sessions. A key derived from it signs
  media tokens.
- A media URL embeds an HMAC-signed `(game, track, expiry)` token, valid for 30
  minutes, and additionally requires the app-access cookie. Tracks are only
  reachable through tokens the game issues for its current or next turn.
- Navidrome credentials and URLs never leave the server.

## Game phases

`LOBBY -> PREPARING -> PLACING -> CHALLENGING -> REVEALING -> ...`

The eight-second preparing cap bounds how long a turn waits for clients'
downloads; a client that is still downloading afterwards joins playback in
sync once it is done. Placement and challenge
timeouts, scoring, reconnect grace, snapshotting, and all game rules are
unchanged.

## Operational constraints

- Transcoded cache size is controlled with `MEDIA_CACHE_MAX_BYTES`.
- At most two FFmpeg processes run concurrently; each track is transcoded once
  and shared by all games.
- FFmpeg must be available in the backend image/native development environment.
