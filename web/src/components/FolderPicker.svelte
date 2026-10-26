<script>
  import { untrack } from 'svelte';
  import { ChevronRight, Folder, FolderOpen } from '@lucide/svelte';
  import { childFolders, folderTrail } from '../lib/vault.svelte.js';

  /** Selection-only folder tree (no navigation). selected: '' is the root. */
  let { selected = '', onselect, label = 'Folder' } = $props();

  // Start with the path to the selected folder open.
  let expanded = $state(new Set(untrack(() => folderTrail(selected).slice(0, -1).map((f) => f.id))));

  function toggle(id) {
    const next = new Set(expanded);
    next.has(id) ? next.delete(id) : next.add(id);
    expanded = next;
  }
</script>

{#snippet nodes(parent, depth)}
  {#each childFolders(parent) as f (f.id)}
    {@const hasKids = childFolders(f.id).length > 0}
    {@const open = expanded.has(f.id)}
    <div class="node" class:current={selected === f.id} style="--depth:{depth}">
      <button
        type="button"
        class="twisty"
        class:open
        class:hidden={!hasKids}
        onclick={() => toggle(f.id)}
        aria-label={open ? `Collapse ${f.label}` : `Expand ${f.label}`}
        aria-expanded={hasKids ? open : undefined}
        tabindex={hasKids ? 0 : -1}
        aria-hidden={hasKids ? undefined : true}
      >
        <ChevronRight size={14} />
      </button>
      <button type="button" class="label" aria-pressed={selected === f.id} onclick={() => onselect(f.id)} title={f.label}>
        {#if selected === f.id}<FolderOpen size={16} />{:else}<Folder size={16} />{/if}
        <span>{f.label}</span>
      </button>
    </div>
    {#if hasKids && open}{@render nodes(f.id, depth + 1)}{/if}
  {/each}
{/snippet}

<div class="picker" role="group" aria-label={label}>
  <div class="node root" class:current={selected === ''} style="--depth:0">
    <button type="button" class="label" aria-pressed={selected === ''} onclick={() => onselect('')}>
      {#if selected === ''}<FolderOpen size={17} />{:else}<Folder size={17} />{/if}
      <span>Folders</span>
    </button>
  </div>
  {@render nodes('', 1)}
</div>

<style>
  .picker { max-height: 220px; overflow: auto; padding: 6px; background: var(--sidebar); border: 1px solid var(--border-strong); border-radius: var(--radius-sm); }
  .node { display: flex; align-items: center; padding-left: calc(var(--depth) * 14px); border-radius: 8px; }
  .node:hover { background: color-mix(in srgb, var(--accent) 8%, transparent); }
  .node.current { background: var(--accent); }
  .node.current .label, .node.current .twisty { color: #fff; }
  .root .label { font-weight: 600; padding-left: 8px; }
  .twisty {
    display: grid; place-items: center; width: 22px; height: 30px; flex: none;
    border: 0; background: transparent; color: var(--muted); cursor: pointer;
  }
  .twisty :global(svg) { transition: transform .12s; }
  .twisty.open :global(svg) { transform: rotate(90deg); }
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
  .label:focus-visible, .twisty:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; border-radius: 8px; }
  .node.current .label:focus-visible, .node.current .twisty:focus-visible { outline-color: #fff; }
</style>
