<script>
  import Toasts from './components/Toasts.svelte';
  import Login from './pages/Login.svelte';
  import Setup from './pages/Setup.svelte';
  import Vault from './pages/Vault.svelte';
  import Backup from './pages/Backup.svelte';
  import Admin from './pages/Admin.svelte';
  import { route, navigate } from './lib/router.svelte.js';
  import { session, loadSession, handleExpired } from './lib/session.svelte.js';
  import { onUnauthorized } from './lib/api.js';

  onUnauthorized(handleExpired);

  let bootError = $state('');
  loadSession().catch((err) => (bootError = err.message));

  // Resolve which page to show; redirect when the URL doesn't fit the state.
  const view = $derived.by(() => {
    if (!session.loaded) return { page: 'loading' };
    if (session.setupRequired) return { page: 'setup', path: '/setup' };
    if (!session.user) return { page: 'login', path: '/login' };
    const p = route.path;
    const m = /^\/folders(?:\/([0-9a-fA-F-]{36}))?\/?$/.exec(p);
    if (m) return { page: 'vault', folder: m[1]?.toLowerCase() ?? '' };
    if (p === '/backup') return { page: 'backup' };
    if (p === '/admin' && session.user.isAdmin) return { page: 'admin' };
    return { page: 'vault', folder: '', path: '/folders' };
  });

  $effect(() => {
    if (view.path && route.path !== view.path) navigate(view.path, { replace: true });
  });
</script>

{#if bootError}
  <div class="boot">Cannot reach passman: {bootError}</div>
{:else if view.page === 'setup'}
  <Setup />
{:else if view.page === 'login'}
  <Login />
{:else if view.page === 'vault'}
  <Vault folderId={view.folder} />
{:else if view.page === 'backup'}
  <Backup />
{:else if view.page === 'admin'}
  <Admin />
{/if}

<Toasts />

<style>
  .boot { padding: 40px; text-align: center; color: var(--muted); }
</style>
