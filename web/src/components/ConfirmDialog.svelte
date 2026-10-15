<script>
  import Modal from './Modal.svelte';

  let { title, message, confirmLabel = 'Delete', onconfirm, onclose } = $props();
  let error = $state('');
  let busy = $state(false);

  async function confirm() {
    busy = true;
    error = '';
    try {
      await onconfirm();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<Modal {title} {onclose} size="sm">
  <p class="msg">{message}</p>
  {#if error}<p class="error-text">{error}</p>{/if}
  {#snippet footer()}
    <button type="button" class="btn" onclick={onclose}>Cancel</button>
    <button type="button" class="btn danger solid" onclick={confirm} disabled={busy}>{confirmLabel}</button>
  {/snippet}
</Modal>

<style>
  .msg { margin: 0; }
</style>
