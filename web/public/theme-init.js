// Runs before first paint (CSP allows script-src 'self'): sets data-theme from the cached theme
// so 'auto' users on a dark OS do not flash light. Same key/shape as src/lib/theme.svelte.js.
try {
  var t = JSON.parse(localStorage.getItem('passman.theme'));
  var m = t && t.mode;
  document.documentElement.setAttribute('data-theme', m === 'auto' || m === 'dark' ? m : 'light');
} catch {
  /* storage unavailable: the bundle applies the theme later */
}
