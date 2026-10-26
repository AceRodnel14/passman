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
| `BACKUP_ALL_ALLOWED_IPS` | (empty) | Comma-separated CIDRs or IPs (bare IP = one host) allowed to call `POST /api/backup-all`, checked against the direct TCP peer. Empty disables that endpoint. |

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

## Backup API

Both endpoints ignore cookies and authenticate with credentials in the JSON body (send `Content-Type: application/json`). They
are exempt from the same-origin check so the browser extension (`chrome-extension://` origin) and scripts can call them, and no
CORS headers are added.

`POST /api/export` with `{"username","password","backupPassword"?}` returns exactly what Backup in the web UI returns: the
plain JSON, or an AES-256 zip when `backupPassword` is set (at least 8 characters here; the web UI has no minimum). Wrong credentials give 401. Failed attempts are throttled per
username (shared with login) and per client IP.

`POST /api/backup-all` with `{"masterKey","backupPassword"}` (both required) returns one AES-256 zip with a `<username>.json`
backup per user, nested so that usernames and sizes stay hidden: the AES zip has a single entry `passman-all.zip`, which is a
plain zip holding the `<username>.json` files. `backupPassword` must be at least 16 characters (400 otherwise). It never
returns plain data. To unpack: `7z x -p'<backup password>' backup-all.zip` gives `passman-all.zip`, then `7z x passman-all.zip`
(or `unzip passman-all.zip`) gives one JSON per user, each restorable on its own. It is disabled (404 for everyone) unless `BACKUP_ALL_ALLOWED_IPS` is set.
It is for **direct connections only**: the client address is the TCP peer (`RemoteAddr`), forwarding headers are never trusted,
and a request carrying any of `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Real-Ip` or `Forwarded` is refused
with 404, so nothing that came through an ingress or reverse proxy can use it. Clients outside the list also get 404, so the
endpoint can't be discovered. A wrong master key gives 401; failures are throttled per client IP and globally (5 per
15 minutes). Because the global limit is shared, anyone inside the allowed range can block backups for 15 minutes with five
bad keys. At most one backup-all runs at a time, and logins plus exports share two concurrent slots (otherwise "Busy, try
again." with 429). Attempts are counted before the slow key derivation, so parallel guesses can't slip past the limits. Every attempt is logged (result and client IP, never keys).

The per-IP throttles on `/api/export` and `/api/backup-all` also use the TCP peer, so behind a proxy they see the proxy's address.

### Deployment notes for backup-all

- **Never allow a proxy.** Do not put an ingress, a TCP proxy (Traefik `IngressRouteTCP`) or nginx without `X-Forwarded-*` headers
  inside `BACKUP_ALL_ALLOWED_IPS`: it would make every client behind it look like an allowed address. Prefer single pod or
  host IPs, or a NetworkPolicy, over wide ranges.
- **Docker / WSL:** publish passman only on loopback (`-p 127.0.0.1:8080:8080`) and allow the Docker network, e.g.
  `BACKUP_ALL_ALLOWED_IPS=172.16.0.0/12`. With Docker Desktop, traffic to a published port reaches the container from the
  network gateway, not the original client, so the loopback binding (not the allow-list) is what keeps the LAN out.
- **Kubernetes:** allow the pod CIDR (k3s default `10.42.0.0/16`) and call the Service directly from a CronJob, never through
  the Ingress (Traefik adds `X-Forwarded-*`, so those requests are refused):
  ```bash
  curl -fsS -X POST http://passman.NAMESPACE.svc:8080/api/backup-all \
    -H 'Content-Type: application/json' \
    -d "{\"masterKey\":\"$MASTER_KEY\",\"backupPassword\":\"$BACKUP_PASSWORD\"}" -o backup-all.zip
  ```
  Keep `MASTER_KEY` in a Secret, and remember every pod in the allowed CIDR can try the endpoint (it is throttled), so use a
  NetworkPolicy to limit who can reach the Service.

## Security model

- Every user has a random data key. All folder names and password records are encrypted with it (AES-256-GCM) before they reach Postgres.
- The data key is wrapped with a key derived from the user's login password (Argon2id). A second copy is wrapped to an escrow public key (X25519). The matching private key is encrypted with the master key.
- So the database alone can't decrypt anything, and admins can't see other users' vaults through passman. Someone holding **both** a database dump **and** the master key could recover any vault. Guard the master key accordingly.
- **`POST /api/backup-all` changes that for the network.** Anyone who has the master key **and** can reach passman from an address in `BACKUP_ALL_ALLOWED_IPS` can download every user's vault over the network (as an encrypted zip whose password they choose). The endpoint is off unless that variable is set; keep the list as tight as possible (single hosts, not wide networks), and note that any workload inside the allowed network can try it. Leave it unset if you don't need unattended full backups.
- Sessions are stored in Postgres. Each session's copy of the data key is encrypted with a key that exists only in the user's cookie. Sessions survive restarts, and passman can run as several replicas.
- Login is throttled after 5 failed attempts per username (5 minutes). `/api/export` and the password check on **Move** share that throttle (`/api/export` adds a per-IP one), so repeated wrong passwords in the Move dialog also lock that account's login for 5 minutes.
- Folder changes require the login password: moving an entry to another folder, through the Move dialog (`POST /api/passwords/{id}/move`) or by changing the folder while editing (`PUT /api/passwords/{id}`), is checked on the server every time. Other edits and creating entries don't ask for it. Restoring a backup in overwrite mode can still change existing entries' folders without it.

## Account and vault API additions

- `GET /api/state` includes the user's appearance: `user.theme = {"mode": "auto"|"light"|"dark", "hue": 0..359 | null}`
  (`null` = the original blue; `auto` follows the device). Login and register responses carry the same `theme`.
- `PUT /api/account/theme` with `{"mode","hue"}` saves it for the account (follows the user across devices); 400 if invalid.
- `POST /api/passwords/{id}/move` with `{"folder","password"}` (`folder` `""` or the zero UUID = top level). Wrong password
  is **403** (not 401, so clients don't treat it as an expired session), unknown folder 400, unknown entry 404,
  throttled 429; success returns `{"folder"}`. Only the folder and revision change; the record is not re-encrypted.
- `PUT /api/passwords/{id}` accepts an optional `loginPassword` (separate from `password`, the entry's secret). It is
  required, with the same 400/403/429 responses as move, only when `folder` differs from the entry's current folder
  (`""` and the zero UUID both mean top level; UUID case is ignored). `POST /api/passwords` (create) never needs it.

## API for the browser extension

The extension keeps the list of saved sites on disk and matches pages locally, so passman only hears from it when the user acts.
All endpoints are read-only GETs and use the session cookie.

`GET /api/sites` returns
`{"user":"alice","sessionExpires":"<RFC3339>","sites":[{"id","label","schemes","host","port","subdomains"}]}`: one item per
saved entry with a usable URL (no username, password or notes). `sessionExpires` is the expiry of the calling session.
- `schemes` is `["https"]` for scheme-less entries, `["http","https"]` for scheme-less single-label hosts (`localhost`), or the
  explicit scheme.
- `host` is ASCII (punycode), lower-case, without a trailing dot.
- `port` is `""` for the default port of the page scheme, otherwise an exact port.
- `subdomains` is false for single-label hosts and public suffixes (`co.uk`, `github.io`, from a compiled-in list, no network).
- Entries whose URL is empty or unparsable, whose host is an IP address, or whose last label is all digits are unusable: they are
  never matched and never listed. Sites are identified by name only.

`GET /api/fill/{id}?url=<page origin>` returns `{"username","password"}` for that entry, but only if its stored URL matches the
given origin by the same rules. A mismatch and an unknown id both give the same 404, so it does not reveal whether an id exists.
An unparsable or non-http(s) `url` returns 400.

Client algorithm for a page URL (must be http or https with a host; otherwise no match). Take `scheme`, the host lower-cased,
without a trailing dot, converted to ASCII (the WHATWG `URL` does this), and the effective port (the explicit port, or 80/443
by scheme). A site matches when all of these hold:
1. `scheme` is in `site.schemes`;
2. the effective port equals `site.port`, or the default port of `scheme` when `site.port` is `""`;
3. the page host equals `site.host`, or `site.subdomains` is true, the page host is not an IP address and it ends with
   `"." + site.host`.

`/api/fill`'s origin check protects the extension's flow only: `GET /api/passwords/{id}` still returns any entry to the session cookie, as the web app needs.

The test cases in `match_test.go` define the expected behaviour; `/api/fill` uses the same matcher (`urlMatches`).

## Versioning

The version is in `GET /api/state` and in the web UI (login card and user menu). It defaults to the `version` variable in
`main.go` and can be set at build time (`-ldflags "-X main.version=1.2.3"`, or `docker build --build-arg VERSION=1.2.3`).
Releases are git tags `v0.2.0` created by the maintainer after the merge; see `CHANGELOG.md`.

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
