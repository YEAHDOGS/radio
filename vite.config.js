import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

// Oldest browsers that get the full app: the first releases with CSS cascade layers, which Tailwind v4 requires.
// Anything older (down to the Chrome 40 Potato Target) gets the static fallback page in index.html instead.
// CSS is lowered by Lightning CSS (oklch -> rgb, color-mix fallbacks, etc.), which costs nothing at runtime.
const APP_CSS_TARGET = ['chrome99', 'edge99', 'firefox97', 'safari15.4', 'ios15.4']
// JS is lowered by Oxc. Safari stays at 16.4: anything lower makes Oxc rewrite every private class field in
// Svelte's runtime into WeakMap helpers for ALL browsers. Safari 15.4-16.3 can still run it natively, and the
// index.html error net swaps in the fallback page if one of them can't.
const APP_JS_TARGET = ['chrome99', 'edge99', 'firefox97', 'safari16.4', 'ios16.4']
// The optional Tauri shell lives in src/tauri; its Rust build output must not trigger dev reloads.
const NATIVE_SHELL_GLOB = '**/src/tauri/**'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    tailwindcss(),
    svelte()
  ],
  server: {
    watch: { ignored: [NATIVE_SHELL_GLOB] },
  },
  build: {
    target: APP_JS_TARGET,
    cssTarget: APP_CSS_TARGET,
  },
})
