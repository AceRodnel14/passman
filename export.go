package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// ipKey is the limiter key for a client; an unparsable peer shares one bucket.
func ipKey(ip netip.Addr) string {
	if !ip.IsValid() {
		return "unknown"
	}
	return ip.String()
}

// handleExport returns one user's backup, authenticated with the login
// password in the body rather than the session cookie (cookies are ignored).
// It is exempt from sameOrigin for that reason; see originExempt.
func (a *App) handleExport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username       string `json:"username"`
		Password       string `json:"password"`
		BackupPassword string `json:"backupPassword"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	key := normUsername(req.Username)
	if key == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Username and password are required.")
		return
	}
	if req.BackupPassword != "" && utf8.RuneCountInString(req.BackupPassword) < minExportBackupPassword {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("The backup password must be at least %d characters.", minExportBackupPassword))
		return
	}
	ip := ipKey(peerIP(r))
	if !a.kdf.tryAcquire() {
		writeError(w, http.StatusTooManyRequests, "Busy, try again.")
		return
	}
	defer a.kdf.release()
	// Attempts are recorded before the slow key derivation (see handleLogin).
	if !a.limiter.take(key) {
		writeError(w, http.StatusTooManyRequests, "Too many failed attempts. Try again in a few minutes.")
		return
	}
	if !a.exportIP.take(ip) {
		a.limiter.release(key)
		writeError(w, http.StatusTooManyRequests, "Too many failed attempts. Try again in a few minutes.")
		return
	}
	u, dek, err := a.checkLogin(r.Context(), key, req.Password)
	if errors.Is(err, errBadLogin) {
		writeError(w, http.StatusUnauthorized, "Wrong username or password.")
		return
	} else if err != nil {
		a.limiter.release(key)
		a.exportIP.release(ip)
		serverError(w, err)
		return
	}
	a.limiter.reset(key)
	a.exportIP.release(ip)
	a.sendBackup(w, r.Context(), vault{userID: u.ID, dek: dek}, req.BackupPassword)
}

const (
	minExportBackupPassword = 8  // the web UI has no minimum; the API asks for a sane one
	minAllBackupPassword    = 16 // one password protects every vault
)

// plainZip packs files into an unencrypted zip, in name order.
func plainZip(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		fw, err := zw.Create(n)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(files[n]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) backupAllAllowed(ip netip.Addr) bool {
	return ip.IsValid() && inPrefixes(ip, a.cfg.BackupAllIPs)
}

// handleBackupAll returns every user's backup in one AES-256 zip, unlocked
// with the master key. It only serves direct connections: a request that
// carries forwarding headers (i.e. came through an ingress/proxy) is refused. It answers 404 when disabled or when the client IP is
// not allowed, so it can't be discovered. It ignores cookies; see originExempt.
func (a *App) handleBackupAll(w http.ResponseWriter, r *http.Request) {
	notFound := func() { writeError(w, http.StatusNotFound, "Not found.") }
	if len(a.cfg.BackupAllIPs) == 0 {
		notFound()
		return
	}
	ip := peerIP(r)
	if viaProxy(r) {
		log.Printf("backup-all: denied: via proxy ip=%s", ipKey(ip))
		notFound()
		return
	}
	if !a.backupAllAllowed(ip) || r.Method != http.MethodPost {
		log.Printf("backup-all: denied ip=%s method=%s", ipKey(ip), r.Method)
		notFound()
		return
	}
	attempt := func(result string, users int) {
		log.Printf("backup-all: %s ip=%s users=%d", result, ipKey(ip), users)
	}
	var req struct {
		MasterKey      string `json:"masterKey"`
		BackupPassword string `json:"backupPassword"`
	}
	if !readJSON(w, r, &req) {
		attempt("bad-request", 0)
		return
	}
	req.MasterKey = strings.TrimSpace(req.MasterKey)
	if req.MasterKey == "" || utf8.RuneCountInString(req.BackupPassword) < minAllBackupPassword {
		attempt("bad-request", 0)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("masterKey is required and backupPassword must be at least %d characters.", minAllBackupPassword))
		return
	}
	k := ipKey(ip)
	if !a.kdfAll.tryAcquire() {
		attempt("busy", 0)
		writeError(w, http.StatusTooManyRequests, "Busy, try again.")
		return
	}
	defer a.kdfAll.release()
	// Record the attempt before unlocking so concurrent guesses are counted;
	// it is released again on success.
	if !a.masterIP.take(k) {
		attempt("throttled", 0)
		writeError(w, http.StatusTooManyRequests, "Too many failed attempts. Try again later.")
		return
	}
	if !a.masterAll.take("*") {
		a.masterIP.release(k)
		attempt("throttled", 0)
		writeError(w, http.StatusTooManyRequests, "Too many failed attempts. Try again later.")
		return
	}
	zipped, n, err := a.backupAll(r.Context(), req.MasterKey, req.BackupPassword)
	if errors.Is(err, ErrBadMasterKey) {
		attempt("bad-key", 0)
		writeError(w, http.StatusUnauthorized, "That master key is not correct.")
		return
	} else if err != nil {
		a.masterIP.release(k)
		a.masterAll.release("*")
		attempt("error", 0)
		serverError(w, err)
		return
	}
	a.masterIP.release(k)
	a.masterAll.release("*")
	attempt("ok", n)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="passwords-backup-all_`+time.Now().Format("2006-01-02")+`.zip"`)
	w.Write(zipped)
}

// backupAll recovers every user's data key through escrow and zips one backup
// per user as <username>.json.
func (a *App) backupAll(ctx context.Context, masterKey, backupPassword string) ([]byte, int, error) {
	priv, err := unlockEscrow(ctx, a.db, masterKey)
	if err != nil {
		return nil, 0, err
	}
	rows, err := a.db.Query(ctx, `SELECT `+userCols+` FROM users ORDER BY id`)
	if err != nil {
		return nil, 0, err
	}
	var users []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		users = append(users, u)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}
	files := map[string][]byte{}
	for _, u := range users {
		dek, err := escrowOpen(priv, u.EscrowDEK, dekAAD(u.Username))
		if err != nil {
			return nil, 0, errors.New("recover key for " + u.Username + ": " + err.Error())
		}
		body, err := a.buildBackup(ctx, vault{userID: u.ID, dek: dek})
		if err != nil {
			return nil, 0, err
		}
		files[u.Username+".json"] = body
	}
	// Per-user files go into an inner plain zip that is the only entry of the
	// AES zip, so usernames and per-user sizes are hidden without the password.
	// (Not streamed: the inner zip is built in memory first anyway, and errors
	// must still be able to produce a proper status code before any output.)
	inner, err := plainZip(files)
	if err != nil {
		return nil, 0, err
	}
	zipped, err := zipBackups(backupPassword, map[string][]byte{"passman-all.zip": inner})
	return zipped, len(users), err
}
