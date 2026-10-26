# Changelog

## 0.2.1

- Appearance per account (saved on the server, follows the user across devices): accent colour hue slider with a reset to
  the original blue, and light / dark / auto (follow the device) mode. The logo and favicon colours are unchanged.
  New `PUT /api/account/theme`; `GET /api/state`, login and register return `user.theme`. DB: `users.theme_mode`,
  `users.theme_hue` (added automatically on start).
- New **Move** action on password entries (row menu and details): pick the destination in a folder tree and confirm
  with your login password, verified on the server on every move (`POST /api/passwords/{id}/move`; wrong password 403).
  It shares the per-username login throttle and the key-derivation concurrency bound.
- The folder field when creating or editing an entry is now a tree picker. Rule: changing an entry's folder always needs
  the login password, in the Move dialog and in the edit form (`PUT /api/passwords/{id}` takes `loginPassword` only when
  the folder changes). Other edits and creating entries need no password.

## 0.2.0

- Browser extension API: `GET /api/sites` (saved sites with ids and labels, no secrets; session expiry) and
  `GET /api/fill/{id}?url=` (credentials, only when the entry matches the page origin). Matching rules are shared with
  the extension (`match.go`).
- `POST /api/export`: a user's backup, authenticated with login credentials instead of the cookie.
- `POST /api/backup-all` (disabled unless `BACKUP_ALL_ALLOWED_IPS` is set): every user's vault in one nested AES-256 zip,
  for direct connections from allowed addresses only. See the README security notes before enabling it.
- Login and export attempts are counted before key derivation and their concurrency is bounded.
- New passman logo (favicon, header and login card); the version is shown in the UI and in `GET /api/state`.
- CI: shared lint jobs (golangci-lint, go test, hadolint, yamllint, gitleaks, npm typecheck and build); `svelte-check`
  added as `typecheck` for the frontend. Minor lint fixes (unchecked errors logged).

## 0.1.0

- Initial release: encrypted per-user vaults, folders, password generator, admin and master key recovery, backup and
  restore (Nextcloud Passman compatible), Docker image.
