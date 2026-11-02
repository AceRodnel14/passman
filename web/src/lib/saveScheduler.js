// Debounced save with an exit flush; plain JS so node:test can cover it without a DOM.
export function createSaveScheduler({ delay, save, saveOnExit }) {
  let timer = null;
  return {
    schedule() {
      clearTimeout(timer);
      timer = setTimeout(() => {
        timer = null;
        save();
      }, delay);
    },
    cancel() {
      clearTimeout(timer);
      timer = null;
    },
    /** Send a still-pending save once (page hide); a no-op when nothing is pending. */
    flushOnExit() {
      if (timer === null) return;
      clearTimeout(timer);
      timer = null;
      saveOnExit();
    },
    get pending() {
      return timer !== null;
    },
  };
}
