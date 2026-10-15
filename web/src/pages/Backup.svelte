<script>
  import { Download, Upload, TriangleAlert, ShieldCheck, FileJson, Archive } from '@lucide/svelte';
  import Layout from '../components/Layout.svelte';
  import PasswordInput from '../components/PasswordInput.svelte';
  import { api } from '../lib/api.js';
  import { toast } from '../lib/toast.svelte.js';
  import { loadVault } from '../lib/vault.svelte.js';
  import { handleExpired } from '../lib/session.svelte.js';

  // --- backup ---
  let backupPassword = $state('');
  let backupBusy = $state(false);
  let backupError = $state('');

  async function download() {
    backupBusy = true;
    backupError = '';
    try {
      const res = await fetch('/api/backup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: backupPassword }),
      });
      if (res.status === 401) return handleExpired();
      if (!res.ok) throw new Error((await res.json().catch(() => null))?.error || 'Backup failed.');
      const name = /filename="([^"]+)"/.exec(res.headers.get('content-disposition') || '')?.[1] || 'passwords-backup.json';
      const blob = await res.blob();
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = name;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(a.href), 5000);
      toast(`Downloaded ${name}`);
    } catch (err) {
      backupError = err.message;
    } finally {
      backupBusy = false;
    }
  }

  // --- restore ---
  let file = $state(null);
  let fileInput;
  let mode = $state('skip');
  let restorePassword = $state('');
  let needPassword = $state(false);
  let restoreBusy = $state(false);
  let restoreError = $state('');
  let result = $state(null);

  function pick(e) {
    file = e.currentTarget.files?.[0] ?? null;
    needPassword = false;
    restorePassword = '';
    restoreError = '';
    result = null;
  }

  async function restore(e) {
    e.preventDefault();
    if (!file) {
      restoreError = 'Choose a backup file first.';
      return;
    }
    restoreBusy = true;
    restoreError = '';
    result = null;
    const form = new FormData();
    form.append('file', file);
    form.append('mode', mode);
    form.append('password', restorePassword);
    try {
      result = await api('POST', '/api/restore', form);
      await loadVault();
      toast('Restore finished');
    } catch (err) {
      restoreError = err.message;
      if (err.data?.needPassword) needPassword = true;
    } finally {
      restoreBusy = false;
    }
  }

  const logText = $derived(result ? (result.log.length ? result.log.join('\n') : 'Nothing was skipped or overwritten.') : '');
</script>

<Layout section="backup">
  <div class="page">
    <h1><Archive size={22} /> Backup and Restore</h1>

    <div class="cards">
      <section class="card panel">
        <header>
          <span class="badge"><Download size={20} /></span>
          <div>
            <h2>Backup</h2>
            <p class="sub">Download all your passwords and folders as one JSON file.</p>
          </div>
        </header>

        <label class="field">
          <span class="field-label">Backup password (optional)</span>
          <PasswordInput bind:value={backupPassword} autocomplete="new-password" mono={false} placeholder="Leave empty for a plain .json file" />
        </label>

        {#if backupPassword}
          <div class="callout info"><ShieldCheck size={18} /><span>The backup will be a password-protected <b>.zip</b> (AES-256). Open it with 7-Zip or restore it here.</span></div>
        {:else}
          <div class="callout warn"><TriangleAlert size={18} /><span>Without a backup password the file contains <b>all your passwords in plain text</b>. Store it somewhere safe or set a password above.</span></div>
        {/if}

        {#if backupError}<p class="error-text">{backupError}</p>{/if}
        <button type="button" class="btn primary" onclick={download} disabled={backupBusy}>
          <Download size={17} /> {backupBusy ? 'Preparing…' : 'Download backup'}
        </button>
      </section>

      <section class="card panel">
        <header>
          <span class="badge"><Upload size={20} /></span>
          <div>
            <h2>Restore</h2>
            <p class="sub">Import a passman backup (<code>.json</code> or password-protected <code>.zip</code>).</p>
          </div>
        </header>

        <form class="restore" onsubmit={restore}>
          <input bind:this={fileInput} class="sr-only" type="file" accept=".json,.zip,application/json,application/zip" onchange={pick} />
          <button type="button" class="drop" onclick={() => fileInput.click()}>
            <FileJson size={22} />
            {#if file}<span><b>{file.name}</b> · {(file.size / 1024).toFixed(1)} KB</span>{:else}<span>Choose backup file…</span>{/if}
          </button>

          {#if needPassword}
            <label class="field">
              <span class="field-label">Backup password</span>
              <PasswordInput bind:value={restorePassword} autocomplete="off" mono={false} />
            </label>
          {/if}

          <fieldset>
            <legend class="field-label">When a password already exists</legend>
            <label class="check"><input type="radio" name="mode" value="skip" bind:group={mode} /> <span><b>Skip</b> — keep my current one</span></label>
            <label class="check"><input type="radio" name="mode" value="overwrite" bind:group={mode} /> <span><b>Overwrite</b> — replace it with the one from the file</span></label>
          </fieldset>

          {#if restoreError}<p class="error-text">{restoreError}</p>{/if}
          <button type="submit" class="btn primary" disabled={restoreBusy}>
            <Upload size={17} /> {restoreBusy ? 'Restoring…' : 'Restore'}
          </button>
        </form>

        {#if result}
          <div class="result">
            <p class="totals">
              <span><b>{result.imported}</b> imported</span>
              <span><b>{result.overwritten}</b> overwritten</span>
              <span><b>{result.skipped}</b> skipped</span>
              <span class="muted">{result.folders} folders added/updated</span>
            </p>
            <label class="field">
              <span class="field-label">Import log</span>
              <textarea class="input log" readonly rows="6">{logText}</textarea>
            </label>
          </div>
        {/if}
      </section>
    </div>
  </div>
</Layout>

<style>
  .page { max-width: 1100px; margin: 0 auto; padding: 22px 24px 40px; }
  @media (max-width: 600px) { .page { padding: 16px; } }
  h1 { display: flex; align-items: center; gap: 10px; font-size: 20px; margin-bottom: 18px; }
  h1 :global(svg) { color: var(--accent); }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 20px; align-items: start; }
  @media (max-width: 400px) { .cards { grid-template-columns: 1fr; } }
  .panel { padding: 20px; display: grid; gap: 16px; }
  .panel header { display: flex; gap: 14px; align-items: flex-start; }
  .badge { display: grid; place-items: center; width: 42px; height: 42px; flex: none; border-radius: 11px; background: var(--accent-soft); color: var(--accent); }
  h2 { font-size: 16px; }
  .sub { margin: 4px 0 0; color: var(--muted); font-size: 13.5px; }
  .panel > .btn, .restore > .btn { justify-self: start; }
  .restore { display: grid; gap: 16px; }
  .drop {
    display: flex; align-items: center; gap: 12px;
    padding: 18px; border: 1.5px dashed var(--border-strong); border-radius: var(--radius);
    background: var(--surface-2); color: var(--muted); font: inherit; cursor: pointer; text-align: left;
  }
  .drop:hover { border-color: var(--accent); color: var(--text); }
  .drop :global(svg) { color: var(--accent); flex: none; }
  .drop span { overflow-wrap: anywhere; }
  fieldset { border: 0; padding: 0; margin: 0; display: grid; gap: 8px; }
  .result { display: grid; gap: 12px; padding-top: 4px; border-top: 1px solid var(--border); }
  .totals { display: flex; flex-wrap: wrap; gap: 6px 18px; margin: 12px 0 0; }
  .muted { color: var(--muted); }
  .log { font-family: var(--mono); font-size: 12.5px; }
  code { font-family: var(--mono); font-size: 12.5px; }
</style>
