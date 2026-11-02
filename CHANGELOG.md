# Changelog

## 1.0.0

First major release: a lot changed at once, some of it breaking for existing setups.

- Breaking: the default image is `eacer/passman:latest` (pin `vX.Y` / `vX.Y.Z` with `IMAGE`).
- Breaking: `.env.example` leaves `POSTGRES_PASSWORD` empty; set it in `.env` before the first start (Postgres 18
  refuses a blank password). Compose stops with `set POSTGRES_PASSWORD` when it is blank or unset, instead of
  starting a database that can't initialise.
- Breaking: the runtime image is now `alpine:3.24` with the `psql` 18 client (was `alpine:3.22`, psql 17).
- Self-hosting with Docker Compose: new `docker-compose.yml` (passman + `postgres:18.6-alpine3.24`, `pg-data` volume, optional
  `.env`, `DATABASE_URL` built from `POSTGRES_*`) and `.env.example`. Default image `eacer/passman:latest`. Env variables are
  unchanged.
- CI uses the shared gitlab-actions `app.yml` pipeline (lint + tests, test build with smoke run, multi-arch image, GitHub
  sync).
- passman starts without the database: the HTTP server comes up first and the connection is retried in the background
  (1 s, doubling up to 30 s; failures are logged without the URL or password). Tables are created and the first-run
  hint is logged once connected. Until then `/api/*` answers `503` JSON. An unset `DATABASE_URL` is logged as an error
  and no longer stops the process. `reset-password` still fails fast.
- New `GET /readyz` (no auth): `200` when the database answers a ping, otherwise `503`. `/healthz` stays dependency-free.
- When the database drops while running, requests that hit it answer `503` with `Retry-After` instead of `500`, and
  the web UI retries loading the session on `503`, so it recovers without a reload.
- Graceful shutdown on SIGTERM/SIGINT: in-flight requests get up to 10 s, then the pool is closed.
- README: new **Self-hosting** section (Compose quick start, variables, backups, upgrading, health). New `dockerdesc.md`
  for Docker Hub. `.env` is git-ignored; env files, compose, CI and tests are kept out of the image build context.
- Appearance: light is now the default mode for new accounts and when nothing is saved (was auto). Existing accounts are
  not migrated: every account created before 0.2.2 has auto stored, so it keeps following the device until its user picks
  another mode in Appearance.
- Appearance: one default brand hue (`DEFAULT_HUE`, passman's blue); the chosen hue now tints backgrounds, surfaces and
  borders as well as the accent, in light and dark mode.
- Appearance: white text on accent-coloured surfaces (topbar, active folder, primary buttons, dialog headers, login screen) now meets
  WCAG AA (4.5:1) at every hue in light and dark mode; the topbar gets proper dark-mode colours instead of staying bright.
  The default blue is slightly darker in light mode as a result.
- Appearance: a theme change is no longer lost when the page is reloaded or closed right after it (the pending save is
  sent when the page is hidden).
- Custom fields: removing a field in the edit form no longer moves its revealed/focused state onto the next field.
- Web UI is linted with ESLint (`npm run lint`, run by CI) using `eslint-plugin-svelte`; findings fixed (keyed each-blocks
  and small cleanups).
- Appearance: link and icon colour (`--accent`) is fitted per hue to meet WCAG AA (4.5:1) on every background in light and
  dark mode (the default blue in light mode was 4.2:1 on white, now hsl(201 100% 34%), 4.8:1 or better); `check-theme.mjs`
  checks every hue 0-359.
- Image: base images pinned to their minor version (`node:22.23-alpine3.24`, `golang:1.26-alpine3.24`, `alpine:3.24`, was
  `alpine:3.22`); the bundled `psql` is now `postgresql18-client`, matching the Postgres 18 server in compose.
- `.env.example` leaves `POSTGRES_PASSWORD` empty and marks it required (was `changeme`); the README Quick start and
  `dockerdesc.md` say to create `.env` and set it before the first start.
- MIT `LICENSE` added.
- The version is built into the binary (`main.version`, Dockerfile `ARG VERSION`) and logged at startup
  (`passman 1.0.0 listening on ...`).
- Restore: drag and drop a backup file (`.json` or `.zip`) onto the Restore box, or click it to choose one as before.

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
