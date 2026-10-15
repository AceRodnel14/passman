<script>
  import Modal from './Modal.svelte';
  import PasswordInput from './PasswordInput.svelte';
  import { api } from '../lib/api.js';
  import { toast } from '../lib/toast.svelte.js';

  let { onclose } = $props();
  let current = $state('');
  let password = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(e) {
    e.preventDefault();
    error = '';
    if (password !== confirm) {
      error = 'New passwords do not match.';
      return;
    }
    busy = true;
    try {
      await api('POST', '/api/account/password', { current, password, confirm });
      toast('Password changed');
      onclose();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<Modal title="Change your password" {onclose} size="sm">
  <form id="change-pw" class="stack" onsubmit={submit}>
    <label class="field">
      <span class="field-label">Current password</span>
      <PasswordInput bind:value={current} autocomplete="current-password" mono={false} />
    </label>
    <label class="field">
      <span class="field-label">New password</span>
      <PasswordInput bind:value={password} autocomplete="new-password" mono={false} />
    </label>
    <label class="field">
      <span class="field-label">Confirm new password</span>
      <PasswordInput bind:value={confirm} autocomplete="new-password" mono={false} />
    </label>
    {#if error}<p class="error-text">{error}</p>{/if}
    <p class="hint">Your saved passwords stay as they are. Other signed-in devices will be signed out.</p>
  </form>
  {#snippet footer()}
    <button type="button" class="btn" onclick={onclose}>Cancel</button>
    <button type="submit" form="change-pw" class="btn primary" disabled={busy}>Change password</button>
  {/snippet}
</Modal>

<style>
  .stack { display: grid; gap: 14px; }
</style>
