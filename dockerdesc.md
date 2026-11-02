> Source code, full documentation and issues: https://github.com/AceRodnel14/passman

# passman

A small self-hosted password manager: Go backend, Svelte UI, PostgreSQL storage, one container.
Nested folders, click-to-copy, a passphrase generator, password health checks, custom fields,
per-user encryption with a master key for resets, and JSON / encrypted-zip backup and restore.
It never contacts the internet at runtime.

## Quick start
```yaml
services:
  app:
    image: eacer/passman:latest
    restart: unless-stopped
    ports: ["8080:8080"]
    env_file: .env
    environment:
      DATABASE_URL: postgres://passman:${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}@db:5432/passman?sslmode=disable
    depends_on: [db]
  db:
    image: postgres:18.6-alpine3.24
    restart: unless-stopped
    environment:
      POSTGRES_USER: passman
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
      POSTGRES_DB: passman
    volumes: ["pg-data:/var/lib/postgresql"]
volumes:
  pg-data:
```
**Before the first start**, create `.env` and set `POSTGRES_PASSWORD` (required; letters and digits);
then `docker compose up -d`, then open
`http://<host>:8080`. The first visitor sets the admin password and gets the **master key once**:
store it safely. Reset a password with `docker compose exec app reset-password <master key>`.
For HTTPS, put a reverse proxy in front and set `SECURE_COOKIE=true`.

## Configuration
| Variable | Default | Description |
| :- | :- | :- |
| `DATABASE_URL` | (required) | Postgres connection string |
| `LISTEN_ADDR` | `:8080` | Listen address inside the container |
| `SESSION_MAX_AGE` | `14d` | Login session lifetime (`14d`, `36h`, …) |
| `SECURE_COOKIE` | `false` | Set `true` when served over HTTPS |
| `BACKUP_ALL_ALLOWED_IPS` | (empty) | CIDRs/IPs allowed to call `POST /api/backup-all`; empty disables it |

Port `8080`. Data lives in PostgreSQL (`pg-data` volume); passman has no `/data` volume.
Health: `/healthz` (liveness), `/readyz` (readiness: database reachable).

## Tags
`latest` (default, latest stable), `main` (default branch), `vX.Y`, `vX.Y.Z` (releases). Platforms: `linux/amd64`, `linux/arm64`.
