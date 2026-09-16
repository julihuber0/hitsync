# Hitsync implementation specification

## Scope

Hitsync is a self-hosted multiplayer timeline game backed by a private
Navidrome library. The Go backend is authoritative for players, game rules,
turns, scoring, and state snapshots. The React frontend renders state and
submits actions over the game WebSocket.

## Song cards

The card collection is a JSON file (`CARDS_FILE`, default
`/config/cards.json`, bind-mounted from `./config`). It is an array of cards:

| Field | Source | Meaning |
|---|---|---|
| `navidromeId` | Navidrome | Subsonic song id, used to stream the track |
| `title`, `artist`, `album` | Navidrome | Shown on the reveal card (album only in the file) |
| `year` | Navidrome | Release year, `null` if Navidrome has none |
| `durationSec` | Navidrome | Used for the `TRACK_MIN_DURATION`/`TRACK_MAX_DURATION` filter |
| `hitsyncyear` | hand-edited, initially `null` | Overrides `year` |
| `excluded` | hand-edited, initially `false` | `true` keeps the song out of games |

1. A scan runs on startup and on `POST /api/admin/cards/scan` (admin scope).
   It pages through the whole library with `search3`, then reads the file,
   merges, and writes it atomically. Only one scan runs at a time (another
   request gets `409`).
2. Merging keeps one card per Navidrome song. New songs get `hitsyncyear: null`
   and `excluded: false`; songs no longer returned are removed; existing cards
   take the Navidrome fields and keep `hitsyncyear` and `excluded`. Cards are
   sorted by artist, album, and title.
3. The merged result replaces the in-memory collection games draw from. Hand
   edits to the file therefore apply on the next scan.
4. A scan fails and leaves the file and the active collection unchanged when
   Navidrome errors, when Navidrome returns no songs although the file has
   cards, or when the file is not valid (JSON syntax, unknown field, missing or
   duplicate `navidromeId`). If no collection is loaded yet and the file is
   valid, it is used as it is.
5. A card is playable when it is not excluded, has a game year
   (`hitsyncyear`, else `year`), and its duration is within the configured
   bounds. The reveal shows the year source: `hitsyncyear` or `Navidrome`.
6. Each turn's card is drawn one turn ahead so clients can preload its track.
   If a scan makes that card unplayable before its turn, a replacement is
   drawn. A game can't start until there are more playable cards than
   players.

## Audio transport

Every client downloads the turn's track and plays it locally. The backend
decides which track plays and when; clients align to a shared start instant on
the server clock.

```text
Navidrome original file -> backend FFmpeg (MP3, 128 kbit/s, no tags) -> LRU cache -> GET /api/media/{token} -> browsers
```

1. When the next turn's card is drawn, the backend starts transcoding its
   track into its cache. While a turn is running, clients receive
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
| `backend` | REST API, game WebSocket, rules, card collection scans, track transcoding and download endpoint |
| `postgres` | Game snapshots and results |

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

In `CHALLENGING`, the other players have `TURN_CHALLENGE_WINDOW` (5 s) to press
Steal (`claim_steal`), or to pass. Pressing Steal spends a token at once and
can't be undone. After the window, claimants choose their slot (`challenge`)
without a time limit, and the turn reveals once every claimant has placed. If
nobody claimed, it reveals when the window ends or everyone has passed. A
claimant who leaves or is removed no longer holds up the turn; a disconnected
one is removed after `PLAYER_RECONNECT_GRACE`, like an absent active player.
The song keeps looping throughout and stops at the reveal.

### Song guess bonus

When the lobby's song guess bonus is enabled, the active player can type the
song's title and artist (`song_guess`) at any time during `PLACING` and
`CHALLENGING`; each message replaces the previous guess. At the reveal the
guess is checked, independently of placement and stealing: title and artist
both correct earns the active player one token, up to the game's maximum
(set by the host in the lobby, 1–10). Nobody else gains or loses a token from
the guess. The reveal shows the guess and which parts were correct to all
players.

Matching (`internal/songmatch`) ignores case, accents, punctuation, spacing,
a leading article ("Beatles"), and "&"/"n" versus "and". Bracketed parts and
anything after " - " (e.g. "(2023 Remix)", "- Live") may be left out. Typos
are tolerated by length: names of up to 4 characters must match exactly, 5–11
characters allow 1 mistake, 12–17 allow 2, and longer names about one per 6
characters. For a credit with several artists, naming any one of them is
enough. The host can still adjust tokens by hand.

A game can start with a single player (`MIN_PLAYERS=1`, the default). With
nobody else holding tokens, `CHALLENGING` is skipped and every placement goes
straight to the reveal. A multiplayer game also continues when players leave,
as long as at least `MIN_PLAYERS` remain.

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
