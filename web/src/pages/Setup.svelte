<script>
  import { adoptTheme } from '../lib/theme.svelte.js';
  import { TriangleAlert, Copy, Check, ArrowRight } from '@lucide/svelte';
  import AuthCard from '../components/AuthCard.svelte';
  import PasswordInput from '../components/PasswordInput.svelte';
  import { api } from '../lib/api.js';
  import { session } from '../lib/session.svelte.js';
  import { navigate } from '../lib/router.svelte.js';
  import { copyText } from '../lib/util.js';
  import { toast } from '../lib/toast.svelte.js';

  let password = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);
  let masterKey = $state('');
  let savedIt = $state(false);
  let copied = $state(false);

  async function submit(e) {
    e.preventDefault();
    error = '';
    if (password !== confirm) {
      error = 'Passwords do not match.';
      return;
    }
    busy = true;
    try {
      const res = await api('POST', '/api/setup', { password, confirm });
      masterKey = res.masterKey;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function copy() {
    if (await copyText(masterKey)) {
      copied = true;
      toast('Master key copied to clipboard');
      setTimeout(() => (copied = false), 2000);
    }
  }

  function finish() {
    session.setupRequired = false;
    session.user = { username: 'admin', isAdmin: true, theme: { mode: 'auto', hue: null } };
    adoptTheme(session.user.theme);
    navigate('/folders', { replace: true });
  }
</script>

<AuthCard wide>
  {#if !masterKey}
    <h1>Welcome to passman</h1>
    <p class="lead">First-time setup. Choose the password for the <b>admin</b> account.</p>
    <form class="form" onsubmit={submit}>
      <label class="field">
        <span class="field-label">Username</span>
        <input class="input" value="admin" disabled />
      </label>
      <label class="field">
        <span class="field-label">Admin password</span>
        <PasswordInput bind:value={password} autocomplete="new-password" mono={false} />
      </label>
      <label class="field">
        <span class="field-label">Confirm password</span>
        <PasswordInput bind:value={confirm} autocomplete="new-password" mono={false} />
      </label>
      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
      <button type="submit" class="btn primary block go" disabled={busy}>
        {busy ? 'Setting up…' : 'Create admin and generate master key'}
      </button>
    </form>
  {:else}
    <h1>Save your master key</h1>
    <p class="lead">This is shown <b>only once</b>.</p>
    <div class="key-box">
      <code class="key">{masterKey}</code>
      <button type="button" class="btn" onclick={copy}>
        {#if copied}<Check size={16} /> Copied{:else}<Copy size={16} /> Copy{/if}
      </button>
    </div>
    <div class="callout danger">
      <TriangleAlert size={20} />
      <div>
        <b>Save this master key somewhere safe right now</b> — for example printed and locked away, or in a separate password manager.
        <ul>
          <li>It is needed to reset <b>any</b> user's password (including admin), from the Manage users page or with <code>reset-password &lt;master key&gt;</code> inside the container.</li>
          <li>It is never stored in readable form and <b>cannot be recovered</b>. Without it, a forgotten password means that user's saved passwords are lost.</li>
          <li>Anyone holding it, together with the database, could recover any vault. Keep it private.</li>
        </ul>
      </div>
    </div>
    <label class="check confirm"><input type="checkbox" bind:checked={savedIt} /> I have saved the master key somewhere safe</label>
    <button type="button" class="btn primary block go" disabled={!savedIt} onclick={finish}>
      Continue to passman <ArrowRight size={17} />
    </button>
  {/if}
</AuthCard>

<style>
  h1 { font-size: 21px; }
  .lead { color: var(--muted); margin: 6px 0 18px; }
  .form { display: grid; gap: 14px; }
  .go { height: 42px; margin-top: 4px; }
  .key-box {
    display: flex; align-items: center; justify-content: space-between; gap: 12px;
    padding: 14px 14px 14px 18px; margin-bottom: 16px;
    background: var(--surface-2); border: 1.5px solid var(--accent); border-radius: var(--radius);
  }
  .key { font-family: var(--mono); font-size: 22px; letter-spacing: .08em; font-weight: 600; overflow-wrap: anywhere; user-select: all; }
  @media (max-width: 480px) { .key { font-size: 17px; } }
  ul { margin: 8px 0 0; padding-left: 18px; display: grid; gap: 4px; }
  code { font-family: var(--mono); font-size: 12.5px; }
  .confirm { margin: 18px 0 12px; font-weight: 600; }
</style>
