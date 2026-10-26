package main

import (
	"context"
	"crypto/ecdh"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value BYTEA NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id                  BIGSERIAL PRIMARY KEY,
    username            TEXT NOT NULL,
    is_admin            BOOLEAN NOT NULL DEFAULT false,
    auth_salt           BYTEA NOT NULL,
    auth_hash           BYTEA NOT NULL,
    kek_salt            BYTEA NOT NULL,
    wrapped_dek         BYTEA NOT NULL,
    escrow_dek          BYTEA NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    password_changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS users_username_idx ON users (lower(username));

CREATE TABLE IF NOT EXISTS sessions (
    token_hash  BYTEA PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wrapped_dek BYTEA NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions (user_id);

CREATE TABLE IF NOT EXISTS folders (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    id         UUID NOT NULL,
    parent_id  UUID,
    label      BYTEA NOT NULL,
    favorite   BOOLEAN NOT NULL DEFAULT false,
    revision   UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    FOREIGN KEY (user_id, parent_id) REFERENCES folders (user_id, id)
);

CREATE TABLE IF NOT EXISTS passwords (
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    id          UUID NOT NULL,
    folder_id   UUID,
    data        BYTEA NOT NULL,     -- AES-GCM(DEK, JSON record)
    fingerprint BYTEA NOT NULL,     -- HMAC(DEK-derived key, password), for duplicate detection
    risk        SMALLINT NOT NULL,  -- 0 ok, 1 contains username, 2 weak
    favorite    BOOLEAN NOT NULL DEFAULT false,
    revision    UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    FOREIGN KEY (user_id, folder_id) REFERENCES folders (user_id, id)
);
CREATE INDEX IF NOT EXISTS passwords_fingerprint_idx ON passwords (user_id, fingerprint);

ALTER TABLE users ADD COLUMN IF NOT EXISTS theme_mode TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE users ADD COLUMN IF NOT EXISTS theme_hue SMALLINT;
`

var ErrNotFound = errors.New("not found")

// querier is satisfied by both the pool and a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func migrate(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, schema)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func getMeta(ctx context.Context, q querier, key string) ([]byte, error) {
	var v []byte
	err := q.QueryRow(ctx, `SELECT value FROM meta WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

func setMeta(ctx context.Context, q querier, key string, value []byte) error {
	_, err := q.Exec(ctx, `INSERT INTO meta (key, value) VALUES ($1, $2)`, key, value)
	return err
}

func isSetUp(ctx context.Context, q querier) (bool, error) {
	_, err := getMeta(ctx, q, "escrow_pub")
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func escrowPublicKey(ctx context.Context, q querier) (*ecdh.PublicKey, error) {
	raw, err := getMeta(ctx, q, "escrow_pub")
	if err != nil {
		return nil, err
	}
	return ecdh.X25519().NewPublicKey(raw)
}

var ErrBadMasterKey = errors.New("invalid master key")

// unlockEscrow turns the master key into the escrow private key.
func unlockEscrow(ctx context.Context, q querier, masterKey string) (*ecdh.PrivateKey, error) {
	salt, err := getMeta(ctx, q, "master_salt")
	if err != nil {
		return nil, err
	}
	sealed, err := getMeta(ctx, q, "escrow_priv")
	if err != nil {
		return nil, err
	}
	raw, err := open(deriveKey(masterKey, salt), sealed, []byte("escrow_priv"))
	if err != nil {
		return nil, ErrBadMasterKey
	}
	return ecdh.X25519().NewPrivateKey(raw)
}

type User struct {
	ID         int64
	Username   string
	IsAdmin    bool
	AuthSalt   []byte
	AuthHash   []byte
	KEKSalt    []byte
	WrappedDEK []byte
	EscrowDEK  []byte
	Theme      Theme
}

func dekAAD(username string) []byte { return []byte("dek:" + normUsername(username)) }

// createUser builds the key material for a fresh account and inserts it.
func createUser(ctx context.Context, q querier, pub *ecdh.PublicKey, username, password string, isAdmin bool) (*User, []byte, error) {
	dek := randomBytes(keyLen)
	u := &User{Username: username, IsAdmin: isAdmin, Theme: Theme{Mode: themeAuto}}
	setUserPassword(u, dek, password)
	escrow, err := escrowSeal(pub, dek, dekAAD(username))
	if err != nil {
		return nil, nil, err
	}
	u.EscrowDEK = escrow
	err = q.QueryRow(ctx,
		`INSERT INTO users (username, is_admin, auth_salt, auth_hash, kek_salt, wrapped_dek, escrow_dek)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		u.Username, u.IsAdmin, u.AuthSalt, u.AuthHash, u.KEKSalt, u.WrappedDEK, u.EscrowDEK,
	).Scan(&u.ID)
	return u, dek, err
}

// setUserPassword (re)derives the login hash and re-wraps the DEK for a new password.
func setUserPassword(u *User, dek []byte, password string) {
	u.AuthSalt = randomBytes(saltLen)
	u.AuthHash = deriveKey(password, u.AuthSalt)
	u.KEKSalt = randomBytes(saltLen)
	u.WrappedDEK = seal(deriveKey(password, u.KEKSalt), dek, dekAAD(u.Username))
}

func savePassword(ctx context.Context, q querier, u *User) error {
	_, err := q.Exec(ctx,
		`UPDATE users SET auth_salt = $2, auth_hash = $3, kek_salt = $4, wrapped_dek = $5, password_changed_at = now()
		 WHERE id = $1`,
		u.ID, u.AuthSalt, u.AuthHash, u.KEKSalt, u.WrappedDEK)
	return err
}

const userCols = `id, username, is_admin, auth_salt, auth_hash, kek_salt, wrapped_dek, escrow_dek, theme_mode, theme_hue`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.AuthSalt, &u.AuthHash, &u.KEKSalt, &u.WrappedDEK, &u.EscrowDEK, &u.Theme.Mode, &u.Theme.Hue)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func userByName(ctx context.Context, q querier, username string) (*User, error) {
	return scanUser(q.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE lower(username) = lower($1)`, username))
}

func userByID(ctx context.Context, q querier, id int64) (*User, error) {
	return scanUser(q.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

// resetPasswordWithMasterKey recovers the user's DEK through escrow and
// re-wraps it under a new password. All of the user's sessions are revoked
// except keepToken (may be nil).
func resetPasswordWithMasterKey(ctx context.Context, db *pgxpool.Pool, masterKey, username, newPassword string, keepToken []byte) error {
	priv, err := unlockEscrow(ctx, db, masterKey)
	if err != nil {
		return err
	}
	u, err := userByName(ctx, db, username)
	if err != nil {
		return err
	}
	dek, err := escrowOpen(priv, u.EscrowDEK, dekAAD(u.Username))
	if err != nil {
		return fmt.Errorf("recover key for %s: %w", u.Username, err)
	}
	setUserPassword(u, dek, newPassword)
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := savePassword(ctx, tx, u); err != nil {
		return err
	}
	if err := revokeSessions(ctx, tx, u.ID, keepToken); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
