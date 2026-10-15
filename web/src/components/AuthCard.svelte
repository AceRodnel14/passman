<script>
  import { KeyRound } from '@lucide/svelte';
  let { children, wide = false } = $props();
</script>

<div class="screen">
  <div class="brand"><span class="logo"><KeyRound size={22} /></span> passman</div>
  <div class="card auth" class:wide>
    {@render children()}
  </div>
  <p class="tagline">passman — keep your passwords close</p>
</div>

<style>
  .screen {
    min-height: 100%;
    display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 22px;
    padding: 32px 16px;
    background:
      radial-gradient(1200px 600px at 10% -10%, color-mix(in srgb, #3cc8f2 55%, transparent), transparent 60%),
      radial-gradient(900px 500px at 110% 110%, color-mix(in srgb, #003e66 70%, transparent), transparent 60%),
      var(--accent);
  }
  .brand { display: flex; align-items: center; gap: 12px; color: #fff; font: 700 26px/1 var(--font); letter-spacing: -.02em; }
  .logo { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 12px; background: rgb(255 255 255 / .2); }
  .auth { width: 100%; max-width: 400px; padding: 26px; border: 0; box-shadow: 0 20px 60px rgb(0 30 60 / .3); }
  .auth.wide { max-width: 520px; }
  .tagline { margin: 0; color: rgb(255 255 255 / .85); font-size: 13px; }
</style>
