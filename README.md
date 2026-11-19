# DOGS RADIO

What ya listenin to?

Built by DOGS with Vite, Svelte 5, Tailwind CSS v4, plain JavaScript, and Cloudflare Workers, D1, R2, and Pages. Database access uses native Cloudflare bindings and APIs. No ORM is used.

## Getting started
```
npm install
npm run dev
```

## Layout

- `src/`: the Svelte web app (the product).
- `api/`: Cloudflare Pages Functions (`api/functions/`), SQL migrations (`api/migrations/`) and `wrangler.jsonc`. Run Wrangler from this folder.
- `src/tauri/`: the Tauri 2 native shell, when native platforms are selected.

## Architecture

This app is Cloudflare serverless. The deployment target depends on what it needs:

- **No data and no logins**: the static build is deployed to Cloudflare Pages. No Worker, D1 or R2.
- **Data or logins**: the app is served by Cloudflare Workers, with an API that connects to D1, R2 and other bindings. Web and native clients reach data only through that API.

## Branches

`master` is the only root branch. There is no `main` branch. Keep `master` as the GitHub default branch and the Cloudflare Pages production branch; all other work happens on feature branches merged into `master`.

## Environments

Never use production data in local development or staging. Native apps call Cloudflare APIs and never connect directly to D1.

**Static Pages app (no data, no logins)**: local and production only, no staging.

- **Local**: `npm run dev`, or `npm run build` and `npx vite preview` to check the production build.
- **Production**: Cloudflare Pages serves the static build. GitHub Actions deploys only from `master`; other branches are not deployed.

**Data app on Workers**: local, staging and production.

- **Local (your laptop)**: run the Worker and its API with `npx wrangler dev` from `api/`. Wrangler uses Miniflare to simulate Workers and D1. Local D1 uses a SQLite file under `api/.wrangler/state/`; it stays on your machine and is ignored by Git. Use SQL migrations and test data.
- **Staging / Preview (pull requests and testing)**: a staging Worker environment serves each branch push and same-repository pull request on its own Cloudflare Worker preview URL. All previews share a dedicated staging D1 database (and staging R2 bucket when used) with synthetic test data only; keep migrations additive and do not reset shared staging data.
- **Production (live users)**: the production Worker serves live traffic with a separate production D1 database and R2 buckets. GitHub Actions deploys production only from `master`.

Configure separate staging and production D1 and R2 bindings. Set a least-privilege Cloudflare API token and account ID as GitHub repository secrets; do not commit credentials. Fork pull requests cannot access deployment secrets and are not deployed.

### Deploying today

DOGS RADIO has no API or D1 database yet, so it ships as a static Pages app. The Pages project `radio` serves `radio.dogs.red`. Pushes to `master` build and upload the site through `deploy-cloudflare.yml` once `CLOUDFLARE_API_TOKEN` (Pages edit access) and `CLOUDFLARE_ACCOUNT_ID` are set as repository secrets; until then the workflow builds, skips the upload and says so. `npm run deploy` uploads from your machine with your own Wrangler login.

### Before the first deploy with an API

Create a Pages Direct Upload project with `npx --yes wrangler pages project create` and use the `name` from `api/wrangler.jsonc`; select `master` as its production branch. Direct Upload keeps GitHub Actions as the deployment path. Create separate staging and production D1 databases, then replace the matching database ID placeholders in `api/wrangler.jsonc`. Add `CLOUDFLARE_API_TOKEN` (Pages and D1 edit access) and `CLOUDFLARE_ACCOUNT_ID` as GitHub repository secrets. The generated `deploy-cloudflare.yml` applies the environment's SQL migrations and deploys previews for branch pushes and pull requests; pushes to `master` deploy production.

For local Pages Functions and D1, run `npm install`, `npm run build`, then `npm run cloudflare:dev`. Run local SQL migrations with `npm run cloudflare:db:migrate:local`. This database is isolated from Cloudflare and is not a copy of staging or production.

## Browser support
- **The app** runs on Chrome/Edge 99+, Firefox 97+ and Safari 15.4+. Vite lowers JS and CSS to these targets in `vite.config.js`.
- **Static fallback**: older browsers (down to Chrome 40) and visitors with JS off get the plain HTML page in `index.html`, styled by `src/styles/legacy-fallback.css`.

## Native apps (Tauri 2)

Selected targets: android, ios.

`npm run tauri:dev` runs the native shell in development. `npm run tauri:build` builds for the current host. Release artifacts are produced by the manually triggered **Build apps** GitHub Actions workflow, with each selected target built on a compatible runner.
