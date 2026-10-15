<script>
  import { ChevronRight, Folder, FolderOpen } from '@lucide/svelte';
  import FolderTree from './FolderTree.svelte';
  import { childFolders } from '../lib/vault.svelte.js';
  import { navigate, folderUrl } from '../lib/router.svelte.js';

  let { parent, depth = 1, current, expanded, ontoggle, onpick } = $props();
  const folders = $derived(childFolders(parent));
</script>

{#each folders as f (f.id)}
  {@const hasKids = childFolders(f.id).length > 0}
  {@const open = expanded.has(f.id)}
  <div class="node" class:current={current === f.id} style="--depth:{depth}">
    <button
      type="button"
      class="twisty"
      class:open
      class:hidden={!hasKids}
      onclick={() => ontoggle(f.id)}
      aria-label={open ? 'Collapse' : 'Expand'}
      tabindex={hasKids ? 0 : -1}
    >
      <ChevronRight size={14} />
    </button>
    <button type="button" class="label" onclick={() => (navigate(folderUrl(f.id)), onpick?.())} title={f.label}>
      {#if current === f.id}<FolderOpen size={16} />{:else}<Folder size={16} />{/if}
      <span>{f.label}</span>
    </button>
  </div>
  {#if hasKids && open}
    <FolderTree parent={f.id} depth={depth + 1} {current} {expanded} {ontoggle} {onpick} />
  {/if}
{/each}

<style>
  .node {
    display: flex; align-items: center;
    padding-left: calc(var(--depth) * 14px);
    border-radius: 8px;
  }
  .node:hover { background: color-mix(in srgb, var(--accent) 8%, transparent); }
  .node.current { background: var(--accent); }
  .node.current .label, .node.current .twisty { color: #fff; }
  .twisty {
    display: grid; place-items: center; width: 22px; height: 30px; flex: none;
    border: 0; background: transparent; color: var(--muted); cursor: pointer;
    transition: transform .12s;
  }
  .twisty.open :global(svg) { transform: rotate(90deg); }
  .twisty :global(svg) { transition: transform .12s; }
  .twisty.hidden { visibility: hidden; }
  .label {
    display: flex; align-items: center; gap: 8px; min-width: 0; flex: 1;
    height: 32px; padding: 0 8px 0 2px;
    border: 0; background: transparent; color: var(--text);
    font: inherit; text-align: left; cursor: pointer;
  }
  .label :global(svg) { flex: none; color: var(--accent); }
  .node.current .label :global(svg) { color: #fff; }
  .label span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
