# Changelog

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
