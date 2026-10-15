<script>
  import { Eye, EyeOff, User, KeyRound, Globe, Folder, StickyNote, Lock, Type, Clock } from '@lucide/svelte';
  import Modal from './Modal.svelte';
  import Shield from './Shield.svelte';
  import PasswordIcon from './PasswordIcon.svelte';
  import { findFolder } from '../lib/vault.svelte.js';
  import { relativeTime, healthText } from '../lib/util.js';
  import { clock } from '../lib/clock.svelte.js';

  let { record, onclose } = $props();
  let revealed = $state(new Set());

  function toggle(key) {
    const next = new Set(revealed);
    next.has(key) ? next.delete(key) : next.add(key);
    revealed = next;
  }

  const folderName = $derived(record.folder ? (findFolder(record.folder)?.label ?? '—') : 'Folders');
  const safeUrl = $derived(/^https?:\/\//i.test(record.url) ? record.url : null);
</script>

{#snippet secret(key, value)}
  <div class="secret">
    <span class="value mono">{revealed.has(key) ? value : '••••••••••'}</span>
    <button type="button" class="icon-btn" onclick={() => toggle(key)} aria-label={revealed.has(key) ? 'Hide' : 'Reveal'} title={revealed.has(key) ? 'Hide' : 'Reveal'}>
      {#if revealed.has(key)}<EyeOff size={17} />{:else}<Eye size={17} />{/if}
    </button>
  </div>
{/snippet}

<Modal title={record.label} {onclose} size="lg">
  <div class="meta">
    <PasswordIcon label={record.label} icon={record.icon} size={40} />
    <span class="pill"><Shield health={record.health} size={16} /> {healthText[record.health]}</span>
    <span class="pill"><Clock size={15} /> Modified {relativeTime(record.updated, clock.now)}</span>
  </div>

  <dl class="grid">
    <div class="item">
      <dt><User size={14} /> Username</dt>
      <dd>{#if record.username}<span class="value">{record.username}</span>{:else}<span class="empty">—</span>{/if}</dd>
    </div>
    <div class="item">
      <dt><KeyRound size={14} /> Password</dt>
      <dd>{@render secret('password', record.password)}</dd>
    </div>
    <div class="item">
      <dt><Globe size={14} /> Website</dt>
      <dd>
        {#if safeUrl}<a href={safeUrl} target="_blank" rel="noopener noreferrer">{record.url}</a>
        {:else if record.url}<span class="value">{record.url}</span>
        {:else}<span class="empty">—</span>{/if}
      </dd>
    </div>
    <div class="item">
      <dt><Folder size={14} /> Folder</dt>
      <dd><span class="value">{folderName}</span></dd>
    </div>
    {#each record.customFields as f, i}
      <div class="item">
        <dt>{#if f.type === 'secret'}<Lock size={14} />{:else}<Type size={14} />{/if} {f.label}</dt>
        <dd>
          {#if f.type === 'secret'}{@render secret('cf' + i, f.value)}
          {:else if f.value}<span class="value">{f.value}</span>
          {:else}<span class="empty">—</span>{/if}
        </dd>
      </div>
    {/each}
  </dl>

  <div class="notes">
    <div class="dt"><StickyNote size={14} /> Notes</div>
    {#if record.notes}<div class="notes-body">{record.notes}</div>{:else}<span class="empty">No notes</span>{/if}
  </div>
</Modal>

<style>
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 18px; }
  .pill {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 4px 10px; border-radius: 999px;
    background: var(--surface-2); border: 1px solid var(--border);
    font-size: 12.5px; color: var(--muted);
  }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 12px 24px; margin: 0; }
  .item { min-width: 0; }
  dt, .dt { display: flex; align-items: center; gap: 6px; font-size: 12.5px; font-weight: 600; color: var(--muted); margin-bottom: 4px; }
  dd { margin: 0; min-height: 36px; display: flex; align-items: center; padding: 0 12px; background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); overflow: hidden; }
  .value { overflow-wrap: anywhere; user-select: text; }
  .mono { font-family: var(--mono); }
  .empty { color: var(--muted); }
  .secret { display: flex; align-items: center; justify-content: space-between; width: 100%; gap: 8px; }
  .secret .icon-btn { margin-right: -8px; }
  .notes { margin-top: 16px; }
  .notes-body {
    white-space: pre-wrap; overflow-wrap: anywhere;
    padding: 10px 12px; min-height: 80px; max-height: 300px; overflow: auto;
    background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm);
  }
</style>
