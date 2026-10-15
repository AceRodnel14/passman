<script>
  import { LogIn, UserPlus, ArrowRight } from '@lucide/svelte';
  import AuthCard from '../components/AuthCard.svelte';
  import PasswordInput from '../components/PasswordInput.svelte';
  import { api } from '../lib/api.js';
  import { session } from '../lib/session.svelte.js';
  import { navigate } from '../lib/router.svelte.js';

  let tab = $state('login');
  let username = $state('');
  let password = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);
  let forgot = $state(false);

  function switchTab(t) {
    tab = t;
    error = '';
    password = '';
    confirm = '';
    forgot = false;
  }

  async function submit(e) {
    e.preventDefault();
    error = '';
    if (!username.trim() || !password) {
      error = 'Enter your username and password.';
      return;
    }
    if (tab === 'register' && password !== confirm) {
      error = 'Passwords do not match.';
      return;
    }
    busy = true;
    try {
      const user =
        tab === 'login'
          ? await api('POST', '/api/login', { username, password })
          : await api('POST', '/api/register', { username, password, confirm });
      session.user = user;
      navigate('/folders', { replace: true });
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<AuthCard>
  <div class="tabs" role="tablist">
    <button type="button" role="tab" aria-selected={tab === 'login'} class:active={tab === 'login'} onclick={() => switchTab('login')}>
      <LogIn size={16} /> Log in
    </button>
    <button type="button" role="tab" aria-selected={tab === 'register'} class:active={tab === 'register'} onclick={() => switchTab('register')}>
      <UserPlus size={16} /> Register
    </button>
  </div>

  <form class="form" onsubmit={submit}>
    <label class="field">
      <span class="field-label">Username</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" autofocus />
    </label>
    <label class="field">
      <span class="field-label">Password</span>
      <PasswordInput bind:value={password} autocomplete={tab === 'login' ? 'current-password' : 'new-password'} mono={false} />
    </label>
    {#if tab === 'register'}
      <label class="field">
        <span class="field-label">Confirm password</span>
        <PasswordInput bind:value={confirm} autocomplete="new-password" mono={false} />
      </label>
      <p class="hint">At least 8 characters. Your password also unlocks your vault, so pick one you'll remember.</p>
    {/if}

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}

    <button type="submit" class="btn primary block go" disabled={busy}>
      {tab === 'login' ? 'Log in' : 'Create account'} <ArrowRight size={17} />
    </button>
  </form>

  {#if tab === 'login'}
    <div class="forgot">
      <button type="button" class="link" onclick={() => (forgot = !forgot)}>Forgot password?</button>
      {#if forgot}
        <p class="callout info">Ask your passman admin to reset it. Resetting needs the master key, and keeps all your saved passwords.</p>
      {/if}
    </div>
  {/if}
</AuthCard>

<style>
  .tabs { display: grid; grid-template-columns: 1fr 1fr; gap: 4px; padding: 4px; margin-bottom: 20px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 10px; }
  .tabs button {
    display: flex; align-items: center; justify-content: center; gap: 8px; height: 36px;
    border: 0; border-radius: 7px; background: transparent; color: var(--muted); font: inherit; font-weight: 600; cursor: pointer;
  }
  .tabs button.active { background: var(--surface); color: var(--accent); box-shadow: var(--shadow); }
  .form { display: grid; gap: 14px; }
  .go { height: 42px; margin-top: 4px; }
  .forgot { margin-top: 14px; text-align: center; }
  .link { border: 0; background: none; color: var(--accent); font: inherit; font-size: 13.5px; cursor: pointer; padding: 4px; }
  .link:hover { text-decoration: underline; }
  .forgot .callout { margin: 10px 0 0; text-align: left; }
</style>
