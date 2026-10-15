package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/yeka/zip"
)

const backupVersion = 3

type backupFile struct {
	Version   *int             `json:"version"`
	Encrypted *bool            `json:"encrypted"`
	Passwords []backupPassword `json:"passwords"`
	Folders   []backupFolder   `json:"folders"`
	Tags      []any            `json:"tags"`
}

type backupPassword struct {
	ID           string        `json:"id"`
	Revision     string        `json:"revision"`
	Label        string        `json:"label"`
	Username     string        `json:"username"`
	Password     string        `json:"password"`
	Notes        string        `json:"notes"`
	URL          string        `json:"url"`
	CustomFields []CustomField `json:"customFields"`
	Folder       string        `json:"folder"`
	Edited       int64         `json:"edited"`
	Favorite     bool          `json:"favorite"`
	Tags         []any         `json:"tags"`
	Icon         *Icon         `json:"icon,omitempty"` // passman extension; absent in Nextcloud exports
}

type backupFolder struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Label    string `json:"label"`
	Parent   string `json:"parent"`
	Edited   int64  `json:"edited"`
	Favorite bool   `json:"favorite"`
}

func rootIfEmpty(id string) string {
	if id == "" {
		return rootFolderID
	}
	return id
}

// --- export ---

func (a *App) handleBackup(w http.ResponseWriter, r *http.Request, s *Session) {
	var req struct{ Password string }
	if !readJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	v := s.vault()
	version, encrypted := backupVersion, false
	out := backupFile{Version: &version, Encrypted: &encrypted, Passwords: []backupPassword{}, Folders: []backupFolder{}, Tags: []any{}}

	rows, err := a.db.Query(ctx,
		`SELECT id::text, revision::text, coalesce(parent_id::text, ''), label, favorite, updated_at FROM folders WHERE user_id = $1 ORDER BY created_at`, v.userID)
	if err != nil {
		serverError(w, err)
		return
	}
	for rows.Next() {
		var f backupFolder
		var label []byte
		var updated time.Time
		if err := rows.Scan(&f.ID, &f.Revision, &f.Parent, &label, &f.Favorite, &updated); err != nil {
			serverError(w, err)
			return
		}
		if f.Label, err = v.openLabel(f.ID, label); err != nil {
			serverError(w, err)
			return
		}
		f.Parent, f.Edited = rootIfEmpty(f.Parent), updated.Unix()
		out.Folders = append(out.Folders, f)
	}
	if rows.Err() != nil {
		serverError(w, rows.Err())
		return
	}

	rows, err = a.db.Query(ctx,
		`SELECT id::text, revision::text, coalesce(folder_id::text, ''), data, favorite, updated_at FROM passwords WHERE user_id = $1 ORDER BY created_at`, v.userID)
	if err != nil {
		serverError(w, err)
		return
	}
	for rows.Next() {
		var p backupPassword
		var data []byte
		var updated time.Time
		if err := rows.Scan(&p.ID, &p.Revision, &p.Folder, &data, &p.Favorite, &updated); err != nil {
			serverError(w, err)
			return
		}
		rec, err := v.openRecord(p.ID, data)
		if err != nil {
			serverError(w, err)
			return
		}
		p.Label, p.Username, p.Password, p.Notes, p.URL = rec.Label, rec.Username, rec.Password, rec.Notes, rec.URL
		p.CustomFields, p.Icon = rec.CustomFields, rec.Icon
		if p.CustomFields == nil {
			p.CustomFields = []CustomField{}
		}
		p.Folder, p.Edited, p.Tags = rootIfEmpty(p.Folder), updated.Unix(), []any{}
		out.Passwords = append(out.Passwords, p)
	}
	if rows.Err() != nil {
		serverError(w, rows.Err())
		return
	}

	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		serverError(w, err)
		return
	}
	base := "passwords-backup_" + time.Now().Format("2006-01-02")
	if req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.json"`)
		w.Write(body)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Encrypt(base+".json", req.Password, zip.AES256Encryption)
	if err == nil {
		_, err = fw.Write(body)
	}
	if err == nil {
		err = zw.Close()
	}
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.zip"`)
	w.Write(buf.Bytes())
}

// --- import ---

const maxBackupSize = 32 << 20

type importResult struct {
	Imported    int      `json:"imported"`
	Overwritten int      `json:"overwritten"`
	Skipped     int      `json:"skipped"`
	Folders     int      `json:"folders"`
	Log         []string `json:"log"`
}

type importError struct {
	msg          string
	needPassword bool
}

func (e *importError) Error() string { return e.msg }

func badFile(format string, args ...any) error {
	return &importError{msg: fmt.Sprintf(format, args...)}
}

func (a *App) handleRestore(w http.ResponseWriter, r *http.Request, s *Session) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBackupSize+1<<20)
	if err := r.ParseMultipartForm(maxBackupSize); err != nil {
		writeError(w, http.StatusBadRequest, "Upload failed or the file is too large (32 MB max).")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Choose a backup file to restore.")
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Could not read the uploaded file.")
		return
	}
	overwrite := r.FormValue("mode") == "overwrite"

	data, err := unpackBackup(raw, r.FormValue("password"))
	if err == nil {
		var b *backupFile
		if b, err = parseBackup(data); err == nil {
			var res *importResult
			res, err = a.importBackup(r.Context(), s.vault(), b, overwrite)
			if err == nil {
				writeJSON(w, http.StatusOK, res)
				return
			}
		}
	}
	var ie *importError
	if errors.As(err, &ie) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": ie.msg, "needPassword": ie.needPassword})
		return
	}
	serverError(w, err)
}

func unpackBackup(raw []byte, password string) ([]byte, error) {
	if !bytes.HasPrefix(raw, []byte("PK\x03\x04")) {
		return raw, nil
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, badFile("The zip file is damaged or not a zip.")
	}
	for _, f := range zr.File {
		if !strings.EqualFold(path.Ext(f.Name), ".json") {
			continue
		}
		if f.IsEncrypted() {
			if password == "" {
				return nil, &importError{msg: "This backup is password-protected. Enter the backup password.", needPassword: true}
			}
			f.SetPassword(password)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, &importError{msg: "Wrong backup password.", needPassword: true}
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, maxBackupSize))
		if err != nil {
			return nil, &importError{msg: "Wrong backup password.", needPassword: true}
		}
		return data, nil
	}
	return nil, badFile("The zip file doesn't contain a .json backup.")
}

func parseBackup(data []byte) (*backupFile, error) {
	var b backupFile
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&b); err != nil {
		return nil, badFile("This isn't a passman backup: the file is not valid JSON in the expected format.")
	}
	if b.Version == nil || b.Passwords == nil || b.Folders == nil {
		return nil, badFile("This isn't a passman backup: missing \"version\", \"passwords\" or \"folders\".")
	}
	if *b.Version != backupVersion {
		return nil, badFile("Unsupported backup version %d (expected %d).", *b.Version, backupVersion)
	}
	if b.Encrypted != nil && *b.Encrypted {
		return nil, badFile("Encrypted Nextcloud backups aren't supported. Export without Nextcloud encryption.")
	}

	folderIDs := map[string]bool{}
	for i, f := range b.Folders {
		if !uuidRe.MatchString(f.ID) || f.ID == rootFolderID {
			return nil, badFile("Folder #%d has an invalid id.", i+1)
		}
		if folderIDs[strings.ToLower(f.ID)] {
			return nil, badFile("Folder id %s appears twice.", f.ID)
		}
		folderIDs[strings.ToLower(f.ID)] = true
		if strings.TrimSpace(f.Label) == "" {
			return nil, badFile("Folder %s has no name.", f.ID)
		}
		if p := rootIfEmpty(f.Parent); !uuidRe.MatchString(p) {
			return nil, badFile("Folder %q has an invalid parent id.", f.Label)
		}
	}
	pwIDs := map[string]bool{}
	for i, p := range b.Passwords {
		if !uuidRe.MatchString(p.ID) {
			return nil, badFile("Password #%d has an invalid id.", i+1)
		}
		if pwIDs[strings.ToLower(p.ID)] {
			return nil, badFile("Password id %s appears twice.", p.ID)
		}
		pwIDs[strings.ToLower(p.ID)] = true
		if f := rootIfEmpty(p.Folder); !uuidRe.MatchString(f) {
			return nil, badFile("Password %q has an invalid folder id.", p.Label)
		}
	}
	return &b, nil
}

func (a *App) importBackup(ctx context.Context, v vault, b *backupFile, overwrite bool) (*importResult, error) {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Existing folder tree (id -> parent), used for reference and cycle checks.
	parents := map[string]string{}
	rows, err := tx.Query(ctx, `SELECT id::text, coalesce(parent_id::text, '') FROM folders WHERE user_id = $1`, v.userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, parent string
		if err := rows.Scan(&id, &parent); err != nil {
			return nil, err
		}
		parents[id] = parent
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	norm := func(id string) string {
		id = strings.ToLower(rootIfEmpty(id))
		if id == rootFolderID {
			return ""
		}
		return id
	}

	// Order folders parents-first; every parent must be root, in the file, or already exist.
	inFile := map[string]backupFolder{}
	for _, f := range b.Folders {
		inFile[norm(f.ID)] = f
	}
	var ordered []backupFolder
	state := map[string]int{} // 1 visiting, 2 done
	var visit func(id string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return badFile("Folders %q form a loop.", inFile[id].Label)
		case 2:
			return nil
		}
		state[id] = 1
		f := inFile[id]
		if p := norm(f.Parent); p != "" {
			if _, ok := inFile[p]; ok {
				if err := visit(p); err != nil {
					return err
				}
			} else if _, ok := parents[p]; !ok {
				return badFile("Folder %q points to a parent folder that doesn't exist (%s).", f.Label, f.Parent)
			}
		}
		state[id] = 2
		ordered = append(ordered, f)
		return nil
	}
	for _, f := range b.Folders {
		if err := visit(norm(f.ID)); err != nil {
			return nil, err
		}
	}
	for _, p := range b.Passwords {
		if fid := norm(p.Folder); fid != "" {
			if _, ok := inFile[fid]; !ok {
				if _, ok := parents[fid]; !ok {
					return nil, badFile("Password %q points to a folder that doesn't exist (%s).", p.Label, p.Folder)
				}
			}
		}
	}

	res := &importResult{Log: []string{}}
	logf := func(format string, args ...any) { res.Log = append(res.Log, fmt.Sprintf(format, args...)) }
	edited := func(unix int64) time.Time {
		if unix <= 0 {
			return time.Now()
		}
		return time.Unix(unix, 0)
	}
	revision := func(r string) string {
		if uuidRe.MatchString(r) {
			return r
		}
		return newUUID()
	}
	isAncestor := func(candidate, of string) bool { // is `of` an ancestor-or-self of candidate?
		for seen := 0; candidate != "" && seen < 10000; seen++ {
			if candidate == of {
				return true
			}
			candidate = parents[candidate]
		}
		return false
	}

	for _, f := range ordered {
		id, parent := norm(f.ID), norm(f.Parent)
		label := strings.TrimSpace(f.Label)
		if _, exists := parents[id]; exists {
			if !overwrite {
				logf("Skipped folder: %q (id %s) — already exists", label, id)
				continue
			}
			if parent != "" && isAncestor(parent, id) {
				logf("Error: folder %q (id %s) can't be moved inside itself; kept its current location", label, id)
				parent = parents[id]
			}
			if _, err := tx.Exec(ctx,
				`UPDATE folders SET label = $3, parent_id = $4::uuid, favorite = $5, revision = $6::uuid, updated_at = $7
				 WHERE user_id = $1 AND id = $2::uuid`,
				v.userID, id, v.sealLabel(id, label), nullFolder(parent), f.Favorite, revision(f.Revision), edited(f.Edited)); err != nil {
				return nil, err
			}
			parents[id] = parent
			logf("Overwritten folder: %q (id %s)", label, id)
			res.Folders++
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO folders (user_id, id, parent_id, label, favorite, revision, created_at, updated_at)
			 VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6::uuid, $7, $7)`,
			v.userID, id, nullFolder(parent), v.sealLabel(id, label), f.Favorite, revision(f.Revision), edited(f.Edited)); err != nil {
			return nil, err
		}
		parents[id] = parent
		res.Folders++
	}

	existing := map[string]bool{}
	rows, err = tx.Query(ctx, `SELECT id::text FROM passwords WHERE user_id = $1`, v.userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		existing[id] = true
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	for _, p := range b.Passwords {
		id := strings.ToLower(p.ID)
		rec := Record{Label: p.Label, Username: p.Username, Password: p.Password, URL: p.URL, Notes: p.Notes, CustomFields: p.CustomFields, Icon: p.Icon}
		if _, err := normalizeIcon(rec.Icon); err != nil {
			logf("Icon ignored: %q (id %s) — %s", p.Label, id, strings.TrimSuffix(err.Error(), "."))
			rec.Icon = nil
		}
		if err := rec.normalize(); err != nil {
			logf("Error: %q (id %s) — %s", p.Label, id, strings.TrimSuffix(err.Error(), "."))
			continue
		}
		folder := nullFolder(norm(p.Folder))
		data := v.sealRecord(id, &rec)
		fp := fingerprint(v.dek, rec.Password)
		risk := passwordRisk(rec.Password, rec.Username)

		if existing[id] {
			if !overwrite {
				logf("Skipped: %q (id %s) — already exists", rec.Label, id)
				res.Skipped++
				continue
			}
			if _, err := tx.Exec(ctx,
				`UPDATE passwords SET folder_id = $3::uuid, data = $4, fingerprint = $5, risk = $6, favorite = $7, revision = $8::uuid, updated_at = $9
				 WHERE user_id = $1 AND id = $2::uuid`,
				v.userID, id, folder, data, fp, risk, p.Favorite, revision(p.Revision), edited(p.Edited)); err != nil {
				return nil, err
			}
			logf("Overwritten: %q (id %s)", rec.Label, id)
			res.Overwritten++
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO passwords (user_id, id, folder_id, data, fingerprint, risk, favorite, revision, created_at, updated_at)
			 VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6, $7, $8::uuid, $9, $9)`,
			v.userID, id, folder, data, fp, risk, p.Favorite, revision(p.Revision), edited(p.Edited)); err != nil {
			return nil, err
		}
		existing[id] = true
		res.Imported++
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return res, nil
}
