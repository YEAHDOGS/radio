# Dogs Radio — Gin live server

Polls **your** Spotify every second. When the track changes, the server
resolves the matching YouTube video and pushes it to every listener on
`radio.dogs.red` over a websocket. The page plays the audio in the
background via the YouTube player API — **listeners never sign in to
anything.**

## Quickstart

```bash
cd radio-server
go mod download
cp .env.example .env.local   # fill in the blanks below
go run .
```

## 1. Spotify app (2 minutes)

1. https://developer.spotify.com/dashboard → Create app
2. Redirect URI: `https://radio-api.dogs.red/auth/callback`
   (use `http://localhost:8080/auth/callback` while testing locally)
3. Copy **Client ID** + **Client secret** into `.env.local`
4. Start the server, visit `/auth/login`, approve → the refresh token
   is saved to `.env.local`. Restart the server. Done — it polls
   `me/player/currently-playing` every second from here on.

Only scope needed: `user-read-playback-state`.

## 2. YouTube API key (2 minutes)

1. https://console.cloud.google.com → new project → enable
   **YouTube Data API v3** → Credentials → Create API key
2. Put it in `.env.local` as `YOUTUBE_API_KEY`. Free tier is plenty
   (a search costs 100 units; only fires when the song changes).

## 3. Build + run 24/7 (Castle)

```bash
go build -o dogsradio .
```

Recommended home: Castle, behind the Cloudflare tunnel you already use:

```bash
cloudflared tunnel --url http://localhost:8080
# point radio-api.dogs.red at the tunnel in the Cloudflare dashboard
```

systemd unit (`/etc/systemd/system/dogsradio.service`):

```ini
[Unit]
Description=Dogs Radio gin server
After=network.target

[Service]
WorkingDirectory=/opt/dogsradio
ExecStart=/opt/dogsradio/dogsradio
Restart=always

[Install]
WantedBy=multi-user.target
```

## API

| Route | What |
|---|---|
| `GET /health` | `{"ok":true}` |
| `GET /api/now` | current now-playing snapshot |
| `GET /ws` | websocket: `{"t":"now",...}` / `{"t":"count","n":N}` / `{"t":"offair"}` |
| `GET /auth/login` | one-time Spotify OAuth bootstrap |
| `GET /auth/callback` | OAuth callback, saves refresh token |

The frontend at `radio.dogs.red` connects to `wss://radio-api.dogs.red/ws`
(see `WS_URL` at the top of the site's `index.html`).

## Auto-DJ

When his Spotify goes quiet for `AUTODJ_IDLE_SECS` (default 120s), the
server takes over: it pulls every track from the `AUTODJ_PLAYLISTS`
(comma-separated playlist IDs), shuffles, and walks the queue on each
track's real duration — broadcasting the same `now` state live tracks
use, with `"src":"autodj"` so the site can badge it. The pool refreshes
every 15 minutes, so playlist adds show up on their own. The instant he
plays anything on Spotify, live cuts back in.

## Notes

- `.env.local` is gitignored — the refresh token never leaves the server.
- Listeners hear the YouTube audio; metadata (title/artists/art) comes
  from Spotify. Track changes cut over within ~1–2 seconds.

## Playlist converter (Spotify <-> YouTube)

An app on the radio site (`/convert.html`) that copies a playlist
between Spotify and YouTube in either direction, using each visitor's
own accounts — no shared credentials.

- `GET /c/status` — which accounts the visitor has connected.
- `/c/auth/spotify/login` + `/c/auth/google/login` — per-user OAuth.
  Spotify needs `playlist-read-private playlist-modify-*`; Google needs
  the `youtube.force-ssl` scope (a sensitive scope: in testing mode the
  consent screen covers 100 users; publishing needs Google verification).
- `POST /c/convert` `{direction, source_id, name}` — starts a job.
- `GET /c/jobs/:id` — `{state, total, done, failed[], result_url}`.

YouTube quota guard: the default 10k units/day only converts ~60 tracks
(search=100 + insert=50 each). The server tracks daily spend in memory
and refuses jobs that would blow the budget instead of dying halfway.
Cached repeat searches cost nothing.

Notes:

- There is no official YouTube Music API; conversions target regular
  YouTube playlists, which open and play inside the YouTube Music app.
- Google requires an `https://` OAuth redirect (localhost excepted), so
  set `PUBLIC_URL=https://radio-api.dogs.red` before creating the
  Google OAuth client.
