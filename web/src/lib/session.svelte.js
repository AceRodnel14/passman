import { api } from './api.js';
import { navigate } from './router.svelte.js';
import { toast } from './toast.svelte.js';
import { clearVault } from './vault.svelte.js';

export const session = $state({ loaded: false, setupRequired: false, user: null });

export async function loadSession() {
  const s = await api('GET', '/api/state');
  session.setupRequired = s.setupRequired;
  session.user = s.user ?? null;
  session.loaded = true;
}

/** Called when the server says the session is gone (expired or revoked). */
export function handleExpired() {
  if (!session.user) return;
  session.user = null;
  clearVault();
  toast('Your session has expired. Please log in again.', 'info', 6000);
  navigate('/login', { replace: true });
}
