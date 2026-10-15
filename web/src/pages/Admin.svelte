<script>
  import { Users, KeyRound, ShieldCheck, Lock } from '@lucide/svelte';
  import Layout from '../components/Layout.svelte';
  import Modal from '../components/Modal.svelte';
  import PasswordInput from '../components/PasswordInput.svelte';
  import { api } from '../lib/api.js';
  import { toast } from '../lib/toast.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { relativeTime } from '../lib/util.js';
  import { clock } from '../lib/clock.svelte.js';

  let users = $state(null);
  let error = $state('');
  let target = $state(null);

  let masterKey = $state('');
  let password = $state('');
  let confirm = $state('');
  let formError = $state('');
  let busy = $state(false);

  async function load() {
    try {
      users = await api('GET', '/api/admin/users');
    } catch (err) {
      error = err.message;
    }
  }
  load();

  function openReset(u) {
    target = u;
    masterKey = password = confirm = formError = '';
  }

  async function reset(e) {
    e.preventDefault();
    formError = '';
    if (!masterKey.trim()) return (formError = 'Enter the master key.');
    if (password !== confirm) return (formError = 'Passwords do not match.');
    busy = true;
    try {
      await api('POST', `/api/admin/users/${target.id}/password`, { masterKey, password, confirm });
      toast(`Password reset for ${target.username}`);
      target = null;
      load();
    } catch (err) {
      formError = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<Layout section="admin">
  <div class="page">
    <h1><Users size={22} /> Manage users</h1>
    <div class="callout info intro">
      <Lock size={18} />
      <span>Admins can reset account passwords but can never see anyone's saved passwords — each vault is encrypted with its owner's key. A reset needs the <b>master key</b> created at setup, and keeps the user's saved passwords.</span>
    </div>

    {#if error}
      <p class="error-text">{error}</p>
    {:else if !users}
      <p class="hint">Loading…</p>
    {:else}
      <div class="list card">
        {#each users as u (u.id)}
          <div class="row">
            <span class="avatar">{u.username.slice(0, 1).toUpperCase()}</span>
            <div class="who">
              <div class="name">
                {u.username}
                {#if u.isAdmin}<span class="tag"><ShieldCheck size={13} /> admin</span>{/if}
                {#if u.username === session.user.username}<span class="tag you">you</span>{/if}
              </div>
              <div class="meta">Joined {relativeTime(u.createdAt, clock.now)} · password changed {relativeTime(u.passwordChangedAt, clock.now)}</div>
            </div>
            <button type="button" class="btn" onclick={() => openReset(u)}><KeyRound size={16} /> <span class="lbl">Reset password</span></button>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</Layout>

{#if target}
  <Modal title={`Reset password — ${target.username}`} onclose={() => (target = null)} size="sm">
    <form id="reset-form" class="stack" onsubmit={reset}>
      <label class="field">
        <span class="field-label">Master key</span>
        <PasswordInput bind:value={masterKey} autocomplete="off" />
      </label>
      <label class="field">
        <span class="field-label">New password</span>
        <PasswordInput bind:value={password} autocomplete="new-password" mono={false} />
      </label>
      <label class="field">
        <span class="field-label">Confirm new password</span>
        <PasswordInput bind:value={confirm} autocomplete="new-password" mono={false} />
      </label>
      {#if formError}<p class="error-text">{formError}</p>{/if}
      <p class="hint">{target.username}'s saved passwords are kept. They will be signed out on all devices{target.username === session.user.username ? ' except this one' : ''}.</p>
    </form>
    {#snippet footer()}
      <button type="button" class="btn" onclick={() => (target = null)}>Cancel</button>
      <button type="submit" form="reset-form" class="btn primary" disabled={busy}>Reset password</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .page { max-width: 900px; margin: 0 auto; padding: 22px 24px 40px; }
  @media (max-width: 600px) { .page { padding: 16px; } }
  h1 { display: flex; align-items: center; gap: 10px; font-size: 20px; margin-bottom: 14px; }
  h1 :global(svg) { color: var(--accent); }
  .intro { margin-bottom: 18px; }
  .list { overflow: hidden; }
  .row { display: flex; align-items: center; gap: 14px; padding: 12px 16px; border-bottom: 1px solid var(--border); }
  .row:last-child { border-bottom: 0; }
  .avatar { display: grid; place-items: center; width: 38px; height: 38px; flex: none; border-radius: 50%; background: var(--accent-soft); color: var(--accent); font-weight: 700; }
  .who { flex: 1; min-width: 0; }
  .name { display: flex; align-items: center; gap: 8px; font-weight: 600; flex-wrap: wrap; }
  .meta { color: var(--muted); font-size: 12.5px; }
  .tag { display: inline-flex; align-items: center; gap: 4px; padding: 1px 8px; border-radius: 999px; background: var(--accent-soft); color: var(--accent); font-size: 11.5px; font-weight: 600; }
  .tag.you { background: var(--surface-2); color: var(--muted); border: 1px solid var(--border); }
  @media (max-width: 600px) { .lbl { display: none; } .row .btn { padding: 0 10px; } }
  .stack { display: grid; gap: 14px; }
</style>
