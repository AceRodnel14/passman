<script>
  import { untrack } from 'svelte';
  import Modal from './Modal.svelte';

  let { title, label = 'Name', initial = '', confirmLabel = 'Create', onsubmit, onclose } = $props();
  let name = $state(untrack(() => initial));
  let error = $state('');
  let busy = $state(false);

  async function submit(e) {
    e.preventDefault();
    if (!name.trim()) {
      error = `${label} is required.`;
      return;
    }
    busy = true;
    error = '';
    try {
      await onsubmit(name.trim());
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<Modal {title} {onclose} size="sm">
  <form onsubmit={submit} id="name-dialog">
    <label class="field">
      <span class="field-label">{label}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" bind:value={name} autofocus maxlength="200" />
    </label>
    {#if error}<p class="error-text">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn" onclick={onclose}>Cancel</button>
    <button type="submit" class="btn primary" form="name-dialog" disabled={busy}>{confirmLabel}</button>
  {/snippet}
</Modal>
