<script>
  import { CircleCheck, CircleAlert, Info, X } from '@lucide/svelte';
  import { toasts, dismiss } from '../lib/toast.svelte.js';

  const icons = { success: CircleCheck, error: CircleAlert, info: Info };
</script>

<div class="stack" aria-live="polite">
  {#each toasts as t (t.id)}
    {@const Icon = icons[t.kind] ?? Info}
    <div class="toast {t.kind}" role="status">
      <Icon size={18} />
      <span>{t.message}</span>
      <button type="button" aria-label="Dismiss" onclick={() => dismiss(t.id)}><X size={16} /></button>
    </div>
  {/each}
</div>

<style>
  .stack {
    position: fixed; z-index: 100; right: 16px; bottom: 16px; left: 16px;
    display: flex; flex-direction: column; align-items: flex-end; gap: 8px;
    pointer-events: none;
  }
  .toast {
    pointer-events: auto;
    display: flex; align-items: center; gap: 10px;
    max-width: 420px;
    padding: 10px 8px 10px 14px;
    background: #1f2a36; color: #f1f5f9;
    border-radius: 10px;
    box-shadow: var(--shadow-lg);
    font-size: 14px;
    animation: slide .16s ease-out;
  }
  .toast.success :global(svg:first-child) { color: #4ade80; }
  .toast.error :global(svg:first-child) { color: #f87171; }
  .toast.info :global(svg:first-child) { color: #7cc4f0; }
  span { flex: 1; }
  button {
    display: grid; place-items: center; width: 26px; height: 26px;
    border: 0; border-radius: 6px; background: transparent; color: #cbd5e1; cursor: pointer;
  }
  button:hover { background: rgb(255 255 255 / 0.1); }
  @keyframes slide { from { opacity: 0; transform: translateY(8px); } }
</style>
