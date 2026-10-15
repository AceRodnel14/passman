<script>
  import { untrack } from 'svelte';
  import { Eye, EyeOff, RefreshCw, SlidersHorizontal, KeyRound, User, Tag, Globe, Folder, StickyNote, Plus, Trash2, Lock, Type, Image as ImageIcon, Upload } from '@lucide/svelte';
  import PasswordIcon from './PasswordIcon.svelte';
  import { palette, graphemes, shrinkImage } from '../lib/icon.js';
  import Modal from './Modal.svelte';
  import { api } from '../lib/api.js';
  import { toast } from '../lib/toast.svelte.js';
  import { folderOptions } from '../lib/vault.svelte.js';
  import { storageGet, storageSet } from '../lib/util.js';

  /** mode: 'create' | 'edit' | 'copy' (Edit as new) */
  let { mode, record = null, folder = '', onclose, onsaved } = $props();

  const titles = { create: 'Create password', edit: 'Edit password', copy: 'Edit as new' };

  // The form is seeded once from its props; each open mounts a fresh form.
  const init = untrack(() => ({
    label: record ? (mode === 'copy' ? `${record.label}-copy` : record.label) : '',
    folder: record ? record.folder : folder,
  }));
  const seed = untrack(() => record);
  let label = $state(init.label);
  let password = $state(seed?.password ?? '');
  let username = $state(seed?.username ?? '');
  let url = $state(seed?.url ?? '');
  let folderId = $state(init.folder);
  let notes = $state(seed?.notes ?? '');
  let fields = $state((seed?.customFields ?? []).map((f) => ({ ...f, show: false })));
  let showPassword = $state(false);

  // Icon: auto (first letter), letters (custom 1–2 chars) or an uploaded logo.
  // The colour is the tile background in every mode, so it also shows behind
  // transparent parts of a logo.
  let iconMode = $state(seed?.icon?.mode ?? 'auto');
  let iconColor = $state(seed?.icon?.color ?? '');
  let iconText = $state(seed?.icon?.text ?? '');
  let iconImage = $state(seed?.icon?.image ?? '');
  let iconError = $state('');
  let iconBusy = $state(false);
  let fileInput = $state();
  const iconPreview = $derived({ mode: iconMode, color: iconColor, text: iconText, image: iconImage });

  function onIconText(e) {
    iconText = graphemes(e.currentTarget.value).slice(0, 2).join('');
    e.currentTarget.value = iconText;
  }

  async function onIconFile(e) {
    const file = e.currentTarget.files?.[0];
    e.currentTarget.value = '';
    if (!file) return;
    iconError = '';
    iconBusy = true;
    try {
      iconImage = await shrinkImage(file); // only this 64×64 PNG is ever saved
      iconMode = 'logo';
      error = '';
    } catch (err) {
      iconError = err.message;
    } finally {
      iconBusy = false;
    }
  }

  function iconBody() {
    const color = iconColor || undefined;
    if (iconMode === 'letters') return { mode: 'letters', text: iconText.trim(), color };
    if (iconMode === 'logo') return { mode: 'logo', image: iconImage, color };
    return color ? { mode: 'auto', color } : null;
  }
  let error = $state('');
  let busy = $state(false);

  const saved = storageGet('passman.generator', {});
  let genLength = $state(String(saved.length ?? 10));
  let genNumbers = $state(saved.numbers ?? false);
  let genSymbols = $state(saved.symbols ?? false);
  let genOpen = $state(false);
  let genError = $state('');

  const options = $derived(folderOptions());

  function validLength() {
    const s = genLength.trim();
    if (!/^\d+$/.test(s)) return null;
    const n = Number(s);
    return n >= 8 && n <= 84 ? n : null;
  }

  async function generate() {
    const n = validLength();
    if (n === null) {
      genError = 'Enter a whole number from 8 to 84.';
      genOpen = true;
      return;
    }
    genError = '';
    storageSet('passman.generator', { length: n, numbers: genNumbers, symbols: genSymbols });
    try {
      const res = await api('GET', `/api/generate?length=${n}&numbers=${genNumbers}&symbols=${genSymbols}`);
      password = res.password;
      showPassword = true;
    } catch (err) {
      genError = err.message;
      genOpen = true;
    }
  }

  function onLengthInput() {
    genError = validLength() === null ? 'Enter a whole number from 8 to 84.' : '';
  }

  function addField() {
    fields.push({ label: '', type: 'text', value: '', show: false });
  }

  async function save(e) {
    e.preventDefault();
    error = '';
    if (!label.trim() && !password) error = 'Name and Password are required.';
    else if (!label.trim()) error = 'Name is required.';
    else if (!password) error = 'Password is required.';
    else if (fields.some((f) => !f.label.trim() && f.value)) error = 'Every custom field needs a name.';
    else if (iconMode === 'letters' && !iconText.trim()) error = 'Enter 1 or 2 characters for the icon, or choose Auto.';
    else if (iconMode === 'logo' && !iconImage) error = 'Upload an image for the icon, or choose another icon type.';
    if (error) return;

    const body = {
      label: label.trim(),
      password,
      username,
      url,
      folder: folderId,
      notes,
      customFields: fields
        .filter((f) => f.label.trim() || f.value)
        .map(({ label, type, value }) => ({ label: label.trim(), type, value })),
      icon: iconBody(),
    };
    busy = true;
    try {
      const res = mode === 'edit' ? await api('PUT', `/api/passwords/${record.id}`, body) : await api('POST', '/api/passwords', body);
      toast(mode === 'edit' ? 'Password updated' : 'Password created');
      onsaved(res.id);
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<svelte:window onclick={() => (genOpen = false)} />

<Modal title={titles[mode]} {onclose} size="lg" escToClose={false}>
  <form id="password-form" class="form" onsubmit={save} novalidate>
    <div class="field">
      <div class="label-row">
        <label class="field-label" for="pf-password"><KeyRound size={14} /> Password <span class="req">*</span></label>
        <div class="tools">
          <button type="button" class="icon-btn" onclick={() => (showPassword = !showPassword)} title={showPassword ? 'Hide' : 'Show'} aria-label={showPassword ? 'Hide password' : 'Show password'}>
            {#if showPassword}<EyeOff size={16} />{:else}<Eye size={16} />{/if}
          </button>
          <button type="button" class="icon-btn" onclick={generate} title="Generate password" aria-label="Generate password"><RefreshCw size={16} /></button>
          <div class="gen-wrap">
            <button type="button" class="icon-btn" class:active={genOpen} onclick={(e) => (e.stopPropagation(), (genOpen = !genOpen))} title="Generator options" aria-label="Generator options" aria-expanded={genOpen}>
              <SlidersHorizontal size={16} />
            </button>
            {#if genOpen}
              <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
              <div class="gen-pop" onclick={(e) => e.stopPropagation()}>
                <label class="field">
                  <span class="field-label">Number of characters</span>
                  <input class="input" type="text" inputmode="numeric" bind:value={genLength} oninput={onLengthInput} />
                </label>
                {#if genError}<p class="error-text">{genError}</p>{/if}
                <label class="check"><input type="checkbox" bind:checked={genNumbers} /> Include numbers</label>
                <label class="check"><input type="checkbox" bind:checked={genSymbols} /> Include special characters</label>
                <button type="button" class="btn primary block" onclick={async () => { await generate(); if (!genError) genOpen = false; }}>
                  <RefreshCw size={15} /> Generate
                </button>
              </div>
            {/if}
          </div>
        </div>
      </div>
      <input
        id="pf-password"
        class="input mono"
        type={showPassword ? 'text' : 'password'}
        bind:value={password}
        autocomplete="new-password"
        spellcheck="false"
      />
    </div>

    <label class="field">
      <span class="field-label"><User size={14} /> Username</span>
      <input class="input" bind:value={username} autocomplete="off" spellcheck="false" />
    </label>

    <label class="field">
      <span class="field-label"><Tag size={14} /> Name <span class="req">*</span></span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" bind:value={label} maxlength="200" autofocus={mode !== 'edit'} />
    </label>

    <label class="field">
      <span class="field-label"><Globe size={14} /> Website</span>
      <input class="input" bind:value={url} placeholder="https://" autocomplete="off" spellcheck="false" />
    </label>

    <label class="field">
      <span class="field-label"><Folder size={14} /> Folder</span>
      <select class="input" bind:value={folderId}>
        {#each options as o (o.id)}
          <option value={o.id}>{'   '.repeat(o.depth)}{o.label}</option>
        {/each}
      </select>
    </label>

    <div class="field">
      <span class="field-label"><ImageIcon size={14} /> Icon</span>
      <div class="icon-field">
        <div class="icon-preview"><PasswordIcon label={label || '?'} icon={iconPreview} size={64} /></div>
        <div class="icon-controls">
          <div class="seg" role="radiogroup" aria-label="Icon type">
            {#each [['auto', 'Auto'], ['letters', 'Letters'], ['logo', 'Logo']] as [value, text]}
              <button type="button" role="radio" aria-checked={iconMode === value} class:on={iconMode === value} onclick={() => (iconMode = value)}>{text}</button>
            {/each}
          </div>
          <div class="icon-row">
            {#if iconMode === 'letters'}
              <input class="input letters" value={iconText} oninput={onIconText} placeholder="AB" aria-label="Icon letters" />
            {:else if iconMode === 'logo'}
              <input bind:this={fileInput} class="sr-only" type="file" accept="image/png,image/x-icon,image/vnd.microsoft.icon,.ico,image/jpeg,image/webp,image/gif,image/svg+xml" onchange={onIconFile} />
              <button type="button" class="btn small-btn" onclick={() => fileInput.click()} disabled={iconBusy}>
                <Upload size={15} /> {iconImage ? 'Replace…' : 'Upload…'}
              </button>
              {#if iconImage}
                <button type="button" class="btn small-btn danger" onclick={() => (iconImage = '')}>Remove</button>
              {/if}
            {/if}
          </div>
          <div class="swatches" role="radiogroup" aria-label="Icon colour">
            <button type="button" role="radio" aria-checked={iconColor === ''} class="sw auto" class:on={iconColor === ''} onclick={() => (iconColor = '')} title="Automatic colour" aria-label="Automatic colour">A</button>
            {#each Object.entries(palette) as [name, hue]}
              <button type="button" role="radio" aria-checked={iconColor === name} class="sw" class:on={iconColor === name} style="--h:{hue}" onclick={() => (iconColor = name)} title={name} aria-label={name}></button>
            {/each}
          </div>
          {#if iconError}<p class="error-text">{iconError}</p>{/if}
          {#if iconMode === 'logo'}<p class="hint">PNG, ICO, JPG, WEBP or SVG — shrunk to 64×64 before saving.</p>{/if}
        </div>
      </div>
    </div>

    <div class="field wide">
      <div class="label-row">
        <span class="field-label">Custom fields</span>
        <button type="button" class="btn ghost small" onclick={addField}><Plus size={15} /> New field</button>
      </div>
      {#if fields.length === 0}
        <p class="hint none">Add extra values such as API tokens, recovery codes or a second password.</p>
      {/if}
      {#each fields as f, i}
        <div class="cf-row">
          <input class="input cf-name" placeholder="Name" bind:value={f.label} aria-label="Field name" />
          <div class="input-wrap cf-value">
            <input
              class="input"
              class:mono={f.type === 'secret'}
              type={f.type === 'secret' && !f.show ? 'password' : 'text'}
              placeholder="Value"
              bind:value={f.value}
              autocomplete="off"
              spellcheck="false"
              aria-label="Field value"
            />
            {#if f.type === 'secret'}
              <button type="button" class="icon-btn" onclick={() => (f.show = !f.show)} aria-label={f.show ? 'Hide' : 'Show'} title={f.show ? 'Hide' : 'Show'}>
                {#if f.show}<EyeOff size={16} />{:else}<Eye size={16} />{/if}
              </button>
            {/if}
          </div>
          <div class="cf-type">
            {#if f.type === 'secret'}<Lock size={14} />{:else}<Type size={14} />{/if}
            <select class="input" bind:value={f.type} aria-label="Field type">
              <option value="text">Text</option>
              <option value="secret">Secret</option>
            </select>
          </div>
          <button type="button" class="icon-btn danger" onclick={() => fields.splice(i, 1)} aria-label="Delete field" title="Delete field"><Trash2 size={16} /></button>
        </div>
      {/each}
    </div>

    <label class="field wide">
      <span class="field-label"><StickyNote size={14} /> Notes</span>
      <textarea class="input" rows="6" bind:value={notes}></textarea>
    </label>

    {#if error}<p class="error-text wide" role="alert">{error}</p>{/if}
  </form>

  {#snippet footer()}
    <button type="submit" form="password-form" class="btn primary save" disabled={busy}>Save</button>
  {/snippet}
</Modal>

<style>
  .form { display: grid; grid-template-columns: 1fr 1fr; gap: 16px 24px; }
  .wide { grid-column: 1 / -1; }
  @media (max-width: 680px) { .form { grid-template-columns: 1fr; } }
  .form .field-label { min-height: 22px; }
  .label-row { display: flex; align-items: center; justify-content: space-between; min-height: 22px; margin-bottom: 6px; }
  .label-row .field-label { margin-bottom: 0; }
  .tools { display: flex; gap: 2px; margin: -6px -6px -6px 0; }
  .tools .icon-btn { width: 30px; height: 30px; }
  .req { color: var(--red); font-weight: 700; }
  .gen-wrap { position: relative; }
  .gen-pop {
    position: absolute; right: 0; top: 36px; z-index: 5;
    width: 250px; padding: 14px;
    display: grid; gap: 10px;
    background: var(--surface); border: 1px solid var(--border); border-radius: 10px;
    box-shadow: var(--shadow-lg);
  }
  .gen-pop .error-text { margin: -4px 0 0; }
  .btn.small { height: 30px; padding: 0 10px; font-weight: 600; color: var(--accent); }
  .none { margin: 0; }
  .cf-row { display: grid; grid-template-columns: minmax(120px, 1fr) minmax(160px, 2fr) 128px 32px; gap: 8px; align-items: center; margin-top: 8px; }
  @media (max-width: 680px) { .cf-row { grid-template-columns: 1fr 1fr; } .cf-value { grid-column: 1 / -1; grid-row: 2; } }
  .cf-type { position: relative; display: flex; align-items: center; }
  .cf-type :global(svg) { position: absolute; left: 10px; color: var(--muted); pointer-events: none; }
  .cf-type select { padding-left: 30px; }
  .icon-btn.danger:hover { color: var(--red); }
  .save { min-width: 220px; }

  .icon-field { display: flex; gap: 14px; align-items: flex-start; }
  .icon-preview { flex: none; }
  .icon-controls { flex: 1; min-width: 0; display: grid; gap: 8px; }
  .seg { display: grid; grid-template-columns: repeat(3, 1fr); gap: 3px; padding: 3px; border-radius: 9px; background: var(--surface-2); border: 1px solid var(--border); }
  .seg button { height: 28px; border: 0; border-radius: 6px; background: transparent; color: var(--muted); font: inherit; font-size: 13px; font-weight: 600; cursor: pointer; }
  .seg button.on { background: var(--surface); color: var(--accent); box-shadow: 0 1px 3px rgb(0 0 0 / .12); }
  .icon-row { display: flex; gap: 8px; flex-wrap: wrap; }
  .icon-row:empty { display: none; }
  .letters { width: 90px; height: 34px; font-weight: 700; text-align: center; }
  .small-btn { height: 34px; padding: 0 12px; }
  .swatches { display: flex; gap: 6px; flex-wrap: wrap; }
  .sw {
    width: 26px; height: 26px; padding: 0; border-radius: 7px; cursor: pointer;
    border: 2px solid transparent; background: hsl(var(--h) 70% 85%);
    box-shadow: inset 0 0 0 1px rgb(0 0 0 / .06);
  }
  .sw.auto { background: var(--surface-2); color: var(--muted); font-size: 11px; font-weight: 700; border-color: var(--border); }
  .sw.on { border-color: var(--text); }
  @media (prefers-color-scheme: dark) { .sw:not(.auto) { background: hsl(var(--h) 40% 30%); } }
  @media (max-width: 680px) { .save { width: 100%; } }
</style>
