<script>
  import { X } from '@lucide/svelte';

  let { title, onclose, size = 'md', escToClose = true, actions, children, footer } = $props();

  function onkeydown(e) {
    if (e.key === 'Escape' && escToClose) {
      e.stopPropagation();
      onclose?.();
    }
  }
</script>

<svelte:window {onkeydown} />

<div class="backdrop">
  <div class="modal {size}" role="dialog" aria-modal="true" aria-label={title}>
    <header>
      <h2>{title}</h2>
      <div class="actions">
        {@render actions?.()}
        <button class="close" type="button" onclick={onclose} aria-label="Close"><X size={18} /></button>
      </div>
    </header>
    <div class="body">
      {@render children()}
    </div>
    {#if footer}
      <footer>{@render footer()}</footer>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed; inset: 0; z-index: 50;
    background: rgb(10 20 30 / 0.45);
    backdrop-filter: blur(2px);
    display: grid; place-items: center;
    padding: 16px;
    animation: fade .12s ease-out;
  }
  .modal {
    width: 100%;
    max-height: calc(100vh - 32px);
    display: flex; flex-direction: column;
    background: var(--surface);
    border-radius: 14px;
    box-shadow: var(--shadow-lg);
    overflow: hidden;
    animation: pop .14s ease-out;
  }
  .modal.sm { max-width: 420px; }
  .modal.md { max-width: 560px; }
  .modal.lg { max-width: 860px; }
  header {
    display: flex; align-items: center; justify-content: space-between; gap: 12px;
    padding: 14px 14px 14px 20px;
    background: linear-gradient(120deg, var(--accent), color-mix(in srgb, var(--accent) 70%, var(--accent-glow)));
    color: #fff;
  }
  h2 { font-size: 17px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .actions { display: flex; align-items: center; gap: 4px; flex: none; }
  .close {
    display: grid; place-items: center; width: 32px; height: 32px;
    border: 0; border-radius: 8px; background: transparent; color: #fff; cursor: pointer;
  }
  .close:hover { background: rgb(255 255 255 / 0.18); }
  .body { padding: 20px; overflow: auto; }
  footer {
    display: flex; justify-content: flex-end; gap: 10px;
    padding: 14px 20px; border-top: 1px solid var(--border); background: var(--surface-2);
  }
  @keyframes fade { from { opacity: 0; } }
  @keyframes pop { from { opacity: 0; transform: translateY(6px) scale(.985); } }
</style>
