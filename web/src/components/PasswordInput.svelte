<script>
  import { Eye, EyeOff } from '@lucide/svelte';

  let { value = $bindable(''), id, autocomplete = 'off', placeholder = '', mono = true, required = false } = $props();
  let visible = $state(false);
</script>

<div class="input-wrap">
  <input
    {id}
    class="input"
    class:mono={visible && mono}
    type={visible ? 'text' : 'password'}
    bind:value
    {autocomplete}
    {placeholder}
    {required}
    spellcheck="false"
  />
  <button type="button" class="icon-btn" onclick={() => (visible = !visible)} aria-label={visible ? 'Hide' : 'Show'} title={visible ? 'Hide' : 'Show'}>
    {#if visible}<EyeOff size={17} />{:else}<Eye size={17} />{/if}
  </button>
</div>
