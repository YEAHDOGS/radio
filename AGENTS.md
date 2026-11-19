# AGENTS.md

Rules for every AI agent working in this repository.

## Project
- **Name**: DOGS RADIO (`radio`)
- **What it is**: What ya listenin to?
- **Target platforms**: web, android, ios
- **Main tech stack**: Vite, Svelte 5, Tailwind CSS v4, JavaScript with JSDoc, Cloudflare Workers, D1, R2, Pages, and Tauri 2. Database access uses Cloudflare bindings and APIs directly; no ORM.
- DOGS RADIO is a new way to listen to music for the world. DOGS RADIO features a hand-selected list of 24/7 music stations, that use streaming platforms to play your favorite hits wherever you are, in sync with other listeners. Create your own group to watch youtube or listen to music with other people in the world. Easily listen to any station live, using your signed in DSP by default and falling back on free Youtube video playback when unavailable. Give to your favorite creators, message other listeners, and find your new favorite song today
- Created from the DOGS Svelte template.

## Cloudflare environments
- The architecture is Cloudflare serverless. Choose the deployment target by what the app needs:
  - **No data and no logins**: deploy the static build to Cloudflare Pages. No Worker, D1 or R2.
  - **Data or logins**: serve the app from Cloudflare Workers, with an API that connects to D1, R2 and other bindings. The web and native clients reach data only through that API.
- Use Cloudflare Workers and D1 for serverless logic and relational data, with R2 for object storage when needed. Use Cloudflare bindings and D1 APIs directly. Stay inside the free tier, cache responses, lazy load, and keep data locally so each visitor makes as few requests as possible.
- Server code lives in `api/`: Pages Functions in `api/functions/`, SQL migrations in `api/migrations/`, Wrangler config in `api/wrangler.jsonc`. The optional Tauri shell lives in `src/tauri/`.
- Database changes require versioned SQL migrations. SQL migrations are the source of truth; do not duplicate database schemas in application code.
- Environments depend on the deployment target, and each one is isolated:
  - **Static Pages apps** have two environments and no staging:
    - **Local**: `npm run dev`, or `npm run build` and `npx vite preview` to check the production build.
    - **Production**: Cloudflare Pages serves the static build. GitHub Actions deploys only from `master`; other branches are not deployed.
  - **Data apps on Workers** have three environments:
    - **Local**: `npx wrangler dev`, run from `api/`, runs the Worker and its API locally with Miniflare. D1 uses a local SQLite database under `api/.wrangler/state/`; this data is machine-local and must never be committed or connected to production.
    - **Staging / Preview**: a staging Worker environment whose Cloudflare Worker preview URLs serve each branch push and same-repository pull request. It is bound to a dedicated staging D1 database (and staging R2 bucket when used) holding only synthetic test data. Fork pull requests cannot access deployment secrets and are not deployed.
    - **Production**: the production Worker serves live users with the production D1 database and R2 buckets. GitHub Actions deploys production only from `master`.
- Never point local or staging at the production D1 database. Keep staging and production bindings separate in Wrangler configuration.
- All Worker previews share staging D1. Keep migrations additive and backward compatible, use unique timestamped migration names, and never reset or destructively seed the shared staging database from a preview.
- Native apps call the same Cloudflare APIs as the web app. Never connect a client app directly to D1 or embed Cloudflare credentials in browser code, Vite `VITE_*` variables, or native bundles.
- Authenticate and authorize every protected Worker route, validate input on the server, and bound query sizes. Cache public reads and rate-limit expensive or write operations to protect Cloudflare quotas.
- Test CORS from each shipped Tauri WebView origin. Allow only the app's web and native origins; do not use wildcard origins for credentialed requests.
- Store secrets in Cloudflare Worker secrets or GitHub Actions secrets, use least-privilege API tokens, and never commit credentials. Preview builds must not receive production secrets.
- SQL migration files are the only database schema source. Apply them to local D1 while developing and to staging before production. Test migrations against staging before promoting them. Destructive production migrations require a tested recovery plan and explicit backup plan.
- Keep dependencies current at stable releases when changing them. Prefer small, maintained packages and avoid unnecessary dependencies.
- Deployments use GitHub Actions. Static Pages apps deploy only `master`, straight to production. Data apps upload a Worker preview against staging for every branch and same-repository pull request, and deploy production only from `master`. Test Worker previews before merging.
- Native release artifacts use the manually triggered GitHub Actions workflow. The generated app may target any selection of Web, Android, iOS, macOS, Windows, and Linux. Keep each selected target buildable without requiring unselected targets.

## Privacy
Never write the owner's name, pseudonym, alias, or any personal information anywhere in the repo: code, comments, commits, docs. The company name DOGS is fine everywhere.

## Git workflow
- `master` is the root, default and production branch, here and in every app made from this template. There is no `main` branch: never create one, push to one, target one with a pull request, or reference one in workflows, docs or Cloudflare settings.
- Start every task from an up-to-date `master` unless told to build on another branch.
- Branch per task, named `[agent]-[month]-[feature-name]`: your agent name in lowercase, the two-digit month, and a short kebab-case feature name. Example: `codex-10-tauri-shell`.
- Commit to your feature branch as you go, push the branch, and open a pull request into master once the task is verified. Merging is done through review.
- Generated data, background-script output and local builds belong in `.gitignore`.

## Stack
- Vite + Svelte, **JavaScript only** (no TypeScript). Add concise JSDoc types to complex functions and components.
- Styling is **Tailwind CSS plus plain CSS**. No SCSS, Sass, Less or other preprocessors.
- Icons come from `@lucide/svelte`, imported one at a time.
- Use Cloudflare Workers bindings and D1's native APIs for database access; no ORM. Put schema changes in SQL migrations and commit them.
- Tauri is the native shell. Keep the web build functional on its own and feature-detect native APIs.

## Dependencies
Every dependency is a cost: bundle size, security surface, install scripts and upkeep. Keep the list as short as possible.
- Native browser APIs, Node built-ins and Svelte features come first. If it can be written cleanly in about 20 lines, write it.
- A new package needs a stated reason: what it replaces, its size, and why a native version won't do. Prefer packages with no dependencies and no install scripts.
- Remove packages the moment nothing uses them.

## Code structure
- **Hoist constants.** Every constant goes at the top of its file as a `const` in descriptive SCREAMING_SNAKE_CASE (`const MAX_BPM = 200;`). No magic numbers or raw strings inside logic.
- **Guard clauses first.** Handle failures with early returns, so the main path reads top to bottom and stays unindented.
- **No one-line wrapper functions.** If a function body fits on one line, inline it.
- **1000-line limit per file.** A file that gets close is split into modules before more is added.
- **Logic stays out of markup.** State machines and business logic live in plain JS modules or classes next to the component that uses them. They move to `src/lib/` only once more than one component needs them.
- **Reusable UI.** Build standalone components and reuse them rather than copying markup.
- **Strings.** Lists of more than 10 UI strings or items go in `.json` files beside their component, ready for translation.

## Theme and styles
- All colors, theme tokens, fonts and custom breakpoint variants live in `src/styles/`. Tokens go in `src/styles/theme.css` as Tailwind `@theme` variables.
- Components use those tokens. No hex, rgb or hsl values in `.svelte` or `.js` files, and no arbitrary color utilities like `bg-[#050508]`.
- Tailwind does the layout and most styling. Plain CSS is for what Tailwind can't express: keyframes, complex selectors, browser fallbacks.
- Comment each token with what it is for and where it is used.

## Responsive layouts
Every screen gets its own deliberate layout on mobile portrait, mobile landscape, tablet portrait, tablet landscape, desktop landscape, desktop portrait, TV, and ultra-wide displays. Check the layout at each size before calling a change done.
- Mobile portrait: one stacked column, primary navigation within thumb reach.
- Mobile landscape: compact columns or a sideways swipe list; no stacked headers.
- Tablet portrait and landscape: three or more grid columns with a persistent side rail.
- Desktop landscape: full dashboard capped at `max-w-7xl`; desktop portrait stacks content into rows.
- TV: large type and spacing for 10-foot viewing. Ultra-wide: spread panels to use available width.
- Fit on one screen. Vertical scrolling is allowed only inside lists. Never show a horizontal scrollbar; sideways content must support swipe, drag and scroll wheel and provide controls.
- Nothing is cropped or unreachable at any size, down to a 320×320 window.

## Layout stability and accessibility
- Layouts stay still during animations or page changes. Reserve space for entering, animating, or leaving elements; animate with `transform` and `opacity`. Animate everything possible, and always think creatively with animations. Scrollbars, tabs, text, it's all fair game. Transitions during page interactions are crucial to the DOGS design philosophy.
- Use each device's controls: back-swipe on mobile, scroll wheel on desktop, touch-and-hold, and keyboard and remote focus on TVs. Use semantic markup, ARIA only where needed, visible focus states, and no keyboard traps.

## Browser support (the Potato Target)
- **The app** runs on Chrome/Edge 99+, Firefox 97+ and Safari 15.4+. Vite lowers JS and CSS to these targets (`vite.config.js`); no polyfill packages.
- **The static fallback** in `index.html` (styles in `src/styles/legacy-fallback.css`) covers older browsers down to Chrome 40 and visitors with JS off. It stays plain HTML and ES5, and its text mirrors the app.
- Use modern features where they pay off. The fallback is how old devices stay covered.

## Svelte
- A `svelte-ignore` comment needed by several elements goes once at the top of the component, not repeated on each element. Prefer fixing the problem directly.
- Zero warnings: the dev server, the Svelte compiler and the browser console stay free of errors and warnings.

## Verifying work
- Before calling a task done, load the site and check every display size, the console, and the layout for cropped or unreachable content.
- No unit tests for ordinary code unless asked. When a change touches the landing page, check that the branding lettering hasn't moved by a single pixel.
- Add JSDoc only to new classes, methods and main utility functions, inline and short.

## Native apps (Tauri 2)
- Selected native targets: android, ios.
- `src/tauri/` is the Rust shell; the Svelte/Vite web app remains the product and must continue to build independently.
- Detect native-only behavior with `isTauri()` from `@tauri-apps/api/core`. Keep web behavior available when running in a browser.
- `npm run tauri:dev` opens the Tauri development shell. `npm run tauri:build` builds the current host target. `npm run tauri:build:platforms` builds the selected native targets where the host toolchain supports them.
- Release builds are manually triggered from `.github/workflows/build-apps.yml`. Each selected platform is built on its required GitHub-hosted runner and uploaded as an artifact.
- Android builds require Android Studio command line tools and an installed SDK/NDK; iOS builds require macOS and Xcode. Linux and Windows builds use their native runner toolchains.
- Check native window layouts down to 320×320; every responsive layout requirement applies.

# System Prompt: AAA Superpower Architecture Blueprint
You are an expert interactive front-end engineer specializing in award-winning digital experiences. Replicate the core visual framework of the Superpower.com design system with flawless token execution. Avoid generic corporate bento grids. Focus on a high-fidelity, fluid layout structured around a continuous vertical alignment concept (human spine aesthetic).

## 1. Brand Tokens & Colors (Strict WCAG 2.2 AA Compliance)
- Primary Color: Thematic based around application
- Background: Thematic
- Utility Text: Neutral dark/light depending on theme setting

## 2. Typography Scale
- Primary Font: Thematic. Google fonts are good.
- Headers: Fluid typography utilizing Tailwind fluid utilities / CSS clamp() formulas

## 3. Key Layout Archetypes
- The Hero Wormhole: A centered, high-contrast, expansive structural framing containing embedded visual canvas anchors. 
- Expanding Layouts: Desktop grids must feel edge-to-edge but anchored dynamically via 1px thin grid boundaries.
- Stacking Alignments: Mobile viewports collapse into tightly tracked, stark vertical components with 0px margins on side containers where layouts butt against screen boundaries.

## 4. Interaction Guidelines
- Micro-interactions on all interactive triggers: minimal 150ms spring transitions.
- Scroll Animations: Section transitions must stagger typography reveals using Framer Motion or clean CSS intersection observers.
