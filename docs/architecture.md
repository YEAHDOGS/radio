# Dogs Radio — Architecture

Rebuild of the original am_radio (Spring Boot 2.5.1 + Angular 12 + AWS RDS + EC2, 2024),
moving entirely to Cloudflare infrastructure.

## What am_radio did

- **Stations**: each station = a Spotify playlist, persisted in Postgres via Spring Data JPA.
  On boot, `StationService.fillStations()` hydrated each station's tracks through the
  Spotify client-credentials API ("transients" — not stored in the DB).
- **24/7 playback**: a `RadioThread` per station advanced the "now playing" cursor so
  every listener heard the same thing at the same time — true radio, not per-user playback.
- **Chatrooms**: Spring WebSocket, one room per station.
- **Extras**: `SpotifyPlayerController` (playback control), `LyricsController`, canvas visualizer.
- **Infra**: AWS RDS (Postgres), EC2 + Docker + Jenkins, Certbot TLS (Spotify requires HTTPS).

## Cloudflare mapping

| Concern | am_radio | dogs-radio |
|---|---|---|
| API | Spring MVC controllers | Workers (Hono) |
| Station state / now-playing cursor | `RadioThread` per station (in-memory) | **Durable Object per station** + alarms to advance tracks |
| Chat | Spring WebSocket | Durable Object WebSocket (same object as the station — chat + cursor colocated) |
| Database | RDS Postgres (JPA) | D1 (stations, users, chat history) |
| Media/assets | EC2 disk | R2 |
| Frontend | Angular 12 | Svelte 5 + Vite on Pages |
| Secrets | env files | Worker secrets (minimal scope — Spotify client id/secret only) |
| Deploys | Jenkins + Docker | `wrangler deploy` |

## Why Durable Objects fit

The old design's hardest part was the per-station radio thread: in-memory,
single-process, died with the server. A Durable Object is that thread, but
persistent and global — one object per station owns the track cursor, the alarm
that advances it, and the WebSocket fan-out for chat + now-playing sync.

## Open questions

- Station creation: anyone can spin up a station, or curated DJs only?
- Playback: Spotify Web Playback SDK (requires listeners to have Spotify Premium)
  vs. 30s previews (no login needed — matches the no-login principle)?
- Persist chat history in D1, or ephemeral per broadcast?
