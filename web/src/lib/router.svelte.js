export const route = $state({ path: location.pathname });

export function navigate(to, { replace = false } = {}) {
  if (to === location.pathname) return;
  history[replace ? 'replaceState' : 'pushState']({}, '', to);
  route.path = to;
}

window.addEventListener('popstate', () => {
  route.path = location.pathname;
});

export function folderUrl(id) {
  return id ? `/folders/${id}` : '/folders';
}
