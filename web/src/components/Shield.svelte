<script>
  import { Shield, ShieldCheck, ShieldAlert } from '@lucide/svelte';
  import { healthText } from '../lib/util.js';

  let { health, size = 18 } = $props();
  const Icon = $derived(health === 'green' ? ShieldCheck : health === 'red' ? ShieldAlert : Shield);
</script>

<span class="shield {health}" title={healthText[health]} aria-label={healthText[health]}>
  <Icon {size} />
</span>

<style>
  .shield { display: inline-grid; place-items: center; }
  .red { color: var(--red); }
  .orange { color: var(--orange); }
  .yellow { color: var(--yellow); }
  .green { color: var(--green); }
  .shield :global(svg) { fill: color-mix(in srgb, currentColor 16%, transparent); }
</style>
