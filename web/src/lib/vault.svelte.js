import { api } from './api.js';

export const vault = $state({ folders: [], passwords: [], loaded: false });

export async function loadVault() {
  const data = await api('GET', '/api/vault');
  vault.folders = data.folders;
  vault.passwords = data.passwords;
  vault.loaded = true;
}

export function clearVault() {
  vault.folders = [];
  vault.passwords = [];
  vault.loaded = false;
}

const byLabel = (a, b) => a.label.localeCompare(b.label, undefined, { sensitivity: 'base', numeric: true });

export function childFolders(parentId) {
  return vault.folders.filter((f) => f.parent === parentId).sort(byLabel);
}

export function folderPasswords(folderId) {
  return vault.passwords.filter((p) => p.folder === folderId).sort(byLabel);
}

export function findFolder(id) {
  return vault.folders.find((f) => f.id === id);
}

/** Folders from root down to id (excluding root). */
export function folderTrail(id) {
  const trail = [];
  const seen = new Set();
  let f = id ? findFolder(id) : null;
  while (f && !seen.has(f.id)) {
    seen.add(f.id);
    trail.unshift(f);
    f = f.parent ? findFolder(f.parent) : null;
  }
  return trail;
}

/** Flat, depth-annotated folder list for <select> pickers. */
export function folderOptions() {
  const out = [{ id: '', label: 'Folders', depth: 0 }];
  const walk = (parent, depth) => {
    for (const f of childFolders(parent)) {
      out.push({ id: f.id, label: f.label, depth });
      walk(f.id, depth + 1);
    }
  };
  walk('', 1);
  return out;
}
