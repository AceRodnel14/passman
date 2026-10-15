package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const rootFolderID = "00000000-0000-0000-0000-000000000000"

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type CustomField struct {
	Label string `json:"label"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Record is everything about a password entry that gets encrypted as one blob.
type Record struct {
	Label        string        `json:"label"`
	Username     string        `json:"username"`
	Password     string        `json:"password"`
	URL          string        `json:"url"`
	Notes        string        `json:"notes"`
	CustomFields []CustomField `json:"customFields"`
	Icon         *Icon         `json:"icon,omitempty"`
}

func (rec *Record) normalize() error {
	rec.Label = strings.TrimSpace(rec.Label)
	rec.Username = strings.TrimSpace(rec.Username)
	rec.URL = strings.TrimSpace(rec.URL)
	if rec.Label == "" {
		return errors.New("Name is required.")
	}
	if rec.Password == "" {
		return errors.New("Password is required.")
	}
	icon, err := normalizeIcon(rec.Icon)
	if err != nil {
		return err
	}
	rec.Icon = icon
	fields := rec.CustomFields[:0]
	for _, f := range rec.CustomFields {
		f.Label = strings.TrimSpace(f.Label)
		if f.Label == "" && f.Value == "" {
			continue
		}
		if f.Label == "" {
			return errors.New("Every custom field needs a name.")
		}
		if f.Type != "secret" {
			f.Type = "text"
		}
		fields = append(fields, f)
	}
	rec.CustomFields = fields
	if rec.CustomFields == nil {
		rec.CustomFields = []CustomField{}
	}
	return nil
}

// vault scopes crypto and queries to one user.
type vault struct {
	userID int64
	dek    []byte
}

func (s *Session) vault() vault { return vault{userID: s.UserID, dek: s.DEK} }

func (v vault) aad(kind, id string) []byte {
	return []byte(fmt.Sprintf("%s:%d:%s", kind, v.userID, strings.ToLower(id)))
}

func (v vault) sealRecord(id string, rec *Record) []byte {
	b, _ := json.Marshal(rec)
	return seal(v.dek, b, v.aad("pw", id))
}

func (v vault) openRecord(id string, data []byte) (*Record, error) {
	b, err := open(v.dek, data, v.aad("pw", id))
	if err != nil {
		return nil, fmt.Errorf("decrypt password %s: %w", id, err)
	}
	var rec Record
	return &rec, json.Unmarshal(b, &rec)
}

func (v vault) sealLabel(id, label string) []byte {
	return seal(v.dek, []byte(label), v.aad("folder", id))
}

func (v vault) openLabel(id string, data []byte) (string, error) {
	b, err := open(v.dek, data, v.aad("folder", id))
	if err != nil {
		return "", fmt.Errorf("decrypt folder %s: %w", id, err)
	}
	return string(b), nil
}

// nullFolder maps the API/backup representation of root to SQL NULL.
func nullFolder(id string) *string {
	if id == "" || id == rootFolderID {
		return nil
	}
	return &id
}

func folderExists(ctx context.Context, q querier, userID int64, id string) (bool, error) {
	if nullFolder(id) == nil {
		return true, nil
	}
	if !uuidRe.MatchString(id) {
		return false, nil
	}
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM folders WHERE user_id = $1 AND id = $2::uuid)`, userID, id).Scan(&ok)
	return ok, err
}

// --- response types ---

type folderView struct {
	ID       string    `json:"id"`
	Parent   string    `json:"parent"`
	Label    string    `json:"label"`
	Favorite bool      `json:"favorite"`
	Updated  time.Time `json:"updated"`
}

type passwordRow struct {
	ID          string    `json:"id"`
	Folder      string    `json:"folder"`
	Label       string    `json:"label"`
	Health      string    `json:"health"`
	Favorite    bool      `json:"favorite"`
	HasUsername bool      `json:"hasUsername"`
	Icon        *Icon     `json:"icon"`
	Updated     time.Time `json:"updated"`
}

type passwordDetail struct {
	passwordRow
	Username     string        `json:"username"`
	Password     string        `json:"password"`
	URL          string        `json:"url"`
	Notes        string        `json:"notes"`
	CustomFields []CustomField `json:"customFields"`
}

func healthColor(risk int16, shared int64) string {
	switch {
	case risk == riskWeak:
		return "red"
	case risk == riskUsername:
		return "orange"
	case shared > 1:
		return "yellow"
	}
	return "green"
}

const passwordSelect = `
	SELECT p.id::text, coalesce(p.folder_id::text, ''), p.data, p.risk, p.favorite, p.updated_at,
	       (SELECT count(*) FROM passwords q WHERE q.user_id = p.user_id AND q.fingerprint = p.fingerprint)
	FROM passwords p WHERE p.user_id = $1`

func (v vault) scanPassword(row pgx.Row) (*passwordDetail, error) {
	var (
		d      passwordDetail
		data   []byte
		risk   int16
		shared int64
	)
	if err := row.Scan(&d.ID, &d.Folder, &data, &risk, &d.Favorite, &d.Updated, &shared); err != nil {
		return nil, err
	}
	rec, err := v.openRecord(d.ID, data)
	if err != nil {
		return nil, err
	}
	d.Label, d.Username, d.Password, d.URL, d.Notes = rec.Label, rec.Username, rec.Password, rec.URL, rec.Notes
	d.CustomFields = rec.CustomFields
	if d.CustomFields == nil {
		d.CustomFields = []CustomField{}
	}
	d.HasUsername = rec.Username != ""
	d.Icon = rec.Icon
	d.Health = healthColor(risk, shared)
	return &d, nil
}

// --- handlers ---

// handleVault returns every folder and password row (no secrets) so the
// client can navigate folders instantly without further round trips.
func (a *App) handleVault(w http.ResponseWriter, r *http.Request, s *Session) {
	v := s.vault()
	ctx := r.Context()
	folders := []folderView{}
	rows, err := a.db.Query(ctx,
		`SELECT id::text, coalesce(parent_id::text, ''), label, favorite, updated_at FROM folders WHERE user_id = $1`, v.userID)
	if err != nil {
		serverError(w, err)
		return
	}
	for rows.Next() {
		var f folderView
		var label []byte
		if err := rows.Scan(&f.ID, &f.Parent, &label, &f.Favorite, &f.Updated); err != nil {
			serverError(w, err)
			return
		}
		if f.Label, err = v.openLabel(f.ID, label); err != nil {
			serverError(w, err)
			return
		}
		folders = append(folders, f)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}

	passwords := []passwordRow{}
	rows, err = a.db.Query(ctx, passwordSelect, v.userID)
	if err != nil {
		serverError(w, err)
		return
	}
	for rows.Next() {
		d, err := v.scanPassword(rows)
		if err != nil {
			serverError(w, err)
			return
		}
		passwords = append(passwords, d.passwordRow)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": folders, "passwords": passwords})
}

func (a *App) handleGetPassword(w http.ResponseWriter, r *http.Request, s *Session) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	v := s.vault()
	d, err := v.scanPassword(a.db.QueryRow(r.Context(), passwordSelect+` AND p.id = $2::uuid`, v.userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type passwordInput struct {
	Record
	Folder string `json:"folder"`
}

func (a *App) handleSavePassword(w http.ResponseWriter, r *http.Request, s *Session) {
	var in passwordInput
	if !readJSON(w, r, &in) {
		return
	}
	if err := in.Record.normalize(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	v := s.vault()
	if ok, err := folderExists(ctx, a.db, v.userID, in.Folder); err != nil {
		serverError(w, err)
		return
	} else if !ok {
		writeError(w, http.StatusBadRequest, "That folder doesn't exist.")
		return
	}

	id := r.PathValue("id")
	creating := id == ""
	if creating {
		id = newUUID()
	} else if !uuidRe.MatchString(id) {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	data := v.sealRecord(id, &in.Record)
	fp := fingerprint(v.dek, in.Password)
	risk := passwordRisk(in.Password, in.Username)

	if creating {
		_, err := a.db.Exec(ctx,
			`INSERT INTO passwords (user_id, id, folder_id, data, fingerprint, risk, revision)
			 VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6, $7::uuid)`,
			v.userID, id, nullFolder(in.Folder), data, fp, risk, newUUID())
		if err != nil {
			serverError(w, err)
			return
		}
	} else {
		tag, err := a.db.Exec(ctx,
			`UPDATE passwords SET folder_id = $3::uuid, data = $4, fingerprint = $5, risk = $6, revision = $7::uuid, updated_at = now()
			 WHERE user_id = $1 AND id = $2::uuid`,
			v.userID, id, nullFolder(in.Folder), data, fp, risk, newUUID())
		if err != nil {
			serverError(w, err)
			return
		}
		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusNotFound, "Not found.")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (a *App) handleDeletePassword(w http.ResponseWriter, r *http.Request, s *Session) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM passwords WHERE user_id = $1 AND id = $2::uuid`, s.UserID, id); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleFavorite(table string) authedHandler {
	return func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req struct{ Favorite bool }
		if !readJSON(w, r, &req) {
			return
		}
		id := r.PathValue("id")
		if !uuidRe.MatchString(id) {
			writeError(w, http.StatusNotFound, "Not found.")
			return
		}
		// table is one of two constants chosen in routes(), never user input.
		if _, err := a.db.Exec(r.Context(),
			`UPDATE `+table+` SET favorite = $3 WHERE user_id = $1 AND id = $2::uuid`, s.UserID, id, req.Favorite); err != nil {
			serverError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *App) handleCreateFolder(w http.ResponseWriter, r *http.Request, s *Session) {
	var req struct{ Label, Parent string }
	if !readJSON(w, r, &req) {
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	if req.Label == "" {
		writeError(w, http.StatusBadRequest, "Folder name is required.")
		return
	}
	v := s.vault()
	if ok, err := folderExists(r.Context(), a.db, v.userID, req.Parent); err != nil {
		serverError(w, err)
		return
	} else if !ok {
		writeError(w, http.StatusBadRequest, "Parent folder doesn't exist.")
		return
	}
	id := newUUID()
	if _, err := a.db.Exec(r.Context(),
		`INSERT INTO folders (user_id, id, parent_id, label, revision) VALUES ($1, $2::uuid, $3::uuid, $4, $5::uuid)`,
		v.userID, id, nullFolder(req.Parent), v.sealLabel(id, req.Label), newUUID()); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (a *App) handleRenameFolder(w http.ResponseWriter, r *http.Request, s *Session) {
	var req struct{ Label string }
	if !readJSON(w, r, &req) {
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	id := r.PathValue("id")
	if req.Label == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, "Folder name is required.")
		return
	}
	v := s.vault()
	tag, err := a.db.Exec(r.Context(),
		`UPDATE folders SET label = $3, revision = $4::uuid, updated_at = now() WHERE user_id = $1 AND id = $2::uuid`,
		v.userID, id, v.sealLabel(id, req.Label), newUUID())
	if err != nil {
		serverError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDeleteFolder(w http.ResponseWriter, r *http.Request, s *Session) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	var children int
	err := a.db.QueryRow(r.Context(),
		`SELECT (SELECT count(*) FROM folders WHERE user_id = $1 AND parent_id = $2::uuid)
		      + (SELECT count(*) FROM passwords WHERE user_id = $1 AND folder_id = $2::uuid)`, s.UserID, id).Scan(&children)
	if err != nil {
		serverError(w, err)
		return
	}
	if children > 0 {
		writeError(w, http.StatusConflict, "Only empty folders can be deleted.")
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM folders WHERE user_id = $1 AND id = $2::uuid`, s.UserID, id); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleGenerate(w http.ResponseWriter, r *http.Request, s *Session) {
	q := r.URL.Query()
	length, err := strconv.Atoi(strings.TrimSpace(q.Get("length")))
	if err != nil || length < minGenLength || length > maxGenLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Number of characters must be a whole number from %d to %d.", minGenLength, maxGenLength))
		return
	}
	pw := generatePassphrase(length, q.Get("numbers") == "true", q.Get("symbols") == "true")
	writeJSON(w, http.StatusOK, map[string]any{"password": pw})
}
