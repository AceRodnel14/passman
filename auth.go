package main

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

const sessionCookie = "passman_session"

type Session struct {
	UserID   int64
	Username string
	IsAdmin  bool
	Theme    Theme
	DEK      []byte
	Expires  time.Time
	token    []byte
}

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

const minPasswordLen = 8

func normUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validateNewPassword(password, confirm string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("Password must be at least %d characters.", minPasswordLen)
	}
	if password != confirm {
		return errors.New("Passwords do not match.")
	}
	return nil
}

func sessionAAD(userID int64) []byte { return []byte(fmt.Sprintf("session:%d", userID)) }

// Sessions are stored in Postgres so they survive restarts and work across
// replicas. The DEK is wrapped with a key derived from the cookie token, so a
// database row alone can't be used to decrypt a vault.
func (a *App) createSession(ctx context.Context, w http.ResponseWriter, userID int64, dek []byte) error {
	token := randomBytes(32)
	hash := sha256.Sum256(token)
	wrapped := seal(subKey(token, "passman session"), dek, sessionAAD(userID))
	expires := time.Now().Add(a.cfg.SessionMaxAge)
	if _, err := a.db.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, wrapped_dek, expires_at) VALUES ($1, $2, $3, $4)`,
		hash[:], userID, wrapped, expires); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    base64.RawURLEncoding.EncodeToString(token),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   a.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (a *App) currentSession(r *http.Request) (*Session, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, nil
	}
	token, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(token) != 32 {
		return nil, nil
	}
	hash := sha256.Sum256(token)
	s := &Session{token: token}
	var wrapped []byte
	err = a.db.QueryRow(r.Context(),
		`SELECT s.user_id, s.wrapped_dek, u.username, u.is_admin, u.theme_mode, u.theme_hue, s.expires_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > now()`, hash[:],
	).Scan(&s.UserID, &wrapped, &s.Username, &s.IsAdmin, &s.Theme.Mode, &s.Theme.Hue, &s.Expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if s.DEK, err = open(subKey(token, "passman session"), wrapped, sessionAAD(s.UserID)); err != nil {
		return nil, nil
	}
	return s, nil
}

func revokeSessions(ctx context.Context, q querier, userID int64, keepToken []byte) error {
	keep := []byte{}
	if keepToken != nil {
		h := sha256.Sum256(keepToken)
		keep = h[:]
	}
	_, err := q.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2`, userID, keep)
	return err
}

func (a *App) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookie, SameSite: http.SameSiteLaxMode})
}

func (a *App) reapSessions() {
	for range time.Tick(time.Hour) {
		if _, err := a.db.Exec(context.Background(), `DELETE FROM sessions WHERE expires_at <= now()`); err != nil {
			log.Printf("reap sessions: %v", err)
		}
	}
}

// --- login throttling ---

// sem is a non-blocking counting semaphore bounding concurrent KDF-heavy work
// (Argon2 uses a lot of memory).
type sem chan struct{}

func (s sem) tryAcquire() bool {
	select {
	case s <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s sem) release() { <-s }

type limiter struct {
	mu     sync.Mutex
	fails  map[string][]time.Time
	max    int
	window time.Duration
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{fails: map[string][]time.Time{}, max: max, window: window}
}

const (
	maxFails   = 5
	failWindow = 5 * time.Minute
)

func (l *limiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(key)) >= l.max
}

// take records an attempt unless the key is already at its limit, as one
// atomic step, so concurrent requests see each other before any slow work.
// It reports whether the attempt may proceed.
func (l *limiter) take(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cur := l.prune(key)
	if len(cur) >= l.max {
		return false
	}
	l.fails[key] = append(cur, time.Now())
	return true
}

// release undoes the most recent take (used for successful attempts).
func (l *limiter) release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if cur := l.prune(key); len(cur) > 0 {
		cur = cur[:len(cur)-1]
		if len(cur) == 0 {
			delete(l.fails, key)
		} else {
			l.fails[key] = cur
		}
	}
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key] = append(l.prune(key), time.Now())
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

func (l *limiter) prune(key string) []time.Time {
	keep := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if time.Since(t) < l.window {
			keep = append(keep, t)
		}
	}
	if len(keep) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = keep
	return keep
}

// --- handlers ---

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	setUp, err := isSetUp(r.Context(), a.db)
	if err != nil {
		serverError(w, err)
		return
	}
	resp := map[string]any{"setupRequired": !setUp, "version": version}
	if s, err := a.currentSession(r); err != nil {
		serverError(w, err)
		return
	} else if s != nil {
		resp["user"] = map[string]any{"username": s.Username, "isAdmin": s.IsAdmin, "theme": s.Theme}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleSetup runs once on an empty database: it creates the master key,
// the escrow keypair and the "admin" account.
func (a *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct{ Password, Confirm string }
	if !readJSON(w, r, &req) {
		return
	}
	if err := validateNewPassword(req.Password, req.Confirm); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7243001)`); err != nil {
		serverError(w, err)
		return
	}
	if done, err := isSetUp(ctx, tx); err != nil {
		serverError(w, err)
		return
	} else if done {
		writeError(w, http.StatusConflict, "passman is already set up.")
		return
	}

	masterKey := randomString(20, masterAlphabet)
	masterSalt := randomBytes(saltLen)
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		serverError(w, err)
		return
	}
	for key, val := range map[string][]byte{
		"master_salt": masterSalt,
		"escrow_priv": seal(deriveKey(masterKey, masterSalt), priv.Bytes(), []byte("escrow_priv")),
		"escrow_pub":  priv.PublicKey().Bytes(),
	} {
		if err := setMeta(ctx, tx, key, val); err != nil {
			serverError(w, err)
			return
		}
	}
	u, dek, err := createUser(ctx, tx, priv.PublicKey(), "admin", req.Password, true)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverError(w, err)
		return
	}
	if err := a.createSession(ctx, w, u.ID, dek); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"masterKey": masterKey})
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Password, Confirm string }
	if !readJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !usernameRe.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "Username must be 3–32 characters: letters, numbers, dot, dash or underscore.")
		return
	}
	if err := validateNewPassword(req.Password, req.Confirm); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pub, err := escrowPublicKey(r.Context(), a.db)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusConflict, "passman has not been set up yet.")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	u, dek, err := createUser(r.Context(), a.db, pub, req.Username, req.Password, false)
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "That username is already taken.")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	if err := a.createSession(r.Context(), w, u.ID, dek); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"username": u.Username, "isAdmin": false, "theme": u.Theme})
}

var errBadLogin = errors.New("wrong username or password")

// checkLogin verifies a username and password and returns the user and their
// unwrapped data key. Unknown users and wrong passwords give errBadLogin.
func (a *App) checkLogin(ctx context.Context, username, password string) (*User, []byte, error) {
	u, err := userByName(ctx, a.db, username)
	if errors.Is(err, ErrNotFound) {
		deriveKey(password, make([]byte, saltLen)) // similar timing for unknown users
		return nil, nil, errBadLogin
	} else if err != nil {
		return nil, nil, err
	}
	if !checkPassword(password, u.AuthSalt, u.AuthHash) {
		return nil, nil, errBadLogin
	}
	dek, err := open(deriveKey(password, u.KEKSalt), u.WrappedDEK, dekAAD(u.Username))
	if err != nil {
		return nil, nil, fmt.Errorf("unwrap key for %s: %w", u.Username, err)
	}
	return u, dek, nil
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Password string }
	if !readJSON(w, r, &req) {
		return
	}
	key := normUsername(req.Username)
	if !a.kdf.tryAcquire() {
		writeError(w, http.StatusTooManyRequests, "Busy, try again.")
		return
	}
	defer a.kdf.release()
	// The attempt is recorded before the slow key derivation so that
	// concurrent guesses count against the limit; success clears it.
	if !a.limiter.take(key) {
		writeError(w, http.StatusTooManyRequests, "Too many failed attempts. Try again in a few minutes.")
		return
	}
	u, dek, err := a.checkLogin(r.Context(), key, req.Password)
	if errors.Is(err, errBadLogin) {
		writeError(w, http.StatusUnauthorized, "Wrong username or password.")
		return
	} else if err != nil {
		a.limiter.release(key)
		serverError(w, err)
		return
	}
	a.limiter.reset(key)
	if err := a.createSession(r.Context(), w, u.ID, dek); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"username": u.Username, "isAdmin": u.IsAdmin, "theme": u.Theme})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if token, err := base64.RawURLEncoding.DecodeString(c.Value); err == nil {
			hash := sha256.Sum256(token)
			if _, err := a.db.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, hash[:]); err != nil {
				log.Printf("logout: delete session: %v", err)
			}
		}
	}
	a.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request, s *Session) {
	var req struct{ Current, Password, Confirm string }
	if !readJSON(w, r, &req) {
		return
	}
	u, err := userByID(r.Context(), a.db, s.UserID)
	if err != nil {
		serverError(w, err)
		return
	}
	if !checkPassword(req.Current, u.AuthSalt, u.AuthHash) {
		writeError(w, http.StatusBadRequest, "Current password is wrong.")
		return
	}
	if err := validateNewPassword(req.Password, req.Confirm); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	setUserPassword(u, s.DEK, req.Password)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err := savePassword(r.Context(), tx, u); err != nil {
		serverError(w, err)
		return
	}
	if err := revokeSessions(r.Context(), tx, u.ID, s.token); err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
