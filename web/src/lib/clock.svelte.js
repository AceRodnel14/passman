// Ticks every 30s so relative times ("5 minutes ago") stay fresh.
export const clock = $state({ now: Date.now() });
setInterval(() => (clock.now = Date.now()), 30_000);
