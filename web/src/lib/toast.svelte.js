export const toasts = $state([]);
let next = 1;

export function toast(message, kind = 'success', timeout = 3500) {
  const id = next++;
  toasts.push({ id, message, kind });
  if (timeout) setTimeout(() => dismiss(id), timeout);
}

export function dismiss(id) {
  const i = toasts.findIndex((t) => t.id === id);
  if (i !== -1) toasts.splice(i, 1);
}
