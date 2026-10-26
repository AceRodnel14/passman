<script>
  import Logo from './Logo.svelte';
  import { Folder, FolderOpen, Archive, Menu as MenuIcon, KeyRound, LogOut, Users, ChevronDown, Palette } from '@lucide/svelte';
  import FolderTree from './FolderTree.svelte';
  import ChangePassword from './ChangePassword.svelte';
  import { session } from '../lib/session.svelte.js';
  import { navigate } from '../lib/router.svelte.js';
  import { folderTrail, clearVault } from '../lib/vault.svelte.js';
  import { api } from '../lib/api.js';
  import { cancelPendingTheme } from '../lib/theme.svelte.js';
  import Appearance from './Appearance.svelte';

  let { section, currentFolder = null, children } = $props();

  let expanded = $state(new Set());
  let drawerOpen = $state(false);
  let userMenu = $state(false);
  let changingPassword = $state(false);
  let appearance = $state(false);

  // Keep the path to the open folder expanded in the tree.
  $effect(() => {
    if (!currentFolder) return;
    const ancestors = folderTrail(currentFolder).slice(0, -1);
    if (ancestors.some((f) => !expanded.has(f.id))) {
      expanded = new Set([...expanded, ...ancestors.map((f) => f.id)]);
    }
  });

  function toggle(id) {
    const next = new Set(expanded);
    next.has(id) ? next.delete(id) : next.add(id);
    expanded = next;
  }

  function go(path) {
    drawerOpen = false;
    navigate(path);
  }

  async function logout() {
    userMenu = false;
    try {
      await api('POST', '/api/logout');
    } finally {
      cancelPendingTheme();
      clearVault();
      session.user = null;
      navigate('/login', { replace: true });
    }
  }
</script>

<svelte:window onclick={() => (userMenu = false)} />

<div class="shell">
  <header class="topbar">
    <button class="icon-btn drawer-btn" type="button" onclick={() => (drawerOpen = !drawerOpen)} aria-label="Toggle sidebar">
      <MenuIcon size={20} />
    </button>
    <button class="brand" type="button" onclick={() => go('/folders')}>
      <span class="logo"><Logo size={20} /></span>
      passman
    </button>
    <div class="spacer"></div>
    <div class="user">
      <button class="user-btn" type="button" onclick={(e) => (e.stopPropagation(), (userMenu = !userMenu))} aria-haspopup="menu" aria-expanded={userMenu}>
        <span class="avatar">{session.user.username.slice(0, 1).toUpperCase()}</span>
        <span class="name">{session.user.username}</span>
        <ChevronDown size={16} />
      </button>
      {#if userMenu}
        <div class="user-menu" role="menu">
          <button type="button" role="menuitem" onclick={() => ((userMenu = false), (appearance = true))}><Palette size={16} /> Appearance</button>
          <button type="button" role="menuitem" onclick={() => ((userMenu = false), (changingPassword = true))}><KeyRound size={16} /> Change password</button>
          {#if session.user.isAdmin}
            <button type="button" role="menuitem" onclick={() => ((userMenu = false), go('/admin'))}><Users size={16} /> Manage users</button>
          {/if}
          <button type="button" role="menuitem" onclick={logout}><LogOut size={16} /> Log out</button>
          {#if session.version}<div class="ver">passman v{session.version}</div>{/if}
        </div>
      {/if}
    </div>
  </header>

  <div class="body">
    {#if drawerOpen}
      <button class="scrim" type="button" aria-label="Close sidebar" onclick={() => (drawerOpen = false)}></button>
    {/if}
    <nav class="sidebar" class:open={drawerOpen} aria-label="Folders">
      <div class="tree">
        <div class="root" class:current={section === 'folders' && !currentFolder}>
          <button type="button" onclick={() => go('/folders')}>
            {#if section === 'folders' && !currentFolder}<FolderOpen size={17} />{:else}<Folder size={17} />{/if}
            Folders
          </button>
        </div>
        <FolderTree parent="" current={section === 'folders' ? currentFolder : null} {expanded} ontoggle={toggle} onpick={() => (drawerOpen = false)} />
        <hr class="divider" />
        <button type="button" class="backup" class:current={section === 'backup'} onclick={() => go('/backup')}>
          <Archive size={17} /> Backup and Restore
        </button>
      </div>
    </nav>

    <main>
      {@render children()}
    </main>
  </div>
</div>

{#if appearance}
  <Appearance onclose={() => (appearance = false)} />
{/if}

{#if changingPassword}
  <ChangePassword onclose={() => (changingPassword = false)} />
{/if}

<style>
  .shell { display: flex; flex-direction: column; height: 100%; }
  .topbar {
    display: flex; align-items: center; gap: 8px;
    height: 56px; padding: 0 12px 0 16px; flex: none;
    background: linear-gradient(110deg, var(--accent) 0%, color-mix(in srgb, var(--accent) 75%, var(--accent-glow)) 100%);
    color: #fff;
  }
  .drawer-btn { display: none; color: #fff; }
  .drawer-btn:hover { background: rgb(255 255 255 / .15); color: #fff; }
  .brand {
    display: flex; align-items: center; gap: 10px;
    border: 0; background: transparent; color: #fff; cursor: pointer;
    font: 650 18px/1 var(--font); letter-spacing: -.01em;
  }
  .logo { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 8px; background: rgb(255 255 255 / .2); }
  .spacer { flex: 1; }
  .user { position: relative; }
  .user-btn {
    display: flex; align-items: center; gap: 8px;
    height: 40px; padding: 0 8px 0 4px;
    border: 0; border-radius: 999px; background: transparent; color: #fff;
    font: inherit; cursor: pointer;
  }
  .user-btn:hover { background: rgb(255 255 255 / .15); }
  .avatar { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 50%; background: #fff; color: var(--accent); font-weight: 700; }
  .user-menu {
    position: absolute; right: 0; top: 46px; z-index: 40; width: 220px;
    padding: 6px; background: var(--surface); color: var(--text);
    border: 1px solid var(--border); border-radius: 10px; box-shadow: var(--shadow-lg);
  }
  .user-menu button {
    display: flex; align-items: center; gap: 10px; width: 100%; height: 36px; padding: 0 10px;
    border: 0; border-radius: 7px; background: transparent; color: var(--text); font: inherit; cursor: pointer; text-align: left;
  }
  .user-menu button:hover { background: var(--hover); }

  .body { flex: 1; display: flex; min-height: 0; position: relative; }
  .sidebar {
    width: 264px; flex: none;
    display: flex; flex-direction: column;
    background: var(--sidebar);
    border-right: 1px solid var(--border);
  }
  .tree { flex: 1; overflow: auto; padding: 12px 10px; }
  .root button, .backup {
    display: flex; align-items: center; gap: 10px; width: 100%;
    height: 36px; padding: 0 10px;
    border: 0; border-radius: 8px; background: transparent; color: var(--text);
    font: inherit; font-weight: 600; cursor: pointer; text-align: left;
  }
  .root button :global(svg), .backup :global(svg) { color: var(--accent); }
  .root button:hover, .backup:hover { background: color-mix(in srgb, var(--accent) 8%, transparent); }
  .root.current button, .backup.current { background: var(--accent); color: #fff; }
  .root.current button :global(svg), .backup.current :global(svg) { color: #fff; }
  .divider { height: 0; margin: 10px 4px; border: 0; border-top: 1px solid var(--border-strong); }
  main { flex: 1; min-width: 0; overflow: auto; }
  .scrim { display: none; }

  @media (max-width: 820px) {
    .drawer-btn { display: inline-grid; }
    .name { display: none; }
    .sidebar {
      position: absolute; inset: 0 auto 0 0; z-index: 30;
      transform: translateX(-100%); transition: transform .18s ease;
    }
    .sidebar.open { transform: none; box-shadow: var(--shadow-lg); }
    .scrim { display: block; position: absolute; inset: 0; z-index: 20; border: 0; background: rgb(10 20 30 / .35); }
  }
  .ver { padding: 6px 12px 4px; font-size: 11px; color: var(--muted); border-top: 1px solid var(--border); margin-top: 4px; }
</style>
