# passman

> **Note:** passman was written with AI assistance (Anthropic's Claude, via Claude Code), under the direction and review of its author. Review it yourself before trusting it with important secrets.

A small self-hosted password manager, replacing Nextcloud Passwords. Go backend, Svelte frontend, PostgreSQL storage, one container.

Highlights: nested folders, click-to-copy (single click copies the password, double click copies the username), a passphrase generator, password health shields, custom fields, and JSON backup/restore (optionally inside a password-protected zip).

passman never contacts the internet at runtime. Fonts, icons, the generator word list and the weak-password list are all built in. Only `docker build` downloads dependencies.

## Run

```bash
docker build -t passman:dev .
docker run -d --name passman -p 8080:8080 --restart unless-stopped \
  -e DATABASE_URL='postgres://USER:PASS@HOST:5432/passman' \
  passman:dev
```

Tables are created on startup. `DATABASE_URL` must point to an existing database.

| Env var | Default | |
|---|---|---|
| `DATABASE_URL` | (required) | Postgres connection string |
| `LISTEN_ADDR` | `:8080` | |
| `SESSION_MAX_AGE` | `14d` | Login session lifetime from sign-in (`14d`, `36h`, …). Saved passwords never expire. |
| `SECURE_COOKIE` | `false` | Set `true` when served over HTTPS |

## First run: admin and master key

1. Open passman in a browser. On an empty database it shows **first-time setup**.
2. Choose the password for the `admin` account.
3. passman then displays a 20-character **master key**, once.

> ⚠️ **Save the master key somewhere safe right away** (printed and locked away, or in a separate password manager). It's needed to reset any user's password. It isn't stored in readable form and can't be recovered.

Whoever opens a fresh instance first becomes admin, so complete setup right after deploying.

Anyone can then create an account from the **Register** tab on the login page.

## Resetting a forgotten password

Either way requires the master key. The user's saved passwords are kept, and all their sessions are signed out.

**From the admin UI:** user menu → *Manage users* → *Reset password*.

**From inside the container:**

```bash
docker exec -it passman reset-password <master key>
```

It then asks for the username and the new password.

Users can change their own password (they need the current one) from the user menu → *Change password*.

## Backups

The export is a JSON file in Nextcloud Passwords' *Database Backup* format, so a Nextcloud export can be restored into passman to migrate.

- **A plain `.json` backup contains every password in readable form.** passman decrypts your vault to produce it. Anyone who gets the file can read everything in it.
- Set a **backup password** to get a `.zip` encrypted with AES-256 instead. passman's Restore can open it, and so can 7-Zip. Windows Explorer can't open AES zips.
- passman adds an optional `icon` field per password for custom letters, colours and logos. It's left out when unset. On restore it's optional, so Nextcloud exports without it import normally. An invalid icon is skipped and noted in the import log; the password itself is still imported.

## Security model

- Every user has a random data key. All folder names and password records are encrypted with it (AES-256-GCM) before they reach Postgres.
- The data key is wrapped with a key derived from the user's login password (Argon2id). A second copy is wrapped to an escrow public key (X25519). The matching private key is encrypted with the master key.
- So the database alone can't decrypt anything, and admins can't see other users' vaults through passman. Someone holding **both** a database dump **and** the master key could recover any vault. Guard the master key accordingly.
- Sessions are stored in Postgres. Each session's copy of the data key is encrypted with a key that exists only in the user's cookie. Sessions survive restarts, and passman can run as several replicas.
- Login is throttled after 5 failed attempts per username (5 minutes).

## Debugging

`psql` is included in the image:

```bash
docker exec -it passman psql "$DATABASE_URL"
```

## Development

The Go binary embeds the built frontend (`web/dist`), so build the frontend before running Go tests:

```bash
# frontend: build once, or run the dev server (proxies /api to localhost:8080)
docker run --rm -v "$PWD/web":/app -w /app node:22-alpine sh -c 'npm ci && npm run build'
cd web && npm install && npm run dev

# backend tests
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test ./...
```
