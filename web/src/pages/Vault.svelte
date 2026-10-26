<script>
  import { Star, Folder, ChevronRight, FolderPlus, KeyRound, Info, Pencil, CopyPlus, FolderInput, Trash2, FolderOpen, MousePointerClick, Plus } from '@lucide/svelte';
  import Layout from '../components/Layout.svelte';
  import Menu from '../components/Menu.svelte';
  import Shield from '../components/Shield.svelte';
  import PasswordIcon from '../components/PasswordIcon.svelte';
  import PasswordForm from '../components/PasswordForm.svelte';
  import MoveDialog from '../components/MoveDialog.svelte';
  import Details from '../components/Details.svelte';
  import NameDialog from '../components/NameDialog.svelte';
  import ConfirmDialog from '../components/ConfirmDialog.svelte';
  import { vault, loadVault, childFolders, folderPasswords, findFolder, folderTrail } from '../lib/vault.svelte.js';
  import { navigate, folderUrl } from '../lib/router.svelte.js';
  import { api } from '../lib/api.js';
  import { toast } from '../lib/toast.svelte.js';
  import { copyText, relativeTime, plural } from '../lib/util.js';
  import { clock } from '../lib/clock.svelte.js';

  let { folderId = '' } = $props();

  let modal = $state(null);
  let loadError = $state('');

  const exists = $derived(folderId === '' || !!findFolder(folderId));
  const current = $derived(folderId ? findFolder(folderId) : null);
  const trail = $derived(folderTrail(folderId));
  const folders = $derived(childFolders(folderId));
  const passwords = $derived(folderPasswords(folderId));

  const summary = $derived.by(() => {
    const parts = [];
    if (passwords.length) parts.push(plural(passwords.length, 'password', 'passwords'));
    if (folders.length) parts.push(plural(folders.length, 'folder', 'folders'));
    return parts.join(' and ');
  });

  async function refresh() {
    try {
      await loadVault();
      loadError = '';
    } catch (err) {
      loadError = err.message;
    }
  }

  if (!vault.loaded) refresh();

  // --- click-to-copy: single click = password, double click = username ---
  let clickTimer = null;

  function rowClick(row) {
    clearTimeout(clickTimer);
    clickTimer = setTimeout(() => copyField(row, 'password'), 250);
  }

  function rowDblClick(row) {
    clearTimeout(clickTimer); // never copy the password as part of a double click
    clickTimer = null;
    if (!row.hasUsername) {
      toast('Nothing to copy, Username is empty', 'info');
      return;
    }
    copyField(row, 'username');
  }

  function rowKey(e, row) {
    if (e.key === 'Enter') copyField(row, e.shiftKey ? 'username' : 'password');
  }

  async function copyField(row, field) {
    try {
      const rec = await api('GET', `/api/passwords/${row.id}`);
      const value = rec[field];
      const name = field === 'password' ? 'Password' : 'Username';
      if (!value) {
        toast(`Nothing to copy, ${name} is empty`, 'info');
        return;
      }
      if (await copyText(value)) toast(`${name} was copied to clipboard`);
      else toast('Copy failed — your browser blocked clipboard access', 'error');
    } catch (err) {
      toast(err.message, 'error');
    }
  }

  async function toggleFavorite(kind, item) {
    const favorite = !item.favorite;
    item.favorite = favorite;
    try {
      await api('PUT', `/api/${kind}/${item.id}/favorite`, { favorite });
    } catch (err) {
      item.favorite = !favorite;
      toast(err.message, 'error');
    }
  }

  async function openRecord(row, mode) {
    try {
      const rec = await api('GET', `/api/passwords/${row.id}`);
      modal = mode === 'details' ? { kind: 'details', record: rec } : { kind: 'form', mode, record: rec };
    } catch (err) {
      toast(err.message, 'error');
    }
  }

  function passwordMenu(row) {
    return [
      { label: 'Details', icon: Info, onclick: () => openRecord(row, 'details') },
      { label: 'Edit', icon: Pencil, onclick: () => openRecord(row, 'edit') },
      { label: 'Edit as new', icon: CopyPlus, onclick: () => openRecord(row, 'copy') },
      { label: 'Move', icon: FolderInput, onclick: () => (modal = { kind: 'move', record: row }) },
      { label: 'Delete', icon: Trash2, danger: true, onclick: () => (modal = { kind: 'deletePassword', row }) },
    ];
  }

  function folderMenu(f) {
    return [
      { label: 'Open', icon: FolderOpen, onclick: () => navigate(folderUrl(f.id)) },
      { label: 'Rename', icon: Pencil, onclick: () => (modal = { kind: 'renameFolder', folder: f }) },
      { label: 'Delete', icon: Trash2, danger: true, onclick: () => (modal = { kind: 'deleteFolder', folder: f }) },
    ];
  }

  const newMenu = [
    { label: 'New folder', icon: FolderPlus, onclick: () => (modal = { kind: 'newFolder' }) },
    { label: 'New password', icon: KeyRound, onclick: () => (modal = { kind: 'form', mode: 'create', record: null }) },
  ];

  const close = () => (modal = null);

  async function createFolder(label) {
    await api('POST', '/api/folders', { label, parent: folderId });
    await refresh();
    toast('Folder created');
    close();
  }

  async function renameFolder(label) {
    await api('PUT', `/api/folders/${modal.folder.id}`, { label });
    await refresh();
    toast('Folder renamed');
    close();
  }

  async function deleteFolder() {
    await api('DELETE', `/api/folders/${modal.folder.id}`);
    await refresh();
    toast('Folder deleted');
    close();
  }

  async function deletePassword() {
    await api('DELETE', `/api/passwords/${modal.row.id}`);
    await refresh();
    toast('Password deleted');
    close();
  }

  async function saved() {
    await refresh();
    close();
  }

</script>

<Layout section="folders" currentFolder={folderId || null}>
  <div class="page">
    <div class="toolbar">
      <nav class="crumbs" aria-label="Breadcrumb">
        <button type="button" class="crumb" class:last={!folderId} onclick={() => navigate('/folders')}>
          <Folder size={17} /> Folders
        </button>
        {#each trail as f, i (f.id)}
          <ChevronRight size={16} class="sep" />
          <button type="button" class="crumb" class:last={i === trail.length - 1} onclick={() => navigate(folderUrl(f.id))}>{f.label}</button>
        {/each}
        {#if exists}<Menu items={newMenu} label="New…" />{/if}
      </nav>
      {#if exists}
        <button type="button" class="btn primary new-btn" onclick={() => (modal = { kind: 'form', mode: 'create', record: null })}>
          <Plus size={17} /> <span>New password</span>
        </button>
      {/if}
    </div>

    {#if loadError}
      <div class="callout danger">{loadError} <button class="btn" type="button" onclick={refresh}>Retry</button></div>
    {:else if !vault.loaded}
      <div class="list skeleton">{#each Array(4) as _}<div class="row"></div>{/each}</div>
    {:else if !exists}
      <div class="empty">
        <Folder size={40} />
        <h3>Folder not found</h3>
        <p>It may have been deleted. <a href="/folders" onclick={(e) => (e.preventDefault(), navigate('/folders'))}>Go to Folders</a></p>
      </div>
    {:else if folders.length === 0 && passwords.length === 0}
      <div class="empty">
        <FolderOpen size={44} />
        <h3>There is nothing here</h3>
        <p>Click on “…” next to {current ? current.label : 'Folders'} to add something.</p>
      </div>
    {:else}
      <p class="tip"><MousePointerClick size={15} /> Click a password to copy it · double-click to copy the username</p>
      <div class="list" role="list">
        <div class="head" aria-hidden="true">
          <span></span><span></span><span>Name</span><span></span><span></span><span class="mod">Modified</span>
        </div>
        {#each folders as f (f.id)}
          <div class="row folder" role="listitem">
            <button type="button" class="star" class:on={f.favorite} onclick={() => toggleFavorite('folders', f)} aria-label={f.favorite ? 'Unfavourite' : 'Favourite'}>
              <Star size={17} />
            </button>
            <span class="tile folder-tile"><Folder size={20} /></span>
            <button type="button" class="name-btn" onclick={() => navigate(folderUrl(f.id))}>{f.label}</button>
            <span></span>
            <Menu items={folderMenu(f)} label="Folder actions" />
            <span class="mod">{relativeTime(f.updated, clock.now)}</span>
          </div>
        {/each}
        {#each passwords as p (p.id)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="row password" role="listitem" title="Click: copy password · Double-click: copy username">
            <button type="button" class="star" class:on={p.favorite} onclick={() => toggleFavorite('passwords', p)} ondblclick={(e) => e.stopPropagation()} aria-label={p.favorite ? 'Unfavourite' : 'Favourite'}>
              <Star size={17} />
            </button>
            <PasswordIcon label={p.label} icon={p.icon} />
            <button
              type="button"
              class="name-btn copy-target"
              onclick={() => rowClick(p)}
              ondblclick={() => rowDblClick(p)}
              onkeydown={(e) => rowKey(e, p)}
            >{p.label}</button>
            <Shield health={p.health} />
            <Menu items={passwordMenu(p)} label="Password actions" />
            <span class="mod">{relativeTime(p.updated, clock.now)}</span>
          </div>
        {/each}
      </div>
      <p class="summary">{summary}</p>
    {/if}
  </div>
</Layout>

{#if modal?.kind === 'form'}
  <PasswordForm mode={modal.mode} record={modal.record} folder={folderId} onclose={close} onsaved={saved} />
{:else if modal?.kind === 'details'}
  <Details record={modal.record} onclose={close} onmove={() => (modal = { kind: 'move', record: modal.record })} />
{:else if modal?.kind === 'move'}
  <MoveDialog record={modal.record} onclose={close} />
{:else if modal?.kind === 'newFolder'}
  <NameDialog title="New folder" label="Folder name" confirmLabel="Create" onsubmit={createFolder} onclose={close} />
{:else if modal?.kind === 'renameFolder'}
  <NameDialog title="Rename folder" label="Folder name" initial={modal.folder.label} confirmLabel="Rename" onsubmit={renameFolder} onclose={close} />
{:else if modal?.kind === 'deleteFolder'}
  <ConfirmDialog title="Delete folder" message={`Delete the folder “${modal.folder.label}”? Only empty folders can be deleted.`} onconfirm={deleteFolder} onclose={close} />
{:else if modal?.kind === 'deletePassword'}
  <ConfirmDialog title="Delete password" message={`Delete “${modal.row.label}”? This can't be undone.`} onconfirm={deletePassword} onclose={close} />
{/if}

<style>
  .page { max-width: 1100px; margin: 0 auto; padding: 20px 24px 40px; }
  @media (max-width: 600px) { .page { padding: 14px 16px 32px; } }
  .toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
  .crumbs { display: flex; align-items: center; flex-wrap: wrap; gap: 2px; min-width: 0; }
  .crumbs :global(.sep) { color: var(--muted); flex: none; }
  .crumb {
    display: inline-flex; align-items: center; gap: 8px;
    height: 34px; padding: 0 8px; border: 0; border-radius: 8px;
    background: transparent; color: var(--muted); font: inherit; font-size: 16px; cursor: pointer;
    max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .crumb:hover { background: var(--hover); color: var(--text); }
  .crumb.last { color: var(--text); font-weight: 650; }
  .crumb :global(svg) { color: var(--accent); }
  @media (max-width: 600px) { .new-btn span { display: none; } .new-btn { padding: 0 10px; } }

  .tip { display: flex; align-items: center; gap: 6px; margin: 0 0 10px; color: var(--muted); font-size: 12.5px; }

  .list { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow); overflow: hidden; }
  .head, .row {
    display: grid;
    grid-template-columns: 40px 44px minmax(0, 1fr) 32px 36px 130px;
    align-items: center; gap: 4px;
    padding: 0 12px 0 6px;
  }
  .head { height: 36px; font-size: 12px; font-weight: 600; color: var(--muted); border-bottom: 1px solid var(--border); background: var(--surface-2); }
  .row { height: 54px; border-bottom: 1px solid var(--border); transition: background .1s; }
  .row:last-child { border-bottom: 0; }
  .row:hover { background: var(--hover); }
  .mod { text-align: right; color: var(--muted); font-size: 13px; white-space: nowrap; }
  @media (max-width: 600px) {
    .head, .row { grid-template-columns: 34px 40px minmax(0, 1fr) 28px 34px; }
    .mod { display: none; }
  }

  .star { display: grid; place-items: center; width: 34px; height: 34px; border: 0; border-radius: 8px; background: transparent; color: var(--border-strong); cursor: pointer; }
  .star:hover { color: var(--yellow); }
  .star.on { color: #f5b301; }
  .star.on :global(svg) { fill: currentColor; }

  .folder-tile { display: grid; place-items: center; width: 34px; height: 34px; border-radius: 9px; background: var(--accent-soft); color: var(--accent); }
  .folder-tile :global(svg) { fill: color-mix(in srgb, currentColor 25%, transparent); }

  .name-btn {
    height: 100%; min-width: 0; padding: 0 8px;
    border: 0; background: transparent; color: var(--text);
    font: inherit; font-weight: 500; text-align: left; cursor: pointer;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  /* Clipboard cursor: dark outline over a white halo so it shows on light and dark rows. */
  .copy-target {
    user-select: none;
    cursor: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke-linecap='round' stroke-linejoin='round'%3E%3Cg stroke='white' stroke-width='4.5'%3E%3Crect x='8' y='2' width='8' height='4' rx='1'/%3E%3Cpath d='M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2'/%3E%3C/g%3E%3Cg stroke='%231f2937' stroke-width='2'%3E%3Crect x='8' y='2' width='8' height='4' rx='1' fill='white'/%3E%3Cpath d='M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2' fill='white'/%3E%3C/g%3E%3C/svg%3E") 12 12, copy;
  }
  .name-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; border-radius: 6px; }

  .summary { margin: 10px 4px 0; color: var(--muted); font-size: 13px; }

  .empty { display: grid; justify-items: center; gap: 6px; padding: 64px 16px; text-align: center; color: var(--muted); }
  .empty :global(svg) { color: var(--accent); opacity: .7; }
  .empty h3 { color: var(--text); font-size: 17px; margin-top: 8px; }
  .empty p { margin: 0; }

  .skeleton .row { background: linear-gradient(90deg, transparent, var(--hover), transparent); background-size: 200% 100%; animation: shimmer 1.2s infinite; }
  @keyframes shimmer { to { background-position: -200% 0; } }
  .callout.danger { align-items: center; justify-content: space-between; }
</style>
