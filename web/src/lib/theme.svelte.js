import { api } from './api.js';
import { toast } from './toast.svelte.js';
import { storageGet, storageSet } from './util.js';

/** Theme: mode 'auto' | 'light' | 'dark'; hue 0..359, or null for the original blue. */
export const DEFAULT_THEME = { mode: 'auto', hue: null };
export const ORIGINAL_HUE = 201;
const CACHE_KEY = 'passman.theme';
const MODES = ['auto', 'light', 'dark'];

function normalize(t) {
  const mode = MODES.includes(t?.mode) ? t.mode : 'auto';
  const h = t?.hue;
  const hue = Number.isInteger(h) && h >= 0 && h < 360 ? h : null;
  return { mode, hue };
}

export const theme = $state(normalize(storageGet(CACHE_KEY, null)));
let saved = { ...theme };
let timer = null;
let seq = 0;

const darkQuery = typeof matchMedia === 'function' ? matchMedia('(prefers-color-scheme: dark)') : null;
const effectiveDark = () => theme.mode === 'dark' || (theme.mode === 'auto' && !!darkQuery?.matches);

const OVERRIDES = ['--accent', '--accent-strong', '--accent-soft', '--accent-glow', '--accent-hi', '--accent-lo'];

/** Pull yellow/green hues darker so white text on the accent stays readable. */
const bump = (h) => Math.max(0, 1 - Math.abs(h - 100) / 75);
const hsl = (h, s, l) => `hsl(${(h + 360) % 360} ${s}% ${l}%)`;

export function accentColors(h, dark) {
  const b = bump(h);
  return dark
    ? {
        '--accent': hsl(h, 74, 52 - 17 * b),
        '--accent-strong': hsl(h, 80, 61 - 12 * b),
        '--accent-soft': hsl(h + 3, 55, 15),
      }
    : {
        '--accent': hsl(h, 100, 39 - 12 * b),
        '--accent-strong': hsl(h, 100, 32 - 9 * b),
        '--accent-soft': hsl(h, 69, 95),
      };
}

function apply() {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  if (theme.mode === 'auto') root.removeAttribute('data-theme');
  else root.setAttribute('data-theme', theme.mode);
  for (const p of OVERRIDES) root.style.removeProperty(p);
  if (theme.hue === null) return;
  const h = theme.hue;
  const b = bump(h);
  const vars = {
    ...accentColors(h, effectiveDark()),
    '--accent-glow': hsl(h - 5, 80, 52 - 14 * b),
    '--accent-hi': hsl(h - 7, 87, 59 - 16 * b),
    '--accent-lo': hsl(h + 3, 100, 20 - 4 * b),
  };
  for (const [k, v] of Object.entries(vars)) root.style.setProperty(k, v);
}

// In auto mode the accent follows the OS light/dark setting.
darkQuery?.addEventListener?.('change', apply);
apply();

function cache() {
  storageSet(CACHE_KEY, { mode: theme.mode, hue: theme.hue });
}

/** Take the server's theme (login, register, state load) without saving it back. */
export function adoptTheme(t) {
  clearTimeout(timer);
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
    const res = await api('PUT', '/api/account/theme', body);
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
  clearTimeout(timer);
  seq++;
  timer = setTimeout(flush, 300);
}

export function resetHue() {
  setTheme({ hue: null });
}

/** Drop any unsent edit (logout / expired session). */
export function cancelPendingTheme() {
  clearTimeout(timer);
  seq++;
}
