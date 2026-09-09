# Dogs Radio

24/7 community radio stations built from Spotify playlists — hosted by Captain Brando.

A Cloudflare-native rebuild of [am_radio](https://github.com/cptnbrando/am_radio)
(Spring Boot + Angular + AWS RDS/EC2, 2024). Same soul — shared stations, live
chatrooms, visualizer — none of the servers.

## Architecture

See [docs/architecture.md](docs/architecture.md) for the full old → new mapping.

| Old (am_radio) | New (dogs-radio) |
|---|---|
| Spring Boot (Java 11) | Cloudflare Workers |
| AWS RDS (Postgres) | Cloudflare D1 |
| EC2 + Docker | Workers + Pages (no servers) |
| Spring WebSocket chatrooms | Durable Objects (one per station) |
| RadioThread per station | Durable Object + alarms |
| Angular 12 | Svelte 5 (DOGS stack) |

## Status

Scaffold + architecture. Implementation starts after Brandon's demo video lands.
