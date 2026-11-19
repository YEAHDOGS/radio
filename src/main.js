// @ts-nocheck
import App from './App.svelte'
import './app.css'
import { mount } from 'svelte'

// index.html leaves the static fallback visible on browsers too old for the app; never mount over it there
const IS_LEGACY_BROWSER = !document.getElementById('legacy-fallback').hidden

const app = IS_LEGACY_BROWSER ? null : mount(App, {
  target: document.getElementById('app'),
});

export default app
