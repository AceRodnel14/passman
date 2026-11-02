import { api } from './api.js';
import { toast } from './toast.svelte.js';
import { storageGet, storageSet } from './util.js';
import { createSaveScheduler } from './saveScheduler.js';
import { themeVars, VAR_NAMES } from './themeVars.js';

export { DEFAULT_HUE } from './themeVars.js';

/** Theme: mode 'light' | 'auto' | 'dark'; hue 0..359, or null for the default brand hue. */
export const DEFAULT_THEME = { mode: 'light', hue: null };
const CACHE_KEY = 'passman.theme';
const MODES = ['auto', 'light', 'dark'];

function normalize(t) {
  const mode = MODES.includes(t?.mode) ? t.mode : DEFAULT_THEME.mode;
  const h = t?.hue;
  const hue = Number.isInteger(h) && h >= 0 && h < 360 ? h : null;
  return { mode, hue };
}

export const theme = $state(normalize(storageGet(CACHE_KEY, null)));
let saved = { ...theme };
let seq = 0;
const THEME_URL = '/api/account/theme';
const scheduler = createSaveScheduler({
  delay: 300,
  save: () => flush(),
  // Page is going away: fire-and-forget with keepalive so the request outlives it.
  saveOnExit: () => {
    fetch(THEME_URL, {
      method: 'PUT',
      keepalive: true,
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode: theme.mode, hue: theme.hue }),
    }).catch(() => {});
  },
});

const darkQuery = typeof matchMedia === 'function' ? matchMedia('(prefers-color-scheme: dark)') : null;
const effectiveDark = () => theme.mode === 'dark' || (theme.mode === 'auto' && !!darkQuery?.matches);

/** Colours are set through the CSSOM (allowed by the CSP, unlike style attributes). */
function apply() {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  // light is the CSS default; auto follows the OS; dark is explicit
  root.setAttribute('data-theme', theme.mode);
  for (const p of VAR_NAMES) root.style.removeProperty(p);
  if (theme.hue === null) return;
  for (const [k, v] of Object.entries(themeVars(theme.hue, effectiveDark()))) root.style.setProperty(k, v);
}

// In auto mode the tints follow the OS light/dark setting.
darkQuery?.addEventListener?.('change', apply);
apply();

if (typeof document !== 'undefined') {
  addEventListener('pagehide', () => scheduler.flushOnExit());
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') scheduler.flushOnExit();
  });
}

function cache() {
  storageSet(CACHE_KEY, { mode: theme.mode, hue: theme.hue });
}

/** Take the server's theme (login, register, state load) without saving it back. */
export function adoptTheme(t) {
  scheduler.cancel();
  seq++;
  Object.assign(theme, normalize(t));
  saved = { ...theme };
  cache();
  apply();
}

async function flush() {
  const mine = ++seq;
  const body = { mode: theme.mode, hue: theme.hue };
  try {
    const res = await api('PUT', THEME_URL, body);
    saved = normalize(res ?? body);
  } catch (err) {
    if (mine !== seq) return; // a newer change owns the state now
    Object.assign(theme, saved);
    cache();
    apply();
    toast(err.message, 'error');
  }
}

/** Apply a change right away and save it ~300ms after the last edit. */
export function setTheme(patch) {
  Object.assign(theme, normalize({ ...theme, ...patch }));
  cache();
  apply();
  seq++;
  scheduler.schedule();
}

export function resetHue() {
  setTheme({ hue: null });
}

/** Drop any unsent edit (logout / expired session). */
export function cancelPendingTheme() {
  scheduler.cancel();
  seq++;
}
