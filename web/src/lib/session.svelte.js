import { api } from './api.js';
import { navigate } from './router.svelte.js';
import { toast } from './toast.svelte.js';
import { clearVault } from './vault.svelte.js';
import { adoptTheme, cancelPendingTheme } from './theme.svelte.js';

export const session = $state({ loaded: false, setupRequired: false, version: '', user: null });

export async function loadSession() {
  const s = await api('GET', '/api/state');
  session.setupRequired = s.setupRequired;
  session.version = s.version ?? '';
  session.user = s.user ?? null;
  if (s.user) adoptTheme(s.user.theme);
  session.loaded = true;
}

/** Called when the server says the session is gone (expired or revoked). */
export function handleExpired() {
  if (!session.user) return;
  session.user = null;
  cancelPendingTheme();
  clearVault();
  toast('Your session has expired. Please log in again.', 'info', 6000);
  navigate('/login', { replace: true });
}
