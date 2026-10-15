<script module>
  let closeOpen = null;
</script>

<script>
  import { Ellipsis } from '@lucide/svelte';

  let { items, label = 'More actions', size = 18 } = $props();
  let open = $state(false);
  let btn;
  let pos = $state({ top: 0, left: 0 });
  const width = 210;

  function close() {
    open = false;
  }

  function toggle(e) {
    e.stopPropagation();
    if (open) return close();
    closeOpen?.();
    const r = btn.getBoundingClientRect();
    const height = items.length * 38 + 12;
    const below = r.bottom + 4 + height < innerHeight;
    pos = {
      top: below ? r.bottom + 4 : Math.max(8, r.top - height - 4),
      left: Math.min(Math.max(8, r.right - width), innerWidth - width - 8),
    };
    open = true;
    closeOpen = close;
  }

  function choose(e, item) {
    e.stopPropagation();
    close();
    item.onclick();
  }
</script>

<svelte:window
  onclick={close}
  onresize={close}
  onkeydown={(e) => e.key === 'Escape' && open && (e.stopPropagation(), close())}
/>
<svelte:document onscrollcapture={close} />

<button
  bind:this={btn}
  type="button"
  class="icon-btn trigger"
  class:active={open}
  aria-label={label}
  aria-haspopup="menu"
  aria-expanded={open}
  onclick={toggle}
  ondblclick={(e) => e.stopPropagation()}
>
  <Ellipsis {size} />
</button>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="menu" role="menu" tabindex="-1" style="top:{pos.top}px; left:{pos.left}px; width:{width}px" onclick={(e) => e.stopPropagation()}>
    {#each items as item (item.label)}
      <button type="button" role="menuitem" class:danger={item.danger} onclick={(e) => choose(e, item)}>
        {#if item.icon}<item.icon size={16} />{/if}
        {item.label}
      </button>
    {/each}
  </div>
{/if}

<style>
  .menu {
    position: fixed; z-index: 60;
    padding: 6px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    box-shadow: var(--shadow-lg);
    animation: pop .1s ease-out;
  }
  .menu button {
    display: flex; align-items: center; gap: 10px;
    width: 100%; height: 36px; padding: 0 10px;
    border: 0; border-radius: 7px;
    background: transparent; color: var(--text);
    font: inherit; text-align: left; cursor: pointer;
  }
  .menu button:hover, .menu button:focus-visible { background: var(--hover); outline: none; }
  .menu button.danger { color: var(--red); }
  @keyframes pop { from { opacity: 0; transform: translateY(-4px); } }
</style>
