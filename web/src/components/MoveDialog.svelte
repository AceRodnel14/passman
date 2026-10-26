<script>
  import { untrack } from 'svelte';
  import Modal from './Modal.svelte';
  import FolderPicker from './FolderPicker.svelte';
  import PasswordInput from './PasswordInput.svelte';
  import { api } from '../lib/api.js';
  import { toast } from '../lib/toast.svelte.js';
  import { vault, findFolder } from '../lib/vault.svelte.js';

  /** record: { id, label, folder } */
  let { record, onclose } = $props();
  const from = untrack(() => record.folder ?? '');
  let target = $state(from);
  let password = $state('');
  let error = $state('');
  let busy = $state(false);

  const nameOf = (id) => (id ? (findFolder(id)?.label ?? 'folder') : 'Folders');

  function close() {
    password = '';
    onclose();
  }

  async function submit(e) {
    e.preventDefault();
    error = '';
    if (target === from) {
      error = 'Choose a different folder.';
      return;
    }
    if (!password) {
      error = 'Enter your login password to confirm.';
      return;
    }
    busy = true;
    try {
      const res = await api('POST', `/api/passwords/${record.id}/move`, { folder: target, password });
      const row = vault.passwords.find((p) => p.id === record.id);
      if (row) row.folder = res.folder ?? target;
      toast(`Moved to ${nameOf(res.folder ?? target)}`);
      close();
    } catch (err) {
      error = err.message;
      password = '';
    } finally {
      busy = false;
    }
  }
</script>

<Modal title={`Move ${record.label}`} onclose={close} size="sm">
  <form id="move-form" class="stack" onsubmit={submit}>
    <div class="field">
      <span class="field-label">Move to</span>
      <FolderPicker selected={target} onselect={(id) => (target = id)} label="Destination folder" />
    </div>
    <label class="field">
      <span class="field-label">Your login password</span>
      <PasswordInput bind:value={password} autocomplete="current-password" mono={false} />
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    <p class="hint">Enter your login password to confirm each move.</p>
  </form>
  {#snippet footer()}
    <button type="button" class="btn" onclick={close}>Cancel</button>
    <button type="submit" form="move-form" class="btn primary" disabled={busy}>Move</button>
  {/snippet}
</Modal>

<style>
  .stack { display: grid; gap: 14px; }
</style>
