<script>
  import { iconHue, firstLetter } from '../lib/icon.js';

  /** icon: null | { mode: 'auto'|'letters'|'logo', color?, text?, image? } */
  let { label, icon = null, size = 34 } = $props();

  const hue = $derived(iconHue(label, icon));
  const text = $derived(icon?.mode === 'letters' && icon.text ? icon.text : firstLetter(label));
  const logo = $derived(icon?.mode === 'logo' && icon.image ? icon.image : null);
</script>

<span
  class="tile"
  class:logo={!!logo}
  style="--h:{hue}; --size:{size}px"
  aria-hidden="true"
>
  {#if logo}
    <img src={logo} alt="" />
  {:else}
    <span class="text" class:two={[...text].length > 1}>{text}</span>
  {/if}
</span>

<style>
  .tile {
    display: grid; place-items: center; flex: none;
    width: var(--size); height: var(--size);
    border-radius: calc(var(--size) * 0.26);
    background: hsl(var(--h) 70% 92%);
    color: hsl(var(--h) 55% 32%);
    font-weight: 700;
    font-size: calc(var(--size) * 0.44);
    line-height: 1;
    overflow: hidden;
  }
  .text.two { font-size: calc(var(--size) * 0.36); letter-spacing: -0.02em; }
  .tile.logo { box-shadow: inset 0 0 0 1px rgb(0 0 0 / 0.06); }
  img { width: 100%; height: 100%; object-fit: contain; image-rendering: auto; }
  @media (prefers-color-scheme: dark) {
    /* Logos keep the light tile so dark logos stay visible. */
    .tile:not(.logo) { background: hsl(var(--h) 40% 22%); color: hsl(var(--h) 70% 80%); }
  }
</style>
