<script>
  import { Sun, Moon, Monitor, RotateCcw } from '@lucide/svelte';
  import Modal from './Modal.svelte';
  import { theme, setTheme, resetHue, ORIGINAL_HUE } from '../lib/theme.svelte.js';

  let { onclose } = $props();
  const modes = [
    ['auto', 'Auto', Monitor],
    ['light', 'Light', Sun],
    ['dark', 'Dark', Moon],
  ];
  const stops = Array.from({ length: 13 }, (_, i) => `hsl(${i * 30} 85% 50%)`).join(', ');
</script>

<Modal title="Appearance" {onclose} size="sm">
  <div class="stack">
    <div class="field">
      <span class="field-label" id="ap-mode">Mode</span>
      <div class="seg" role="radiogroup" aria-labelledby="ap-mode">
        {#each modes as [value, text, Icon]}
          <button type="button" role="radio" aria-checked={theme.mode === value} class:on={theme.mode === value} onclick={() => setTheme({ mode: value })}>
            <Icon size={15} /> {text}
          </button>
        {/each}
      </div>
      <p class="hint">Auto follows your device's light or dark setting.</p>
    </div>

    <div class="field">
      <label class="field-label" for="ap-hue">Accent colour</label>
      <input
        id="ap-hue"
        class="hue"
        type="range"
        min="0"
        max="359"
        step="1"
        value={theme.hue ?? ORIGINAL_HUE}
        style="--stops:{stops}"
        oninput={(e) => setTheme({ hue: Number(e.currentTarget.value) })}
        aria-valuetext={theme.hue === null ? 'Original blue' : `Hue ${theme.hue}`}
      />
      <p class="hint">{theme.hue === null ? 'Original blue' : `Custom hue ${theme.hue}°`}</p>
    </div>
  </div>

  {#snippet footer()}
    <button type="button" class="btn" onclick={resetHue} disabled={theme.hue === null}><RotateCcw size={15} /> Reset colour</button>
    <button type="button" class="btn primary" onclick={onclose}>Done</button>
  {/snippet}
</Modal>

<style>
  .stack { display: grid; gap: 20px; }
  .seg { display: grid; grid-template-columns: repeat(3, 1fr); gap: 3px; padding: 3px; border-radius: 9px; background: var(--surface-2); border: 1px solid var(--border); }
  .seg button {
    display: inline-flex; align-items: center; justify-content: center; gap: 6px;
    height: 32px; border: 0; border-radius: 6px; background: transparent; color: var(--muted);
    font: inherit; font-size: 13px; font-weight: 600; cursor: pointer;
  }
  .seg button.on { background: var(--surface); color: var(--accent); box-shadow: 0 1px 3px rgb(0 0 0 / .12); }
  .hue {
    -webkit-appearance: none; appearance: none;
    width: 100%; height: 14px; margin: 8px 0 0; border-radius: 999px;
    background: linear-gradient(to right, var(--stops));
    cursor: pointer;
  }
  .hue::-webkit-slider-thumb {
    -webkit-appearance: none; width: 22px; height: 22px; border-radius: 50%;
    background: #fff; border: 3px solid var(--text); box-shadow: 0 1px 4px rgb(0 0 0 / .35);
  }
  .hue::-moz-range-thumb { width: 16px; height: 16px; border-radius: 50%; background: #fff; border: 3px solid var(--text); }
  .hue:focus-visible { outline: 2px solid var(--accent); outline-offset: 4px; }
</style>
